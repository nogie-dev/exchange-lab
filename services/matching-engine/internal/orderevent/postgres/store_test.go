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
	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/numeric"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

type execCall struct {
	sql  string
	args []interface{}
}

type fakeDatabase struct {
	rows                  map[string][]interface{}
	calls                 []execCall
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

	eventID, _ := args[0].(string)
	if _, exists := f.rows[eventID]; exists {
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	f.rows[eventID] = append([]interface{}(nil), args...)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (f *fakeTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("query not implemented in fakeTx")
}

func (f *fakeTx) QueryRow(_ context.Context, _ string, args ...interface{}) pgx.Row {
	eventID, _ := args[0].(string)
	stored, ok := f.rows[eventID]
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

func testOrderEvent() orderevent.Event {
	event := orderevent.Event{
		CommandID:       "taker-command",
		CommandSequence: 2,
		EventIndex:      0,
		OrderID:         "maker-order",
		UserID:          "maker-user",
		Ticker:          "BTC-USD",
		Type:            orderevent.PartiallyFilled,
		Reason:          orderevent.ReasonPartialFill,
		OrderType:       models.Limit,
		Side:            models.Ask,
		PreviousPrice:   numeric.MustPrice("100"),
		Price:           numeric.MustPrice("100"),
		PreviousAmount:  numeric.MustQuantity("1"),
		FilledAmount:    numeric.MustQuantity("0.4"),
		RemainingAmount: numeric.MustQuantity("0.6"),
		OccurredAt:      time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
	}
	event.EventID = orderevent.GenerateEventID(event.Ticker, event.CommandID, event.OrderID, event.Type, event.EventIndex)
	return event
}

func TestStoreSaveEventExecutesTransactionalInsert(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	event := testOrderEvent()

	if err := store.SaveEvent(context.Background(), event); err != nil {
		t.Fatalf("SaveEvent returned error: %v", err)
	}
	if database.begins != 1 || database.commits != 1 || database.rollbacks != 0 {
		t.Fatalf("transaction counts want begin=1 commit=1 rollback=0, got %d/%d/%d", database.begins, database.commits, database.rollbacks)
	}
	if len(database.calls) != 1 || !strings.Contains(database.calls[0].sql, "INSERT INTO order_events") {
		t.Fatalf("expected one order_events insert, got %#v", database.calls)
	}
}

func TestStoreSaveEventsRollsBackWholeBatch(t *testing.T) {
	database := newFakeDatabase()
	database.execErr = errors.New("insert failed")
	database.execErrAt = 2
	store := newTestStore(database)
	events := []orderevent.Event{testOrderEvent(), testOrderEvent()}
	events[1].EventIndex = 1
	events[1].OrderID = "taker-order"
	events[1].UserID = "taker-user"
	events[1].EventID = orderevent.GenerateEventID(events[1].Ticker, events[1].CommandID, events[1].OrderID, events[1].Type, events[1].EventIndex)

	if err := store.SaveEvents(context.Background(), events); err == nil {
		t.Fatal("SaveEvents should return the insert error")
	}
	if len(database.rows) != 0 || database.commits != 0 || database.rollbacks != 1 {
		t.Fatalf("failed batch must roll back: rows=%d commits=%d rollbacks=%d", len(database.rows), database.commits, database.rollbacks)
	}
}

func TestStoreSaveEventsAllowsIdenticalRetry(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	event := testOrderEvent()

	if err := store.SaveEvent(context.Background(), event); err != nil {
		t.Fatalf("first SaveEvent returned error: %v", err)
	}
	if err := store.SaveEvent(context.Background(), event); err != nil {
		t.Fatalf("retry SaveEvent returned error: %v", err)
	}
	if len(database.rows) != 1 {
		t.Fatalf("identical retry must keep one row, got %d", len(database.rows))
	}
}

func TestStoreSaveEventsRejectsConflictingRetry(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	event := testOrderEvent()

	if err := store.SaveEvent(context.Background(), event); err != nil {
		t.Fatalf("first SaveEvent returned error: %v", err)
	}
	conflicting := event
	conflicting.Reason = "DIFFERENT_REASON"
	err := store.SaveEvent(context.Background(), conflicting)
	if !errors.Is(err, ErrOrderEventConflict) {
		t.Fatalf("SaveEvent want ErrOrderEventConflict, got %v", err)
	}
}

func TestStoreSaveEventRetriesAmbiguousCommitWithoutDuplicates(t *testing.T) {
	database := newFakeDatabase()
	database.commitErr = errors.New("commit result unknown")
	database.commitErrorsRemaining = 1
	database.commitWritesOnError = true
	store := newTestStore(database)
	event := testOrderEvent()

	if err := store.SaveEvent(context.Background(), event); err == nil {
		t.Fatal("first SaveEvent should report the ambiguous commit error")
	}
	if err := store.SaveEvent(context.Background(), event); err != nil {
		t.Fatalf("retry SaveEvent returned error: %v", err)
	}
	if len(database.rows) != 1 {
		t.Fatalf("ambiguous commit retry must keep one row, got %d", len(database.rows))
	}
}

func TestStoreSaveEventRejectsInvalidEventBeforeTransaction(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	event := testOrderEvent()
	event.EventID = "wrong-event-id"

	err := store.SaveEvent(context.Background(), event)
	if !errors.Is(err, orderevent.ErrInvalidEvent) {
		t.Fatalf("SaveEvent want ErrInvalidEvent, got %v", err)
	}
	if database.begins != 0 {
		t.Fatalf("invalid event should not begin a transaction, got %d", database.begins)
	}
}

func TestStoreSaveEventsIgnoresEmptyBatch(t *testing.T) {
	database := newFakeDatabase()
	store := newTestStore(database)
	if err := store.SaveEvents(context.Background(), nil); err != nil {
		t.Fatalf("empty SaveEvents returned error: %v", err)
	}
	if database.begins != 0 {
		t.Fatalf("empty batch should not begin a transaction, got %d", database.begins)
	}
}
