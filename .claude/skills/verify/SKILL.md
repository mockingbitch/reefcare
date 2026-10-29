---
name: verify
description: Run ReefCare's verification checks (format, vet, lint, unit and integration tests, sqlc drift, migrations, frontend checks, build) and report actual results honestly. Use before declaring any task done.
---

Run every applicable check below from the repository root. Prefer `make` targets when the
Makefile defines them; otherwise use the raw commands. Skip checks whose prerequisites don't exist
yet (e.g. no `web/` directory) and say they were skipped and why.

1. Toolchain: `go version` (and `sqlc version`, `goose -version` if used).
2. Formatting: `gofmt -l .` (must print nothing) — exclude generated code if needed.
3. Static analysis: `go vet ./...`; `golangci-lint run` if configured.
4. sqlc drift: `sqlc generate` then `git diff --exit-code -- '*.sql.go' 'internal/**/db/**'`
   (or `sqlc diff` / `sqlc vet` if configured).
5. Unit tests: `go test -race ./...`.
6. Integration tests: only if PostgreSQL is available (`docker compose ps`); run the project's
   integration target (e.g. `make test-integration` or `go test -tags=integration ./...`).
7. Migrations: on a disposable test database, `goose up`, `goose down-to 0`, `goose up` to prove
   reversibility. Never run against a non-local database.
8. Build: `go build ./...`.
9. Frontend (if `web/` exists): `npm run check`, `npm run lint`, `npm test`, `npm run build`.
10. Security (if available): `govulncheck ./...`, `npm audit --omit=dev`.

Report as a table: check · command · result (passed / failed / skipped / could not run) · notes.
Include the relevant failing output verbatim (trimmed). Never mark a check passed unless it ran
and exited successfully in this session.
