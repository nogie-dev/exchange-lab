---
work_item: issue-24
branch: "feat/24-idempotent-matchlog-persistence"
status: completed
updated: 2026-07-17
issue_or_pr: "https://github.com/nogie-dev/matching-engine/pull/28"
---

# Work Handoff

## Objective

Persist every match log produced by one incoming order in a single PostgreSQL transaction, with stable execution identities that make retry idempotent and reject conflicting payloads.

## Completed

- Confirmed issue #14 and PR #27 are complete and synchronized the branch from `main`.
- Verified that the current store inserts each log independently and has no execution identity or database uniqueness constraint.
- Selected the raw match-log/sqlc/migration route from `context/route.md`.
- Added deterministic per-fill execution IDs to engine-emitted match logs.
- Added a forward migration with a unique execution ID constraint and regenerated sqlc queries.
- Made each `SaveMatchLogs` batch atomic, idempotent for identical retries, and conflicting for reused IDs with different payloads.
- Required `MatchedAt` instead of generating persistence-time data that would make retry payloads unstable.
- Added rollback, identical retry, conflicting retry, and ambiguous commit retry tests.
- Opened draft PR #28 with `Closes #24` and the issue #25/#26 durability boundaries documented.

## Remaining

- None. PR #28 is ready for CI review and merge.

## Decisions

- Generate an execution ID from the incoming order identity, match timestamp, and fill sequence; issue #26 will replace timestamp-based command identity with durable journal sequencing.
- Validate the entire batch before opening a transaction.
- Treat an identical existing execution ID as successful retry and a different payload as an explicit consistency error.
- Use one transaction per incoming order batch and add no cross-order timer batching or dependencies.

## Risks or Blockers

- Process-crash-safe command identity and deterministic replay remain deferred to issue #26.
- Persistence acknowledgement and fail-closed worker behavior remain deferred to issue #25.

## Relevant Files

- `internal/matchlog/log.go`
- `internal/matchlog/postgres/store.go`
- `internal/matchlog/postgres/store_test.go`
- `internal/engine/match.go`
- `db/migrations`
- `db/query/match_logs.sql`

## Verification

- Issue #24, route context, current schema, generated sqlc code, store, writer, and engine emission path were inspected on 2026-07-17.
- `sqlc generate` completed without additional changes.
- `go test ./internal/matchlog/... ./internal/engine`, `go vet ./...`, and `go test ./...` passed with a task-local Go build cache.

## Next Action

After PR #28 merges, start issue #25 from the updated `main` branch.
