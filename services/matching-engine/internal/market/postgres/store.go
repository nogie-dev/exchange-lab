package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nogie-dev/clob-trading/internal/market"
	"github.com/nogie-dev/clob-trading/internal/market/postgres/db"
	"github.com/nogie-dev/clob-trading/internal/numeric"
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
		registered, err := toMarket(row)
		if err != nil {
			return market.AddResult{}, fmt.Errorf("add market %q: %w", normalized, err)
		}
		return market.AddResult{
			Market:   registered,
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
	registered, err := toMarket(existing)
	if err != nil {
		return market.AddResult{}, fmt.Errorf("read existing market %q: %w", normalized, err)
	}
	return market.AddResult{
		Market:   registered,
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
		registered, err := toMarket(row)
		if err != nil {
			return nil, fmt.Errorf("market row %d: %w", i, err)
		}
		markets[i] = registered
	}
	return markets, nil
}

func toMarket(row db.Market) (market.Market, error) {
	precision := numeric.Precision{
		PriceScale:      int32(row.PriceScale),
		QuantityScale:   int32(row.QuantityScale),
		QuoteScale:      int32(row.QuoteScale),
		TickSizeUnits:   row.TickSizeUnits,
		LotSizeUnits:    row.LotSizeUnits,
		MinPriceTicks:   numeric.PriceTicks(row.MinPriceTicks),
		MaxPriceTicks:   numeric.PriceTicks(row.MaxPriceTicks),
		MinQuantityLots: numeric.QuantityLots(row.MinQuantityLots),
		MaxQuantityLots: numeric.QuantityLots(row.MaxQuantityLots),
		ConfigVersion:   row.ConfigVersion,
	}.WithDefaults()
	if err := precision.Validate(); err != nil {
		return market.Market{}, fmt.Errorf("invalid numeric precision: %w", err)
	}
	return market.Market{
		Ticker:    row.Ticker,
		CreatedAt: row.CreatedAt.Time,
		Precision: precision,
	}, nil
}
