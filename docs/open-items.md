# ReefCare — Open Items

Last updated: 2026-10-31 · State: Phase 1 done. Phase 2 done on branch `phase-1-foundation`:
2.3–2.9 backend, 2.10 web (water changes 2dc0b29, dosing 321ab17, equipment & maintenance
ed1d76d, livestock bea468b, notifications d4d5baa, tank dashboard fc30d55), and 2.1/2.2
parameter catalog, water tests, targets and trend charts. **The catalog values (conversions,
bounds, freshness) are provisional until the D5 reviewer confirms them (L3, §3g).**

Everything below is known to be **not done**, **partially done**, or **waiting on a decision**.
Sources: the design docs, the db-architect / go-reviewer / security-auditor reviews, and
verification runs. Remove an item only when it is done and verified; link the commit or PR.

Priority: **P1** blocks launch or the next milestone · **P2** needed before production ·
**P3** improvement / later phase.

## Done on 2026-10-30 (committed)

F1 profile (`PATCH`/`DELETE /me`) · F2 tanks, shares, `tanks.Authorizer`, timeline recorder,
cross-user 404 suite · F3 CI workflow + Renovate · F5 `api/openapi.yaml` with a route-sync test ·
F6 general write limit · F7 Prometheus metrics · S3 tokens bound to their email · S6 logout-all ·
P2 serializable retry · P3 field-level errors for custom JSON types (incl. nested and miscased
members) · R3 `/new-migration` skill · R5 Go installed locally on the Linux dev machine (not on the macOS one; the Makefile falls back to the `golang:1.27` container) · F4 web shell (`web/`, accepted
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
| L3 | Domain-expert review of unit conversions and rule thresholds (decision D5) | P1 for Phase 3 | The water-parameter catalog (`internal/waterparameters/catalog.go`: units, factors, hard bounds, freshness, unit-mistake warnings) was checked by the reef-domain-expert agent only and is labelled provisional in code, API and UI. Also covers the display-unit choices in `identity.unitPrefOptions` | Name a reviewer |
| L4 | Production deployment config (Caddy, HSTS, CSP, `frame-ancestors`, log redaction of query strings, metrics port kept private) | P2 | `deploy/` holds only dev config | Phase 5, or earlier for staging |
| L5 | Backups and restore drill | P2 | Not started | Phase 5 |
| L6 | Check that the `MAIL_FROM` mailbox is monitored, or add a reactivation link | P2 | The account-deactivated email tells a victim to reply; nothing reads replies yet (security review L6) | Decide with D6; reactivation within the 30-day window is a product decision |

## 2. Unfinished Phase 1 milestones

| # | Item | Priority | Notes |
|---|---|---|---|
| F8 | CI has never run on GitHub | P1 | `.github/workflows/ci.yml` (jobs go, web, e2e, docker) passes `actionlint` and every step passes locally; the GitHub remote exists and the branch is pushed, but no run has been checked yet (needs a PR, R1). Actions are pinned to commit SHAs |
| F9 | Serve the web build in production | P2 | `web/build` is a static SPA (`200.html` fallback); Caddy must serve it and proxy `/api`, `/healthz`, `/readyz` on the same origin, and add the response headers (part of L4). Until then the app can be framed: `frame-ancestors 'none'` cannot go in a meta CSP (web security review L1). Add a CI check that asserts the headers on the served `200.html` |
| F11 | Sign-out does not reach other open tabs | P3 | Other tabs show the user until their next API call returns 401 (web security review L4). Use a `BroadcastChannel` signal |
| F10 | Dev-only npm advisories | P3 | `npm audit` reports 4 low-severity issues in `deepmerge` via `@sveltejs/vite-plugin-svelte` (build tooling only); `npm audit --omit=dev` is clean and runs in CI. Wait for the upstream fix |
| F12 | SvelteKit's route announcer sets an inline style the CSP blocks | P3 | Console error on navigation (`style-src 'self'`); harmless for users, but noisy. Allow its hash in `kit.csp` or hide the announcer with a class |
| F13 | The tank dashboard makes 9 requests | P3 | Each block loads on its own (`TankDashboard.svelte`; latest water values added in 2.2). docs/api-guidelines.md planned `GET /tanks/{id}/dashboard` for one round trip; add it if the page feels slow on mobile networks |

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
| W1 | Measured salinity/temperature and pre/post water-test links on water changes | P2 | Unblocked: the parameter catalog and water tests exist (2.1/2.2); values stay provisional until D5 |
| W3 | ~~Web screens for schedules and water changes~~ | done | `web/src/routes/(app)/tanks/[id]/water-changes/`: overdue / upcoming / schedules / history, complete and skip dialogs, schedule create/edit with pause/resume/end, one-off record/plan; e2e `web/e2e/water-changes.e2e.ts` with axe. Schedule previews use the new `GET /api/v1/recurrence/preview` (`internal/scheduling`), which applies the server's own recurrence rules |
| W4 | `Idempotency-Key` header for retried creates (`POST /tanks/{id}/water-changes`) | P3 | Complete/skip are naturally idempotent; a retried POST of a one-off change can create a duplicate. Reuse the per-row pattern from dosing ad-hoc doses (idempotency_key + request_hash) |
| W5 | Dilution projection after n changes (docs/domain-model.md, with its caveats) | P3 | Needs measured new-water values (2.2), exact decimal powers, a batch/continuous `method` field, and D5 review |
| W6 | Remaining UI requirement from the domain review | P2 | Done in 2.10: capped month day note, next 3 dates before saving (server preview), schedule cards use `next_occurrences`, "< 0.1 %", calm overdue wording. **Open:** optional prompt to record new-water salinity and temperature on completion (needs 2.2 / W1) |
| W10 | Schedules with several times a day or several month days can't be edited in the form | P3 | The form edits one time and one month day; such rules (created through the API) show read-only with a note, and volume/notes/status stay editable |
| W11 | ~~"Upcoming" could miss the soonest changes~~ | done | The list now asks for the next 14 days only (dosing: next 48 h), so a newest-first page can no longer push the soonest items out |
| W7 | "Skip all overdue" with one reason | P3 | Several missed weekly changes otherwise need one skip each |
| W8 | Ordinal weekdays (`BYDAY=1SA`, "first Saturday") and "last day of month" | P3 | Rejected today with `invalid_rrule`; common enough to add with `BYSETPOS` |
| W9 | Gallon input | P3 | Volumes are litres only. If gallons are accepted, store the original value and unit and convert explicitly (US 3.785411784 L, Imperial 4.54609 L — never guess which) |

## 3c. Dosing (milestone 2.4 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| DS1 | Product stocks (bottle/bag on hand, deducted per dose, low-stock reminder) | P2 | `product_stocks` from the design is not built yet |
| DS2 | Target parameters on plans (e.g. alkalinity) | P2 | Unblocked by the catalog (2.1); values provisional until D5 |
| DS3 | Curated product catalog seed | P2 | Seed data in `db/seed/` must be reviewed by the D5 domain expert; until then only private products exist |
| DS4 | Device-reported doses (pump integrations) | P3 | `reported_by = 'device'` is reserved; ingestion must be idempotent per device event id. ReefCare never controls a pump |
| DS5 | ~~Web screens for products, plans and history~~ | done | `web/src/routes/(app)/tanks/[id]/dosing/` and `/dosing/products`. Domain-review requirements implemented and re-reviewed by reef-domain-expert (2026-10-31): per-dose dialog (no "mark all"); late doses start with no outcome and need "when"; one-tap confirm only for doses due within 6 h ("Record early…" with a warning otherwise); a failed/partial dose never defaults to the planned amount; same-product proximity hint over half the plan spacing (≥ 6 h, off for plans dosing more often than every 4 h; never-given doses ignored); ad-hoc check at > 3× the largest plan (pump plans: daily total) or an unusual unit; powder-as-solution and drop-size hints. Ad-hoc doses send an `Idempotency-Key`. e2e `web/e2e/dosing.e2e.ts` with axe |
| DS10 | No end-to-end test of resolving a missed or unconfirmed dose | P3 | Needs the scheduler to mark a dose missed (time travel); helpers are unit-tested. Add an API test hook or a seeded missed dose for e2e |
| DS11 | Products without an active plan get no ad-hoc sanity check | P3 | The > 3× / unit check needs a plan to compare with. Consider a per-product "typical dose" later |
| DS12 | Hint to keep alkalinity and calcium additions apart in time or place | P3 | Common practice to avoid precipitation (domain review); not a DS5 requirement. Needs product categories |
| DS6 | Pump plans as a daily total | P3 | Micro-dosing pumps run 24–48+ times a day; today each occurrence becomes `unconfirmed`. Consider one occurrence per day (daily total) for `dosing_pump` plans |
| DS7 | Count-style units (scoop, capful, ampoule) and a "prepared solution" product linked to its dry product | P3 | Never treated as mass or volume; no derived concentration |
| DS8 | `reason_code` for water changes | P3 | Water changes still store the archive skip as `skip_reason = 'tank_archived'` (free text); dosing uses a separate `reason_code` column |
| DS9 | Composite FK from water_change_events (schedule_id, tank_id) to schedules | P3 | Dosing has it (plan_id, tank_id, product_id); add to water changes in a new migration |

## 3d. Equipment & maintenance (milestone 2.6 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| EQ1 | Shared equipment model catalog (`equipment_models`, specs) | P3 | Manufacturer/model are free text on each device today |
| EQ2 | Consumables on maintenance events (filter socks, carbon, media) | P3 | Not stored yet |
| EQ4 | ~~Web screens for equipment, configurations and maintenance~~ | done | `web/src/routes/(app)/tanks/[id]/equipment/` (list with removed devices, add/edit, remove, free-form settings editor with typed values, configuration history with per-version change summary) and `.../maintenance/` (due now, plans with deactivate/reactivate, history, mark done and unplanned records with `Idempotency-Key`). e2e `web/e2e/equipment.e2e.ts` with axe |
| EQ7 | Settings are free-form per device | P3 | No per-category templates (e.g. light channels, pump % ) yet; consider suggested keys per category once real usage shows common ones |
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
| LS6 | ~~Web screens for livestock~~ | done | `web/src/routes/(app)/tanks/[id]/livestock/`: grouped by category with totals, gone groups on request, add with species search or a private species added inline, edit description, events (losses / arrivals / observations with hints: "missing" is never a death, "moved out / rehomed / traded / returned"), reconcile dialog; counting help ("a colony, a frag plug or a rock of zoanthids is 1", approximate counts for algae, pods and clean-up crews). Adds, events and reconciliations send an `Idempotency-Key`. e2e `web/e2e/livestock.e2e.ts` with axe |
| LS7 | Enforce `current_quantity = Σ deltas` in the database | P3 | Go-only today (under the entry lock, tested); a deferred constraint trigger could recompute it (≤ 2000 moves per entry) |
| LS8 | Frag lineage (`parent_entry_id`) and approximate counts | P3 | From the domain review; also coral size / tissue extent later |
| LS9 | Search depends on the database locale for `lower()` of Vietnamese | P3 | Works with en_US.utf8; pin the production locale or solve with LS4 |

## 3f. Notifications (milestone 2.9 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| NT1 | Reminders for stale measurements, threshold breaches and new assessments | P2 | Stale and off-target are now computable (`/water-tests/latest`); add a reminder source in `internal/waterparameters`. Assessments come with Phase 3 |
| NT2 | Upcoming-dose reminders | P3 | Not built: one per dose would be noisy; consider a daily "today's doses" summary |
| NT3 | Daily digest email (`email_digest`) and assessment email (`email_assessments`) | P3 | Keys reserved; only `email_reminders` is delivered |
| NT4 | Email provider for production | P1 | Decision D6; dev uses Mailpit. Add List-Unsubscribe and a one-click opt-out link |
| NT5 | ~~Web notification centre and settings~~ | done | `/notifications` (unread filter, open → tank section and mark read, mark all read, pagination), unread count in the navigation ("99+" when capped), per-category in-app reminder switches on the profile page. Unknown kinds render generically. e2e covers the page and preferences only: reminders come from the 15-minute scan (NT10) |
| NT10 | No end-to-end test with a real reminder | P3 | Needs a way to run the reminder scan on demand in e2e (test-only job trigger) or seeded notifications; rendering per kind is unit-tested |
| NT6 | Reminders for viewers of shared tanks | P3 | Owner only today |
| NT7 | Scan cost | P3 | Each scan re-emits the whole window (≤ 7 days) and inserts row by row (ON CONFLICT no-op); batch with `unnest` or anti-join on dedupe keys if scan time grows |
| NT8 | First deploy sends one backlog email | P3 | `email_reminders` was settable before delivery existed; the first scan emails up to 7 days of overdue items once (counts only). Accepted: no production users yet |
| NT9 | One-click unsubscribe | P2 | With NT4: `List-Unsubscribe` + signed token turning `email_reminders` off without login |

## 3g. Knowledge base (milestone 3.1 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| KB1 | Seed content | P1 for Phase 3 release | Empty until the D5 reviewer writes or approves articles with real sources; nothing is generated. Rules work without knowledge refs |
| KB2 | Markdown rendering | P3 | Bodies are shown as plain text (`white-space: pre-wrap`); rendering Markdown needs a sanitizer and a review |
| KB3 | "Verified specialist" reviewers | P3 | Only `administrator` edits and reviews; a separate specialist role (docs/knowledge-engine.md §2) is not built |
| KB4 | Accent-insensitive knowledge search | P3 | Same as LS4 |
| KB5 | Editor lists are capped without a cursor | P3 | Sources 1000, articles 2000, published list 500 (sorted by title); fine for a curated KB, add keyset pagination if it grows |
| KB6 | Revoking the last administrator | P3 | The CLI allows it; recovery is another `roles grant` |
| KB7 | Abandoned drafts are never deleted | P3 | Versions are referenced by history; add an explicit "discard draft" if clutter grows |

## 3h. Reef Intelligence engine (milestones 3.2/3.3 done)

| # | Item | Priority | Notes |
|---|---|---|---|
| IN1 | D5 confirmation of every rule threshold | P1 for release | All values in `internal/intelligence/rules.go` and docs/knowledge-engine.md §5 are provisional; open questions from the domain review: nitrite urgency (warning alone, urgent with ammonia?), hyposalinity handling, a test-kit range-maximum reading treated as "≥", time-of-day for pH comparisons |
| IN2 | Unit context in ammonia findings | P3 | The verdict uses canonical NH3; add the entered value and unit to the observation |
| IN3 | `found` events don't offset `missing` in LC-LOSSES | P3 | Minor; a later find is shown in the history |
| IN4 | Dosing adherence linked to the plan's target parameter | P2 | Needs DS2 |
| IN5 | Notifications for urgent assessments and daily staleness | P2 | docs §7 "daily staleness" and an urgent-finding reminder need a notifications category `assessment` (migration + source); not built (NT1) |
| IN6 | Rate limit for manual assessments is per process | P3 | Same as S1 |
| IN7 | Snapshot bounds | P3 | 90 days, ≤ 5000 readings, 5000 dosing events, 2000 livestock events; past a limit only the newest rows are kept and `DQ-SNAPSHOT-LIMIT` says so. Sensor imports will need per-parameter windows |
| IN8 | Retention of automatic assessments | P3 | Each keeps a snapshot (≤ 3.5 MB); bounded to 24 per tank per day, but nothing is ever purged. Consider keeping only the latest automatic one per day after 90 days |
| IN11 | Reviewer view of "wrong" feedback | P2 | Feedback is stored (index on wrong ratings) but no admin page lists it yet; reviewers query it for now |
| IN12 | Evidence drill-down in the web app | P3 | Findings show how many facts back them; linking each to its reading or event page is not built |

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
| R1 | Work is on branch `phase-1-foundation`; nothing merged to `master` | P1 | The remote exists (GitHub) and the branch is pushed, but no PR is open | Open a PR to `master`; check that CI ran (F8) |
| R7 | `.claude/settings.json` hooks call `/home/feng/.local/bin/graphify`, an absolute path from one machine | P2 | On other machines (e.g. macOS) the hook command does not exist; the hooks fail on every Bash/Grep/Read/Glob call (non-blocking, but noisy and not doing their job) | Install graphify on every dev machine and call it via `PATH` or `$CLAUDE_PROJECT_DIR`, or move these hooks to each developer's `.claude/settings.local.json` |
| R8 | E2E without local Go | P3 | `web/playwright.config.ts` now accepts `E2E_API_URL` to use an API that is already running (e.g. in Docker) instead of `go run` | Documented in web/README.md |
| R9 | E2E registration budget | P3 | Registration allows 5 per hour per IP and all e2e traffic shares one address. Module tests now share one account from `web/e2e/global-setup.ts` (3 sign-ups per run). Two local runs within an hour against the same API still hit the limit: restart the API to clear its in-memory limiter |
| R4 | Go module path is `reefcare` | P3 | The remote is github.com/mockingbitch/reefcare | Rename the module path to it when convenient (touches every import) |

## 3g. Water parameters (milestones 2.1/2.2 done, values provisional)

Backend `internal/waterparameters` (catalog, tests, corrections, versioned targets, latest,
trends with annotations), migrations 00011–00012, web `/tanks/{id}/water-tests` (latest values,
history with corrections, entry form, targets, trend charts) and a dashboard block. Reviewed by
reef-domain-expert (catalog) and go-reviewer; all their major findings were fixed.

| # | Item | Priority | Notes |
|---|---|---|---|
| WP1 | Confirm catalog values with the D5 reviewer | P1 | See L3. Until then the UI says "awaiting expert review" on the latest-values card |
| WP2 | Specific gravity is assumed 25/25 °C | P2 | Readings converted from SG are flagged `estimate`. Non-ATC hydrometers at other temperatures would need a temperature input |
| WP3 | Free-ammonia (NH3 only) kits are not accepted | P3 | Their value depends on pH and temperature; only total ammonia as NH3, NH3-N or NH4+ is accepted |
| WP4 | Values are shown in the canonical unit | P2 | Unit preferences only preselect the input unit. Converting displayed values and targets to the preferred unit needs the reverse conversions (same D5 review) |
| WP5 | Extra parameters (K, I, Sr, Fe, ORP, TDS, …) and ICP imports | P2 | Catalog has 10 system parameters; custom parameters and ICP result import (`source = imported`) are not built |
| WP6 | Sensor ingestion | P3 | `source = sensor` exists in the schema; no device API. Trends cap at 3000 points per series (newest kept, `truncated` flag) |
| WP7 | Unit-mistake warnings are shown after saving | P3 | The reading is saved and the warning shown with a "Correct" path. A confirm-before-save would need a dry-run endpoint |
| WP8 | Corrections are not idempotent | P3 | Documented: a retry after a lost response gets `already_superseded`; reload shows the correction |
| WP9 | Chart interactions | P3 | No zoom/brush or date picker (30/90/365 days); target bands only appear from when a target took effect |
| WP10 | Past readings keep the target in effect when measured | — | By design (history in tests and charts). `/latest` compares the newest value with the current target so the dashboard answers "is it fine now?" |

## 7. Open product and infrastructure decisions

Accepted with defaults on 2026-10-29, but these still need a concrete owner or choice:

| ID | Decision | Current default | Needed by |
|---|---|---|---|
| D5 | Who reviews thresholds and seed knowledge | — (unassigned) | Phase 3 |
| D6 | Production email provider (SES, Postmark, other SMTP) | Mailpit locally | Before staging |
| D7 | Hosting and managed PostgreSQL | Single VPS + managed PG, Singapore | Before staging |
| D9 | CI platform and dependency bot | GitHub Actions + Renovate (configured) | First run with the PR (R1) |
| D11 | Chart library | **Decided 2026-10-31: no library.** One hand-written SVG chart (`TrendChart.svelte`, ~300 lines) covers the needs: one series per chart, target bands, event markers, crosshair tooltip by pointer and keyboard, text summary and table. ECharts (~1 MB) stays the fallback if zoom/brush or dense sensor data are needed (WP9) | — |
| — | Local S3-compatible storage (MinIO's status is uncertain; Garage or SeaweedFS) | Not chosen | When media ships (Phase 2.7 / 4) |
