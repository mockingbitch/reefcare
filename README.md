# ReefCare

Reef tank management and intelligence platform. Design documents live in [docs/](docs/);
contributor and AI-assistant conventions are in [CLAUDE.md](CLAUDE.md).

## Requirements

- Docker with Compose v2
- Go 1.27 (optional — the Makefile runs Go inside the `golang:1.27` container when `go` is not installed)

## Quick start

```sh
make up                 # postgres 18, mailpit, migrations, API on http://localhost:8080
curl localhost:8080/readyz
make logs               # follow API logs
make down               # stop (data volume is kept)
```

Mailpit UI: http://localhost:8025.

## Development

```sh
make check              # gofmt check + sqlc drift + go vet + golangci-lint + unit tests (race)
make sqlc               # regenerate internal/db/dbq from db/queries (never edit generated code)
make test-integration   # integration tests against the compose database (needs `make up`)
make build              # bin/api
make vuln               # govulncheck
make migration name=create_users   # new sequential Goose migration in db/migrations
make migrate-status | migrate-up | migrate-down
```

## API binary

```text
api serve            run the HTTP API (default)
api migrate up       apply pending migrations
api migrate down     roll back the latest migration
api migrate status   show migration status
```

Configuration is read from environment variables and validated at startup; every problem is
reported at once. See `internal/config/config.go` and [docs/architecture.md](docs/architecture.md) §6.

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | `development`, `test`, `production` |
| `HTTP_ADDR` | `:8080` | |
| `PUBLIC_BASE_URL` | — | required (https) in production |
| `SHUTDOWN_TIMEOUT` | `30s` | graceful shutdown budget |
| `TRUSTED_PROXY_CIDRS` | — | comma-separated CIDRs/IPs of reverse proxies whose `X-Forwarded-For` and `X-Request-ID` are trusted; required in production; ranges wider than /8 (IPv4) or /32 (IPv6) are rejected |
| `DATABASE_URL` | — | required, `postgres://…` |
| `DB_MAX_CONNS` | `10` | 1–500 |
| `DB_CONNECT_TIMEOUT` | `5s` | |
| `ALLOWED_ORIGINS` | — | extra origins (besides `PUBLIC_BASE_URL`'s) allowed to send state-changing requests, e.g. `http://localhost:5173` |
| `SMTP_HOST` / `SMTP_PORT` | — / `587` | required in production (STARTTLS enforced there); without it, dev only logs that an email would be sent |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | — | optional SMTP auth |
| `MAIL_FROM` | `ReefCare <no-reply@localhost>` | required in production |
| `ARGON2_MAX_CONCURRENCY` | GOMAXPROCS | max simultaneous password hashes (64 MiB each); keep × 64 MiB well under the memory limit |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json`, `text` |

## Endpoints so far

| Path | Purpose |
|---|---|
| `GET /healthz` | liveness — process is up |
| `GET /readyz` | readiness — database reachable and schema at least the version embedded in the binary (503 otherwise) |
| `/api/v1/auth/*`, `/api/v1/me` | registration, email verification, login/logout, password reset and change — see [docs/api-guidelines.md](docs/api-guidelines.md) §5 |

Emails sent in development are visible in Mailpit at http://localhost:8025.
