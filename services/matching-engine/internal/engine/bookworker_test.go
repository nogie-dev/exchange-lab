package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

type testJournalStore struct {
	sequence int64
}

type blockingJournalStore struct {
	started chan struct{}
	release chan struct{}
	err     error
}

func (s *blockingJournalStore) Append(_ context.Context, command journal.Command) (journal.AppendResult, error) {
	close(s.started)
	<-s.release
	if s.err != nil {
		return journal.AppendResult{}, s.err
	}
	command.Sequence = 1
	command.RecordedAt = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	return journal.AppendResult{Command: command, Inserted: true}, nil
}

func (s *blockingJournalStore) List(context.Context) ([]journal.Command, error) {
	return nil, nil
}

func (s *testJournalStore) Append(_ context.Context, command journal.Command) (journal.AppendResult, error) {
	s.sequence++
	command.Sequence = s.sequence
	command.RecordedAt = time.Date(2026, 7, 17, 12, 0, 0, int(s.sequence), time.UTC)
	return journal.AppendResult{Command: command, Inserted: true}, nil
}

func (s *testJournalStore) List(context.Context) ([]journal.Command, error) {
	return nil, nil
}

func acceptingOrderEventOut(t *testing.T) chan orderevent.PersistenceRequest {
	t.Helper()
	out := make(chan orderevent.PersistenceRequest, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for request := range out {
			request.Acknowledge(nil)
		}
	}()
	t.Cleanup(func() {
		close(out)
		<-done
	})
	return out
}

func routeAndDrain(t *testing.T, router *Router, worker *BookWorker, ev Event) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		worker.Run()
		close(done)
	}()

	if err := router.OrderRouter(ev); err != nil {
		t.Fatalf("OrderRouter returned error: %v", err)
	}
	close(worker.in)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not drain routed event before timeout")
	}
}

func TestBookWorkerRejectsNewOrderPayloadTickerMismatch(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeAndDrain(t, router, worker, Event{
		Type:   NewOrder,
		Ticker: "BTC-USD",
		NewOrder: &models.CreateOrderRequest{
			Ticker:    "ETH-USD",
			UserID:    "alice",
			OrderType: models.Limit,
			Position:  models.Bid,
			Price:     100,
			Amount:    1,
			Nonce:     1,
		},
	})

	if len(worker.OrderBook.Bids) != 0 {
		t.Fatalf("mismatched NewOrder payload should not create bids, got %d levels", len(worker.OrderBook.Bids))
	}
	if len(worker.OrderBook.Asks) != 0 {
		t.Fatalf("mismatched NewOrder payload should not create asks, got %d levels", len(worker.OrderBook.Asks))
	}
	if len(worker.OrderBook.Index) != 0 {
		t.Fatalf("mismatched NewOrder payload should not index orders, got %d entries", len(worker.OrderBook.Index))
	}
}

func TestBookWorkerEmitsMatchLogs(t *testing.T) {
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		OrderEventOut:  acceptingOrderEventOut(t),
		Journal:        &testJournalStore{},
	})
	maker := newOrder("ask-1", models.Ask, 100, 0.5)
	maker.UserID = "maker-user"
	worker.OrderBook.AddOrder(maker)

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	persisted := make(chan []matchlog.MatchLog, 1)
	go func() {
		request := <-persistenceOut
		persisted <- request.Logs
		request.Acknowledge(nil)
	}()

	routeAndDrain(t, router, worker, Event{
		Type:   NewOrder,
		Ticker: "BTC-USD",
		NewOrder: &models.CreateOrderRequest{
			CommandID: "create-1",
			Ticker:    "BTC-USD",
			UserID:    "taker-user",
			OrderType: models.Limit,
			Position:  models.Bid,
			Price:     101,
			Amount:    0.25,
			Nonce:     1,
		},
	})

	var logs []matchlog.MatchLog
	select {
	case logs = <-persisted:
	default:
		t.Fatal("expected emitted match logs")
	}

	if len(logs) != 1 {
		t.Fatalf("emitted match logs want 1, got %d", len(logs))
	}
	if logs[0].MakerOrderID != "ask-1" || logs[0].TakerUserID != "taker-user" {
		t.Fatalf("unexpected emitted match log: %#v", logs[0])
	}
}

func TestBookWorkerCancelsMarketResidualInsteadOfResting(t *testing.T) {
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		OrderEventOut:  acceptingOrderEventOut(t),
		Journal:        &testJournalStore{},
	})
	worker.OrderBook.AddOrder(newOrder("ask-1", models.Ask, 100, 0.25))

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	logs := make(chan []matchlog.MatchLog, 1)
	go func() {
		request := <-persistenceOut
		logs <- request.Logs
		request.Acknowledge(nil)
	}()

	err := router.OrderRouter(Event{
		Type:   NewOrder,
		Ticker: "BTC-USD",
		NewOrder: &models.CreateOrderRequest{
			CommandID: "market-create-1",
			Ticker:    "BTC-USD",
			UserID:    "taker-user",
			OrderType: models.Market,
			Position:  models.Bid,
			Amount:    1,
		},
	})
	if err != nil {
		t.Fatalf("market order returned error: %v", err)
	}
	if got := <-logs; len(got) != 1 || got[0].Amount != 0.25 {
		t.Fatalf("unexpected market execution logs: %#v", got)
	}
	if len(worker.OrderBook.Index) != 0 || len(worker.OrderBook.Bids) != 0 {
		t.Fatalf("market residual must not rest on the book: %#v", worker.OrderBook.Snapshot(1))
	}

	shutdownRouter(t, router)
}

func TestBookWorkerCancelsMarketSellResidualInsteadOfResting(t *testing.T) {
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		OrderEventOut:  acceptingOrderEventOut(t),
		Journal:        &testJournalStore{},
	})
	worker.OrderBook.AddOrder(newOrder("bid-1", models.Bid, 100, 0.25))

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	logs := make(chan []matchlog.MatchLog, 1)
	go func() {
		request := <-persistenceOut
		logs <- request.Logs
		request.Acknowledge(nil)
	}()

	err := router.OrderRouter(Event{
		Type:   NewOrder,
		Ticker: "BTC-USD",
		NewOrder: &models.CreateOrderRequest{
			CommandID: "market-sell-1",
			Ticker:    "BTC-USD",
			UserID:    "taker-user",
			OrderType: models.Market,
			Position:  models.Ask,
			Amount:    1,
		},
	})
	if err != nil {
		t.Fatalf("market order returned error: %v", err)
	}
	if got := <-logs; len(got) != 1 || got[0].Amount != 0.25 {
		t.Fatalf("unexpected market execution logs: %#v", got)
	}
	if len(worker.OrderBook.Index) != 0 || len(worker.OrderBook.Asks) != 0 {
		t.Fatalf("market residual must not rest on the book: %#v", worker.OrderBook.Snapshot(1))
	}

	shutdownRouter(t, router)
}

func TestBookWorkerWaitsForPersistenceBeforeNextCommand(t *testing.T) {
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		OrderEventOut:  acceptingOrderEventOut(t),
		Journal:        &testJournalStore{},
	})
	maker := newOrder("ask-1", models.Ask, 100, 0.5)
	maker.UserID = "maker-user"
	worker.OrderBook.AddOrder(maker)

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "taker-user",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     101,
				Amount:    0.25,
				Nonce:     1,
			},
		})
	}()

	request := <-persistenceOut
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- router.OrderRouter(Event{
			Type:   CancelOrder,
			Ticker: "BTC-USD",
			CancelReq: &models.CancelOrderRequest{
				CommandID: "cancel-1",
				Ticker:    "BTC-USD",
				OrderID:   maker.OrderID,
			},
		})
	}()
	select {
	case err := <-secondResult:
		t.Fatalf("next command completed before persistence acknowledgement: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	if _, ok := worker.OrderBook.Index[maker.OrderID]; !ok {
		t.Fatal("worker processed the next command before persistence acknowledgement")
	}

	request.Acknowledge(nil)
	if err := <-firstResult; err != nil {
		t.Fatalf("route matching order: %v", err)
	}
	if err := <-secondResult; err != nil {
		t.Fatalf("route queued cancel: %v", err)
	}
	snapshot, err := router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("snapshot after acknowledgement: %v", err)
	}
	if len(snapshot.Asks) != 0 {
		t.Fatalf("queued cancel was not processed after acknowledgement: %#v", snapshot.Asks)
	}

	shutdownRouter(t, router)
}

func TestBookWorkerWaitsForOrderEventPersistenceBeforeNextCommand(t *testing.T) {
	orderEventOut := make(chan orderevent.PersistenceRequest, 2)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		OrderEventOut: orderEventOut,
		Journal:       &testJournalStore{},
	})
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "alice",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     100,
				Amount:    1,
				Nonce:     1,
			},
		})
	}()

	firstRequest := <-orderEventOut
	if len(firstRequest.Events) != 1 || firstRequest.Events[0].Type != orderevent.Resting {
		t.Fatalf("first event batch want one resting event, got %#v", firstRequest.Events)
	}
	select {
	case err := <-firstResult:
		t.Fatalf("create completed before order event acknowledgement: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	secondResult := make(chan error, 1)
	go func() {
		secondResult <- router.OrderRouter(Event{
			Type:   CancelOrder,
			Ticker: "BTC-USD",
			CancelReq: &models.CancelOrderRequest{
				CommandID: "cancel-1",
				Ticker:    "BTC-USD",
				OrderID:   firstRequest.Events[0].OrderID,
			},
		})
	}()
	select {
	case err := <-secondResult:
		t.Fatalf("next command completed before first event acknowledgement: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	firstRequest.Acknowledge(nil)
	if err := <-firstResult; err != nil {
		t.Fatalf("route create order: %v", err)
	}

	secondRequest := <-orderEventOut
	if len(secondRequest.Events) != 1 || secondRequest.Events[0].Type != orderevent.Canceled {
		t.Fatalf("second event batch want one canceled event, got %#v", secondRequest.Events)
	}
	select {
	case err := <-secondResult:
		t.Fatalf("cancel completed before its event acknowledgement: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	secondRequest.Acknowledge(nil)
	if err := <-secondResult; err != nil {
		t.Fatalf("route cancel order: %v", err)
	}

	shutdownRouter(t, router)
}

func TestBookWorkerOrderEventPersistenceFailureHaltsRouter(t *testing.T) {
	orderEventOut := make(chan orderevent.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		OrderEventOut: orderEventOut,
		Journal:       &testJournalStore{},
	})
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	result := make(chan error, 1)
	go func() {
		result <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "alice",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     100,
				Amount:    1,
				Nonce:     1,
			},
		})
	}()

	request := <-orderEventOut
	request.Acknowledge(errors.New("order event database unavailable"))
	if err := <-result; !errors.Is(err, ErrEngineHalted) {
		t.Fatalf("failed event persistence want ErrEngineHalted, got %v", err)
	}
	waitForHalt(t, router)
	shutdownRouter(t, router)
}

func TestBookWorkerPersistenceFailureHaltsRouter(t *testing.T) {
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		Journal:        &testJournalStore{},
	})
	maker := newOrder("ask-1", models.Ask, 100, 0.5)
	maker.UserID = "maker-user"
	worker.OrderBook.AddOrder(maker)

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	result := make(chan error, 1)
	go func() {
		result <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "taker-user",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     101,
				Amount:    0.25,
				Nonce:     1,
			},
		})
	}()

	request := <-persistenceOut
	request.Acknowledge(errors.New("database unavailable"))
	if err := <-result; !errors.Is(err, ErrEngineHalted) {
		t.Fatalf("failed matching order want ErrEngineHalted, got %v", err)
	}
	waitForHalt(t, router)

	err := router.OrderRouter(Event{
		Type:   CancelOrder,
		Ticker: "BTC-USD",
		CancelReq: &models.CancelOrderRequest{
			CommandID: "cancel-1",
			Ticker:    "BTC-USD",
			OrderID:   maker.OrderID,
		},
	})
	if !errors.Is(err, ErrEngineHalted) {
		t.Fatalf("halted router should return ErrEngineHalted, got %v", err)
	}

	shutdownRouter(t, router)
}

func waitForHalt(t *testing.T, router *Router) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if errors.Is(router.Ready(), ErrEngineHalted) {
			return
		}
		select {
		case <-deadline:
			t.Fatal("router did not enter halted state before timeout")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func shutdownRouter(t *testing.T, router *Router) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := router.Shutdown(ctx); err != nil {
		t.Fatalf("router shutdown: %v", err)
	}
}

func TestNewBookWorkerWithOptionsUsesInputBufferSize(t *testing.T) {
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		InputBufferSize: 3,
	})

	if cap(worker.in) != 3 {
		t.Fatalf("worker input channel cap want 3, got %d", cap(worker.in))
	}
}

func TestBookWorkerRejectsCancelOrderPayloadTickerMismatch(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	existing := newOrder("bid-1", models.Bid, 100, 1)
	worker.OrderBook.AddOrder(existing)

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeAndDrain(t, router, worker, Event{
		Type:   CancelOrder,
		Ticker: "BTC-USD",
		CancelReq: &models.CancelOrderRequest{
			Ticker:  "ETH-USD",
			OrderID: existing.OrderID,
		},
	})

	if _, ok := worker.OrderBook.Index[existing.OrderID]; !ok {
		t.Fatalf("mismatched CancelOrder payload should not remove order %q from index", existing.OrderID)
	}
	if _, ok := worker.OrderBook.Bids[existing.Price]; !ok {
		t.Fatalf("mismatched CancelOrder payload should not remove bid level %.2f", existing.Price)
	}
}

func TestBookWorkerRejectsEditOrderPayloadTickerMismatch(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	existing := newOrder("bid-1", models.Bid, 100, 1)
	worker.OrderBook.AddOrder(existing)

	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	newAmount := 2.0
	routeAndDrain(t, router, worker, Event{
		Type:   EditOrder,
		Ticker: "BTC-USD",
		EditReq: &models.EditOrderRequest{
			Ticker:  "ETH-USD",
			OrderID: existing.OrderID,
			Price:   101,
			Amount:  &newAmount,
		},
	})

	if existing.Price != 100 {
		t.Fatalf("mismatched EditOrder payload should not change price, got %.2f", existing.Price)
	}
	if existing.Amount != 1 {
		t.Fatalf("mismatched EditOrder payload should not change amount, got %.2f", existing.Amount)
	}
	if _, ok := worker.OrderBook.Bids[100]; !ok {
		t.Fatal("mismatched EditOrder payload should keep original bid level")
	}
	if _, ok := worker.OrderBook.Bids[101]; ok {
		t.Fatal("mismatched EditOrder payload should not create edited bid level")
	}
}

func TestRouterOrderBookSnapshotRunsAfterQueuedCommand(t *testing.T) {
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		OrderEventOut: acceptingOrderEventOut(t),
		Journal:       &testJournalStore{},
	})
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		worker.Run()
		close(done)
	}()

	if err := router.OrderRouter(Event{
		Type:   NewOrder,
		Ticker: "BTC-USD",
		NewOrder: &models.CreateOrderRequest{
			CommandID: "create-1",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Limit,
			Position:  models.Bid,
			Price:     100,
			Amount:    2,
			Nonce:     1,
		},
	}); err != nil {
		t.Fatalf("OrderRouter returned error: %v", err)
	}

	snapshot, err := router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("OrderBookSnapshot returned error: %v", err)
	}
	assertLevel(t, snapshot.Bids, 0, 100, 2, 2)

	close(worker.in)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop before timeout")
	}
}

func TestRouterOrderBookSnapshotRejectsUnknownTicker(t *testing.T) {
	router := NewRouter()

	if _, err := router.OrderBookSnapshot("ETH-USD", 1); err == nil {
		t.Fatal("OrderBookSnapshot should reject an unknown ticker")
	}
}

func TestBookWorkerCommitsJournalBeforeBookMutation(t *testing.T) {
	journalStore := &blockingJournalStore{started: make(chan struct{}), release: make(chan struct{})}
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		OrderEventOut: acceptingOrderEventOut(t),
		Journal:       journalStore,
	})
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	result := make(chan error, 1)
	go func() {
		result <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "alice",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     100,
				Amount:    1,
				Nonce:     1,
			},
		})
	}()

	<-journalStore.started
	if len(worker.OrderBook.Index) != 0 {
		t.Fatal("orderbook changed before journal commit")
	}
	close(journalStore.release)
	if err := <-result; err != nil {
		t.Fatalf("OrderRouter returned error: %v", err)
	}
	if len(worker.OrderBook.Index) != 1 {
		t.Fatalf("journaled command was not applied, indexed orders: %d", len(worker.OrderBook.Index))
	}
	shutdownRouter(t, router)
}

func TestBookWorkerJournalFailureHaltsBeforeBookMutation(t *testing.T) {
	journalStore := &blockingJournalStore{
		started: make(chan struct{}),
		release: make(chan struct{}),
		err:     errors.New("journal unavailable"),
	}
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{Journal: journalStore})
	router := NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()

	result := make(chan error, 1)
	go func() {
		result <- router.OrderRouter(Event{
			Type:   NewOrder,
			Ticker: "BTC-USD",
			NewOrder: &models.CreateOrderRequest{
				CommandID: "create-1",
				Ticker:    "BTC-USD",
				UserID:    "alice",
				OrderType: models.Limit,
				Position:  models.Bid,
				Price:     100,
				Amount:    1,
				Nonce:     1,
			},
		})
	}()

	<-journalStore.started
	close(journalStore.release)
	if err := <-result; !errors.Is(err, ErrEngineHalted) {
		t.Fatalf("journal failure want ErrEngineHalted, got %v", err)
	}
	if len(worker.OrderBook.Index) != 0 {
		t.Fatal("orderbook changed after journal failure")
	}
	shutdownRouter(t, router)
}

func TestBookWorkerReplayIsDeterministic(t *testing.T) {
	commands := replayCommands()
	firstLogs := replayLogs(t, commands)
	secondLogs := replayLogs(t, commands)

	if len(firstLogs) != 1 || len(secondLogs) != 1 {
		t.Fatalf("replayed logs want 1/1, got %d/%d", len(firstLogs), len(secondLogs))
	}
	if firstLogs[0].ExecutionID != secondLogs[0].ExecutionID || firstLogs[0].MatchedAt != secondLogs[0].MatchedAt {
		t.Fatalf("replay produced different execution identity: %#v / %#v", firstLogs[0], secondLogs[0])
	}
}

func TestBookWorkerReplayCancelsMarketResidualDeterministically(t *testing.T) {
	commands := marketReplayCommands()
	firstLogs := replayLogs(t, commands)
	secondLogs := replayLogs(t, commands)

	if len(firstLogs) != 1 || len(secondLogs) != 1 {
		t.Fatalf("replayed market logs want 1/1, got %d/%d", len(firstLogs), len(secondLogs))
	}
	if firstLogs[0].Amount != 0.25 || firstLogs[0].Price != 100 {
		t.Fatalf("unexpected replayed market execution: %#v", firstLogs[0])
	}
	if firstLogs[0].ExecutionID != secondLogs[0].ExecutionID || firstLogs[0].MatchedAt != secondLogs[0].MatchedAt {
		t.Fatalf("market replay produced different execution identity: %#v / %#v", firstLogs[0], secondLogs[0])
	}
}

func TestBookWorkerReplayAppliesAmendAndCancel(t *testing.T) {
	baseTime := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	create := models.CreateOrderRequest{
		CommandID: "create-command", Ticker: "BTC-USD", UserID: "alice",
		OrderType: models.Limit, Position: models.Bid, Price: 100, Amount: 1, Nonce: 1,
	}
	orderID := CreateOrderAt(create, baseTime).OrderID
	amount := 2.0
	commands := []journal.Command{
		{CommandID: create.CommandID, Ticker: create.Ticker, Sequence: 1, Type: journal.CreateCommand, RecordedAt: baseTime, Create: &create},
		{
			CommandID: "amend-command", Ticker: create.Ticker, Sequence: 2, Type: journal.AmendCommand, RecordedAt: baseTime.Add(time.Second),
			Amend: &models.EditOrderRequest{CommandID: "amend-command", Ticker: create.Ticker, OrderID: orderID, Price: 100, Amount: &amount},
		},
		{
			CommandID: "cancel-command", Ticker: create.Ticker, Sequence: 3, Type: journal.CancelCommand, RecordedAt: baseTime.Add(2 * time.Second),
			Cancel: &models.CancelOrderRequest{CommandID: "cancel-command", Ticker: create.Ticker, OrderID: orderID},
		},
	}

	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{OrderEventOut: acceptingOrderEventOut(t)})
	if err := worker.Replay(commands); err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}
	if len(worker.OrderBook.Index) != 0 || len(worker.OrderBook.Bids) != 0 {
		t.Fatalf("amend and cancel replay should leave an empty book: %#v", worker.OrderBook.Snapshot(1))
	}
}

func replayCommands() []journal.Command {
	makerTime := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	takerTime := makerTime.Add(time.Second)
	return []journal.Command{
		{
			CommandID: "maker-command", Ticker: "BTC-USD", Sequence: 1, Type: journal.CreateCommand, RecordedAt: makerTime,
			Create: &models.CreateOrderRequest{CommandID: "maker-command", Ticker: "BTC-USD", UserID: "maker", OrderType: models.Limit, Position: models.Ask, Price: 100, Amount: 1, Nonce: 1},
		},
		{
			CommandID: "taker-command", Ticker: "BTC-USD", Sequence: 2, Type: journal.CreateCommand, RecordedAt: takerTime,
			Create: &models.CreateOrderRequest{CommandID: "taker-command", Ticker: "BTC-USD", UserID: "taker", OrderType: models.Limit, Position: models.Bid, Price: 100, Amount: 1, Nonce: 1},
		},
	}
}

func marketReplayCommands() []journal.Command {
	makerTime := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	takerTime := makerTime.Add(time.Second)
	return []journal.Command{
		{
			CommandID: "maker-command", Ticker: "BTC-USD", Sequence: 1, Type: journal.CreateCommand, RecordedAt: makerTime,
			Create: &models.CreateOrderRequest{CommandID: "maker-command", Ticker: "BTC-USD", UserID: "maker", OrderType: models.Limit, Position: models.Ask, Price: 100, Amount: 0.25, Nonce: 1},
		},
		{
			CommandID: "market-command", Ticker: "BTC-USD", Sequence: 2, Type: journal.CreateCommand, RecordedAt: takerTime,
			Create: &models.CreateOrderRequest{CommandID: "market-command", Ticker: "BTC-USD", UserID: "taker", OrderType: models.Market, Position: models.Bid, Amount: 1, Nonce: 1},
		},
	}
}

func replayLogs(t *testing.T, commands []journal.Command) []matchlog.MatchLog {
	t.Helper()
	persistenceOut := make(chan matchlog.PersistenceRequest, 1)
	worker := NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{
		PersistenceOut: persistenceOut,
		OrderEventOut:  acceptingOrderEventOut(t),
	})
	logs := make(chan []matchlog.MatchLog, 1)
	go func() {
		request := <-persistenceOut
		logs <- request.Logs
		request.Acknowledge(nil)
	}()
	if err := worker.Replay(commands); err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}
	if len(worker.OrderBook.Index) != 0 {
		t.Fatalf("fully matched replay should leave no indexed orders, got %d", len(worker.OrderBook.Index))
	}
	return <-logs
}
