package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nogie-dev/clob-trading/internal/journal"
	"github.com/nogie-dev/clob-trading/internal/journal/postgres/db"
)

type Store struct {
	queries *db.Queries
}

func NewStore(conn db.DBTX) *Store {
	return &Store{queries: db.New(conn)}
}

func (s *Store) Append(ctx context.Context, command journal.Command) (journal.AppendResult, error) {
	payload, err := journal.EncodePayload(command)
	if err != nil {
		return journal.AppendResult{}, err
	}
	row, err := s.queries.CreateOrderJournalEntry(ctx, db.CreateOrderJournalEntryParams{
		CommandID:   command.CommandID,
		Ticker:      command.Ticker,
		CommandType: string(command.Type),
		Payload:     payload,
	})
	if err == nil {
		if row.Sequence <= 0 {
			return journal.AppendResult{}, fmt.Errorf("append journal command %q: invalid sequence", command.CommandID)
		}
		if !row.RecordedAt.Valid {
			return journal.AppendResult{}, fmt.Errorf("append journal command %q: missing recorded time", command.CommandID)
		}
		command.Sequence = row.Sequence
		command.RecordedAt = row.RecordedAt.Time
		return journal.AppendResult{Command: command, Inserted: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return journal.AppendResult{}, fmt.Errorf("append journal command %q: %w", command.CommandID, err)
	}

	existingRow, err := s.queries.GetOrderJournalEntry(ctx, command.CommandID)
	if err != nil {
		return journal.AppendResult{}, fmt.Errorf("read existing journal command %q: %w", command.CommandID, err)
	}
	existing, err := decodeRow(existingRow)
	if err != nil {
		return journal.AppendResult{}, err
	}
	if !journal.SamePayload(command, existing) {
		return journal.AppendResult{}, fmt.Errorf("%w: command id %q", journal.ErrCommandConflict, command.CommandID)
	}
	return journal.AppendResult{Command: existing, Inserted: false}, nil
}

func (s *Store) List(ctx context.Context) ([]journal.Command, error) {
	rows, err := s.queries.ListOrderJournalEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list order journal: %w", err)
	}
	commands := make([]journal.Command, len(rows))
	for i, row := range rows {
		commands[i], err = decodeRow(row)
		if err != nil {
			return nil, fmt.Errorf("decode order journal row %d: %w", i, err)
		}
	}
	return commands, nil
}

func decodeRow(row db.OrderJournal) (journal.Command, error) {
	if row.Sequence <= 0 {
		return journal.Command{}, fmt.Errorf("%w: sequence must be positive", journal.ErrInvalidCommand)
	}
	if !row.RecordedAt.Valid {
		return journal.Command{}, fmt.Errorf("%w: recorded time is required", journal.ErrInvalidCommand)
	}
	return journal.Decode(
		row.CommandID,
		row.Ticker,
		row.Sequence,
		journal.CommandType(row.CommandType),
		row.Payload,
		row.RecordedAt.Time,
	)
}
