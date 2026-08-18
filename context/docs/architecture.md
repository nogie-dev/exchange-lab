# Architecture Context

Core flow:

1. API or caller builds an `engine.Event` with a stable command ID.
2. `Router` dispatches by `Ticker` and waits for command completion.
3. One `BookWorker` owns one ticker's `OrderBook`.
4. Worker commits the command to `internal/journal`, then handles it sequentially.
5. `Match` mutates the book and returns residual order plus raw match logs.
6. `BookWorker` waits for `internal/matchlog` commit acknowledgement.
7. Startup replays the journal before opening the HTTP server.

Key files:

- `cmd/server/main.go` wires a sample router and worker.
- `internal/engine/router.go` maps ticker symbols to workers.
- `internal/engine/bookworker.go` owns event handling per ticker.
- `internal/engine/order.go` owns order book mutation.
- `internal/engine/match.go` owns execution logic.
- `internal/matchlog` owns raw match log storage boundaries.
- `internal/journal` owns durable commands and replay decoding.
- `asset/architecture.png` is the current diagram.

Design constraints:

- Keep `Router` unaware of matching details.
- Keep `Match` unaware of DB details.
- Keep `OrderBook` mostly lock-free; single ticker workers provide serialization.
- Reuse existing domain names: `OrderBook`, `BookWorker`, `PriceLevel`, `Bid`, `Ask`.
- Leave ETL, analytics DB, WebSocket streaming, and TimescaleDB-specific features outside this repo for now.

Verify:

- Use `go test ./...` for behavior changes.
- Use file references only for explanation-only answers.
