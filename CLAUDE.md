# ReefCare — Claude Code Project Guide

ReefCare is a production-oriented platform for marine reef aquarium management: water parameter
monitoring, dosing, water changes, equipment, livestock, a unified tank timeline, a curated
knowledge base, a provisional-assessment "Reef Intelligence" engine, and a community "Reef Clinic"
with reputation.

It is **not** a generic CRUD app or social network. Every tank is a structured, evolving dataset;
measurements, dosing, water changes, equipment changes, livestock changes and symptoms are linked
through a chronological timeline, and analysis must use that context, not isolated values.

Act as Principal Architect, Senior Go Engineer, Product Analyst, PostgreSQL Architect and UI/UX
Designer. Detailed, path-scoped rules live in `.claude/rules/` and load automatically.

## Current status

- Phase 0 design docs in `docs/` were accepted with the proposed defaults for D1–D12
  (2026-10-29); the docs are the source of truth for design decisions.
- Phase 1 — Foundation is complete (2026-10-30). Milestone 1.1 (skeleton, config, logging, chi server,
  health/readiness, graceful shutdown, pgx pool, embedded Goose migrations, Docker/Compose,
  Makefile, golangci-lint) is done. 1.2 (sqlc → `internal/db/dbq`, `database.InTx`,
  `dbtest.New` per-test databases, identity schema + queries) and 1.3 (`apperr` typed errors,
  `httpserver.DecodeJSON` / `WriteAppError` / pagination / `ClientIP`, `ratelimit`) are done.
  1.4 identity is done (`internal/identity`: register, verify email, login/logout, sessions
  with auth_epoch revocation, CSRF, password reset/change, audit, async mail via
  `internal/platform/mail`). 1.5 profile (`PATCH`/`DELETE /me`, logout-all), 1.6 tanks
  (`internal/tanks` with `Authorizer`, shares, `internal/timeline` recorder), 1.7 CI
  (`.github/workflows/ci.yml`, Renovate) and `api/openapi.yaml` (route-sync test in
  `internal/app`) are done and reviewed (2026-10-30). Shared helpers: `platform/decimal`
  (exact numerics ↔ `numeric`), `platform/patch` (merge-patch fields), `platform/textcheck`,
  `platform/metrics`, `internal/apptest` (API integration harness).
  1.8 web shell is done (`web/`: SvelteKit 3 SPA, paraglide vi/en, typed client generated
  from `api/openapi.yaml`, Playwright e2e with axe; see `web/README.md`). **Phase 1 is
  complete.**
- Active phase: **Phase 2 — Reef management.** 2.3 job runner is done (`internal/platform/jobs`;
  identity emails, failed-login audits, purges and 30-day account erasure run as jobs; modules
  register account erasers with `identity.Service.OnErase`). 2.5 water changes is done
  (`internal/waterchanges`, shared recurrence subset in `internal/platform/recurrence`; tank-owned
  modules mount under `/tanks/{tankID}` via `tanks.Handler.Mount(api, subs...)` and authorize with
  `tanks.Authorizer`). 2.4 dosing is done (`internal/dosing`: products, plans, missed-dose
  scheduler, resolve, ad-hoc doses with per-row Idempotency-Key). 2.6 equipment & maintenance is done
  (`internal/equipment`: devices removed-not-deleted, immutable configuration versions with
  timeline events, interval maintenance plans with derived overdue, performed/unplanned events).
  2.7 livestock is done (`internal/livestock`: private species, entries whose quantity is the
  event sum with no negative balance in event order, events, reconcile; photos are open item LS1).
  2.8 timeline feed is done (`GET /tanks/{id}/timeline` in package `tanks`: category and time
  filters, keyset pagination). 2.9 notifications is done (`internal/notifications`: modules
  register reminder sources, a 15-minute scan stores one notification per occurrence, opt-in
  batched email, per-category prefs). 2.10 web screens in progress: water changes done
  (`web/src/routes/(app)/tanks/[id]/water-changes/`, `#lib/recurrence.js`, schedule previews via
  `GET /recurrence/preview` in `internal/scheduling` — never reimplement recurrence in the web
  app). Dosing done (`web/src/routes/(app)/tanks/[id]/dosing/`, `/dosing/products`,
  `#lib/dosing.js`; `api()` takes extra headers, e.g. `Idempotency-Key`). Equipment &
  maintenance done (`.../equipment/`, `.../maintenance/`, `#lib/equipment.js`). Note: Svelte
  binds `<input type="number">` as a number — never call string methods on such values.
  Livestock done (`.../livestock/`, `#lib/livestock.js`). E2E module tests share one account
  from `web/e2e/global-setup.ts` (registration is rate limited per IP).
  Notifications done (`/notifications`, `#lib/notifications.js`, unread count in
  `#lib/unread.svelte.js`). Tank dashboard done (`TankDashboard.svelte`, `#lib/timeline.js`);
  2.10 is complete. 2.1/2.2 water parameters done (`internal/waterparameters`: catalog in Go
  mirrored by `parameter_types`, tests with readings kept as entered + exact canonical value,
  corrections supersede (append-only triggers), versioned targets, `/latest`, `/trends`; web
  `.../water-tests/`, `#lib/water-tests.js`, SVG `TrendChart.svelte` — D11 decided: no chart
  library). Catalog values are **provisional until the D5 reviewer confirms them** (open item
  L3/WP1). **Phase 2 is complete.**
- Active phase: **Phase 3 — Reef Intelligence** (started 2026-10-31 on request). 3.1 knowledge
  base is done (`internal/knowledge`: articles, per-locale immutable versions, sources, review
  workflow with four eyes and DB-frozen content; editors = role `administrator`, granted with
  `api roles grant <email> <role>`; web `/knowledge`, `/admin/knowledge`, `#lib/knowledge.js`).
  No seed content until the D5 reviewer provides it (open item KB1) — never generate articles
  or sources. Rule thresholds stay provisional until D5 confirms them.
  3.2/3.3 metrics and rules v1.0.0 are done (`internal/intelligence`: pure `Evaluate(Snapshot)`,
  11 rules incl. welfare bounds, evidence validation, golden fixtures in `testdata/`; update
  goldens with `go test ./internal/intelligence -run TestRulesOnFixtures -args -update` only
  after checking the change). 3.4 assessment service is done (`intelligence.Service`: snapshot
  from `waterparameters.ForAssessment` + registered `intelligence.Source`s, persists immutable
  assessments with evidence and knowledge refs, timeline `assessment.created`, manual POST and the
  `intelligence.assess_after_test` job). 3.5 history and feedback is done (feedback + retraction
  API; web `.../assessments/`, `#lib/assessments.js` renders `ia_*` messages — every rule text key
  must have vi/en messages, checked by `TestEveryTextKeyIsTranslated`). Next: 3.6 optional LLM
  narrative adapter (ask which provider first; default none).
- Unfinished work, known gaps and pending decisions are tracked in `docs/open-items.md`;
  update it when an item is completed or a new gap is found.
- Go module path is `reefcare`. Migrations use sequential numbering (`make migration name=...`).
- Go may not be installed locally; the Makefile falls back to the `golang:1.27` container.
- Do not generate the whole application. After Phase 0, summarize the architecture and **wait for
  review** before large-scale implementation, unless explicitly told to proceed (then one phase at
  a time, verifying each).
- Update this section when a phase is completed.

## Stack (unless the repo later establishes a better-justified alternative)

- Backend: Go (latest stable), REST/JSON under `/api/v1`, `chi` router, `pgx/v5`, `sqlc`,
  Goose migrations, `log/slog`, `context.Context` everywhere, OpenAPI in `api/openapi.yaml`.
- Database: PostgreSQL (supported stable major) is the source of truth.
- Frontend: SvelteKit + TypeScript in `web/` (propose and document before implementing).
- Infra: Docker + Docker Compose (`compose.yaml`), S3-compatible object storage, CI pipeline.
- Do **not** add ORMs, DI frameworks, Kubernetes, Kafka, microservices, or caching layers without
  a measured, documented need. Prefer few, well-maintained dependencies.

## Architecture: modular monolith

```text
cmd/api/main.go
internal/{app,config,platform/{database,httpserver,logging,clock},identity,users,tanks,
          waterparameters,dosing,waterchanges,equipment,livestock,maintenance,timeline,
          knowledge,intelligence,community,reputation,notifications,moderation,audit}
db/{migrations,queries}   api/openapi.yaml   web/   deploy/   scripts/   docs/
```

Create packages only when a phase needs them — no empty placeholder packages.

Dependency rules:
- Handlers translate HTTP ↔ application commands/queries. No business logic in handlers.
- Application services own workflows and **explicit** transaction boundaries.
- Domain logic is pure: no HTTP, SQL driver, or frontend imports.
- Data access uses sqlc-generated code; no generic repository abstractions hiding SQL.
- Constructor injection, no global mutable state, no circular package dependencies.
- External providers (LLM, object storage, devices) sit behind small interfaces.

## Non-negotiable domain rules

- Store timestamps in UTC (`TIMESTAMPTZ`); render in the user's timezone. Vietnamese-first, i18n-ready.
- Exact numerics (`NUMERIC`) for measurements and dosing quantities; preserve original value + unit.
- Display volume ≠ total system volume — store both explicitly.
- Never silently rewrite history (observations, livestock, case snapshots). Corrections are audited.
- A missed schedule is never auto-marked completed.
- Intelligence output is a **provisional assessment**, never a diagnosis; it never executes dosing
  or equipment commands; it never fabricates sources. Deterministic features must work without an LLM.
- Community rank is not an authorization role. Reputation is a ledger, not a mutable score.
- Every tank-owned resource is authorization-checked; private tank data never leaks via public endpoints.

## Workflow for every coding task

1. **Inspect** the repo, docs, conventions; don't recreate working modules.
2. **Analyze**: user problem, actors/permissions, business rules, data model, API contract,
   failure cases, security, acceptance criteria, explicit assumptions.
3. **Design** a short plan: files, schema/migrations, endpoints, domain logic, tests, risks.
4. **Implement** in small reviewable increments; migrations + sqlc + validation + authz + tests +
   OpenAPI + docs together. No placeholder code to fake progress.
5. **Verify** with real commands (see below).
6. **Review** for logic errors, data loss, races, missing authz, SQL inefficiency, duplication.
7. **Summarize**: what was implemented, decisions, files, migrations, tests with *actual* results,
   limitations, next step.

Ask a clarifying question only when a decision materially affects correctness or causes major
rework; otherwise choose a reasonable default and document it. Use the `/feature`, `/verify`,
`/new-migration` and `/assessment-rule` skills for the matching workflows.

## Commands

```sh
make up / down / logs   # dev stack (host ports 4000 API · 4001 web · 4002 postgres · 4003/4004 mailpit)
make fmt | fmt-check    # gofmt
make lint               # go vet (incl. integration tag) + golangci-lint v2 (container)
make test               # unit tests, -race
make test-integration   # -tags=integration against compose db `reefcare_test` (needs `make up`)
make build | vuln       # bin/api | govulncheck
make migration name=x   # new sequential Goose migration
make migrate-up | migrate-down | migrate-status
make sqlc | sqlc-check  # sqlc 1.31 (container), config db/sqlc.yaml
# Metrics: METRICS_ADDR (compose :9090, internal only) → GET /metrics
make web-install | web-dev | web-check | web-test | web-build | web-api-check | web-audit
make web-e2e            # Playwright vs real API (needs `docker compose up -d db mailpit`)
```

If a command does not exist yet or a tool is missing locally, say so — never report a check as
passed unless it actually ran and passed.

## Definition of Done

Business rules documented · schema + migrations correct · authorization implemented · API in
OpenAPI · validation and error handling · automated tests for critical behavior · usable frontend
states (loading/empty/error/success) · no sensitive data in logs · reproducible migrations and
codegen · docs match code · verification reported honestly.

## Roadmap

- **Phase 0** — `docs/`: product-requirements, user-stories, domain-model, architecture,
  database-design (with ER diagram), api-guidelines, security, knowledge-engine,
  implementation-roadmap; acceptance criteria, risks, open product decisions.
- **Phase 1** — Foundation: bootstrap, config, DB, migrations, sqlc, logging, health, auth,
  profiles, tank CRUD, ownership policies, Compose, CI.
- **Phase 2** — Reef management: water tests, targets, trends API, dosing, water changes,
  equipment, maintenance, livestock, timeline, notifications/jobs.
- **Phase 3** — Reef Intelligence: knowledge base, deterministic metrics, rules, evidence-linked
  assessments, history, feedback, optional LLM behind an interface.
- **Phase 4** — Community: posts/media, cases, comments, votes, outcomes, reputation ledger,
  ranks, badges, leaderboards, moderation.
- **Phase 5** — Production readiness: security review, query tuning, backups, observability,
  rate limiting, load tests, deployment, ops docs.

Complete and verify each vertical slice before expanding scope.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- When graphify-out/graph.json exists, run `graphify query "<question>"` before your first grep, search or source-file read for a codebase question, and put this rule in every subagent prompt that explores code. Read raw files only for lines the graph or an earlier read already located. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
- A committed graphify-out/graph.json is rewritten whole on every build, so a large diff to it in a pull request is expected. Do not flag it as churn or suggest skipping the graph rebuild or its hooks.
