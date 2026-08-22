package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nogie-dev/clob-trading/internal/numeric"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
	"github.com/nogie-dev/clob-trading/internal/orderevent/postgres/db"
)

var ErrOrderEventConflict = errors.New("order event id conflict")

type transaction interface {
	db.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Store appends order lifecycle events with sqlc-generated pgx queries.
type Store struct {
	begin func(context.Context) (transaction, error)
}

func NewStore(conn transactionBeginner) *Store {
	return &Store{
		begin: func(ctx context.Context) (transaction, error) {
			return conn.Begin(ctx)
		},
	}
}

func (s *Store) SaveEvent(ctx context.Context, event orderevent.Event) error {
	return s.SaveEvents(ctx, []orderevent.Event{event})
}

func (s *Store) SaveEvents(ctx context.Context, events []orderevent.Event) error {
	if len(events) == 0 {
		return nil
	}

	params := make([]db.CreateOrderEventParams, len(events))
	for i, event := range events {
		var err error
		params[i], err = eventParams(event)
		if err != nil {
			return fmt.Errorf("validate order event %d: %w", i, err)
		}
	}

	tx, err := s.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin order event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := db.New(tx)
	for i, param := range params {
		inserted, err := queries.CreateOrderEvent(ctx, param)
		if err != nil {
			return fmt.Errorf("insert order event %d: %w", i, err)
		}
		if inserted == 1 {
			continue
		}

		matches, err := queries.OrderEventPayloadMatches(ctx, eventPayloadParams(param))
		if err != nil {
			return fmt.Errorf("check existing order event %d: %w", i, err)
		}
		if !matches.Valid || !matches.Bool {
			return fmt.Errorf("%w: event id %q", ErrOrderEventConflict, param.EventID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit order events: %w", err)
	}
	return nil
}

func eventParams(event orderevent.Event) (db.CreateOrderEventParams, error) {
	if err := orderevent.Validate(event); err != nil {
		return db.CreateOrderEventParams{}, err
	}
	configVersion := event.MarketConfigVersion
	if configVersion == 0 {
		configVersion = numeric.DefaultConfigVersion
	}
	return db.CreateOrderEventParams{
		EventID:             event.EventID,
		CommandID:           event.CommandID,
		CommandSequence:     event.CommandSequence,
		EventIndex:          event.EventIndex,
		OrderID:             event.OrderID,
		UserID:              event.UserID,
		Ticker:              event.Ticker,
		EventType:           string(event.Type),
		Reason:              event.Reason,
		OrderType:           string(event.OrderType),
		Side:                string(event.Side),
		PreviousPriceTicks:  int64(event.PreviousPrice),
		PriceTicks:          int64(event.Price),
		PreviousAmountLots:  int64(event.PreviousAmount),
		FilledAmountLots:    int64(event.FilledAmount),
		CanceledAmountLots:  int64(event.CanceledAmount),
		RemainingAmountLots: int64(event.RemainingAmount),
		MarketConfigVersion: configVersion,
		OccurredAt: pgtype.Timestamptz{
			Time:  event.OccurredAt.UTC(),
			Valid: true,
		},
	}, nil
}

func eventPayloadParams(param db.CreateOrderEventParams) db.OrderEventPayloadMatchesParams {
	return db.OrderEventPayloadMatchesParams{
		EventID:             param.EventID,
		CommandID:           param.CommandID,
		CommandSequence:     param.CommandSequence,
		EventIndex:          param.EventIndex,
		OrderID:             param.OrderID,
		UserID:              param.UserID,
		Ticker:              param.Ticker,
		EventType:           param.EventType,
		Reason:              param.Reason,
		OrderType:           param.OrderType,
		Side:                param.Side,
		PreviousPriceTicks:  param.PreviousPriceTicks,
		PriceTicks:          param.PriceTicks,
		PreviousAmountLots:  param.PreviousAmountLots,
		FilledAmountLots:    param.FilledAmountLots,
		CanceledAmountLots:  param.CanceledAmountLots,
		RemainingAmountLots: param.RemainingAmountLots,
		MarketConfigVersion: param.MarketConfigVersion,
		OccurredAt:          param.OccurredAt,
	}
}
