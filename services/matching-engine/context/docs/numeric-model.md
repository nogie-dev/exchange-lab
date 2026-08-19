# Fixed-Point Numeric Model

Issue #32 defines the numeric boundary for the matching engine.

## Boundary

- HTTP order requests carry `price` and `amount` as decimal strings.
- The handler parses each value using the registered market precision and
  validates tick and lot alignment.
- The journal payload, orderbook, matching loop, lifecycle events, and raw
  execution logs use integer units only:
  - `numeric.PriceTicks`
  - `numeric.QuantityLots`
  - `numeric.QuoteAtoms`
- Orderbook query responses are formatted back to decimal strings using the
  same market precision.

The engine does not parse floats and does not use epsilon comparisons. The
canonical journal payload is therefore stable for equivalent inputs such as
`"100"` and `"100.00"`.

## Market configuration

Each market stores an immutable numeric configuration version with:

- price, quantity, and quote decimal scales;
- tick and lot sizes in scaled integer units;
- minimum and maximum price and quantity units;
- `config_version` used to reject replay under the wrong numeric contract.

The initial local default is eight decimal places for price, quantity, and
quote values, with one-unit tick and lot sizes. New market rules must be
introduced as a new configuration version rather than silently changing the
meaning of existing journal rows.

## Quote amount

Execution quote atoms are derived from price ticks and quantity lots with
checked integer arithmetic. Positive results are rounded down at the quote
scale. Account reservation, settlement, fees, and quote-budget market-buy
policy remain platform/ledger responsibilities and are outside issue #32.

## Storage and migration

Raw journal and event/execution tables store `BIGINT` units and
`market_config_version`. Migrations `00006` through `00009` add market
precision, journal versioning, and fixed-unit raw columns. Existing local
floating-point rows are backfilled using the initial `10^8` scales before the
legacy columns are removed.

Verify with:

- `sqlc generate`
- `go test ./...`
- `go vet ./...`
