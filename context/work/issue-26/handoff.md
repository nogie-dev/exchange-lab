---
work_item: issue-26
branch: "feat/26-durable-order-journal-replay"
status: completed
updated: 2026-07-17
issue_or_pr: "https://github.com/nogie-dev/matching-engine/pull/30"
---

# Work Handoff

## Objective

Commit every accepted order command to an append-only PostgreSQL journal before book mutation, then replay that journal deterministically at startup so missing match logs are reconciled through issue #24 idempotency.

## Completed

- Confirmed issues #24 and #25 and PRs #28 and #29 are complete and synchronized the branch from `main`.
- Reviewed the current command DTOs, worker mutation path, execution ID derivation, server startup, shared pool, and persistence acknowledgement flow.
- Chosen client-supplied command IDs, database-assigned ticker-local sequences, and database-recorded command time as replay identity.
- Added required command IDs to create, amend, and cancel DTOs and API validation.
- Added append-only journal schema, ticker sequence allocation, sqlc output, and an idempotent/conflict-aware PostgreSQL store.
- Made command completion synchronous: API success now follows journal append, command application, and any match-log commit acknowledgement.
- Added append-before-mutation worker flow using database-recorded time for deterministic create/edit execution identity.
- Added startup replay before HTTP listen and reconciliation through the idempotent match-log store.
- Added recovery documentation covering invariants, startup, operator procedure, conflict handling, and deferred snapshots.
- Added journal failure, crash/restart, replay determinism, duplicate command, reconciliation, amend, and cancel tests.
- Opened draft PR #30 with `Closes #26` and the required `command_id` API contract documented.

## Remaining

- None. PR #30 is ready for CI review and merge.

## Decisions

- Require a stable `command_id` from the upstream platform; payload hashing cannot distinguish two legitimate identical commands.
- Allocate a monotonically increasing sequence per ticker in PostgreSQL and tolerate gaps caused by idempotent retries.
- Treat an identical command ID retry as already accepted and do not apply it twice in a live worker; reject the same ID with different payload.
- Use the journal row's database timestamp for live execution and replay so execution IDs remain stable.
- Start the writer before replay, keep HTTP closed until replay and match-log reconciliation finish, and fail startup on any recovery error.
- Do not add snapshots, compaction, an external queue, or another dependency.

## Risks or Blockers

- PostgreSQL remains the shared durability domain; infrastructure-level loss is outside this repository.
- Full replay cost grows with journal size; snapshot/checkpoint optimization remains deferred until measured.

## Relevant Files

- `internal/engine/bookworker.go`
- `internal/engine/order.go`
- `internal/models/dto.go`
- `internal/matchlog`
- `internal/journal`
- `db/migrations`
- `db/query`
- `cmd/server/main.go`

## Verification

- Issue #26, current worker/API/server persistence flow, execution ID generation, models, and SQL generation layout were inspected on 2026-07-17.
- `sqlc generate`, `go test -race ./...`, `go vet ./...`, and `go test ./...` passed with a task-local Go build cache.

## Next Action

After PR #30 merges, verify all three follow-up issues and the final `main` branch state.
