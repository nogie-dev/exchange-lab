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
- Matching, cancel, and edit behavior belong in `BookWorker` and `OrderBook`, not `Router`.
- `BookWorker` may forward `MatchResult.Logs` to a `matchlog.Store`.

Current behavior:

- Unknown ticker returns an error.
- Empty ticker returns an error.
- Worker input channel is buffered with size 128.
- Sending to a full worker channel blocks.
- Mismatched payload ticker is logged and skipped by the worker.
- Match log durability policy is not decided yet.

Workflow:

1. Decide whether the request is about dispatch, backpressure, or event handling.
2. Keep routing thin; do not move matching logic into the router.
3. Do not add async batching, retry, drop, or backpressure policy until it is explicitly decided.

Verify:

- `go test ./internal/engine`
