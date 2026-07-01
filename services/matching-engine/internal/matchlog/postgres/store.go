package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/matchlog/postgres/db"
	"github.com/nogie-dev/clob-trading/internal/models"
)

var ErrInvalidMatchLog = errors.New("invalid match log")

// Store appends raw match logs with sqlc-generated pgx queries.
type Store struct {
	queries *db.Queries
	now     func() time.Time
}

func NewStore(conn db.DBTX) *Store {
	return &Store{
		queries: db.New(conn),
		now:     time.Now,
	}
}

func (s *Store) SaveMatchLog(ctx context.Context, log matchlog.MatchLog) error {
	params, err := s.params(log)
	if err != nil {
		return err
	}
	return s.queries.CreateMatchLog(ctx, params)
}

func (s *Store) SaveMatchLogs(ctx context.Context, logs []matchlog.MatchLog) error {
	for i, log := range logs {
		if err := s.SaveMatchLog(ctx, log); err != nil {
			return fmt.Errorf("save match log %d: %w", i, err)
		}
	}
	return nil
}

func (s *Store) params(log matchlog.MatchLog) (db.CreateMatchLogParams, error) {
	if err := validateMatchLog(log); err != nil {
		return db.CreateMatchLogParams{}, err
	}

	matchedAt := log.MatchedAt
	if matchedAt.IsZero() {
		matchedAt = s.now().UTC()
	}

	quoteAmount := log.QuoteAmount
	if quoteAmount == 0 {
		quoteAmount = log.Price * log.Amount
	}

	return db.CreateMatchLogParams{
		Ticker:       log.Ticker,
		Price:        log.Price,
		Amount:       log.Amount,
		QuoteAmount:  quoteAmount,
		MakerOrderID: log.MakerOrderID,
		TakerOrderID: log.TakerOrderID,
		MakerUserID:  log.MakerUserID,
		TakerUserID:  log.TakerUserID,
		MakerSide:    string(log.MakerSide),
		TakerSide:    string(log.TakerSide),
		MatchedAt: pgtype.Timestamptz{
			Time:  matchedAt,
			Valid: true,
		},
	}, nil
}

func validateMatchLog(log matchlog.MatchLog) error {
	switch {
	case log.Ticker == "":
		return fmt.Errorf("%w: ticker is required", ErrInvalidMatchLog)
	case log.Price <= 0:
		return fmt.Errorf("%w: price must be positive", ErrInvalidMatchLog)
	case log.Amount <= 0:
		return fmt.Errorf("%w: amount must be positive", ErrInvalidMatchLog)
	case log.QuoteAmount < 0:
		return fmt.Errorf("%w: quote amount cannot be negative", ErrInvalidMatchLog)
	case log.MakerOrderID == "":
		return fmt.Errorf("%w: maker order id is required", ErrInvalidMatchLog)
	case log.TakerOrderID == "":
		return fmt.Errorf("%w: taker order id is required", ErrInvalidMatchLog)
	case log.MakerUserID == "":
		return fmt.Errorf("%w: maker user id is required", ErrInvalidMatchLog)
	case log.TakerUserID == "":
		return fmt.Errorf("%w: taker user id is required", ErrInvalidMatchLog)
	case !validSide(log.MakerSide):
		return fmt.Errorf("%w: maker side must be BID or ASK", ErrInvalidMatchLog)
	case !validSide(log.TakerSide):
		return fmt.Errorf("%w: taker side must be BID or ASK", ErrInvalidMatchLog)
	}
	return nil
}

func validSide(side models.Position) bool {
	return side == models.Bid || side == models.Ask
}
