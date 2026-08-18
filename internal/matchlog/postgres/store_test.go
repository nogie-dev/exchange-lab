package postgres

import (
	"context"
	"errors"
	"reflect"
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

type fakeDatabase struct {
	rows                  map[string][]interface{}
	calls                 []execCall
	beginErr              error
	execErr               error
	execErrAt             int
	commitErr             error
	commitErrorsRemaining int
	commitWritesOnError   bool
	begins                int
	commits               int
	rollbacks             int
}

func newFakeDatabase() *fakeDatabase {
	return &fakeDatabase{rows: make(map[string][]interface{})}
}

func (f *fakeDatabase) begin(context.Context) (transaction, error) {
	f.begins++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return &fakeTx{database: f, rows: cloneRows(f.rows)}, nil
}

type fakeTx struct {
	database  *fakeDatabase
	rows      map[string][]interface{}
	execCount int
	closed    bool
}

func (f *fakeTx) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.execCount++
	f.database.calls = append(f.database.calls, execCall{sql: sql, args: args})
	if f.database.execErr != nil && f.execCount == f.database.execErrAt {
		return pgconn.CommandTag{}, f.database.execErr
	}

	executionID, _ := args[0].(string)
	if _, exists := f.rows[executionID]; exists {
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	f.rows[executionID] = append([]interface{}(nil), args...)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (f *fakeTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("query not implemented in fakeTx")
}

func (f *fakeTx) QueryRow(_ context.Context, _ string, args ...interface{}) pgx.Row {
	executionID, _ := args[0].(string)
	stored, ok := f.rows[executionID]
	if !ok {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return fakeRow{matches: reflect.DeepEqual(stored, args)}
}

func (f *fakeTx) Commit(context.Context) error {
	if f.closed {
		return pgx.ErrTxClosed
	}
	f.closed = true
	f.database.commits++

	if f.database.commitErrorsRemaining > 0 {
		f.database.commitErrorsRemaining--
		if f.database.commitWritesOnError {
			f.database.rows = cloneRows(f.rows)
		}
		return f.database.commitErr
	}
	f.database.rows = cloneRows(f.rows)
	return nil
}

func (f *fakeTx) Rollback(context.Context) error {
	if f.closed {
		return pgx.ErrTxClosed
	}
	f.closed = true
	f.database.rollbacks++
	return nil
}

type fakeRow struct {
	matches bool
	err     error
}

func (f fakeRow) Scan(dest ...interface{}) error {
	if f.err != nil {
		return f.err
	}
	value, ok := dest[0].(*pgtype.Bool)
	if !ok {
		return errors.New("unexpected scan destination")
	}
	*value = pgtype.Bool{Bool: f.matches, Valid: true}
	return nil
}

func cloneRows(rows map[string][]interface{}) map[string][]interface{} {
	cloned := make(map[string][]interface{}, len(rows))
	for id, row := range rows {
		cloned[id] = append([]interface{}(nil), row...)
	}
	return cloned
}

func newTestStore(database *fakeDatabase) *Store {
	return &Store{begin: database.begin}
}

func testMatchLog() matchlog.MatchLog {
	return matchlog.MatchLog{
		ExecutionID:  "execution-1",
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

func TestStoreSaveMatchLogExecutesTransactionalInsert(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	log := testMatchLog()

	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("SaveMatchLog returned error: %v", err)
	}

	if database.begins != 1 || database.commits != 1 || database.rollbacks != 0 {
		t.Fatalf("transaction counts want begin=1 commit=1 rollback=0, got %d/%d/%d", database.begins, database.commits, database.rollbacks)
	}
	if len(database.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(database.calls))
	}
	call := database.calls[0]
	if !strings.Contains(call.sql, "INSERT INTO match_logs") {
		t.Fatalf("expected INSERT INTO match_logs query, got %q", call.sql)
	}

	want := []interface{}{
		log.ExecutionID,
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
	if !reflect.DeepEqual(call.args, want) {
		t.Fatalf("insert args want %#v, got %#v", want, call.args)
	}
}

func TestStoreSaveMatchLogDerivesQuoteAmount(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	log := testMatchLog()
	log.QuoteAmount = 0

	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("SaveMatchLog returned error: %v", err)
	}

	args := database.calls[0].args
	if got, want := args[4], 50.0; got != want {
		t.Fatalf("quote amount want %v, got %v", want, got)
	}
	if got, want := args[11], (pgtype.Timestamptz{Time: log.MatchedAt, Valid: true}); got != want {
		t.Fatalf("matched_at want %#v, got %#v", want, got)
	}
}

func TestStoreSaveMatchLogsRollsBackWholeBatch(t *testing.T) {
	database := newFakeDatabase()
	database.execErr = errors.New("insert failed")
	database.execErrAt = 2
	store := newTestStore(database)
	logs := []matchlog.MatchLog{testMatchLog(), testMatchLog()}
	logs[1].ExecutionID = "execution-2"
	logs[1].TakerOrderID = "bid-2"

	if err := store.SaveMatchLogs(context.Background(), logs); err == nil {
		t.Fatal("SaveMatchLogs should return the insert error")
	}
	if len(database.rows) != 0 {
		t.Fatalf("rolled back batch must persist no rows, got %d", len(database.rows))
	}
	if database.commits != 0 || database.rollbacks != 1 {
		t.Fatalf("transaction counts want commit=0 rollback=1, got %d/%d", database.commits, database.rollbacks)
	}
}

func TestStoreSaveMatchLogsAllowsIdenticalRetry(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	logs := []matchlog.MatchLog{testMatchLog(), testMatchLog()}
	logs[1].ExecutionID = "execution-2"
	logs[1].TakerOrderID = "bid-2"

	if err := store.SaveMatchLogs(context.Background(), logs); err != nil {
		t.Fatalf("first SaveMatchLogs returned error: %v", err)
	}
	if err := store.SaveMatchLogs(context.Background(), logs); err != nil {
		t.Fatalf("retry SaveMatchLogs returned error: %v", err)
	}
	if len(database.rows) != len(logs) {
		t.Fatalf("identical retry must keep %d rows, got %d", len(logs), len(database.rows))
	}
}

func TestStoreSaveMatchLogsRejectsConflictingRetry(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	log := testMatchLog()

	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("first SaveMatchLog returned error: %v", err)
	}
	conflicting := log
	conflicting.Amount = 0.75
	conflicting.QuoteAmount = 75

	err := store.SaveMatchLog(context.Background(), conflicting)
	if !errors.Is(err, ErrMatchLogConflict) {
		t.Fatalf("expected ErrMatchLogConflict, got %v", err)
	}
	if got := database.rows[log.ExecutionID][3]; got != log.Amount {
		t.Fatalf("conflicting retry changed stored amount: want %v, got %v", log.Amount, got)
	}
}

func TestStoreSaveMatchLogsRetriesAmbiguousCommitWithoutDuplicates(t *testing.T) {
	database := newFakeDatabase()
	database.commitErr = errors.New("commit result unknown")
	database.commitErrorsRemaining = 1
	database.commitWritesOnError = true
	store := newTestStore(database)
	log := testMatchLog()

	if err := store.SaveMatchLog(context.Background(), log); err == nil {
		t.Fatal("first SaveMatchLog should report the ambiguous commit error")
	}
	if err := store.SaveMatchLog(context.Background(), log); err != nil {
		t.Fatalf("retry SaveMatchLog returned error: %v", err)
	}
	if len(database.rows) != 1 {
		t.Fatalf("ambiguous commit retry must keep one row, got %d", len(database.rows))
	}
}

func TestStoreSaveMatchLogRejectsInvalidLogBeforeTransaction(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	log := testMatchLog()
	log.ExecutionID = ""

	err := store.SaveMatchLog(context.Background(), log)
	if !errors.Is(err, ErrInvalidMatchLog) {
		t.Fatalf("expected ErrInvalidMatchLog, got %v", err)
	}
	if database.begins != 0 {
		t.Fatalf("invalid log should not begin a transaction, got %d", database.begins)
	}
}

func TestStoreSaveMatchLogRejectsMissingMatchedAt(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	log := testMatchLog()
	log.MatchedAt = time.Time{}

	err := store.SaveMatchLog(context.Background(), log)
	if !errors.Is(err, ErrInvalidMatchLog) {
		t.Fatalf("expected ErrInvalidMatchLog, got %v", err)
	}
	if database.begins != 0 {
		t.Fatalf("invalid log should not begin a transaction, got %d", database.begins)
	}
}

func TestStoreSaveMatchLogsIgnoresEmptyBatch(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)

	if err := store.SaveMatchLogs(context.Background(), nil); err != nil {
		t.Fatalf("empty SaveMatchLogs returned error: %v", err)
	}
	if database.begins != 0 {
		t.Fatalf("empty batch should not begin a transaction, got %d", database.begins)
	}
}
