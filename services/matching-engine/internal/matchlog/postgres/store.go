package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/matchlog/postgres/db"
	"github.com/nogie-dev/clob-trading/internal/models"
)

var (
	ErrInvalidMatchLog  = errors.New("invalid match log")
	ErrMatchLogConflict = errors.New("match log execution id conflict")
)

type transaction interface {
	db.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Store appends raw match logs with sqlc-generated pgx queries.
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

func (s *Store) SaveMatchLog(ctx context.Context, log matchlog.MatchLog) error {
	return s.SaveMatchLogs(ctx, []matchlog.MatchLog{log})
}

func (s *Store) SaveMatchLogs(ctx context.Context, logs []matchlog.MatchLog) error {
	if len(logs) == 0 {
		return nil
	}

	params := make([]db.CreateMatchLogParams, len(logs))
	for i, log := range logs {
		var err error
		params[i], err = s.params(log)
		if err != nil {
			return fmt.Errorf("validate match log %d: %w", i, err)
		}
	}

	tx, err := s.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin match log transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := db.New(tx)
	for i, param := range params {
		inserted, err := queries.CreateMatchLog(ctx, param)
		if err != nil {
			return fmt.Errorf("insert match log %d: %w", i, err)
		}
		if inserted == 1 {
			continue
		}

		matches, err := queries.MatchLogPayloadMatches(ctx, payloadParams(param))
		if err != nil {
			return fmt.Errorf("check existing match log %d: %w", i, err)
		}
		if !matches.Valid || !matches.Bool {
			return fmt.Errorf("%w: execution id %q", ErrMatchLogConflict, param.ExecutionID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit match logs: %w", err)
	}
	return nil
}

func payloadParams(param db.CreateMatchLogParams) db.MatchLogPayloadMatchesParams {
	return db.MatchLogPayloadMatchesParams{
		ExecutionID:  param.ExecutionID,
		Ticker:       param.Ticker,
		Price:        param.Price,
		Amount:       param.Amount,
		QuoteAmount:  param.QuoteAmount,
		MakerOrderID: param.MakerOrderID,
		TakerOrderID: param.TakerOrderID,
		MakerUserID:  param.MakerUserID,
		TakerUserID:  param.TakerUserID,
		MakerSide:    param.MakerSide,
		TakerSide:    param.TakerSide,
		MatchedAt:    param.MatchedAt,
	}
}

func (s *Store) params(log matchlog.MatchLog) (db.CreateMatchLogParams, error) {
	if err := validateMatchLog(log); err != nil {
		return db.CreateMatchLogParams{}, err
	}

	quoteAmount := log.QuoteAmount
	if quoteAmount == 0 {
		quoteAmount = log.Price * log.Amount
	}

	return db.CreateMatchLogParams{
		ExecutionID:  log.ExecutionID,
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
			Time:  log.MatchedAt.UTC(),
			Valid: true,
		},
	}, nil
}

func validateMatchLog(log matchlog.MatchLog) error {
	switch {
	case log.ExecutionID == "":
		return fmt.Errorf("%w: execution id is required", ErrInvalidMatchLog)
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
	case log.MatchedAt.IsZero():
		return fmt.Errorf("%w: matched at is required", ErrInvalidMatchLog)
	}
	return nil
}

func validSide(side models.Position) bool {
	return side == models.Bid || side == models.Ask
}
