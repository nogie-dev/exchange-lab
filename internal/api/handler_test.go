package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nogie-dev/clob-trading/internal/engine"
	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

type testJournal struct {
	sequence int64
}

type testTickerAdder struct {
	ticker string
	err    error
}

func (a *testTickerAdder) AddTicker(_ context.Context, ticker string) error {
	a.ticker = ticker
	return a.err
}

func (j *testJournal) Append(_ context.Context, command journal.Command) (journal.AppendResult, error) {
	j.sequence++
	command.Sequence = j.sequence
	command.RecordedAt = time.Date(2026, 7, 17, 12, 0, 0, int(j.sequence), time.UTC)
	return journal.AppendResult{Command: command, Inserted: true}, nil
}

func (j *testJournal) List(context.Context) ([]journal.Command, error) {
	return nil, nil
}

func TestOrderBookQuery(t *testing.T) {
	book := engine.NewOrderBook("BTC-USD")
	book.AddOrder(&models.BookOrder{OrderID: "bid-1", Ticker: "BTC-USD", UserID: "buyer", OrderType: models.Limit, Position: models.Bid, Price: 100, Amount: 2})
	book.AddOrder(&models.BookOrder{OrderID: "ask-1", Ticker: "BTC-USD", UserID: "seller", OrderType: models.Limit, Position: models.Ask, Price: 101, Amount: 3})
	handler, _ := newTestHandler(t, book)

	request := httptest.NewRequest(http.MethodGet, "/queries/orderbook?ticker=BTC-USD&depth=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status want %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type want application/json, got %q", got)
	}

	body := response.Body.Bytes()
	var snapshot engine.OrderBookSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if snapshot.Ticker != "BTC-USD" || len(snapshot.Bids) != 1 || len(snapshot.Asks) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Bids[0].Price != 100 || snapshot.Bids[0].Amount != 2 {
		t.Fatalf("unexpected best bid: %#v", snapshot.Bids[0])
	}
	if snapshot.Asks[0].Price != 101 || snapshot.Asks[0].Amount != 3 {
		t.Fatalf("unexpected best ask: %#v", snapshot.Asks[0])
	}
	if !bytes.Contains(body, []byte(`"cumulativeAmount":`)) {
		t.Fatalf("response should use cumulativeAmount JSON field: %s", body)
	}
}

func TestCreateOrderCommand(t *testing.T) {
	handler, router := newTestHandler(t, nil)
	body := []byte(`{
		"command_id":"create-1",
		"ticker":"BTC-USD",
		"user_id":"alice",
		"order_type":"LIMIT",
		"position":"BID",
		"price":100,
		"amount":2,
		"nonce":1
	}`)

	response := serveJSON(handler, "/commands/orders/create", body)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status want %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}

	snapshot, err := router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("OrderBookSnapshot returned error: %v", err)
	}
	if len(snapshot.Bids) != 1 || snapshot.Bids[0].Amount != 2 {
		t.Fatalf("create command was not processed: %#v", snapshot)
	}
}

func TestCreateMarketOrderWithoutPrice(t *testing.T) {
	handler, router := newTestHandler(t, nil)
	body := []byte(`{
		"command_id":"market-create-1",
		"ticker":"BTC-USD",
		"user_id":"alice",
		"order_type":"MARKET",
		"position":"BID",
		"amount":2,
		"nonce":1
	}`)

	response := serveJSON(handler, "/commands/orders/create", body)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status want %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}

	snapshot, err := router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("OrderBookSnapshot returned error: %v", err)
	}
	if len(snapshot.Bids) != 0 || len(snapshot.Asks) != 0 {
		t.Fatalf("unmatched market order should not rest on the book: %#v", snapshot)
	}
}

func TestAmendAndCancelOrderCommands(t *testing.T) {
	book := engine.NewOrderBook("BTC-USD")
	book.AddOrder(&models.BookOrder{OrderID: "bid-1", Ticker: "BTC-USD", UserID: "alice", OrderType: models.Limit, Position: models.Bid, Price: 100, Amount: 2})
	handler, router := newTestHandler(t, book)

	amendedAmount := 1.0
	amendBody, err := json.Marshal(models.EditOrderRequest{
		CommandID: "amend-1",
		Ticker:    "BTC-USD",
		OrderID:   "bid-1",
		Price:     100,
		Amount:    &amendedAmount,
	})
	if err != nil {
		t.Fatalf("marshal amend request: %v", err)
	}
	response := serveJSON(handler, "/commands/orders/amend", amendBody)
	if response.Code != http.StatusAccepted {
		t.Fatalf("amend status want %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}

	snapshot, err := router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("OrderBookSnapshot after amend returned error: %v", err)
	}
	if len(snapshot.Bids) != 1 || snapshot.Bids[0].Amount != amendedAmount {
		t.Fatalf("amend command was not processed: %#v", snapshot)
	}

	cancelBody, err := json.Marshal(models.CancelOrderRequest{CommandID: "cancel-1", Ticker: "BTC-USD", OrderID: "bid-1"})
	if err != nil {
		t.Fatalf("marshal cancel request: %v", err)
	}
	response = serveJSON(handler, "/commands/orders/cancel", cancelBody)
	if response.Code != http.StatusAccepted {
		t.Fatalf("cancel status want %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}

	snapshot, err = router.OrderBookSnapshot("BTC-USD", 1)
	if err != nil {
		t.Fatalf("OrderBookSnapshot after cancel returned error: %v", err)
	}
	if len(snapshot.Bids) != 0 {
		t.Fatalf("cancel command was not processed: %#v", snapshot)
	}
}

func TestAddTickerCommand(t *testing.T) {
	adder := &testTickerAdder{}
	handler := NewHandlerWithTickerAdder(engine.NewRouter(), adder)

	response := serveJSON(handler, "/commands/tickers/add", []byte(`{"ticker":" ETH-USD "}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("status want %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if adder.ticker != "ETH-USD" {
		t.Fatalf("ticker adder got %q, want ETH-USD", adder.ticker)
	}
}

func TestAddTickerCommandRejectsUnavailableOrConflictingAdder(t *testing.T) {
	withoutAdder := NewHandler(engine.NewRouter())
	response := serveJSON(withoutAdder, "/commands/tickers/add", []byte(`{"ticker":"ETH-USD"}`))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing adder status want %d, got %d: %s", http.StatusServiceUnavailable, response.Code, response.Body.String())
	}

	adder := &testTickerAdder{err: engine.ErrTickerExists}
	withConflict := NewHandlerWithTickerAdder(engine.NewRouter(), adder)
	response = serveJSON(withConflict, "/commands/tickers/add", []byte(`{"ticker":"ETH-USD"}`))
	if response.Code != http.StatusConflict {
		t.Fatalf("conflicting ticker status want %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
}

func TestHandlerRejectsInvalidRequestsAndUnknownTickers(t *testing.T) {
	handler := NewHandler(engine.NewRouter())
	tests := []struct {
		name   string
		method string
		target string
		body   string
		status int
	}{
		{name: "invalid depth", method: http.MethodGet, target: "/queries/orderbook?ticker=BTC-USD&depth=0", status: http.StatusBadRequest},
		{name: "unknown query ticker", method: http.MethodGet, target: "/queries/orderbook?ticker=ETH-USD&depth=1", status: http.StatusNotFound},
		{name: "malformed create", method: http.MethodPost, target: "/commands/orders/create", body: `{`, status: http.StatusBadRequest},
		{name: "invalid create", method: http.MethodPost, target: "/commands/orders/create", body: `{"ticker":"BTC-USD"}`, status: http.StatusBadRequest},
		{name: "limit order without price", method: http.MethodPost, target: "/commands/orders/create", body: `{"command_id":"create-1","ticker":"BTC-USD","user_id":"alice","order_type":"LIMIT","position":"BID","amount":1}`, status: http.StatusBadRequest},
		{name: "market order with price", method: http.MethodPost, target: "/commands/orders/create", body: `{"command_id":"create-1","ticker":"BTC-USD","user_id":"alice","order_type":"MARKET","position":"BID","price":100,"amount":1}`, status: http.StatusBadRequest},
		{name: "market order with negative price", method: http.MethodPost, target: "/commands/orders/create", body: `{"command_id":"create-1","ticker":"BTC-USD","user_id":"alice","order_type":"MARKET","position":"BID","price":-1,"amount":1}`, status: http.StatusBadRequest},
		{name: "unknown order type", method: http.MethodPost, target: "/commands/orders/create", body: `{"command_id":"create-1","ticker":"BTC-USD","user_id":"alice","order_type":"STOP","position":"BID","price":100,"amount":1}`, status: http.StatusBadRequest},
		{name: "unknown create ticker", method: http.MethodPost, target: "/commands/orders/create", body: `{"command_id":"create-1","ticker":"ETH-USD","user_id":"alice","order_type":"LIMIT","position":"BID","price":100,"amount":1}`, status: http.StatusNotFound},
		{name: "invalid amend", method: http.MethodPost, target: "/commands/orders/amend", body: `{"command_id":"amend-1","ticker":"BTC-USD","order_id":"id","price":100,"amount":0}`, status: http.StatusBadRequest},
		{name: "invalid cancel", method: http.MethodPost, target: "/commands/orders/cancel", body: `{"command_id":"cancel-1","ticker":"BTC-USD"}`, status: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, bytes.NewBufferString(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status want %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
		})
	}
}

func TestReadinessAndCommandsFailWhenEngineHalted(t *testing.T) {
	state := engine.NewEngineState()
	worker := engine.NewBookWorkerWithOptions("BTC-USD", nil, engine.BookWorkerOptions{State: state})
	router := engine.NewRouterWithState(state)
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()
	t.Cleanup(func() { shutdownRouter(t, router) })
	handler := NewHandler(router)

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status want %d, got %d: %s", http.StatusOK, ready.Code, ready.Body.String())
	}

	state.Halt(errors.New("database unavailable"))
	unhealthy := httptest.NewRecorder()
	handler.ServeHTTP(unhealthy, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if unhealthy.Code != http.StatusServiceUnavailable {
		t.Fatalf("halted readiness status want %d, got %d: %s", http.StatusServiceUnavailable, unhealthy.Code, unhealthy.Body.String())
	}

	response := serveJSON(handler, "/commands/orders/create", []byte(`{
		"command_id":"create-1",
		"ticker":"BTC-USD",
		"user_id":"alice",
		"order_type":"LIMIT",
		"position":"BID",
		"price":100,
		"amount":1,
		"nonce":1
	}`))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("halted command status want %d, got %d: %s", http.StatusServiceUnavailable, response.Code, response.Body.String())
	}
}

func newTestHandler(t *testing.T, book *engine.OrderBook) (http.Handler, *engine.Router) {
	t.Helper()

	orderEventOut := make(chan orderevent.PersistenceRequest, 16)
	orderEventDone := make(chan struct{})
	go func() {
		defer close(orderEventDone)
		for request := range orderEventOut {
			request.Acknowledge(nil)
		}
	}()
	t.Cleanup(func() {
		close(orderEventOut)
		<-orderEventDone
	})

	worker := engine.NewBookWorkerWithOptions("BTC-USD", book, engine.BookWorkerOptions{
		OrderEventOut: orderEventOut,
		Journal:       &testJournal{},
	})
	router := engine.NewRouter()
	if err := router.Register("BTC-USD", worker); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	go worker.Run()
	t.Cleanup(func() { shutdownRouter(t, router) })
	return NewHandler(router), router
}

func shutdownRouter(t *testing.T, router *engine.Router) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := router.Shutdown(ctx); err != nil {
		t.Fatalf("router shutdown: %v", err)
	}
}

func serveJSON(handler http.Handler, target string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
