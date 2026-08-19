---
work_item: issue-32
branch: issue/32-numeric-model
status: complete
updated: 2026-08-19
issue_or_pr: https://github.com/nogie-dev/matching-engine/issues/32
---

# Work Handoff

## Objective

Replace floating-point price and quantity handling with versioned fixed-point
integer units across the matching-engine API, journal, orderbook, matching,
events, raw logs, and PostgreSQL persistence.

## Completed

- Added `numeric.PriceTicks`, `numeric.QuantityLots`, and `numeric.QuoteAtoms`.
- Added strict decimal-string parsing, tick/lot validation, formatting, and
  checked quote calculation.
- Changed HTTP order inputs to decimal strings and orderbook outputs to
  decimal strings.
- Migrated orderbook maps/heaps, matching arithmetic, lifecycle events, and
  raw match logs to integer units.
- Added market precision metadata and `market_config_version` propagation.
- Added migrations `00006` through `00009` and regenerated sqlc output.
- Updated focused tests and documentation.

## Remaining before merge

- Commit and push the matching-engine changes after GitHub CLI authentication
  is restored; leave the unrelated repository-level `infra/` files untouched.
- Run the Goose up/down migration smoke test against a disposable PostgreSQL
  instance. The local Docker daemon is unavailable in this environment.
- Review the production backfill from the legacy floating-point columns before
  applying migrations to non-local data.

## Decisions

- Initial market default uses price, quantity, and quote scale 8, with unit
  tick/lot sizes and config version 1.
- Raw quote atoms are rounded down at quote scale; settlement and quote-budget
  market-buy policy remain outside issue #32.
- Core arithmetic uses `int64`; `big.Int` is used only to check quote
  multiplication before storing `QuoteAtoms`.

## Risks or Blockers

- Existing floating-point rows are backfilled with the initial `10^8` scales;
  production data migration must be reviewed before deployment.
- The repository's default Go build cache is not writable in this environment;
  verification uses `GOCACHE=/private/tmp/exchange-lab-go-cache`.

## Relevant Files

- `internal/numeric/numeric.go`
- `internal/api/handler.go`
- `internal/engine/order.go`
- `internal/engine/match.go`
- `internal/engine/bookworker.go`
- `internal/journal/journal.go`
- `internal/market/market.go`
- `db/migrations/00006_add_market_precision.sql`
- `db/migrations/00007_add_journal_config_version.sql`
- `db/migrations/00008_convert_match_logs_to_fixed_units.sql`
- `db/migrations/00009_convert_order_events_to_fixed_units.sql`

## Verification

- `sqlc generate` passed.
- `GOCACHE=/private/tmp/exchange-lab-go-cache go test ./...` passed.
- `GOCACHE=/private/tmp/exchange-lab-go-cache go test -race ./...` passed.
- `GOCACHE=/private/tmp/exchange-lab-go-cache go vet ./...` passed.
- `GOCACHE=/private/tmp/exchange-lab-go-cache go test ./internal/testdata` passed.
- `git diff --check` passed.

## Next Action

Run the migration smoke test once PostgreSQL/Docker is available, then review
the four new migration files before opening a pull request.
