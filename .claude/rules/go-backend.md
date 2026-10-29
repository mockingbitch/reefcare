---
paths:
  - "**/*.go"
  - "go.mod"
---

# Go backend rules

- Propagate `context.Context` as the first parameter through handlers → services → queries.
- Use `log/slog` with structured fields. Never log passwords, tokens, session IDs, raw cookies,
  email bodies, or unnecessary personal data. Include the request ID on request-scoped logs.
- Errors: wrap with `%w` and context (`fmt.Errorf("create dosing plan: %w", err)`); map domain
  errors to HTTP status codes in one place. Never leak SQL or internal errors to clients.
- Domain packages must not import `net/http`, `pgx`, sqlc output, or frontend concerns.
- Handlers decode → validate → call application service → encode. No business rules in handlers.
- Transactions are opened explicitly in the application service (`pgx.BeginFunc` or equivalent);
  pass `sqlc.Queries.WithTx(tx)` down. No hidden transactions in helpers.
- Constructor injection (`NewService(db *pgxpool.Pool, clock clock.Clock, ...)`). No package-level
  mutable state, no `init()` side effects beyond registration-free constants.
- Use an injectable `clock.Clock` for anything time-dependent so schedules and staleness are testable.
- Every outbound call (DB, LLM, S3, SMTP) has a timeout. Retries only for idempotent operations,
  with explicit, bounded policy.
- Graceful shutdown: stop accepting requests, drain, close pool, stop workers.
- Config is read once from environment at startup; missing/invalid required settings fail fast
  with a clear message.
- Measurements and quantities: use a decimal type (e.g. `pgtype.Numeric` at the DB edge, a decimal
  library or explicit integer scaling in domain) — not `float64` — where precision matters. Unit
  conversions live in tested, explicit functions.
- Keep packages small and named by domain (`dosing`, `livestock`), not by layer (`models`, `utils`).
- Run `gofmt`/`goimports` and `go vet`; code must pass `golangci-lint` once configured.
