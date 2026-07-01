# GitHub Issues

Use the `github-issue-manager` skill for issue inspection, creation, editing, triage, comments, and branch-from-issue work.

Rules:

- Inspect the assigned issue before deciding whether current work fits.
- If implementation exceeds the issue's explicit scope, create or propose a follow-up issue.
- Do not close, reopen, delete, relabel, or edit issues without previewing the change.
- Keep PR scope aligned with the issue.
- Use "supersedes" only when the issue intent changed.

Branch guidance:

- Prefer `feat/<issue>-short-slug`, `fix/<issue>-short-slug`, or `docs/<issue>-short-slug`.
- If a branch's work no longer matches the issue, rename it or create a new branch.

Verify:

- Re-read the issue or branch after mutation.
- Report the issue URL, branch name, changed fields, and any remaining scope risk.
