# Task Routing

Use this file when the task area changes or is unclear. Pick the first matching route, read the listed context, then stop when the route's verification is satisfied.

| Request type | Read first | Workflow | Stop condition |
|---|---|---|---|
| Project scaffolding, module init, server skeleton, CI setup | `context/bootstrap.md`, `.github/workflows/` | Build the smallest runnable skeleton following the planned layout in `AGENTS.md`; keep configuration external | `go build ./...`, `go vet ./...`, and `go test ./...` pass |
| User account, signup, login, auth, session issue | `context/bootstrap.md` scope section, `internal/account` | Trace the account lifecycle, change the fewest files, never store credentials in plaintext | Targeted `internal/account` tests pass |
| Balance, deposit, withdrawal, ledger, reservation | `context/bootstrap.md` scope section, `internal/balance` | Keep mutations as immutable ledger entries; reserve and release atomically with order state; fixed-point integers only | `go test ./internal/balance` passes |
| Chart, candle, ticker, market data serving | `context/bootstrap.md` scope section, `internal/marketdata` | Serve read-only data; do not compute matches or analytics here | Targeted `internal/marketdata` tests pass |
| Order placement/cancel forwarding, matching-engine integration | `context/bootstrap.md`, `internal/gateway`, matching-engine `context/docs/api-surface.md` | Keep the client thin, map errors explicitly, pair every forward with reservation handling | Targeted `internal/gateway` tests pass |
| Tests, CI, quality gate | `.github/workflows/go.yml`, `AGENTS.md` | Reproduce locally, make the smallest fix, keep CI commands aligned | `gofmt`, `go vet ./...`, and `go test ./...` pass |
| Architecture or explanation question | `context/bootstrap.md`, `README.md` | Inspect relevant code and answer with file references | No file edits unless asked |
| Documentation request | Relevant `context/docs/*.md`, `context/bootstrap.md`, `AGENTS.md` | Edit docs only and verify paths or commands mentioned | Docs are accurate; code unchanged |
| ETL, analytics DB, candles pipeline, data marts | `context/bootstrap.md` relationship section | Explain that this belongs outside this repo unless the user explicitly changes scope | No code change here |
| GitHub issue, branch, PR scope, issue split | `context/docs/github-issues.md` | Use `github-issue-manager`; inspect the issue before changing scope | Issue or branch decision is documented |
| Git commit, branch hygiene, commit message | `context/docs/git-workflow.md`, `AGENTS.md` | Keep commits issue-aligned and use the required prefix | Commit message follows repo convention |

For destructive work, history rewrites, dependency additions, or public API removals, pause and ask before acting.
