-- name: CreateOrderJournalEntry :one
WITH next_sequence AS (
    INSERT INTO order_journal_sequences (ticker, last_sequence)
    VALUES ($2, 1)
    ON CONFLICT (ticker) DO UPDATE
    SET last_sequence = order_journal_sequences.last_sequence + 1
    RETURNING last_sequence
)
INSERT INTO order_journal (
    command_id,
    ticker,
    sequence,
    command_type,
    payload
)
SELECT $1, $2, last_sequence, $3, $4
FROM next_sequence
ON CONFLICT (command_id) DO NOTHING
RETURNING sequence, recorded_at;

-- name: GetOrderJournalEntry :one
SELECT command_id, ticker, sequence, command_type, payload, recorded_at
FROM order_journal
WHERE command_id = $1;

-- name: ListOrderJournalEntries :many
SELECT command_id, ticker, sequence, command_type, payload, recorded_at
FROM order_journal
ORDER BY ticker, sequence;
