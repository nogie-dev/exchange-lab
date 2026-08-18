# Durable Order Journal Context

Core files:

- `internal/journal/journal.go`
- `internal/journal/postgres/store.go`
- `internal/journal/postgres/db`
- `db/migrations/00003_create_order_journal.sql`
- `db/query/order_journal.sql`
- `internal/engine/bookworker.go`
- `cmd/server/main.go`

## Invariants

- Every create, amend, and cancel command has an upstream-supplied stable
  `command_id`.
- PostgreSQL commits the append-only command before `BookWorker` changes its
  orderbook.
- Each ticker receives a monotonically increasing database sequence. Gaps are
  allowed after idempotent retries; reordering and duplicate sequence values
  are not.
- Reusing a command ID with the same payload is an idempotent retry. Reusing it
  with a different payload is a fatal consistency error.
- The database-recorded command time is used for live execution and replay, so
  generated order timestamps and match execution IDs remain deterministic.
- API command success is returned only after journal append, command handling,
  and any match-log commit acknowledgement complete.

## Startup Recovery

1. Open and ping the shared PostgreSQL pool.
2. Start the match-log writer while the HTTP server is still closed.
3. Load journal rows ordered by ticker and sequence.
4. Rebuild each orderbook from an empty state using the stored command time.
5. Persist replayed match logs through the idempotent transactional store.
   Existing execution IDs remain single rows; missing rows are inserted.
6. Start workers and the HTTP server only after the complete replay succeeds.

Any journal decode, sequence, command conflict, or match-log persistence error
fails startup. Readiness and order intake never become available in that state.

## Operator Recovery

1. Keep the halted process out of service and do not attempt in-process resume.
2. Preserve PostgreSQL state and verify connectivity and required migrations.
3. Resolve the underlying database or consistency error without deleting or
   rewriting journal or match-log rows.
4. Restart exactly one engine instance for the journal ownership domain.
5. Wait for startup replay and match-log reconciliation to finish.
6. Return the instance to service only after `GET /ready` returns `200`.

If startup reports a reused command/execution ID with different payload, keep
trading stopped and investigate the upstream identity source and database rows.
Do not bypass the conflict or auto-resume.

## Deferred

- Snapshot/checkpoint optimization.
- Journal compaction, retention, and archive.
- Multi-region disaster recovery.
- Continuing matching while PostgreSQL is unavailable.

Verify:

- `sqlc generate`
- `go test ./internal/journal/... ./internal/engine ./cmd/server`
- `go vet ./...`
- `go test ./...`
