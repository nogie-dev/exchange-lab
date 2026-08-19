package engine

import (
	"testing"
	"time"

	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

func TestApplyCreateCommandReturnsRestingEvent(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	command := lifecycleCreateCommand(1, "resting-command", "alice", models.Bid, models.Limit, 100, 2, 1)

	events := applyCommandEvents(t, worker, command)

	if len(events) != 1 {
		t.Fatalf("resting order events want 1, got %#v", events)
	}
	event := events[0]
	assertOrderEvent(t, event, orderevent.Resting, 0, 2, 0, 0, 2)
	if event.Reason != orderevent.ReasonNoMatch {
		t.Fatalf("resting reason want %q, got %q", orderevent.ReasonNoMatch, event.Reason)
	}
}

func TestApplyMarketCommandReturnsMakerTakerAndRemainderEvents(t *testing.T) {
	worker := newLifecycleTestWorker(t)
	firstMaker := lifecycleCreateCommand(1, "maker-1-command", "maker-1", models.Ask, models.Limit, 100, 5, 1)
	secondMaker := lifecycleCreateCommand(2, "maker-2-command", "maker-2", models.Ask, models.Limit, 101, 10, 2)
	firstMakerID := applyCommandEvents(t, worker, firstMaker)[0].OrderID
	secondMakerID := applyCommandEvents(t, worker, secondMaker)[0].OrderID

	taker := lifecycleCreateCommand(3, "market-command", "taker", models.Bid, models.Market, 0, 18, 3)
	events := applyCommandEvents(t, worker, taker)

	if len(events) != 4 {
		t.Fatalf("market order events want 4, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.Filled, 0, 5, 5, 0, 0)
	assertOrderEvent(t, events[1], orderevent.Filled, 1, 10, 10, 0, 0)
	assertOrderEvent(t, events[2], orderevent.PartiallyFilled, 2, 18, 15, 0, 3)
	assertOrderEvent(t, events[3], orderevent.RemainderCanceled, 3, 3, 0, 3, 0)
	if events[0].OrderID != firstMakerID || events[1].OrderID != secondMakerID {
		t.Fatalf("maker event order mismatch: %#v", events)
	}
	if events[2].OrderID != events[3].OrderID || events[2].UserID != "taker" {
		t.Fatalf("taker events should describe the same order: %#v", events[2:])
	}
	if events[3].Reason != orderevent.ReasonInsufficientLiquidity {
		t.Fatalf("remainder reason want %q, got %q", orderevent.ReasonInsufficientLiquidity, events[3].Reason)
	}
	if len(worker.OrderBook.Index) != 0 {
		t.Fatalf("market residual must not rest on the book: %#v", worker.OrderBook.Index)
	}
}

func TestApplyPartiallyFilledLimitCommandDoesNotAddRedundantRestingEvent(t *testing.T) {
	worker := newLifecycleTestWorker(t)
	maker := lifecycleCreateCommand(1, "maker-command", "maker", models.Ask, models.Limit, 100, 5, 1)
	applyCommandEvents(t, worker, maker)

	taker := lifecycleCreateCommand(2, "limit-command", "taker", models.Bid, models.Limit, 100, 10, 2)
	events := applyCommandEvents(t, worker, taker)

	if len(events) != 2 {
		t.Fatalf("partial limit order events want maker+taker only, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.Filled, 0, 5, 5, 0, 0)
	assertOrderEvent(t, events[1], orderevent.PartiallyFilled, 1, 10, 5, 0, 5)
	if _, ok := worker.OrderBook.Index[events[1].OrderID]; !ok {
		t.Fatal("partially filled limit taker should rest with its remaining amount")
	}
}

func TestApplyCommandReturnsPartialMakerAndFilledTakerEvents(t *testing.T) {
	worker := newLifecycleTestWorker(t)
	maker := lifecycleCreateCommand(1, "maker-command", "maker", models.Ask, models.Limit, 100, 10, 1)
	applyCommandEvents(t, worker, maker)

	taker := lifecycleCreateCommand(2, "market-command", "taker", models.Bid, models.Market, 0, 4, 2)
	events := applyCommandEvents(t, worker, taker)

	if len(events) != 2 {
		t.Fatalf("full taker events want maker+taker, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.PartiallyFilled, 0, 10, 4, 0, 6)
	assertOrderEvent(t, events[1], orderevent.Filled, 1, 4, 4, 0, 0)
}

func TestApplySequentialTakersConsumeOneMakerFromLatestRemainingAmount(t *testing.T) {
	worker := newLifecycleTestWorker(t)
	maker := lifecycleCreateCommand(1, "maker-command", "maker", models.Ask, models.Limit, 100, 10, 1)
	makerID := applyCommandEvents(t, worker, maker)[0].OrderID

	first := lifecycleCreateCommand(2, "taker-1-command", "taker-1", models.Bid, models.Market, 0, 4, 2)
	firstEvents := applyCommandEvents(t, worker, first)
	if len(firstEvents) != 2 {
		t.Fatalf("first taker events want 2, got %#v", firstEvents)
	}
	assertMakerEvent(t, firstEvents[0], makerID, orderevent.PartiallyFilled, 10, 4, 6)
	assertOrderEvent(t, firstEvents[1], orderevent.Filled, 1, 4, 4, 0, 0)

	second := lifecycleCreateCommand(3, "taker-2-command", "taker-2", models.Bid, models.Market, 0, 3, 3)
	secondEvents := applyCommandEvents(t, worker, second)
	if len(secondEvents) != 2 {
		t.Fatalf("second taker events want 2, got %#v", secondEvents)
	}
	assertMakerEvent(t, secondEvents[0], makerID, orderevent.PartiallyFilled, 6, 3, 3)
	assertOrderEvent(t, secondEvents[1], orderevent.Filled, 1, 3, 3, 0, 0)

	third := lifecycleCreateCommand(4, "taker-3-command", "taker-3", models.Bid, models.Market, 0, 5, 4)
	thirdEvents := applyCommandEvents(t, worker, third)
	if len(thirdEvents) != 3 {
		t.Fatalf("third taker events want 3, got %#v", thirdEvents)
	}
	assertMakerEvent(t, thirdEvents[0], makerID, orderevent.Filled, 3, 3, 0)
	assertOrderEvent(t, thirdEvents[1], orderevent.PartiallyFilled, 1, 5, 3, 0, 2)
	assertOrderEvent(t, thirdEvents[2], orderevent.RemainderCanceled, 2, 2, 0, 2, 0)

	if _, ok := worker.OrderBook.Index[makerID]; ok {
		t.Fatal("fully consumed maker should be removed after the third taker")
	}
}

func TestApplyMarketCommandWithoutLiquidityReturnsOnlyRemainderCanceled(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	command := lifecycleCreateCommand(1, "market-command", "taker", models.Bid, models.Market, 0, 3, 1)

	events := applyCommandEvents(t, worker, command)

	if len(events) != 1 {
		t.Fatalf("market no-liquidity events want 1, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.RemainderCanceled, 0, 3, 0, 3, 0)
}

func TestApplyAmendAndCancelCommandsReturnLifecycleEvents(t *testing.T) {
	worker := newLifecycleTestWorker(t)
	ask := lifecycleCreateCommand(1, "ask-command", "maker", models.Ask, models.Limit, 101, 1, 1)
	applyCommandEvents(t, worker, ask)
	bid := lifecycleCreateCommand(2, "bid-command", "owner", models.Bid, models.Limit, 100, 2, 2)
	bidOrderID := applyCommandEvents(t, worker, bid)[0].OrderID

	amendPrice := lifecycleCommandTime(3)
	priceCommand := journal.Command{
		CommandID:  "price-amend-command",
		Ticker:     "BTC-USD",
		Sequence:   3,
		Type:       journal.AmendCommand,
		RecordedAt: amendPrice,
		Amend: &models.EditOrderRequest{
			CommandID: "price-amend-command",
			Ticker:    "BTC-USD",
			OrderID:   bidOrderID,
			Price:     testPrice(101),
		},
	}
	events := applyCommandEvents(t, worker, priceCommand)
	if len(events) != 3 {
		t.Fatalf("price amendment events want amend+maker+taker, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.Amended, 0, 2, 0, 0, 2)
	if events[0].PreviousPrice != testPrice(100) || events[0].Price != testPrice(101) || events[0].Reason != orderevent.ReasonPriceChanged {
		t.Fatalf("unexpected amendment event: %#v", events[0])
	}
	assertOrderEvent(t, events[1], orderevent.Filled, 1, 1, 1, 0, 0)
	assertOrderEvent(t, events[2], orderevent.PartiallyFilled, 2, 2, 1, 0, 1)

	increasedAmount := testQuantity(2.0)
	amountCommand := journal.Command{
		CommandID:  "amount-amend-command",
		Ticker:     "BTC-USD",
		Sequence:   4,
		Type:       journal.AmendCommand,
		RecordedAt: lifecycleCommandTime(4),
		Amend: &models.EditOrderRequest{
			CommandID: "amount-amend-command",
			Ticker:    "BTC-USD",
			OrderID:   bidOrderID,
			Price:     testPrice(101),
			Amount:    &increasedAmount,
		},
	}
	events = applyCommandEvents(t, worker, amountCommand)
	if len(events) != 1 {
		t.Fatalf("amount amendment events want 1, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.Amended, 0, 1, 0, 0, 2)
	if events[0].Reason != orderevent.ReasonAmountIncreased {
		t.Fatalf("amount amendment reason want %q, got %q", orderevent.ReasonAmountIncreased, events[0].Reason)
	}

	cancelCommand := journal.Command{
		CommandID:  "cancel-command",
		Ticker:     "BTC-USD",
		Sequence:   5,
		Type:       journal.CancelCommand,
		RecordedAt: lifecycleCommandTime(5),
		Cancel: &models.CancelOrderRequest{
			CommandID: "cancel-command",
			Ticker:    "BTC-USD",
			OrderID:   bidOrderID,
		},
	}
	events = applyCommandEvents(t, worker, cancelCommand)
	if len(events) != 1 {
		t.Fatalf("cancel events want 1, got %#v", events)
	}
	assertOrderEvent(t, events[0], orderevent.Canceled, 0, 2, 0, 2, 0)
	if events[0].Reason != orderevent.ReasonUserRequest {
		t.Fatalf("cancel reason want %q, got %q", orderevent.ReasonUserRequest, events[0].Reason)
	}
}

func TestApplyCommandsWithoutOrderbookChangeReturnNoEvents(t *testing.T) {
	worker := NewBookWorker("BTC-USD", nil)
	missingAmend := journal.Command{
		CommandID:  "missing-amend-command",
		Ticker:     "BTC-USD",
		Sequence:   1,
		Type:       journal.AmendCommand,
		RecordedAt: lifecycleCommandTime(1),
		Amend: &models.EditOrderRequest{
			CommandID: "missing-amend-command",
			Ticker:    "BTC-USD",
			OrderID:   "missing",
			Price:     testPrice(100),
		},
	}
	if events := applyCommandEvents(t, worker, missingAmend); len(events) != 0 {
		t.Fatalf("missing amendment must not emit lifecycle events: %#v", events)
	}

	missingCancel := journal.Command{
		CommandID:  "missing-cancel-command",
		Ticker:     "BTC-USD",
		Sequence:   2,
		Type:       journal.CancelCommand,
		RecordedAt: lifecycleCommandTime(2),
		Cancel: &models.CancelOrderRequest{
			CommandID: "missing-cancel-command",
			Ticker:    "BTC-USD",
			OrderID:   "missing",
		},
	}
	if events := applyCommandEvents(t, worker, missingCancel); len(events) != 0 {
		t.Fatalf("missing cancellation must not emit lifecycle events: %#v", events)
	}
}

func newLifecycleTestWorker(t *testing.T) *BookWorker {
	t.Helper()
	persistenceOut := make(chan matchlog.PersistenceRequest, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for request := range persistenceOut {
			request.Acknowledge(nil)
		}
	}()
	t.Cleanup(func() {
		close(persistenceOut)
		<-done
	})
	return NewBookWorkerWithOptions("BTC-USD", nil, BookWorkerOptions{PersistenceOut: persistenceOut})
}

func lifecycleCreateCommand(sequence int64, commandID, userID string, side models.Position, orderType models.OrderType, price, amount float64, nonce uint64) journal.Command {
	request := models.CreateOrderRequest{
		CommandID: commandID,
		Ticker:    "BTC-USD",
		UserID:    userID,
		OrderType: orderType,
		Position:  side,
		Price:     testPrice(price),
		Amount:    testQuantity(amount),
		Nonce:     nonce,
	}
	return journal.Command{
		CommandID:  commandID,
		Ticker:     "BTC-USD",
		Sequence:   sequence,
		Type:       journal.CreateCommand,
		RecordedAt: lifecycleCommandTime(sequence),
		Create:     &request,
	}
}

func lifecycleCommandTime(sequence int64) time.Time {
	return time.Date(2026, 8, 13, 12, 0, int(sequence), 0, time.UTC)
}

func applyCommandEvents(t *testing.T, worker *BookWorker, command journal.Command) []orderevent.Event {
	t.Helper()
	events, err := worker.applyCommand(command)
	if err != nil {
		t.Fatalf("applyCommand returned error: %v", err)
	}
	for i, event := range events {
		if err := orderevent.Validate(event); err != nil {
			t.Fatalf("event %d is invalid: %v", i, err)
		}
		if event.CommandID != command.CommandID || event.CommandSequence != command.Sequence || event.OccurredAt != command.RecordedAt {
			t.Fatalf("event %d command metadata mismatch: %#v", i, event)
		}
	}
	return events
}

func assertOrderEvent(t *testing.T, event orderevent.Event, eventType orderevent.Type, index int32, previous, filled, canceled, remaining float64) {
	t.Helper()
	if event.Type != eventType ||
		event.EventIndex != index ||
		!approxEqual(event.PreviousAmount, previous) ||
		!approxEqual(event.FilledAmount, filled) ||
		!approxEqual(event.CanceledAmount, canceled) ||
		!approxEqual(event.RemainingAmount, remaining) {
		t.Fatalf("unexpected order event: %#v", event)
	}
}

func assertMakerEvent(t *testing.T, event orderevent.Event, orderID string, eventType orderevent.Type, previous, filled, remaining float64) {
	t.Helper()
	assertOrderEvent(t, event, eventType, 0, previous, filled, 0, remaining)
	if event.OrderID != orderID || event.UserID != "maker" {
		t.Fatalf("unexpected maker event identity: %#v", event)
	}
}
