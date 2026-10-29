# ReefCare — Product Requirements

Status: **Draft for review** (Phase 0) · Last updated: 2026-10-29

## 1. Problem

Reef hobbyists track water chemistry, dosing, water changes, equipment and livestock across
notebooks, spreadsheets, controller apps and forum threads. When something goes wrong (coral
recession, algae, fish loss) the relevant history is scattered, so both the owner and the people
they ask for help reason from isolated numbers. Advice in forums is unstructured, unverifiable and
rarely followed up with an outcome.

## 2. Product vision

ReefCare treats every aquarium as a **structured, evolving dataset**. Measurements, dosing, water
changes, equipment changes, livestock changes and observed symptoms live on one chronological
timeline. On top of that data, ReefCare provides:

1. **Reef management** — reliable record keeping, schedules and reminders.
2. **Reef Intelligence** — deterministic metrics and rule-based, evidence-linked *provisional*
   assessments grounded in a curated knowledge base.
3. **Reef Clinic** — community diagnostic cases that carry an immutable snapshot of the tank
   context, with reputation earned for genuinely helpful answers.

## 3. Target users and personas

| Persona | Needs | Primary features |
|---|---|---|
| **Beginner (Minh, first 6 months)** | What to test, how often, what "normal" is, what to do when numbers move | Targets, reminders, stale-data hints, plain-language assessments, Clinic |
| **Experienced keeper (Lan, SPS tank)** | Fast data entry, trends, consumption, correlation with changes | Bulk entry, charts with event annotations, dosing plan vs actual, timeline |
| **Community advisor** | Enough context to give good advice quickly | Case snapshot, parameter history excerpt, reputation |
| **Verified specialist** | Credible voice, review knowledge content | Specialist badge, knowledge review workflow |
| **Moderator / Admin** | Keep community healthy, handle abuse, manage accounts | Reports queue, moderation actions, audit log |

## 4. Market and localization

- Vietnamese-first launch (`vi-VN`), English (`en`) supported from day one in the i18n layer.
- Default timezone for new Vietnamese users: `Asia/Ho_Chi_Minh`; every user can change it.
- All timestamps stored in UTC; displayed in the user's timezone.
- Units: metric defaults (°C, L, ppt / SG, dKH); user-selectable display units.

## 5. Scope

### 5.1 MVP (Release 1) — "Reliable reef log with honest insights"

Corresponds to roadmap Phases 1, 2 and the deterministic part of Phase 3.

| Area | In MVP |
|---|---|
| Accounts | Register, login, logout, email verification, password reset, profile, language, timezone, unit preferences |
| Tanks | Multiple tanks, volumes (display / sump / estimated system), type, setup date, cycling status, salt mix, water source, status, private-by-default visibility, share with specific users (read-only) |
| Water parameters | Core 10 parameters + custom, multi-parameter test entry, method/kit, source, corrections with audit, targets, trends API, staleness |
| Dosing | Product catalog (user-defined + seeded common products), plans with schedules, materialized occurrences, actual dosing events, skip/fail reasons, simple stock tracking |
| Water changes | One-time and recurring schedules, overdue indicator, completion with actual volume, % of system volume estimate |
| Equipment & maintenance | Equipment instances, configuration history, maintenance plans and events |
| Livestock | Entries, events (add / death / transfer / sale / correction), derived quantity, health observations |
| Timeline | Unified, filterable by category and time range |
| Notifications | In-app notifications; email for reminders (opt-in); DB-backed jobs |
| Intelligence | Deterministic metrics, versioned rules, stale/missing data detection, parameter deviation & instability, evidence-linked assessments, assessment history and feedback. **No LLM in MVP.** |
| Knowledge base | Small curated, reviewed seed set backing the MVP rules; admin-only authoring |

### 5.2 Release 2 — Community

Phase 4: posts, Reef Clinic cases with snapshots, media, comments, votes, accepted answers, case
lifecycle, reputation ledger, ranks, badges, leaderboards, reports and moderation.

### 5.3 Later

Optional LLM-assisted assessment narrative (behind an interface), sensor/controller integrations,
CSV import from popular controllers, native mobile apps, specialist verification workflow,
public tank profiles.

### 5.4 Explicitly out of scope

- Automatic control of dosing pumps or any equipment.
- Definitive diagnosis of diseases or livestock deaths.
- Marketplace / livestock sales.
- Real-time chat.

## 6. Functional requirements (summary)

Detailed stories and acceptance criteria are in [user-stories.md](user-stories.md).

- **FR-1 Identity**: email + password accounts, verified email required for community writes,
  session cookies, password reset, account deletion with documented retention.
- **FR-2 Tanks**: owner CRUD; archive instead of delete when history exists; share read access
  with specific users; visibility per data category for public/case use.
- **FR-3 Water parameters**: record one or more readings at a measured time; preserve original
  value and unit; reject physically impossible values; flag atypical values; corrections create
  a new reading that supersedes the old one.
- **FR-4 Targets**: per tank, per parameter min/max (and optional ideal) in canonical units;
  changes are versioned so historical comparisons use the target valid at that time.
- **FR-5 Dosing**: plans with schedule + timezone; occurrences generated idempotently; actual
  events record planned vs actual amount, status and reason; stock decremented when known.
- **FR-6 Water changes**: schedules; overdue occurrences stay overdue until the user records the
  actual event or skips it; calculation outputs labeled as estimates with assumptions.
- **FR-7 Equipment**: instances of models; configuration changes create versions and timeline
  events; maintenance plans with intervals and overdue indicators.
- **FR-8 Livestock**: quantity derived from events; reconciliation event when actual count
  differs; nothing hard-deleted.
- **FR-9 Timeline**: every domain event appears with time, category, source and actor.
- **FR-10 Notifications**: reminders and alerts generated by persistent idempotent jobs.
- **FR-11 Intelligence**: on-demand and event-triggered assessments with stored input snapshot,
  rule version and knowledge references; user feedback on usefulness and outcome.
- **FR-12 Community (R2)**: see [domain-model.md](domain-model.md) §Community.

## 7. Non-functional requirements

| Category | Requirement |
|---|---|
| Security | OWASP ASVS L2-aligned; see [security.md](security.md) |
| Privacy | Tanks private by default; explicit sharing; account deletion within 30 days |
| Availability | Single-region; target 99.5% monthly for MVP; deterministic features independent of any LLM |
| Performance | p95 < 300 ms for dashboard, list and trend endpoints at MVP scale (to be verified by load tests — not a claim) |
| Data integrity | DB-enforced invariants, no silent history rewrites, daily backups with tested restore |
| Accessibility | WCAG 2.2 AA where practical; status never conveyed by color alone |
| Observability | Structured logs, request IDs, metrics, health/readiness |
| Portability | Docker-based; no lock-in to a single LLM, storage or device vendor |

## 8. Success metrics (MVP)

- Activation: % of new users who create a tank and log ≥ 1 water test within 7 days.
- Retention: % of active tanks with ≥ 4 water tests in the last 30 days.
- Reminder effectiveness: % of scheduled dosing / water-change occurrences that get recorded.
- Assessment usefulness: share of assessments rated "useful" in feedback; share of "wrong"
  feedback reviewed within 7 days.

## 9. Assumptions

- A1. Most users enter data manually from hobby test kits; sensor data comes later.
- A2. Users have a single owner per tank in MVP; co-owners are a later feature.
- A3. Email is the only login identifier in MVP (no social login).
- A4. Vietnamese copy will be written by the team; knowledge content requires a reviewer with reef
  expertise before publication.
- A5. Initial scale: up to ~10k users, ~20k tanks, ~5M water readings in year one.

## 10. Open product decisions

See [implementation-roadmap.md](implementation-roadmap.md) §Open decisions.
