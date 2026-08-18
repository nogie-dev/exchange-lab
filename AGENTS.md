# Repository Guidelines

## Session Context

On the first repository task in a session, read `context/bootstrap.md` once. When starting or resuming persistent work, read only the matching `context/work/issue-N/handoff.md` (or `task-SLUG` when no issue exists). Treat handoffs as leads and verify mutable claims against code, tests, Git, issues, and pull requests.

Do not bulk-load historical handoffs or keep raw command logs. Update a handoff only when work starts, pauses, transfers, changes direction materially, or completes. Use the issue or task ID as the stable directory key and keep the branch name as metadata.

## Agent Routing

Read `context/route.md` when the task area changes or is unclear, then choose the first matching route and follow its context files, workflow, and stop condition. Reuse the selected route during the same work cycle. If no route fits, inspect the repository first and take the smallest reversible action.

## GitHub Issue Management

When creating, inspecting, editing, triaging, or linking GitHub issues, use the `github-issue-manager` skill and follow `context/docs/github-issues.md`.

## Project Structure & Module Organization

This is a Go CLOB trading engine. The runnable entry point is `cmd/server/main.go`. Core matching and order book logic lives in `internal/engine`, durable command journal boundaries in `internal/journal`, raw matching log boundaries in `internal/matchlog`, shared domain types in `internal/models`, and utilities such as heaps and queues in `internal/util`. SQL migrations are in `db/migrations`, hand-written queries in `db/query`, and sqlc output lives under each PostgreSQL persistence package. Keep diagrams and static documentation assets in `asset/`, longer learning notes in `til/`, and compact agent context in `context/`.

## Build, Test, and Development Commands

- `go run ./cmd/server` starts the local server entry point.
- `go test ./...` runs all unit and repository tests.
- `go vet ./...` runs Go static checks used by CI.
- `gofmt -w $(find . -name '*.go' -not -path './tmp/*')` formats Go files.
- `sqlc generate` regenerates PostgreSQL query code from `sqlc.yaml`.

CI runs formatting, vet, and tests on pushes to `main` and pull requests.

## Coding Style & Naming Conventions

Use standard Go formatting and idioms. Package names should stay short and lowercase (`engine`, `models`, `postgres`). Export only APIs needed across packages; keep helpers private when they are package-local. Tests should sit beside the code they cover and use `_test.go` suffixes. Prefer existing domain names such as `OrderBook`, `BookWorker`, `PriceLevel`, `Bid`, and `Ask` instead of introducing parallel vocabulary.

## Testing Guidelines

Add focused tests for matching, order lifecycle, heap behavior, repository validation, and any bug fix that changes behavior. Use table-driven tests where cases share setup, but keep simple single-case tests simple. Run `go test ./...` before submitting. Repository tests may use fakes or mocks rather than requiring a live database unless the change is explicitly integration-focused.

## Commit & Pull Request Guidelines

Commit subjects must start with `Feat:` for new behavior, `Fix:` for bug fixes, `Docs:` for documentation-only changes, `Refactor:` for behavior-preserving restructuring, `Test:` for test-only changes, or `Chore:` for tooling, generated files, and maintenance. Use the prefix that describes the user-visible intent, not every file touched. Keep the first line specific and under roughly 72 characters. Pull requests should describe the behavior change, list verification commands, link any related issue, and include screenshots only when diagrams or user-visible docs change.

## Security & Configuration Tips

Do not commit credentials, connection strings, or generated local artifacts. Keep temporary outputs under `tmp/`, which is ignored. When changing SQL, update migrations, queries, generated sqlc code, and repository tests together.
