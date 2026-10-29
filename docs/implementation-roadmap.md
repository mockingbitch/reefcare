# ReefCare — Implementation Roadmap

Status: **Draft for review** (Phase 0)

Each milestone is a vertical slice (migration → queries → domain → service → API → OpenAPI → tests
→ minimal UI) and is verified before the next one starts.

## Phase 0 — Product and technical design ✅ drafted, awaiting review

Deliverables: the documents in `docs/`. Exit: open decisions below resolved or explicitly deferred.

## Phase 1 — Foundation

| # | Milestone | Acceptance criteria |
|---|---|---|
| 1.1 | Project skeleton | `go.mod`, `cmd/api`, config with fail-fast validation, slog JSON logging, chi server, `/healthz`, `/readyz`, graceful shutdown; `Makefile`; `Dockerfile` (distroless, non-root); `compose.yaml` with PostgreSQL 18 (healthcheck, volume) + Mailpit; `.env.example`; `.golangci.yml` |
| 1.2 | DB plumbing | pgxpool config, Goose embedded migrations + CLI target, sqlc config with decimal/uuid overrides, `InTx` helper, test helper creating an isolated database per package; migration up/down/up passes |
| 1.3 | HTTP platform | request id, access log, recover, body limit, error envelope + mapper, JSON decode with unknown-field rejection, pagination cursor helper, rate limiter |
| 1.4 | Identity | register, verify email, login/logout, sessions, CSRF, password reset, change password; rate limits; audit log; API tests incl. enumeration-safe responses |
| 1.5 | Profile | `GET/PATCH /me`, timezone/locale validation, account deactivation request |
| 1.6 | Tanks + authorization | tank CRUD/archive, shares, `Authorizer`, timeline recorder with `tank.created`; cross-user 404 test suite |
| 1.7 | CI | GitHub Actions running the `/verify` checklist |
| 1.8 | Web shell | SvelteKit app: i18n (vi/en), auth pages, tank list/create/detail, accessible layout & navigation |

Exit criteria: all checks green in CI; a user can register, verify, log in, create and share a tank.

## Phase 2 — Reef management

| # | Milestone | Key acceptance criteria |
|---|---|---|
| 2.1 | Parameter catalog + unit conversion | Conversion functions with table tests reviewed by domain expert; hard bounds seeded |
| 2.2 | Water tests + corrections + targets | US-WP-1…5; idempotent creation; trend + dashboard endpoints with EXPLAIN-checked queries |
| 2.3 | Job runner | `jobs` table, worker pool, leases, backoff, dedupe; integration tests for crash/retry |
| 2.4 | Dosing | products, stocks, plans (RRULE subset), materialization, resolve, missed marking, history |
| 2.5 | Water changes | schedules, overdue derivation, completion/skip, estimates with assumptions |
| 2.6 | Equipment & maintenance | instances, configuration versions → timeline, maintenance plans/events |
| 2.7 | Livestock | entries, events, quantity invariant under concurrency, reconciliation |
| 2.8 | Timeline | unified feed, filters, keyset pagination |
| 2.9 | Notifications | in-app + email (opt-in), dedupe, preferences |
| 2.10 | Web | dashboard, entry forms, charts with targets & annotations + accessible table alternative, dosing/water-change/livestock/equipment screens |

Exit: a keeper can run their tank in ReefCare for a month with reminders working.

## Phase 3 — Reef Intelligence

| # | Milestone | Key acceptance criteria |
|---|---|---|
| 3.1 | Knowledge base | articles, versions, sources, review workflow (admin UI minimal), seed set reviewed |
| 3.2 | Metrics library | pure functions + tests over fixtures |
| 3.3 | Rules v1.0.0 | rules in knowledge-engine.md §5 with golden tests |
| 3.4 | Assessment service | pipeline, validation step, persistence, timeline event, triggers, rate limits |
| 3.5 | History & feedback | list/detail/feedback endpoints and UI |
| 3.6 | LLM adapter (optional) | interface, `none` default, one provider implementation behind config, LLM-down tests |

## Phase 4 — Community (Release 2)

Media pipeline → posts & cases with snapshots → comments/answers → votes → accept/lifecycle/outcomes →
reputation ledger + ranks → badges + leaderboards → reports & moderation. Each with abuse tests
(self-vote, duplicates, concurrent votes, duplicate rewards).

## Phase 5 — Production readiness

Security review (security-auditor + external if possible), query optimization with real-ish data
volumes, backups (nightly dump + PITR decision) with restore drill, metrics dashboards and alerts,
load tests (k6) for dashboard/trends/timeline/water-test create, deployment automation, runbooks,
data export.

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Incorrect chemistry conversions or thresholds | Harmful advice, loss of trust | Domain-expert review, tests from published conversion factors, conservative "re-test first" rules |
| Users read assessments as diagnoses | Livestock harm, liability | Wording rules, visible "provisional" labeling, no dose amounts in MVP, disclaimers |
| Recurring schedule edge cases (DST, timezone change, plan edits) | Wrong reminders | RRULE in plan TZ, regenerate future occurrences on edit, tests incl. DST zones (VN has none, but users abroad may) |
| Manual data entry friction | Low retention → weak intelligence | Multi-parameter fast entry, last-used kits, mobile-first form |
| Scope creep (community + intelligence + management at once) | Delays | Strict phase gates; community in Release 2 |
| Reputation gaming | Untrustworthy advice | Ledger, caps, delayed finalization, moderation |
| Knowledge content bottleneck (needs expert review in Vietnamese) | Rules without backing | Small seed set; rules can ship with `expert` content; recruit reviewers early |
| Single-instance in-memory rate limiting | Ineffective when scaled out | Documented; move to shared store when >1 instance |
| Object-storage choice for local dev | Rework | Defer until media ships; code against S3 API only |

## Open decisions (need product owner input)

| ID | Decision | Default proposed |
|---|---|---|
| D1 | MVP boundary: is the community (Release 2) required for launch? | No — launch with management + deterministic intelligence |
| D2 | Do livestock/tank photos ship in MVP (requires object storage earlier)? | One tank cover photo + livestock photos in Phase 2.7, private only |
| D3 | Job runner: in-house table vs `riverqueue/river` | In-house (small scope) |
| D4 | Tank sharing in MVP: viewer only, or also "advisor" with comment rights? | Viewer only |
| D5 | Who reviews thresholds and seed knowledge (named reef expert)? | Required before Phase 3 release |
| D6 | Email provider for production (SES, Postmark, local SMTP)? | Decide in Phase 1.4; Mailpit locally |
| D7 | Hosting target (VPS in VN/SG, managed PG provider) | Single VPS + managed PostgreSQL in Singapore region |
| D8 | Video uploads in community (cost, moderation) | Images only in Release 2 |
| D9 | CI/CD platform and dependency bot | GitHub Actions + Renovate |
| D10 | MFA / social login timeline | Post-MVP |
| D11 | Chart library | ECharts, confirm in 2.10 spike |
| D12 | Data retention for deleted accounts' public answers | Anonymize, keep content |
