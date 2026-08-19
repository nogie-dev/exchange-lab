# Durable Order Lifecycle Events

Core files:

- `internal/orderevent/event.go`
- `internal/orderevent/postgres/store.go`
- `internal/orderevent/postgres/db`
- `db/migrations/00005_create_order_events.sql`
- `db/migrations/00009_convert_order_events_to_fixed_units.sql`
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

All price and amount fields are fixed-point integer units. The PostgreSQL
columns use `*_price_ticks` and `*_amount_lots` names, and each event carries
`market_config_version`; no floating-point comparison or epsilon is used.

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
acknowledges success or failure. `BookWorker` sends the validated events it
generates after applying each journal command and waits for that acknowledgement
before acknowledging the command or accepting the next command. A persistence
failure halts the shared engine state and prevents subsequent commands from
being accepted.

Startup replay uses the same match-log and order-event persistence path, so a
journaled command with a missing or partially committed event batch can be
reconciled through deterministic event IDs. `cmd/server` starts the order-event
writer, injects it into every ticker worker, and waits for it to drain during
shutdown.

Verify:

- `sqlc generate`
- `go test ./internal/orderevent/...`
- `go test ./...`
- `go vet ./...`
