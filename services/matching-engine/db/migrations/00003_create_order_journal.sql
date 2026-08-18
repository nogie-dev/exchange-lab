-- +goose Up
CREATE TABLE order_journal_sequences (
    ticker TEXT PRIMARY KEY,
    last_sequence BIGINT NOT NULL CHECK (last_sequence > 0)
);

CREATE TABLE order_journal (
    command_id TEXT PRIMARY KEY,
    ticker TEXT NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    command_type TEXT NOT NULL CHECK (command_type IN ('CREATE', 'AMEND', 'CANCEL')),
    payload JSONB NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ticker, sequence)
);

-- +goose Down
DROP TABLE IF EXISTS order_journal;
DROP TABLE IF EXISTS order_journal_sequences;
