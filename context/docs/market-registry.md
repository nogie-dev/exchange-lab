# Market Registry Context

The `markets` table is the durable registry of tickers that the engine should
restore at startup. It is separate from `order_journal` and `match_logs`:

- `markets` records that a ticker exists.
- `order_journal` records commands needed to rebuild its orderbook.
- `match_logs` records executions produced while replaying or processing those
  commands.

`POST /commands/tickers/add` writes the ticker to `markets` before creating its
worker. The worker then loads the ticker's journal commands, replays them, and
registers with the router. A ticker registration is therefore recoverable even
when it has no orders yet.

Startup loads registered markets and journal rows, takes their union, and
restores one worker per ticker. Journal-only tickers are backfilled into
`markets` for compatibility with rows written before the registry existed.

The current table intentionally stores only ticker identity and creation time.
Market status, price/amount precision, fees, and removal lifecycle remain
follow-up market-catalog concerns.
