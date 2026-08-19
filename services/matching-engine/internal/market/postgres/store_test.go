package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nogie-dev/clob-trading/internal/market"
	"github.com/nogie-dev/clob-trading/internal/numeric"
)

type fakeDBTX struct {
	createValues []any
	createErr    error
	existing     []any
}

func (f *fakeDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("exec not implemented")
}

func (f *fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("query not implemented")
}

func (f *fakeDBTX) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	if f.createValues != nil || f.createErr != nil {
		row := fakeRow{values: f.createValues, err: f.createErr}
		f.createValues = nil
		f.createErr = nil
		return row
	}
	return fakeRow{values: f.existing}
}

type fakeRow struct {
	values []any
	err    error
}

func marketRow(ticker string, createdAt time.Time) []any {
	precision := numeric.DefaultPrecision()
	return []any{
		ticker,
		pgtype.Timestamptz{Time: createdAt, Valid: true},
		int16(precision.PriceScale),
		int16(precision.QuantityScale),
		int16(precision.QuoteScale),
		precision.TickSizeUnits,
		precision.LotSizeUnits,
		int64(precision.MinPriceTicks),
		int64(precision.MaxPriceTicks),
		int64(precision.MinQuantityLots),
		int64(precision.MaxQuantityLots),
		precision.ConfigVersion,
	}
}

func (f fakeRow) Scan(dest ...interface{}) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) != len(f.values) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(f.values[i]))
	}
	return nil
}

func TestStoreAddReturnsInsertedMarket(t *testing.T) {
	createdAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewStore(&fakeDBTX{createValues: marketRow("ETH-USD", createdAt)})

	result, err := store.Add(context.Background(), " ETH-USD ")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if !result.Inserted || result.Market.Ticker != "ETH-USD" || result.Market.CreatedAt != createdAt {
		t.Fatalf("unexpected add result: %#v", result)
	}
}

func TestStoreAddReturnsExistingMarketOnRetry(t *testing.T) {
	createdAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store := NewStore(&fakeDBTX{
		createErr: pgx.ErrNoRows,
		existing:  marketRow("ETH-USD", createdAt),
	})

	result, err := store.Add(context.Background(), "ETH-USD")
	if err != nil {
		t.Fatalf("Add retry returned error: %v", err)
	}
	if result.Inserted || result.Market.Ticker != "ETH-USD" || result.Market.CreatedAt != createdAt {
		t.Fatalf("unexpected retry result: %#v", result)
	}
}

func TestStoreAddRejectsEmptyTicker(t *testing.T) {
	store := NewStore(&fakeDBTX{})
	_, err := store.Add(context.Background(), " ")
	if !errors.Is(err, market.ErrInvalidTicker) {
		t.Fatalf("Add want ErrInvalidTicker, got %v", err)
	}
}
