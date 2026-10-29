---
paths:
  - "**/*_test.go"
  - "**/testdata/**"
  - "web/**/*.test.ts"
  - "web/**/*.spec.ts"
---

# Testing rules

- Write tests alongside the feature, not afterwards.
- Unit tests (table-driven) for: unit conversions, trend calculations, dosing schedules,
  water-change calculations, livestock reconciliation, reputation scoring, rank thresholds,
  permission policies, assessment rules.
- Integration tests use a real PostgreSQL (Compose service or testcontainers), never mocks of SQL
  behavior. Cover migrations, constraints, rollback, concurrent voting, idempotent ingestion,
  duplicate reward prevention, ownership/sharing, historical record preservation.
- Gate integration tests with a build tag (`//go:build integration`) or env var so `go test ./...`
  stays fast.
- API tests cover auth, validation errors, authz failures (including cross-user IDOR), pagination,
  filtering, visibility, error envelope, upload permissions.
- Intelligence tests use deterministic fixtures in `testdata/`: missing/stale data, wrong units,
  conflicting observations, real knowledge references only, no unsupported conclusions, LLM-down
  behavior, preserved safety warnings, LLM output cannot bypass authorization.
- Use the injectable clock; never depend on wall-clock time or test ordering.
- No real production user data in tests or fixtures.
- Report real results; a skipped or un-runnable suite is reported as such.
