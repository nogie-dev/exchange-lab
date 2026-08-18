---
work_item: issue-14
branch: "docs/14-db-write-strategy"
status: completed
updated: 2026-07-17
issue_or_pr: "https://github.com/nogie-dev/matching-engine/pull/27"
---

# Work Handoff

## Objective

Decide and document a zero-loss persistence policy for raw match logs. Split implementation into dependency-ordered follow-up issues without expanding issue #14 beyond design work.

## Completed

- Verified the current boundary: `BookWorker` sends `[]MatchLog` through a blocking in-process channel, `Writer` owns `Store`, and the default server does not wire persistence.
- Confirmed that `SaveMatchLogs` currently performs one insert per log and that writer failures are only logged.
- Chosen the governing invariant: a match is not committed or exposed until every raw match log is durably committed to PostgreSQL.
- Chosen fail-closed behavior: persistence failure or an ambiguous commit result halts order processing instead of dropping logs or continuing in a degraded mode.
- Created issue #24 for idempotent transactional match-log persistence.
- Created issue #25 for commit acknowledgement and fail-closed engine halt; it depends on #24.
- Created issue #26 for durable order journaling and deterministic replay; it depends on #24 and #25.
- Updated issue #14 with the final decision, failure modes, test strategy, and follow-up links.
- Promoted the decision into `context/docs/matching-log.md` and marked the backpressure consideration decided.
- Opened draft PR #27 with `Closes #14` and the dependency-ordered follow-up list.
- Marked PR #27 ready after its required Go CI workflow passed.

## Remaining

- None. PR #27 is ready to merge and issue #14 will close through `Closes #14`.

## Decisions

- PostgreSQL commit is the initial trade durability boundary.
- Connection pooling is connection management, not a durability mechanism.
- In-memory channels may provide bounded handoff/backpressure but never count as durable storage.
- Logs from one incoming order may be persisted as one atomic batch; cross-order timer batching is deferred.
- Safe retry requires a stable execution ID and a uniqueness constraint.
- Process-crash recovery requires a durable order journal and deterministic replay before production-grade zero-loss claims are valid.

## Risks or Blockers

- The current matching path mutates the in-memory book before persistence and has no recovery journal.
- The current API acknowledges enqueueing rather than durable processing.
- PostgreSQL infrastructure durability, replication, and backup remain deployment concerns outside issue #14.

## Relevant Files

- `context/docs/matching-log.md`
- `context/considerations/matching-log-backpressure.md`
- `internal/engine/bookworker.go`
- `internal/matchlog/writer.go`
- `internal/matchlog/postgres/store.go`

## Verification

- Issue #14, issue #15, PR #16, current code, and repository context were inspected on 2026-07-17.
- `git diff --check`, `go vet ./...`, and `go test ./...` passed before PR #27 was opened.
- PR #27 Go CI run 22 completed successfully.

## Next Action

After PR #27 merges, start issue #24 from the updated `main` branch.
