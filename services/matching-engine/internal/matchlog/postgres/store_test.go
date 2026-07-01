package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/models"
)

type execCall struct {
	sql  string
	args []interface{}
}

type fakeDBTX struct {
	calls []execCall
	err   error
}

func (f *fakeDBTX) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.calls = append(f.calls, execCall{sql: sql, args: args})
	return pgconn.NewCommandTag("INSERT 0 1"), f.err
}

func (f *fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("query not implemented in fakeDBTX")
}

func (f *fakeDBTX) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return nil
}

func testMatchLog() matchlog.MatchLog {
	return matchlog.MatchLog{
		Ticker:       "BTC-USD",
		Price:        100,
		Amount:       0.5,
		QuoteAmount:  50,
		MakerOrderID: "ask-1",
		TakerOrderID: "bid-1",
		MakerUserID:  "maker-user",
		TakerUserID:  "taker-user",
		MakerSide:    models.Ask,
		TakerSide:    models.Bid,
		MatchedAt:    time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	}
}

func TestStoreSaveMatchLogExecutesInsert(t *testing.T) {
	fake := &fakeDBTX{}
	store := NewStore(fake)
	log := testMatchLog()

	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("SaveMatchLog returned error: %v", err)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	if !strings.Contains(call.sql, "INSERT INTO match_logs") {
		t.Fatalf("expected INSERT INTO match_logs query, got %q", call.sql)
	}

	want := []interface{}{
		log.Ticker,
		log.Price,
		log.Amount,
		log.QuoteAmount,
		log.MakerOrderID,
		log.TakerOrderID,
		log.MakerUserID,
		log.TakerUserID,
		string(log.MakerSide),
		string(log.TakerSide),
		pgtype.Timestamptz{Time: log.MatchedAt, Valid: true},
	}
	if len(call.args) != len(want) {
		t.Fatalf("arg count want %d, got %d", len(want), len(call.args))
	}
	for i := range want {
		if call.args[i] != want[i] {
			t.Fatalf("arg %d want %#v, got %#v", i, want[i], call.args[i])
		}
	}
}

func TestStoreSaveMatchLogDerivesQuoteAmountAndMatchedAt(t *testing.T) {
	fake := &fakeDBTX{}
	store := NewStore(fake)
	now := time.Date(2026, 6, 27, 13, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	log := testMatchLog()
	log.QuoteAmount = 0
	log.MatchedAt = time.Time{}

	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("SaveMatchLog returned error: %v", err)
	}

	args := fake.calls[0].args
	if got, want := args[3], 50.0; got != want {
		t.Fatalf("quote amount want %v, got %v", want, got)
	}
	if got, want := args[10], (pgtype.Timestamptz{Time: now, Valid: true}); got != want {
		t.Fatalf("matched_at want %#v, got %#v", want, got)
	}
}

func TestStoreSaveMatchLogsPersistsAllLogs(t *testing.T) {
	fake := &fakeDBTX{}
	store := NewStore(fake)
	logs := []matchlog.MatchLog{testMatchLog(), testMatchLog()}
	logs[1].TakerOrderID = "bid-2"

	if err := store.SaveMatchLogs(context.Background(), logs); err != nil {
		t.Fatalf("SaveMatchLogs returned error: %v", err)
	}
	if len(fake.calls) != len(logs) {
		t.Fatalf("exec calls want %d, got %d", len(logs), len(fake.calls))
	}
	if got := fake.calls[1].args[5]; got != "bid-2" {
		t.Fatalf("second taker order id want bid-2, got %v", got)
	}
}

func TestStoreSaveMatchLogRejectsInvalidLog(t *testing.T) {
	fake := &fakeDBTX{}
	store := NewStore(fake)
	log := testMatchLog()
	log.Price = 0

	err := store.SaveMatchLog(context.Background(), log)
	if !errors.Is(err, ErrInvalidMatchLog) {
		t.Fatalf("expected ErrInvalidMatchLog, got %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("invalid log should not execute SQL, got %d calls", len(fake.calls))
	}
}
