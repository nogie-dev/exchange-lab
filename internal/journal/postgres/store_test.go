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
	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/models"
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

func testCommand() journal.Command {
	return journal.Command{
		CommandID: "command-1",
		Ticker:    "BTC-USD",
		Type:      journal.CreateCommand,
		Create: &models.CreateOrderRequest{
			CommandID: "command-1",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Limit,
			Position:  models.Bid,
			Price:     100,
			Amount:    1,
			Nonce:     1,
		},
	}
}

func TestStoreAppendReturnsAssignedSequenceAndTime(t *testing.T) {
	recordedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := NewStore(&fakeDBTX{createValues: []any{
		int64(3), pgtype.Timestamptz{Time: recordedAt, Valid: true},
	}})

	result, err := store.Append(context.Background(), testCommand())
	if err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if !result.Inserted || result.Command.Sequence != 3 || result.Command.RecordedAt != recordedAt {
		t.Fatalf("unexpected append result: %#v", result)
	}
}

func TestStoreAppendAllowsIdenticalRetry(t *testing.T) {
	command := testCommand()
	payload, err := journal.EncodePayload(command)
	if err != nil {
		t.Fatalf("EncodePayload returned error: %v", err)
	}
	recordedAt := pgtype.Timestamptz{Time: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC), Valid: true}
	store := NewStore(&fakeDBTX{
		createErr: pgx.ErrNoRows,
		existing:  []any{command.CommandID, command.Ticker, int64(1), string(command.Type), payload, recordedAt},
	})

	result, err := store.Append(context.Background(), command)
	if err != nil {
		t.Fatalf("Append retry returned error: %v", err)
	}
	if result.Inserted || result.Command.Sequence != 1 {
		t.Fatalf("unexpected retry result: %#v", result)
	}
}

func TestStoreAppendRejectsConflictingRetry(t *testing.T) {
	command := testCommand()
	conflicting := testCommand()
	conflicting.Create.Amount = 2
	payload, err := journal.EncodePayload(conflicting)
	if err != nil {
		t.Fatalf("EncodePayload returned error: %v", err)
	}
	recordedAt := pgtype.Timestamptz{Time: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC), Valid: true}
	store := NewStore(&fakeDBTX{
		createErr: pgx.ErrNoRows,
		existing:  []any{command.CommandID, command.Ticker, int64(1), string(command.Type), payload, recordedAt},
	})

	_, err = store.Append(context.Background(), command)
	if !errors.Is(err, journal.ErrCommandConflict) {
		t.Fatalf("Append conflict want ErrCommandConflict, got %v", err)
	}
}
