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
R6 scratch database removed · R2 `.env.example` added (deny rules in `.claude/settings.json` narrowed to real env files) ·
2.3 job queue with J1 durable email, J2 durable identity background work, J3 session/token purge,
J4 account erasure after 30 days (also resolves S5: the address is free again after erasure). All reviewed by db-architect,
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

## 3. Background jobs (milestone 2.3 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| J5 | No admin view of dead jobs | P3 | Dead jobs are logged at error level and kept 30 days (`SELECT … FROM jobs WHERE status = 'dead'`); an admin endpoint or alert on `reefcare_jobs_total{outcome="dead"}` comes with Phase 5 observability |
| J6 | Long jobs have no lease heartbeat | P3 | The lease is handler timeout (2 min) + 30 s. Erasure of very large accounts or future bulk jobs must stay under the timeout or batch across runs |
| J7 | Erasure deletes a large account's tank data in one transaction | P2 before Phase 2 data | Fine for Phase 1 volumes. Before water tests, doses and events grow: add an `erasure_started_at` fence, delete per tank in bounded batches across transactions, anonymize last (DB review). Every new tank child table must also be added to `tanks.EraseUserData` / `DeleteOwnerTankData` |
| J8 | Two accounts that share tanks with each other and are erased at the same moment can deadlock (40P01) | P3 | The job retry absorbs it; lock shares in a fixed order in `EraseUserData` if it shows up in logs |
| J9 | `erase_due` pagination beyond one batch (500) has no test | P3 | Keyset pagination on `(deactivated_at, id)` replaced the loop that could spin; a test needs an injectable batch size |
| J10 | Index changes on large tables | P3 | 00005 builds indexes in its transaction (fine pre-launch). From launch on, `CREATE INDEX CONCURRENTLY` in a separate non-transactional migration |

## 3b. Water changes (milestone 2.5 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| W1 | Measured salinity/temperature and pre/post water-test links on water changes | P2 | Need the parameter catalog and units (2.1/2.2, decision D5) |
| W2 | Water-change reminders (in-app/email) | P2 | Milestone 2.9; overdue is already derivable |
| W3 | Web screens for schedules and water changes | P2 | Milestone 2.10 (API and generated types are ready) |
| W4 | `Idempotency-Key` header for retried creates (`POST /tanks/{id}/water-changes`) | P3 | Complete/skip are naturally idempotent; a retried POST of a one-off change can create a duplicate. Reuse the per-row pattern from dosing ad-hoc doses (idempotency_key + request_hash) |
| W5 | Dilution projection after n changes (docs/domain-model.md, with its caveats) | P3 | Needs measured new-water values (2.2), exact decimal powers, a batch/continuous `method` field, and D5 review |
| W6 | UI requirements from the domain review (for 2.10) | P2 | Show "monthly on the 28th" when a start on the 29th–31st is capped; show the next 3 dates before saving a schedule; use `next_occurrences` (not materialized rows) for "next change" when the interval exceeds 35 days; "< 0.1 %" for tiny fractions; overdue wording calm ("complete or skip when you can"); prompt (optional) to record new-water salinity and temperature on completion |
| W7 | "Skip all overdue" with one reason | P3 | Several missed weekly changes otherwise need one skip each |
| W8 | Ordinal weekdays (`BYDAY=1SA`, "first Saturday") and "last day of month" | P3 | Rejected today with `invalid_rrule`; common enough to add with `BYSETPOS` |
| W9 | Gallon input | P3 | Volumes are litres only. If gallons are accepted, store the original value and unit and convert explicitly (US 3.785411784 L, Imperial 4.54609 L — never guess which) |

## 3c. Dosing (milestone 2.4 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| DS1 | Product stocks (bottle/bag on hand, deducted per dose, low-stock reminder) | P2 | `product_stocks` from the design is not built yet |
| DS2 | Target parameters on plans (e.g. alkalinity) | P2 | Needs the parameter catalog (2.1, D5) |
| DS3 | Curated product catalog seed | P2 | Seed data in `db/seed/` must be reviewed by the D5 domain expert; until then only private products exist |
| DS4 | Device-reported doses (pump integrations) | P3 | `reported_by = 'device'` is reserved; ingestion must be idempotent per device event id. ReefCare never controls a pump |
| DS5 | Web screens for products, plans and history | P2 | Milestone 2.10. Domain-review requirements: one-tap "Dosed {planned} {unit} of {product}?" (never auto-confirm, never "mark all done"); "When did you dose this?" for late doses (at the scheduled time / just now); warn when a late dose falls near the next one or another dose of the same product; confirm ad-hoc doses > ~3× the plan or in an unusual unit (heuristic); wording "Missed doses are not made up automatically…" (expert guidance, not established fact); drop-size hint for `drops`; "ReefCare does not know whether your pump delivered this dose" for unconfirmed |
| DS6 | Pump plans as a daily total | P3 | Micro-dosing pumps run 24–48+ times a day; today each occurrence becomes `unconfirmed`. Consider one occurrence per day (daily total) for `dosing_pump` plans |
| DS7 | Count-style units (scoop, capful, ampoule) and a "prepared solution" product linked to its dry product | P3 | Never treated as mass or volume; no derived concentration |
| DS8 | `reason_code` for water changes | P3 | Water changes still store the archive skip as `skip_reason = 'tank_archived'` (free text); dosing uses a separate `reason_code` column |
| DS9 | Composite FK from water_change_events (schedule_id, tank_id) to schedules | P3 | Dosing has it (plan_id, tank_id, product_id); add to water changes in a new migration |

## 3d. Equipment & maintenance (milestone 2.6 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| EQ1 | Shared equipment model catalog (`equipment_models`, specs) | P3 | Manufacturer/model are free text on each device today |
| EQ2 | Consumables on maintenance events (filter socks, carbon, media) | P3 | Not stored yet |
| EQ3 | Maintenance reminders | P2 | With notifications (2.9); overdue is already derived |
| EQ4 | Web screens for equipment, configurations and maintenance | P2 | Milestone 2.10 |
| EQ5 | Correcting a mistaken maintenance event, configuration version or `installed_at` | P3 | All are fixed today; a correction must be audited (timeline correction event), never a silent rewrite |
| EQ6 | Filtered maintenance history uses the `(col IS NULL OR col = $x)` pattern | P3 | Under a generic plan a filtered page may scan the tank's events (bounded per tank); split into per-filter queries if it shows up in query stats |

## 3e. Livestock (milestone 2.7 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| LS1 | Livestock and tank photos (decision D2) | P2 | Needs S3-compatible storage and the upload pipeline from `.claude/rules/security.md` (magic bytes, EXIF strip, signed URLs); build it as its own slice |
| LS2 | Curated species catalog seed | P2 | `db/seed/`, reviewed by the D5 domain expert; until then only private species. Add a unique index on `lower(scientific_name) WHERE owner_id IS NULL` with the seed |
| LS3 | Transfer between the keeper's own tanks in one step | P3 | Today: `transferred_out` here + a new entry (or `transferred_in`) there |
| LS4 | Accent-insensitive species search (`unaccent`/trigram index) | P3 | Plain lower-case substring today ("ca he" doesn't find "Cá hề") |
| LS5 | Structured observation details (symptoms, tissue/colour) | P3 | Phase 3 symptoms/intelligence; only a free-text note today |
| LS6 | Web screens for livestock | P2 | Milestone 2.10. Domain-review requirements: counting help ("a colony, a frag plug or a rock of zoanthids is 1"; partial die-off is an observation); "rehomed / traded / returned" maps to `transferred_out`; quantity is approximate for algae, pods and clean-up crews |
| LS7 | Enforce `current_quantity = Σ deltas` in the database | P3 | Go-only today (under the entry lock, tested); a deferred constraint trigger could recompute it (≤ 2000 moves per entry) |
| LS8 | Frag lineage (`parent_entry_id`) and approximate counts | P3 | From the domain review; also coral size / tissue extent later |
| LS9 | Search depends on the database locale for `lower()` of Vietnamese | P3 | Works with en_US.utf8; pin the production locale or solve with LS4 |

## 4. Security and identity follow-ups

| # | Item | Priority | Source | Notes |
|---|---|---|---|---|
| S1 | Rate limiting is per process | P2 | Decision D7 | Needs a shared store before running more than one API instance |
| S2 | Separate DB roles: app role DML-only, migration role for DDL, `REVOKE UPDATE/DELETE` on append-only tables | P2 | docs/security.md §5 | Today a single role is used; `audit_logs` is protected by a trigger only |
| S4 | Account pre-hijacking policy | P3 | Security review L6 (1.4) | Should verifying an old, never-verified registration also invalidate its password? (product decision). Shares already require a verified grantee |
| S7 | MFA / social login | P3 | Decision D10 | Post-MVP |
| S8 | Personal data export (JSON) | P3 | docs/security.md §9 | Phase 5 |
| S9 | Monitor `golang.org/x/crypto` advisory GO-2026-5932 (openpgp) | P3 | govulncheck | Not reachable from our code; no upstream fix |
| S10 | Share endpoint is an account-existence oracle | P3 | Security review L3 | Mitigated: verified owner, 30/h per owner, verified grantees only. Options: add a per-IP tier, audit `grantee_not_found` lookups |
| S12 | Retention period for `audit_logs` (incl. erased accounts' IP prefixes) | P2 | Security review L4 | Rows are append-only by trigger; decide a retention (e.g. 1 year) and a privileged purge path |
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
