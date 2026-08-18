---
work_item: issue-25
branch: "feat/25-fail-closed-matchlog-persistence"
status: completed
updated: 2026-07-17
issue_or_pr: "https://github.com/nogie-dev/matching-engine/pull/29"
---

# Work Handoff

## Objective

Make PostgreSQL commit acknowledgement part of the worker control flow, halt the whole engine on persistence failure, reject commands while halted, and wire the default server to one shared PostgreSQL pool with graceful draining.

## Completed

- Confirmed issue #24 and PR #28 are complete and synchronized the branch from `main`.
- Verified that the existing channel acknowledges only enqueueing, writer errors are logged and ignored, and the default server does not wire persistence.
- Selected the router/worker/channel route and reviewed the API, server, configuration, and writer boundaries.
- Added persistence requests that carry a buffered commit acknowledgement back to the requesting worker.
- Added shared fail-closed engine state; workers wait for ACK and the router rejects commands after the first persistence error.
- Added `GET /ready`, returning `503` while halted or shutting down, while snapshots remain available for inspection.
- Wired the default server to a required environment-supplied database URL, one shared `pgxpool.Pool`, the transactional store, and the writer.
- Added ordered shutdown that stops HTTP intake, drains workers and persistence acknowledgements, then closes the pool.
- Updated bootstrap, router/worker, API, match-log, and README facts to match the new runtime behavior.
- Opened draft PR #29 with `Closes #25` and the issue #26 crash-recovery boundary documented.

## Remaining

- None. PR #29 is ready for CI review and merge.

## Decisions

- Keep `BookWorker` dependent only on a match-log request channel, not on PostgreSQL or `Store`.
- Let the first persistence failure permanently halt a shared engine state; no automatic resume exists.
- Continue serving snapshots while halted, but reject create/amend/cancel commands and fail readiness.
- Stop accepting commands before closing worker queues; drain workers before closing the writer channel and pool.
- Read the database URL from `MATCHING_ENGINE_DATABASE_URL`; do not place credentials in JSON configuration.

## Risks or Blockers

- Matching mutates the in-memory book before persistence; a failed write therefore requires process restart and issue #26 replay rather than in-process resume.
- HTTP command responses acknowledge queue acceptance, while commit acknowledgement controls worker progression; durable command-result responses remain outside this issue.

## Relevant Files

- `internal/engine/bookworker.go`
- `internal/engine/router.go`
- `internal/matchlog/writer.go`
- `internal/api/handler.go`
- `cmd/server/main.go`
- `internal/config`

## Verification

- Issue #25, route context, worker/router tests, API tests, writer tests, configuration, and default server wiring were inspected on 2026-07-17.
- `go test -race ./...`, `go vet ./...`, `go test ./...`, and `git diff --check` passed with a task-local Go build cache.

## Next Action

After PR #29 merges, start issue #26 from the updated `main` branch.
