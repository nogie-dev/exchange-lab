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
- `postgres.Store` appends logs with sqlc-generated pgx queries.
- `match_logs` is plain PostgreSQL schema, not TimescaleDB-specific.

Out of scope:

- WebSocket streaming.
- ETL repository implementation.
- Analytics tables, candles, volume, or data marts.
- Retry, backpressure, drop, async batching, and durability policy.
- TimescaleDB hypertables, compression, and continuous aggregates.

Rules:

- Keep `Match` independent of DB code.
- `BookWorker` may pass logs to the storage boundary.
- Change migration, query, generated sqlc code, store code, and tests together.

Verify:

- `sqlc generate`
- `go test ./internal/matchlog/...`
