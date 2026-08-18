# Session Bootstrap

## Project

- Purpose: Go CLOB matching engine with an internal command/query HTTP API.
- Runtime: Go, using the standard `net/http` server.
- Primary entry point: `go run ./cmd/server`.
- Default wiring: `cmd/server/main.go` requires `MATCHING_ENGINE_DATABASE_URL`, opens one shared PostgreSQL pool, registers one `BTC-USD` worker, and listens on `:8080`.

## Architecture at a Glance

1. `internal/api` accepts create, amend, and cancel commands plus ticker-specific orderbook snapshot queries.
2. API handlers translate requests into `engine.Event` values and send them through `engine.Router`.
3. `Router` maps each ticker to one `BookWorker`. Router sends to the worker input channel are blocking; a full worker channel applies backpressure to the caller.
4. Each `BookWorker` owns one ticker's `OrderBook` and processes commands sequentially. Order creation and amendment can invoke `Match`; cancellation and snapshots also run on the worker-owned path.
5. `OrderBook` and `Match` implement price-time-priority matching and return raw match logs without depending on persistence code.
6. `internal/matchlog` defines the raw append-only storage boundary, persistence request/commit acknowledgement, writer, and PostgreSQL store. One incoming order's logs are committed atomically and identical execution IDs are retry-safe.
7. Every accepted command is committed to the PostgreSQL order journal before mutation. Startup replays ticker-local sequences and reconciles match logs before the HTTP server opens.
8. `BookWorker` blocks on persistence acknowledgement before continuing. A journal or match-log error permanently halts the shared engine state, rejects new commands, and makes `GET /ready` unhealthy; recovery requires a clean restart and successful replay.

## Current Surface and Boundaries

Implemented HTTP operations are create, amend, and cancel order commands and ticker-specific orderbook snapshots. The engine assumes upstream systems handle authentication, account ownership, balances, reservations, and public-facing validation.

Authentication, balances, custody, settlement, ETL, analytics, candles, volume, data marts, and WebSocket delivery are outside the current repository scope.

## Sources of Truth

- Behavior and invariants: code and tests.
- Change history and work scope: Git, issues, and pull requests.
- Durable decisions: permanent repository documentation.
- Active work state: `context/work/issue-N/handoff.md` or `context/work/task-SLUG/handoff.md`.

Handoffs are compact leads for resuming work. Verify mutable claims against their original sources. Do not preserve raw command output, chat transcripts, or per-request logs.

## Session Loading

1. Read this file once on the first repository task in a session.
2. Read only the active issue or task handoff when starting or resuming that work.
3. Read `context/route.md` when the task area changes or is unclear.
4. Do not bulk-load historical handoffs.

## Quality Gate

- `go vet ./...`
- `go test ./...`
