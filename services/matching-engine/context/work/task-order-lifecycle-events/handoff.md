# Order lifecycle events handoff

Branch: `main`

Related architecture issue: #31 — durable engine events and async projections

Status: complete; order-event persistence is wired through workers, replay,
and server shutdown, verified, and approved for commit

Decisions:

- Platform owns request acceptance and rejection history.
- Matching engine persists only authoritative order state transitions.
- Partial fill and remainder cancellation are separate events.
- Both maker and taker transitions must be emitted.
- The first review stage covers the event contract and idempotent PostgreSQL
  persistence only; engine emission wiring follows after review.

Implemented:

- `orderevent.Event`, transition validation, and deterministic event IDs.
- `00005_create_order_events.sql` and `order_events.sql`.
- sqlc output under `internal/orderevent/postgres/db`.
- Atomic batch persistence, identical retry, conflicting retry, and ambiguous
  commit tests.
- `RemoveOrder` returns an immutable snapshot only for a successful removal.
- `EditOrderAt` distinguishes no-op/failure from successful amendments and
  returns immutable before/after snapshots plus whether price-time rematching
  is required.
- `MatchResult` returns one maker fill transition per execution in the same
  order as its match logs. Each transition contains one immutable maker order
  snapshot plus explicit previous, filled, and remaining quantities; no-match
  results return no transitions.
- `BookWorker.applyCommand` returns validated lifecycle events with stable
  command sequence and event indexes. It aggregates taker fills, keeps maker
  events per affected order, and separates market remainder cancellation.
- Create, price/amount amend, cancel, no-liquidity market, partial maker, and
  no-state-change cases have focused lifecycle tests.
- `orderevent.PersistenceRequest` carries one command-sized event batch with
  buffered acknowledgement, and `orderevent.Writer` stores requests in channel
  order while reporting store, missing-store, and cancellation errors.
- `BookWorker` sends each command-sized lifecycle batch after applying the
  command and waits for the writer acknowledgement before accepting the next
  command. A persistence error halts the router's shared engine state.
- Replay follows the same match-log and order-event persistence path, allowing
  deterministic event IDs and idempotent storage to reconcile journaled
  commands after a failed write or restart.
- `cmd/server` starts both persistence writers, injects both channels into
  every ticker worker, and closes and awaits both writers after all workers
  stop. The order-event queue size is configurable.
- Focused tests cover worker acknowledgement ordering, persistence failure,
  server store wiring, replay idempotency, and restart recovery after a failed
  order-event write.
- A sequential-taker regression test verifies one maker transitions from
  10 to 6 to 3 to 0 across three separate commands.
- Verification passes: `sqlc generate`, `go test ./...`, `go vet ./...`,
  focused `go test -race`, and `git diff --check`.

Deferred:

- Throughput optimization, queue-lag metrics, and benchmark-driven writer
  partitioning. The current baseline intentionally waits synchronously for
  both match-log and order-event durability.
- Issue #31 records the proposed next boundary: keep the durable engine event
  handoff synchronous, then publish at-least-once to asynchronous platform,
  market-data, and ETL projections.
