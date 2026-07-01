# Order Book Context

Core files:

- `internal/engine/order.go`
- `internal/engine/order_test.go`
- `internal/util/price_level.go`
- `internal/util/min_heap.go`
- `internal/util/max_heap.go`
- `internal/util/queue.go`

Invariants:

- `OrderBook.Bids` and `OrderBook.Asks` map price to `PriceLevel`.
- `bidLevels` is a max heap; `askLevels` is a min heap.
- `OrderBook.Index` maps `orderID` to the queue element for O(1) lookup.
- Each `PriceLevel.Queue` preserves FIFO within the same price.
- `PriceLevel.TotalAmount` must equal the live order amount at that level.
- Empty price levels must leave both the heap and side map.

Edit rules:

- Price change resets priority.
- Amount increase resets priority.
- Amount decrease keeps queue position.

Workflow:

1. Trace add, remove, edit, and match effects on heap, map, queue, and index.
2. Prefer fixing shared helpers such as `removeElement` or `dropPriceLevel`.
3. Add one focused regression test for each broken invariant.

Verify:

- `go test ./internal/engine`
