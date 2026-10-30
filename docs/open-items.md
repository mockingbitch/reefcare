# ReefCare — Open Items

Last updated: 2026-10-30 · State: Phase 1 done (milestones 1.1–1.8). Changes since c9f6c47 are
not committed yet.

Everything below is known to be **not done**, **partially done**, or **waiting on a decision**.
Sources: the design docs, the db-architect / go-reviewer / security-auditor reviews, and
verification runs. Remove an item only when it is done and verified; link the commit or PR.

Priority: **P1** blocks launch or the next milestone · **P2** needed before production ·
**P3** improvement / later phase.

## Done on 2026-10-30 (uncommitted)

F1 profile (`PATCH`/`DELETE /me`) · F2 tanks, shares, `tanks.Authorizer`, timeline recorder,
cross-user 404 suite · F3 CI workflow + Renovate · F5 `api/openapi.yaml` with a route-sync test ·
F6 general write limit · F7 Prometheus metrics · S3 tokens bound to their email · S6 logout-all ·
P2 serializable retry · P3 field-level errors for custom JSON types (incl. nested and miscased
members) · R3 `/new-migration` skill · R5 Go installed locally · F4 web shell (`web/`, accepted
proposal in [frontend.md](frontend.md); unit, component and Playwright e2e tests with axe) ·
R6 scratch database removed · R2 `.env.example` added (deny rules in `.claude/settings.json` narrowed to real env files). All reviewed by db-architect,
go-reviewer and security-auditor; their findings were fixed or are listed below.

## 1. Must do before launch

| # | Item | Priority | Why it is open | Where / next step |
|---|---|---|---|---|
| L1 | Replace the password blocklist with a reviewed top-100k breached-password list | P1 | Only a ~70-entry seed list exists; inventing a list is not acceptable and no reviewed source has been chosen | `internal/identity/common_passwords.txt`; docs/security.md §2 |
| L2 | Benchmark Argon2id on production hardware | P2 | Login takes ~77 ms on the dev machine, below the 100–250 ms target | `identity.DefaultArgon2Params`, `ARGON2_MAX_CONCURRENCY` |
| L3 | Domain-expert review of unit conversions and rule thresholds (decision D5) | P1 for Phase 3 | Unreviewed; also covers the display-unit choices in `identity.unitPrefOptions` | Name a reviewer |
| L4 | Production deployment config (Caddy, HSTS, CSP, `frame-ancestors`, log redaction of query strings, metrics port kept private) | P2 | `deploy/` holds only dev config | Phase 5, or earlier for staging |
| L5 | Backups and restore drill | P2 | Not started | Phase 5 |
| L6 | Check that the `MAIL_FROM` mailbox is monitored, or add a reactivation link | P2 | The account-deactivated email tells a victim to reply; nothing reads replies yet (security review L6) | Decide with D6; reactivation within the 30-day window is a product decision |

## 2. Unfinished Phase 1 milestones

| # | Item | Priority | Notes |
|---|---|---|---|
| F8 | CI has never run on GitHub | P1 | `.github/workflows/ci.yml` (jobs go, web, e2e, docker) passes `actionlint` and every step passes locally, but there is no remote repository yet (see R4). Actions are pinned to commit SHAs |
| F9 | Serve the web build in production | P2 | `web/build` is a static SPA (`200.html` fallback); Caddy must serve it and proxy `/api`, `/healthz`, `/readyz` on the same origin, and add the response headers (part of L4). Until then the app can be framed: `frame-ancestors 'none'` cannot go in a meta CSP (web security review L1). Add a CI check that asserts the headers on the served `200.html` |
| F11 | Sign-out does not reach other open tabs | P3 | Other tabs show the user until their next API call returns 401 (web security review L4). Use a `BroadcastChannel` signal |
| F10 | Dev-only npm advisories | P3 | `npm audit` reports 4 low-severity issues in `deepmerge` via `@sveltejs/vite-plugin-svelte` (build tooling only); `npm audit --omit=dev` is clean and runs in CI. Wait for the upstream fix |

## 3. Waiting on the job queue (milestone 2.3)

| # | Item | Priority | Notes |
|---|---|---|---|
| J1 | Durable email delivery | P1 | `mail.Async` is in memory: queued emails are lost if the process crashes |
| J2 | Durable background work in identity (failed-login audit, reset-token issue) | P2 | Bounded to 64 in flight; extra work is dropped and logged |
| J3 | Purge expired sessions and auth tokens | P2 | Indexes on `expires_at` exist; the purge job doesn't |
| J4 | Erase/anonymize deactivated accounts after 30 days | P2 | `DELETE /me` records `erase_after`. The job must keep the `users` row (anonymized: email, names, username, bio, prefs) because the ledger and audit log reference it, and delete the tank subtree explicitly in FK order (`tank_events` → future tank children → `tank_shares` → `tanks`). `tanks.owner_id` and `tank_events.tank_id` are `RESTRICT` on purpose — do not change them to `CASCADE`. Every new tank child table must be added to the job. Also releases the username (security review L2) |

## 4. Security and identity follow-ups

| # | Item | Priority | Source | Notes |
|---|---|---|---|---|
| S1 | Rate limiting is per process | P2 | Decision D7 | Needs a shared store before running more than one API instance |
| S2 | Separate DB roles: app role DML-only, migration role for DDL, `REVOKE UPDATE/DELETE` on append-only tables | P2 | docs/security.md §5 | Today a single role is used; `audit_logs` is protected by a trigger only |
| S4 | Account pre-hijacking policy | P3 | Security review L6 (1.4) | Should verifying an old, never-verified registration also invalidate its password? (product decision). Shares already require a verified grantee |
| S5 | Deactivated accounts keep their email and username until erasure | P3 | DB review L8, security review L2 | Decide whether the address may be re-registered after erasure; J4 frees both |
| S7 | MFA / social login | P3 | Decision D10 | Post-MVP |
| S8 | Personal data export (JSON) | P3 | docs/security.md §9 | Phase 5 |
| S9 | Monitor `golang.org/x/crypto` advisory GO-2026-5932 (openpgp) | P3 | govulncheck | Not reachable from our code; no upstream fix |
| S10 | Share endpoint is an account-existence oracle | P3 | Security review L3 | Mitigated: verified owner, 30/h per owner, verified grantees only. Options: add a per-IP tier, audit `grantee_not_found` lookups |
| S11 | Email change flow | P3 | DB review | When built: revoke all live tokens in the same transaction (the one-live-token index would otherwise block the new verification), audit, notify the old address |

## 5. Platform and data follow-ups

| # | Item | Priority | Notes |
|---|---|---|---|
| P1 | Per-route request body limits | P2 | The 1 MiB JSON cap is global; upload/import endpoints will need larger limits |
| P4 | Unknown-field detection matches `encoding/json`'s error text | P3 | Fragile if the JSON implementation changes; pinned by tests in `httpserver` |
| P5 | Repeatable tank events need their own source rows | P2 | `tank_events` is unique on `(source_type, source_id, event_type)`. Status changes, configuration edits and re-shares must record a row per occurrence (e.g. a status-change or configuration-version table) and use its id as `source_id`. Today only `tank.created` and `tank.archived` (both once per tank) are recorded; tank edits (e.g. volume changes) are not on the timeline yet |
| P6 | Per-category share visibility | P3 | .claude/rules/reef-domain.md asks for owner-controlled visibility per data category; shares are all-or-nothing viewer grants for now (decision D4). Design with Phase 4 snapshots |
| P7 | Re-check `ListTanksForUser` with `EXPLAIN (ANALYZE, BUFFERS)` on realistic data | P3 | Rewritten as `UNION ALL` after the DB review measured 358 ms → 0.23 ms on 200k tanks for that shape; re-measure in Phase 5 load tests |

## 6. Repository and tooling

| # | Item | Priority | Why it is open | Next step |
|---|---|---|---|---|
| R1 | Work since c9f6c47 is uncommitted; only `master` exists, no remote | P1 | No commit has been requested | Commit on a branch; create the remote (R4) |
| R4 | Go module path is `reefcare`; no remote repository | P3 | — | Rename to `github.com/<org>/reefcare` once it exists |

## 7. Open product and infrastructure decisions

Accepted with defaults on 2026-10-29, but these still need a concrete owner or choice:

| ID | Decision | Current default | Needed by |
|---|---|---|---|
| D5 | Who reviews thresholds and seed knowledge | — (unassigned) | Phase 3 |
| D6 | Production email provider (SES, Postmark, other SMTP) | Mailpit locally | Before staging |
| D7 | Hosting and managed PostgreSQL | Single VPS + managed PG, Singapore | Before staging |
| D9 | CI platform and dependency bot | GitHub Actions + Renovate (configured) | Needs the remote (R4) |
| D11 | Chart library | ECharts, confirm in a spike | Milestone 2.10 |
| — | Local S3-compatible storage (MinIO's status is uncertain; Garage or SeaweedFS) | Not chosen | When media ships (Phase 2.7 / 4) |
