# Matching Log Backpressure Considerations

Status: decided by issue #14

This note preserves the tradeoffs considered for raw match log delivery. The
final durability policy lives in `context/docs/matching-log.md`.

## Decision

- Prefer correctness over matching availability: a match is not committed
  until its raw logs are committed to PostgreSQL.
- Keep the writer boundary, but require an explicit commit acknowledgement so
  the affected worker cannot continue after an enqueue-only success.
- Halt order processing on persistence failure or an ambiguous commit result.
- Use a bounded channel only for transport and backpressure. Never treat it as
  durable storage and never silently drop logs.
- Batch only the logs produced by one incoming order in a single transaction.
  Do not add cross-order timer batching before measurements justify it.
- Require stable execution IDs and a uniqueness constraint before retrying.
- Add a durable order journal and deterministic replay as a separate recovery
  capability for process-crash safety.

## Current Direction

- `BookWorker` should not own a `matchlog.Store` or call DB writes directly.
- `BookWorker` sends `MatchResult.Logs` as a persistence request and waits for
  commit acknowledgement.
- A separate match log writer should own `matchlog.Store` and persistence.
- Raw match logs are source-of-truth execution records, not analytics trades.

## Selected Initial Policy

- Use a bounded channel as in-process transport.
- Default channel slot counts are configurable in `config/default.json`.
- Block until PostgreSQL commit acknowledgement instead of dropping logs.
- Treat the channel as an in-process handoff, not durable storage.
- Use shared DB connection pooling; do not dedicate one DB connection per writer.

This favors raw log integrity over ticker-level matching availability. If the
writer or database stalls, the engine fails closed and stops accepting work
until recovery is verified.

## Why Not Drop Or Buffer Forever

- Dropping logs can leave executed matches without source records.
- Unbounded in-memory buffering can hide DB failure until the process runs out
  of memory.
- An in-memory database is not a safe source of truth for executed matches.

## Deferred Implementation Details

- Exact channel capacity and connection pool limits.
- Transactional batch mechanism for one order's logs.
- Retry count, timeout, and operator recovery procedure.
- Whether the first implementation halts one ticker or the whole engine.
- Shutdown drain and restart sequencing.
- Snapshot/checkpoint format and replay performance optimization.

Redis, Kafka, Redpanda, or NATS JetStream are not required while database
failure intentionally stops trading. Reconsider an external durable queue
only if a future requirement allows matching to continue while PostgreSQL is
unavailable.

Implementation was delivered in dependency order by issues #24, #25, and #26.
