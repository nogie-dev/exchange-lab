# Session Bootstrap

## Project

- Purpose: minimal CEX trading platform service inside exchange-lab. It provides the least needed for trading: user accounts, money management, market data for charts, and order forwarding.
- Runtime: Go, using the standard `net/http` server.
- Primary entry point: `go run ./cmd/server`.
- Persistence: PostgreSQL through sqlc, following the same pattern as the matching-engine service.

## Relationship to exchange-lab

- This repository is the source of truth for the trading-cex service code.
- exchange-lab consumes it as a git subtree at `services/trading-cex`; sync pull requests are automated via repository dispatch on pushes to `main`.
- exchange-lab's primary goal is ETL over matching-engine data. This service stays minimal: analytics databases, candles pipelines beyond read serving, and data marts belong outside this repository.

## Scope

Owned here:

1. User accounts: registration, authentication, account lookup.
2. Money management: balances, deposits and withdrawals recorded as ledger entries, and reservations held while orders are open.
3. Market data for charts: candles, tickers, and snapshots served read-only.
4. Order gateway: balance checks, reservations, and forwarding commands to the matching-engine internal API.

Out of scope:

- KYC/AML, fiat rails, custody wallets, admin backoffice.
- ETL, analytics databases, data marts (exchange-lab concern).
- Matching logic itself (nogie-dev/matching-engine).

## Architecture at a Glance

Planned boundaries; update this section as real components land.

1. `internal/api` exposes HTTP endpoints for accounts, balances, market data, and order placement.
2. Account handlers authenticate the caller and resolve the acting account.
3. Balance reads come from the ledger; deposits and withdrawals append immutable entries.
4. Order placement validates available balance, creates reservations atomically with the ledger, then forwards the command through `internal/gateway` to matching-engine.
5. Market data handlers serve chart queries from stored candle/ticker tables or proxied matching-engine snapshots.

## Current Surface and Boundaries

The service assumes upstream clients handle presentation and that matching-engine handles price-time-priority execution. Authentication details, custody, settlement, ETL, analytics, and WebSocket delivery are outside current scope.

Implementation status: pre-scaffolding. No runtime code exists yet; this file records target boundaries agreed at project start. Keep every claim here verifiable against code as it lands.

## Sources of Truth

- Behavior and invariants: code and tests.
- Change history and work scope: Git, issues, and pull requests.
- Durable decisions: permanent repository documentation.
- Active work state: `context/work/issue-N/handoff.md` or `context/work/task-SLUG/handoff.md`.

Handoffs are compact leads for resuming work. Verify mutable claims against their original sources. Do not preserve raw command output, chat transcripts, or per-request logs.

## Session Loading

1. Read this file once on the first repository task in a session.
2. Read only the active issue or task handoff when starting or resuming that work.
3. Read `context/route.md` when the task area changes or is unclear.
4. Do not bulk-load historical handoffs.

## Quality Gate

- `go vet ./...`
- `go test ./...`
