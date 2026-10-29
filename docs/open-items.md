# ReefCare — Open Items

Last updated: 2026-10-29 · State: milestones 1.1–1.4 done, nothing committed yet.

Everything below is known to be **not done**, **partially done**, or **waiting on a decision**.
Sources: the design docs, the db-architect / go-reviewer / security-auditor reviews, and
verification runs. Remove an item only when it is done and verified; link the commit or PR.

Priority: **P1** blocks launch or the next milestone · **P2** needed before production ·
**P3** improvement / later phase.

## 1. Must do before launch

| # | Item | Priority | Why it is open | Where / next step |
|---|---|---|---|---|
| L1 | Replace the password blocklist with a reviewed top-100k breached-password list | P1 | Only a ~70-entry seed list exists; downloading a list failed (network) and inventing one is not acceptable | `internal/identity/common_passwords.txt`; docs/security.md §2 |
| L2 | Benchmark Argon2id on production hardware | P2 | Login takes ~77 ms on the dev machine, below the 100–250 ms target; tune `m/t/p` and `ARGON2_MAX_CONCURRENCY` against the real memory limit | `identity.DefaultArgon2Params` |
| L3 | Domain-expert review of unit conversions and rule thresholds (decision D5) | P1 for Phase 3 | Conversion factors and thresholds in docs/domain-model.md and docs/knowledge-engine.md §5 are unreviewed | Name a reviewer |
| L4 | Production deployment config (Caddy, HSTS, CSP, `frame-ancestors`, log redaction of query strings) | P2 | `deploy/` holds only dev config; the security headers in docs/security.md §4 belong to Caddy/SvelteKit, which don't exist yet | Phase 5, or earlier for staging |
| L5 | Backups and restore drill | P2 | Not started | Phase 5 |

## 2. Unfinished Phase 1 milestones

| # | Item | Priority | Notes |
|---|---|---|---|
| F1 | **1.5 Profile**: `PATCH /me` (with `version`), account deactivation and deletion | P1 | Deletion and retention rules are in docs/security.md §9; a job is needed for the 30-day erase |
| F2 | **1.6 Tanks + authorization**: tank CRUD/archive, shares, `tanks.Authorizer`, timeline recorder, cross-user 404 test suite | P1 | Next big slice |
| F3 | **1.7 CI** (GitHub Actions running the `/verify` checklist) | P1 | Decision D9 assumed GitHub Actions + Renovate |
| F4 | **1.8 Web shell** (SvelteKit, i18n, auth pages) | P1 | Email links already point to `/verify-email`, `/reset-password`, `/login`, `/forgot-password`, which don't exist yet. The pages must read `#token=` from the fragment, remove it from history, and set `Referrer-Policy: no-referrer` |
| F5 | **OpenAPI document** `api/openapi.yaml` | P1 | Required by the Definition of Done; the API is so far documented only in docs/api-guidelines.md |
| F6 | General write rate limit (120/min per user) from docs/api-guidelines.md §6 | P2 | Only identity endpoints are limited |
| F7 | Prometheus metrics endpoint (docs/architecture.md §8) | P2 | Logs, request IDs and health checks exist; no metrics yet |

## 3. Waiting on the job queue (milestone 2.3)

| # | Item | Priority | Notes |
|---|---|---|---|
| J1 | Durable email delivery | P1 | `mail.Async` is in memory: queued emails are lost if the process crashes |
| J2 | Durable background work in identity (failed-login audit, reset-token issue) | P2 | Bounded to 64 in flight; extra work is dropped and logged |
| J3 | Purge expired sessions and auth tokens | P2 | Indexes on `expires_at` exist; the purge job doesn't |
| J4 | Erase/anonymize deactivated accounts after 30 days | P2 | Depends on F1 |

## 4. Security and identity follow-ups

| # | Item | Priority | Source | Notes |
|---|---|---|---|---|
| S1 | Rate limiting is per process | P2 | Decision D7 | Needs a shared store (PostgreSQL or Redis) before running more than one API instance |
| S2 | Separate DB roles: app role DML-only, migration role for DDL, `REVOKE UPDATE/DELETE` on append-only tables | P2 | docs/security.md §5 | Today a single role is used; `audit_logs` is protected by a trigger only |
| S3 | Store the email on verification tokens | P3 | DB review M3 | Needed once users can change their email, so a token for address A can't verify address B |
| S4 | Account pre-hijacking policy | P3 | Security review L6 | A completed reset now marks the email verified. Open: should verifying an old, never-verified registration also invalidate its password? (product decision) |
| S5 | Deactivated accounts keep their email (UNIQUE) | P3 | DB review L8 | Decide whether the address can be re-registered after deletion |
| S6 | "Log out all sessions" endpoint | P3 | docs/security.md §8 | The audit action is listed; no endpoint yet |
| S7 | MFA / social login | P3 | Decision D10 | Post-MVP |
| S8 | Personal data export (JSON) | P3 | docs/security.md §9 | Phase 5 |
| S9 | Monitor `golang.org/x/crypto` advisory GO-2026-5932 (openpgp) | P3 | govulncheck | Not reachable from our code (only `argon2` is used); no upstream fix |

## 5. Platform code follow-ups (from the go-reviewer review)

| # | Item | Priority | Notes |
|---|---|---|---|
| P1 | Per-route request body limits | P2 | The 1 MiB JSON cap is global; upload/import endpoints will need larger limits |
| P2 | Bounded retry for `database.InSerializableTx` (SQLSTATE 40001/40P01) | P3 | Callers must retry themselves today |
| P3 | Field-level errors for custom JSON types (decimal, time, UUID) | P2 | Errors from custom `UnmarshalJSON` return a generic `malformed_json` without the field. Needed before Phase 2 measurement endpoints |
| P4 | Unknown-field detection matches `encoding/json`'s error text | P3 | Fragile if the JSON implementation changes; the format is pinned by tests |

## 6. Repository and tooling

| # | Item | Priority | Why it is open | Next step |
|---|---|---|---|---|
| R1 | Nothing is committed | P1 | No commit has been requested yet | Commit on a branch, open a PR to `main` |
| R2 | `.env.example` missing | P2 | The `deny` rules for `.env.*` in `.claude/settings.json` block it, and editing settings is not allowed for the assistant | Change the deny list to name specific env files (snippet in the chat history), then create the file. Variables are listed in README.md |
| R3 | The `/new-migration` skill describes timestamp file names, but the project uses sequential numbers (`make migration`) | P3 | Not edited: changes inside `.claude/` may be blocked | Update `.claude/skills/new-migration/SKILL.md` |
| R4 | Go module path is `reefcare` | P3 | No remote repository yet | Rename to `github.com/<org>/reefcare` once it exists |
| R5 | Go is not installed locally | P3 | The Makefile falls back to the `golang:1.27` container | Optional: install Go 1.27 for a faster loop |

## 7. Open product and infrastructure decisions

Accepted with defaults on 2026-10-29, but these still need a concrete owner or choice:

| ID | Decision | Current default | Needed by |
|---|---|---|---|
| D5 | Who reviews thresholds and seed knowledge | — (unassigned) | Phase 3 |
| D6 | Production email provider (SES, Postmark, other SMTP) | Mailpit locally | Before staging |
| D7 | Hosting and managed PostgreSQL | Single VPS + managed PG, Singapore | Before staging |
| D9 | CI platform and dependency bot | GitHub Actions + Renovate | Milestone 1.7 |
| D11 | Chart library | ECharts, confirm in a spike | Milestone 2.10 |
| — | Local S3-compatible storage (MinIO's status is uncertain; Garage or SeaweedFS) | Not chosen | When media ships (Phase 2.7 / 4) |
