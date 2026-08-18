# Matching Engine Context

Core files:

- `internal/engine/match.go`
- `internal/engine/match_test.go`
- `internal/engine/order.go`
- `internal/models/order.go`

Invariants:

- Limit bids match while incoming bid price is greater than or equal to best ask.
- Limit asks match while incoming ask price is less than or equal to best bid.
- Market orders have no price limit and consume FIFO from each best opposite
  price level until filled or the opposite book is empty.
- A market-order residual is canceled by `BookWorker`; it never rests on the
  orderbook. A limit-order residual rests normally.
- Fill price comes from the resting order's price level.
- Matching consumes FIFO from the best opposite price level.
- Filled resting orders must be removed from both queue and `OrderBook.Index`.
- Empty price levels must be removed from heap and side map.
- A non-filled incoming order is returned as `MatchResult.Residual`; fully filled orders have nil residual. The caller owns the residual policy.
- Each execution appends one raw `matchlog.MatchLog` to `MatchResult.Logs`.

Workflow:

1. Reproduce the scenario with a focused `internal/engine` test.
2. Fix `Match` or the shared order book helper, not one caller.
3. Keep raw log generation inside `MatchResult`; keep DB persistence outside `Match`.

Verify:

- `go test ./internal/engine`
