# Repository Guidelines

## Session Context

On the first repository task in a session, read `context/bootstrap.md` once. When starting or resuming persistent work, read only the matching `context/work/issue-N/handoff.md` (or `task-SLUG` when no issue exists). Treat handoffs as leads and verify mutable claims against code, tests, Git, issues, and pull requests.

Do not bulk-load historical handoffs or keep raw command logs. Update a handoff only when work starts, pauses, transfers, changes direction materially, or completes. Use the issue or task ID as the stable directory key and keep the branch name as metadata.

## Agent Routing

Read `context/route.md` when the task area changes or is unclear, then choose the first matching route and follow its context files, workflow, and stop condition. Reuse the selected route during the same work cycle. If no route fits, inspect the repository first and take the smallest reversible action.

## GitHub Issue Management

When creating, inspecting, editing, triaging, or linking GitHub issues, use the `github-issue-manager` skill and follow `context/docs/github-issues.md`.

## Project Structure & Module Organization

This is a minimal CEX trading platform service inside exchange-lab. It owns user accounts, balances and money management, market data for chart display, and forwarding orders to the matching-engine service. The runnable entry point is `cmd/server`. The HTTP boundary lives in `internal/api`. User accounts and authentication live in `internal/account`, money and balance handling with ledger entries and order reservations in `internal/balance`, candles/tickers/snapshot serving for charts in `internal/marketdata`, and order forwarding to the matching engine in `internal/gateway`. SQL migrations are in `db/migrations`, hand-written queries in `db/query`, and sqlc output lives under each PostgreSQL persistence package. Keep compact agent context in `context/`.

## Build, Test, and Development Commands

- `go run ./cmd/server` starts the local server entry point.
- `go test ./...` runs all unit and repository tests.
- `go vet ./...` runs Go static checks used by CI.
- `gofmt -w $(find . -name '*.go' -not -path './tmp/*')` formats Go files.
- `sqlc generate` regenerates PostgreSQL query code from `sqlc.yaml`.

CI runs formatting, vet, and tests on pushes to `main` and pull requests.

## Coding Style & Naming Conventions

Use standard Go formatting and idioms. Package names should stay short and lowercase (`account`, `balance`, `marketdata`, `gateway`). Export only APIs needed across packages; keep helpers private when they are package-local. Tests should sit beside the code they cover and use `_test.go` suffixes. Prefer existing domain names such as `Account`, `Balance`, `Ledger`, `Reservation`, `Candle`, and `Ticker` instead of introducing parallel vocabulary.

## Testing Guidelines

Add focused tests for balance transitions, reservation hold and release, authentication boundaries, and any bug fix that changes behavior. Money paths always get tests before merging. Use table-driven tests where cases share setup, but keep simple single-case tests simple. Run `go test ./...` before submitting. Repository tests may use fakes or mocks rather than requiring a live database unless the change is explicitly integration-focused.

## Commit & Pull Request Guidelines

Commit subjects must start with `Feat:` for new behavior, `Fix:` for bug fixes, `Docs:` for documentation-only changes, `Refactor:` for behavior-preserving restructuring, `Test:` for test-only changes, or `Chore:` for tooling, generated files, and maintenance. Use the prefix that describes the user-visible intent, not every file touched. Keep the first line specific and under roughly 72 characters. Pull requests should describe the behavior change, list verification commands, link any related issue, and include screenshots only when diagrams or user-visible docs change.

## Security & Configuration Tips

Do not commit credentials, connection strings, or generated local artifacts. Never store passwords or session tokens in plaintext; hash credentials and keep session secrets in configuration. Keep temporary outputs under `tmp/`, which is ignored. When changing money logic, use fixed-point integer units consistent with the matching-engine numeric model and never floats. When changing SQL, update migrations, queries, generated sqlc code, and repository tests together.
