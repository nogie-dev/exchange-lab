# Raw Matching Log Context

Core files:

- `internal/matchlog/log.go`
- `internal/matchlog/postgres/store.go`
- `internal/matchlog/postgres/store_test.go`
- `internal/matchlog/postgres/db`
- `db/migrations/00001_create_match_logs.sql`
- `db/query/match_logs.sql`
- `sqlc.yaml`

Responsibilities:

- `matchlog.MatchLog` is the raw engine-emitted execution event.
- `matchlog.Store` is the storage boundary for raw logs.
- `matchlog.Writer` owns persistence requests and sends a commit acknowledgement back to the requesting worker.
- `postgres.Store` appends logs with sqlc-generated pgx queries.
- `match_logs` is plain PostgreSQL schema, not TimescaleDB-specific.

Durability policy:

- A match is not committed or exposed until all of its raw match logs are
  durably committed to PostgreSQL.
- Persist all logs produced by one incoming order in one atomic transaction.
- Use a stable execution ID and a database uniqueness constraint so an
  ambiguous commit result can be retried without creating duplicate matches.
- Treat the match log channel as bounded in-process transport only. Enqueueing
  a log is not a persistence acknowledgement.
- Wait for an explicit persistence acknowledgement before processing the next
  order on the affected worker.
- Halt order processing and reject new commands when persistence fails or the
  commit outcome is unknown. Never drop a log or continue in degraded mode.
- Use shared connection pooling for connection reuse and limits, not as a
  substitute for transactional durability.
- Use the durable order journal and deterministic startup replay to reconstruct
  process state and reconcile missing match logs before serving traffic.

Implementation sequence:

1. Issue #24 adds stable execution IDs, uniqueness, and atomic transactional
   persistence for one order's match logs.
2. Issue #25 adds commit acknowledgement, shared connection-pool wiring, and
   fail-closed engine halt behavior. It depends on #24.
3. Issue #26 adds a durable order journal and deterministic replay recovery.
   It depends on #24 and #25. See `context/docs/order-journal.md`.

Out of scope:

- WebSocket streaming.
- ETL repository implementation.
- Analytics tables, candles, volume, or data marts.
- Cross-order timer batching and unbounded in-memory buffering.
- Continuing trading while PostgreSQL is unavailable.
- PostgreSQL replication, backup, and infrastructure disaster recovery.
- TimescaleDB hypertables, compression, and continuous aggregates.

Rules:

- Keep `Match` independent of DB code.
- Keep `BookWorker` independent of `matchlog.Store`; emit logs through an output channel.
- Do not treat a channel send as durable completion; require a commit acknowledgement.
- Do not retry an ambiguous write until the persisted event has an idempotency key.
- Change migration, query, generated sqlc code, store code, and tests together.

Verify:

- `sqlc generate`
- `go test ./internal/matchlog/...`
