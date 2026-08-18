package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nogie-dev/clob-trading/internal/market"
	"github.com/nogie-dev/clob-trading/internal/market/postgres/db"
)

type Store struct {
	queries *db.Queries
}

func NewStore(conn db.DBTX) *Store {
	return &Store{queries: db.New(conn)}
}

func (s *Store) Add(ctx context.Context, ticker string) (market.AddResult, error) {
	normalized, err := market.NormalizeTicker(ticker)
	if err != nil {
		return market.AddResult{}, err
	}

	row, err := s.queries.CreateMarket(ctx, normalized)
	if err == nil {
		if !row.CreatedAt.Valid {
			return market.AddResult{}, fmt.Errorf("add market %q: created time is required", normalized)
		}
		return market.AddResult{
			Market:   market.Market{Ticker: row.Ticker, CreatedAt: row.CreatedAt.Time},
			Inserted: true,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return market.AddResult{}, fmt.Errorf("add market %q: %w", normalized, err)
	}

	existing, err := s.queries.GetMarket(ctx, normalized)
	if err != nil {
		return market.AddResult{}, fmt.Errorf("read existing market %q: %w", normalized, err)
	}
	if !existing.CreatedAt.Valid {
		return market.AddResult{}, fmt.Errorf("read existing market %q: created time is required", normalized)
	}
	return market.AddResult{
		Market:   market.Market{Ticker: existing.Ticker, CreatedAt: existing.CreatedAt.Time},
		Inserted: false,
	}, nil
}

func (s *Store) List(ctx context.Context) ([]market.Market, error) {
	rows, err := s.queries.ListMarkets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}
	markets := make([]market.Market, len(rows))
	for i, row := range rows {
		if !row.CreatedAt.Valid {
			return nil, fmt.Errorf("market row %d: created time is required", i)
		}
		markets[i] = market.Market{Ticker: row.Ticker, CreatedAt: row.CreatedAt.Time}
	}
	return markets, nil
}
