package engine

import (
	"fmt"

	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

type orderEventBuilder struct {
	command journal.Command
	events  []orderevent.Event
}

func newOrderEventBuilder(command journal.Command) *orderEventBuilder {
	return &orderEventBuilder{command: command}
}

func (b *orderEventBuilder) addMakerFills(transitions []MakerFillTransition) {
	for _, transition := range transitions {
		eventType := orderevent.PartiallyFilled
		reason := orderevent.ReasonPartialFill
		if transition.RemainingAmount == 0 {
			eventType = orderevent.Filled
			reason = orderevent.ReasonNone
		}
		b.add(transition.Order, orderevent.Event{
			Type:            eventType,
			Reason:          reason,
			PreviousPrice:   transition.Order.Price,
			Price:           transition.Order.Price,
			PreviousAmount:  transition.PreviousAmount,
			FilledAmount:    transition.FilledAmount,
			RemainingAmount: transition.RemainingAmount,
		})
	}
}

func (b *orderEventBuilder) addTakerFill(order models.BookOrder, filledAmount, remainingAmount float64) {
	if filledAmount <= 0 {
		return
	}
	eventType := orderevent.PartiallyFilled
	reason := orderevent.ReasonPartialFill
	if remainingAmount == 0 {
		eventType = orderevent.Filled
		reason = orderevent.ReasonNone
	}
	b.add(order, orderevent.Event{
		Type:            eventType,
		Reason:          reason,
		PreviousPrice:   order.Price,
		Price:           order.Price,
		PreviousAmount:  order.Amount,
		FilledAmount:    filledAmount,
		RemainingAmount: remainingAmount,
	})
}

func (b *orderEventBuilder) addResting(order models.BookOrder) {
	b.add(order, orderevent.Event{
		Type:            orderevent.Resting,
		Reason:          orderevent.ReasonNoMatch,
		PreviousPrice:   order.Price,
		Price:           order.Price,
		PreviousAmount:  order.Amount,
		RemainingAmount: order.Amount,
	})
}

func (b *orderEventBuilder) addAmended(before, after models.BookOrder) {
	reason := orderevent.ReasonAmountDecreased
	switch {
	case before.Price != after.Price:
		reason = orderevent.ReasonPriceChanged
	case before.Amount < after.Amount:
		reason = orderevent.ReasonAmountIncreased
	}
	b.add(after, orderevent.Event{
		Type:            orderevent.Amended,
		Reason:          reason,
		PreviousPrice:   before.Price,
		Price:           after.Price,
		PreviousAmount:  before.Amount,
		RemainingAmount: after.Amount,
	})
}

func (b *orderEventBuilder) addCanceled(order models.BookOrder) {
	b.add(order, orderevent.Event{
		Type:           orderevent.Canceled,
		Reason:         orderevent.ReasonUserRequest,
		PreviousPrice:  order.Price,
		Price:          order.Price,
		PreviousAmount: order.Amount,
		CanceledAmount: order.Amount,
	})
}

func (b *orderEventBuilder) addRemainderCanceled(order models.BookOrder, remainingAmount float64) {
	b.add(order, orderevent.Event{
		Type:           orderevent.RemainderCanceled,
		Reason:         orderevent.ReasonInsufficientLiquidity,
		PreviousPrice:  order.Price,
		Price:          order.Price,
		PreviousAmount: remainingAmount,
		CanceledAmount: remainingAmount,
	})
}

func (b *orderEventBuilder) add(order models.BookOrder, event orderevent.Event) {
	index := int32(len(b.events))
	event.CommandID = b.command.CommandID
	event.CommandSequence = b.command.Sequence
	event.EventIndex = index
	event.OrderID = order.OrderID
	event.UserID = order.UserID
	event.Ticker = b.command.Ticker
	event.OrderType = order.OrderType
	event.Side = order.Position
	event.OccurredAt = b.command.RecordedAt
	event.EventID = orderevent.GenerateEventID(event.Ticker, event.CommandID, event.OrderID, event.Type, event.EventIndex)
	b.events = append(b.events, event)
}

func (b *orderEventBuilder) result() ([]orderevent.Event, error) {
	for i, event := range b.events {
		if err := orderevent.Validate(event); err != nil {
			return nil, fmt.Errorf("validate generated order event %d: %w", i, err)
		}
	}
	return b.events, nil
}
