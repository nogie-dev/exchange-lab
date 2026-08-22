package orderevent

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/numeric"
)

func TestGenerateEventIDIsStableAndTransitionSpecific(t *testing.T) {
	first := GenerateEventID("BTC-USD", "command-1", "order-1", PartiallyFilled, 0)
	if first == "" {
		t.Fatal("event ID must not be empty")
	}
	if got := GenerateEventID("BTC-USD", "command-1", "order-1", PartiallyFilled, 0); got != first {
		t.Fatalf("event ID must be stable: want %q, got %q", first, got)
	}
	if got := GenerateEventID("BTC-USD", "command-1", "maker-order", PartiallyFilled, 0); got == first {
		t.Fatal("maker and taker transitions must have different event IDs")
	}
	if got := GenerateEventID("BTC-USD", "command-1", "order-1", RemainderCanceled, 1); got == first {
		t.Fatal("separate transitions must have different event IDs")
	}
}

func TestValidateAcceptsLifecycleTransitions(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{name: "resting", event: testEvent(Resting, 1, 0, 0, 1)},
		{name: "partially filled", event: testEvent(PartiallyFilled, 1, 0.4, 0, 0.6)},
		{name: "filled", event: testEvent(Filled, 1, 1, 0, 0)},
		{name: "amended", event: testEvent(Amended, 1, 0, 0, 2)},
		{name: "canceled", event: testEvent(Canceled, 1, 0, 1, 0)},
		{name: "market remainder canceled", event: marketRemainderEvent()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.event); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
		})
	}
}

func TestValidateRejectsInconsistentTransition(t *testing.T) {
	event := testEvent(PartiallyFilled, 1, 0.4, 0, 0.7)
	if err := Validate(event); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Validate want ErrInvalidEvent, got %v", err)
	}
}

func testEvent(eventType Type, previous, filled, canceled, remaining float64) Event {
	event := Event{
		CommandID:       "command-1",
		CommandSequence: 7,
		EventIndex:      0,
		OrderID:         "order-1",
		UserID:          "alice",
		Ticker:          "BTC-USD",
		Type:            eventType,
		Reason:          ReasonNone,
		OrderType:       models.Limit,
		Side:            models.Bid,
		PreviousPrice:   numeric.MustPrice("100"),
		Price:           numeric.MustPrice("100"),
		PreviousAmount:  numeric.MustQuantity(strconv.FormatFloat(previous, 'f', -1, 64)),
		FilledAmount:    numeric.MustQuantity(strconv.FormatFloat(filled, 'f', -1, 64)),
		CanceledAmount:  numeric.MustQuantity(strconv.FormatFloat(canceled, 'f', -1, 64)),
		RemainingAmount: numeric.MustQuantity(strconv.FormatFloat(remaining, 'f', -1, 64)),
		OccurredAt:      time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
	}
	event.EventID = GenerateEventID(event.Ticker, event.CommandID, event.OrderID, event.Type, event.EventIndex)
	return event
}

func marketRemainderEvent() Event {
	event := testEvent(RemainderCanceled, 0.4, 0, 0.4, 0)
	event.OrderType = models.Market
	event.PreviousPrice = 0
	event.Price = 0
	event.Reason = ReasonInsufficientLiquidity
	event.EventIndex = 1
	event.EventID = GenerateEventID(event.Ticker, event.CommandID, event.OrderID, event.Type, event.EventIndex)
	return event
}
