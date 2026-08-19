package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nogie-dev/clob-trading/internal/api"
	"github.com/nogie-dev/clob-trading/internal/config"
	"github.com/nogie-dev/clob-trading/internal/engine"
	"github.com/nogie-dev/clob-trading/internal/journal"
	journalpostgres "github.com/nogie-dev/clob-trading/internal/journal/postgres"
	"github.com/nogie-dev/clob-trading/internal/market"
	marketpostgres "github.com/nogie-dev/clob-trading/internal/market/postgres"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	matchlogpostgres "github.com/nogie-dev/clob-trading/internal/matchlog/postgres"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
	ordereventpostgres "github.com/nogie-dev/clob-trading/internal/orderevent/postgres"
)

const databaseURLEnv = "MATCHING_ENGINE_DATABASE_URL"

type engineRuntime struct {
	router                *engine.Router
	persistenceOut        chan matchlog.PersistenceRequest
	writerDone            chan struct{}
	orderEventOut         chan orderevent.PersistenceRequest
	orderEventWriterDone  chan struct{}
	journalStore          journal.Store
	marketStore           market.Store
	workerInputBufferSize int
	lifecycleMu           sync.Mutex
}

func main() {
	configPath := flag.String("config", "config/default.json", "path to JSON config file")
	address := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	databaseURL, err := requiredDatabaseURL(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if err := serve(cfg, *address, databaseURL); err != nil {
		log.Fatal(err)
	}
}

func requiredDatabaseURL(getenv func(string) string) (string, error) {
	url := strings.TrimSpace(getenv(databaseURLEnv))
	if url == "" {
		return "", fmt.Errorf("%s is required", databaseURLEnv)
	}
	return url, nil
}

func serve(cfg config.Config, address, databaseURL string) error {
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStartup()

	pool, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		return fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}

	runtime, err := startEngine(
		context.Background(),
		cfg,
		matchlogpostgres.NewStore(pool),
		journalpostgres.NewStore(pool),
		marketpostgres.NewStore(pool),
		ordereventpostgres.NewStore(pool),
	)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              address,
		Handler:           api.NewHandlerWithTickerAdder(runtime.router, runtime),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	log.Printf("internal API listening on %s", address)
	var listenErr error
	select {
	case listenErr = <-serverErr:
	case <-signalCtx.Done():
	}

	httpShutdownCtx, cancelHTTPShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	httpShutdownErr := server.Shutdown(httpShutdownCtx)
	cancelHTTPShutdown()
	if httpShutdownErr != nil {
		httpShutdownErr = fmt.Errorf("shutdown HTTP server: %w", httpShutdownErr)
	}
	if listenErr == nil {
		listenErr = <-serverErr
	}
	engineShutdownErr := runtime.shutdown(context.Background())
	if listenErr != nil && errors.Is(listenErr, http.ErrServerClosed) {
		listenErr = nil
	} else if listenErr != nil {
		listenErr = fmt.Errorf("serve HTTP: %w", listenErr)
	}
	return errors.Join(httpShutdownErr, engineShutdownErr, listenErr)
}

func startEngine(
	ctx context.Context,
	cfg config.Config,
	matchStore matchlog.Store,
	journalStore journal.Store,
	marketStore market.Store,
	orderEventStore orderevent.Store,
) (*engineRuntime, error) {
	if matchStore == nil {
		return nil, matchlog.ErrStoreUnavailable
	}
	if journalStore == nil {
		return nil, journal.ErrStoreUnavailable
	}
	if marketStore == nil {
		return nil, market.ErrStoreUnavailable
	}
	if orderEventStore == nil {
		return nil, orderevent.ErrStoreUnavailable
	}
	persistenceOut := make(chan matchlog.PersistenceRequest, cfg.Engine.MatchLogOutputBufferSize)
	writerDone := make(chan struct{})
	writer := matchlog.NewWriter(matchStore)
	go func() {
		defer close(writerDone)
		writer.Run(context.Background(), persistenceOut)
	}()
	orderEventOut := make(chan orderevent.PersistenceRequest, cfg.Engine.OrderEventOutputBufferSize)
	orderEventWriterDone := make(chan struct{})
	orderEventWriter := orderevent.NewWriter(orderEventStore)
	go func() {
		defer close(orderEventWriterDone)
		orderEventWriter.Run(context.Background(), orderEventOut)
	}()

	router := engine.NewRouter()
	runtime := &engineRuntime{
		router:                router,
		persistenceOut:        persistenceOut,
		writerDone:            writerDone,
		orderEventOut:         orderEventOut,
		orderEventWriterDone:  orderEventWriterDone,
		journalStore:          journalStore,
		marketStore:           marketStore,
		workerInputBufferSize: cfg.Engine.WorkerInputBufferSize,
	}
	cleanup := func() {
		_ = router.Shutdown(context.Background())
		close(persistenceOut)
		close(orderEventOut)
		<-writerDone
		<-orderEventWriterDone
	}
	markets, err := marketStore.List(ctx)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("load markets: %w", err)
	}
	commands, err := journalStore.List(ctx)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("load order journal: %w", err)
	}
	commandsByTicker := make(map[string][]journal.Command)
	marketsByTicker := make(map[string]market.Market, len(markets))
	tickers := make(map[string]struct{}, len(markets)+len(commands))
	for _, registered := range markets {
		tickers[registered.Ticker] = struct{}{}
		marketsByTicker[registered.Ticker] = registered
	}
	for _, command := range commands {
		commandsByTicker[command.Ticker] = append(commandsByTicker[command.Ticker], command)
		if _, registered := tickers[command.Ticker]; !registered {
			added, err := marketStore.Add(ctx, command.Ticker)
			if err != nil {
				cleanup()
				return nil, fmt.Errorf("persist journal ticker %q: %w", command.Ticker, err)
			}
			marketsByTicker[command.Ticker] = added.Market
		}
		tickers[command.Ticker] = struct{}{}
	}
	tickerNames := make([]string, 0, len(tickers))
	for ticker := range tickers {
		tickerNames = append(tickerNames, ticker)
	}
	sort.Strings(tickerNames)
	for _, ticker := range tickerNames {
		if err := runtime.registerTicker(marketsByTicker[ticker], commandsByTicker[ticker]); err != nil {
			cleanup()
			return nil, fmt.Errorf("restore ticker %q: %w", ticker, err)
		}
	}

	return runtime, nil
}

func (r *engineRuntime) AddTicker(ctx context.Context, ticker string) error {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return engine.ErrEmptyTicker
	}

	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if err := r.router.Ready(); err != nil {
		return err
	}
	if r.router.HasTicker(ticker) {
		return fmt.Errorf("%w: %s", engine.ErrTickerExists, ticker)
	}
	added, err := r.marketStore.Add(ctx, ticker)
	if err != nil {
		return fmt.Errorf("persist ticker %q: %w", ticker, err)
	}

	commands, err := r.journalStore.List(ctx)
	if err != nil {
		return fmt.Errorf("load order journal for ticker %q: %w", ticker, err)
	}
	tickerCommands := make([]journal.Command, 0)
	for _, command := range commands {
		if command.Ticker == ticker {
			tickerCommands = append(tickerCommands, command)
		}
	}
	if err := r.registerTicker(added.Market, tickerCommands); err != nil {
		return fmt.Errorf("restore ticker %q: %w", ticker, err)
	}
	return nil
}

func (r *engineRuntime) registerTicker(registered market.Market, commands []journal.Command) error {
	if registered.Ticker == "" {
		return fmt.Errorf("market ticker is required")
	}
	worker := engine.NewBookWorkerWithOptions(registered.Ticker, nil, engine.BookWorkerOptions{
		InputBufferSize: r.workerInputBufferSize,
		Precision:       registered.Precision,
		PersistenceOut:  r.persistenceOut,
		OrderEventOut:   r.orderEventOut,
		Journal:         r.journalStore,
	})
	if err := worker.Replay(commands); err != nil {
		return fmt.Errorf("replay order journal: %w", err)
	}
	if err := r.router.Register(registered.Ticker, worker); err != nil {
		return err
	}
	go worker.Run()
	return nil
}

func (r *engineRuntime) shutdown(ctx context.Context) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if err := r.router.Shutdown(ctx); err != nil {
		return err
	}
	close(r.persistenceOut)
	close(r.orderEventOut)
	return errors.Join(
		waitForWriter(ctx, r.writerDone, "match log"),
		waitForWriter(ctx, r.orderEventWriterDone, "order event"),
	)
}

func waitForWriter(ctx context.Context, done <-chan struct{}, name string) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown %s writer: %w", name, ctx.Err())
	}
}
