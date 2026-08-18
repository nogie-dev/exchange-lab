# Durable Order Lifecycle Events

Core files:

- `internal/orderevent/event.go`
- `internal/orderevent/postgres/store.go`
- `internal/orderevent/postgres/db`
- `db/migrations/00005_create_order_events.sql`
- `db/query/order_events.sql`

## Responsibility boundary

The trading platform owns request history before the matching engine makes an
authoritative orderbook decision. Authentication, balance, ownership, request
acceptance, and request rejection are therefore not matching-engine order
events.

The matching engine records only durable order state transitions:

- `ORDER_RESTING`
- `ORDER_PARTIALLY_FILLED`
- `ORDER_FILLED`
- `ORDER_AMENDED`
- `ORDER_CANCELED`
- `ORDER_REMAINDER_CANCELED`

`order_journal` remains the command and orderbook-recovery source.
`match_logs` remains the per-execution source. `order_events` describes the
resulting lifecycle of each affected order for a platform projection.

## Event granularity

State transitions are separate events. For example, a partially filled market
order produces a partial-fill transition followed by a remainder-canceled
transition. It is not stored as one combined terminal event.

Both sides of matching produce lifecycle transitions. An incoming taker event
does not replace the maker event: every maker order whose remaining amount
changes must receive its own partial-fill or filled transition.

`command_sequence` is the durable journal sequence for the ticker.
`event_index` is the deterministic event order within that command. Together
they allow all maker and taker transitions from one command to be ordered.

## Amount semantics

Each event records a transition, not a mutable order row:

- `previous_amount`: active amount immediately before the transition.
- `filled_amount`: amount filled by this transition.
- `canceled_amount`: amount canceled by this transition.
- `remaining_amount`: active amount immediately after the transition.
- `previous_price` and `price`: prices before and after the transition; market
  order prices remain zero.

Fill and cancellation transitions must balance their previous amount. Amend
events instead store the before and after price and amount.

## Idempotency and replay

`event_id` is generated from ticker, command ID, affected order ID, event type,
and event index. Replay must generate the same identity and payload.

The PostgreSQL store writes every event produced by one command in one
transaction. An identical event-ID retry succeeds without inserting another
row. Reusing an event ID with a different payload returns a consistency
conflict. Ambiguous commit outcomes can therefore be retried safely.

## Current implementation stage

The event contract, migration, sqlc queries, and idempotent PostgreSQL store
exist. `OrderBook` returns successful amend and cancel snapshots, `MatchResult`
returns maker quantity transitions, and `BookWorker` builds and validates the
ordered maker and aggregate taker lifecycle events for each journal command.

`orderevent.Writer` consumes command-sized persistence requests in channel
order, stores each request atomically through `Store.SaveEvents`, and
acknowledges success or failure. The worker does not send its generated events
to that writer yet. BookWorker acknowledgment and fail-closed behavior,
startup replay reconciliation, and server writer lifecycle wiring are the next
review stages.

Verify:

- `sqlc generate`
- `go test ./internal/orderevent/...`
- `go test ./...`
- `go vet ./...`
