# Git Workflow

Commit subjects must use one of these prefixes:

- `Feat:` for new behavior or capability.
- `Fix:` for bug fixes.
- `Docs:` for documentation-only changes.
- `Refactor:` for behavior-preserving restructuring.
- `Test:` for test-only changes.
- `Chore:` for tooling, generated files, or maintenance.

Use the prefix that describes the user-visible intent. A feature commit may include tests, docs, and generated code when they support the same change.

When work exceeds the assigned issue:

- Inspect the issue before deciding.
- Create or propose a follow-up issue.
- Move work to an issue-aligned branch.
- Keep unrelated local artifacts out of the commit.

Commit bodies should follow the Lore protocol when context matters: explain the decision, list meaningful constraints, rejected alternatives, confidence, scope risk, tests, and known gaps.
