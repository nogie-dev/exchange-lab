# Task Routing

Use this file when the task area changes or is unclear. Pick the first matching route, read the listed context, then stop when the route's verification is satisfied.

| Request type | Read first | Workflow | Stop condition |
|---|---|---|---|
| Matching bug, fill logic, price-time priority | `context/docs/matching-engine.md`, `internal/engine/match.go` | Reproduce or inspect the case, fix the shared matching path, add a focused engine test | `go test ./internal/engine` passes |
| Order add/cancel/edit, heap, FIFO, index issue | `context/docs/orderbook.md`, `internal/engine/order.go`, `internal/util/price_level.go` | Trace the order lifecycle, preserve index and heap invariants, change the fewest files | `go test ./internal/engine` passes |
| Router, worker, ticker dispatch, channel behavior | `context/docs/router-worker.md`, `internal/engine/router.go`, `internal/engine/bookworker.go`, `config/default.json` | Keep routing thin, keep matching in workers, decide block/reject behavior explicitly | Targeted worker/router tests pass |
| Internal API surface, endpoint design, platform integration | `context/docs/api-surface.md`, `context/docs/architecture.md`, `internal/engine/router.go`, `internal/engine/bookworker.go` | Keep API command-oriented and platform-to-engine only | API boundary is documented; implementation is split unless requested |
| Raw match logs, order journal, sqlc, migrations | `context/docs/matching-log.md`, `context/docs/order-journal.md`, `internal/matchlog`, `internal/journal`, `sqlc.yaml`, `db/query`, `db/migrations` | Keep durable commands and raw execution logs here; leave ETL and analytics DB out of scope | `sqlc generate` and persistence package tests pass |
| Tests, CI, quality gate | `context/docs/testing.md`, `.github/workflows/go.yml` | Reproduce locally, make the smallest fix, keep CI commands aligned | `gofmt`, `go vet ./...`, and `go test ./...` pass |
| Architecture or explanation question | `context/docs/architecture.md`, `README.md` | Inspect relevant code and answer with file references | No file edits unless asked |
| Documentation request | Relevant `context/docs/*.md`, `README.md`, `AGENTS.md` | Edit docs only and verify paths or commands mentioned | Docs are accurate; code unchanged |
| ETL, analytics DB, candles, volume, trading data marts | `context/docs/matching-log.md` | Explain that this belongs outside this repo unless the user explicitly changes scope | No code change here |
| GitHub issue, branch, PR scope, issue split | `context/docs/github-issues.md` | Use `github-issue-manager`; inspect the issue before changing scope | Issue or branch decision is documented |
| Git commit, branch hygiene, commit message | `context/docs/git-workflow.md`, `AGENTS.md` | Keep commits issue-aligned and use the required prefix | Commit message follows repo convention |

For destructive work, history rewrites, dependency additions, or public API removals, pause and ask before acting.
