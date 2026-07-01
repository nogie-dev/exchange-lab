# Testing and CI Context

CI file:

- `.github/workflows/go.yml`

CI checks:

- `gofmt` over all Go files except `tmp`.
- `go vet ./...`
- `go test ./...`

Test locations:

- Engine tests live beside engine code in `internal/engine/*_test.go`.
- Match log store tests live in `internal/matchlog/postgres/*_test.go`.
- Shared sample orders live in `internal/testdata`.

Guidelines:

- Add focused regression tests for bug fixes.
- Use table tests when cases share setup.
- Keep single-case tests simple when one scenario proves the behavior.
- Prefer package-level tests over broad end-to-end setup for engine invariants.

Verify:

- Small engine changes: `go test ./internal/engine`
- Match log storage changes: `go test ./internal/matchlog/...`
- Before final delivery: `gofmt`, `go vet ./...`, `go test ./...`
