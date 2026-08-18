# Router and Worker Context

Core files:

- `internal/engine/router.go`
- `internal/engine/bookworker.go`
- `internal/engine/event.go`
- `cmd/server/main.go`

Responsibilities:

- `Router` maps `Ticker` to `BookWorker`.
- `Router.OrderRouter` validates ticker presence, finds the worker, and sends the event.
- `BookWorker` validates payload ticker consistency and handles event types.
- `BookWorker` appends validated commands to the durable journal before any
  orderbook mutation.
- Matching, cancel, and edit behavior belong in `BookWorker` and `OrderBook`, not `Router`.
- `BookWorker` sends `MatchResult.Logs` as a persistence request and waits for
  its commit acknowledgement before processing another command.

Current behavior:

- Unknown ticker returns an error.
- Empty ticker returns an error.
- Worker input channel defaults to buffer size 128 and can be changed through
  `engine.worker_input_buffer_size` in `config/default.json`.
- Sending to a full worker channel blocks.
- Mismatched payload ticker is logged and skipped by the worker.
- Persistence failure halts the shared engine state. The router rejects new
  create, amend, and cancel commands while snapshots remain available for
  inspection.
- Router shutdown rejects new work, closes worker queues, and waits for them to
  drain before the writer channel and PostgreSQL pool are closed.
- `Router.OrderRouter` waits for the worker's command result, so success means
  journal and match-log commit acknowledgement have completed.

Workflow:

1. Decide whether the request is about dispatch, backpressure, or event handling.
2. Keep routing thin; do not move matching logic into the router.
3. Preserve blocking commit acknowledgement and fail-closed behavior; never
   replace it with drop or unbounded buffering.

Verify:

- `go test ./internal/engine`
