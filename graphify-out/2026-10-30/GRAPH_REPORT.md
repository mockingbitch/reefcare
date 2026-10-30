# Graph Report - reefcare  (2026-10-30)

## Corpus Check
- 204 files · ~175,907 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 2803 nodes · 8021 edges · 142 communities (106 shown, 36 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 255 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `67ce5e4c`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- English UI Messages
- Vietnamese UI Messages
- Service
- New
- Identity Integration Tests
- Outbox
- Decimal
- Typed API Client
- Svelte UI Components
- DBTX
- Docker Compose Stack
- New
- Session and Locale Runtime
- Profile Page Form
- time.Time
- Identity HTTP Handlers
- Account Security Page
- serve
- Tank HTTP Handlers
- models.go
- Golangci-lint Config
- Tank Authorization
- Frontend Error Handling
- Tank List and Labels
- Date Formatting and Tank Detail
- github.com/google/uuid.UUID
- sqlc Configuration
- Frontend Dev Tooling
- Tank Sharing Page
- Playwright E2E Tests
- ESLint Configuration
- Renovate Config
- Web Package Manifest
- NPM Scripts
- App Wiring and Config
- Typed App Errors
- Load
- IsKind
- CI Workflow Jobs
- net/http.Handler
- SvelteKit Build Config
- Claude Settings Hooks
- CI Workflow Steps
- CI Action Inputs
- Rate Limiter
- E2E CI Services
- CI Postgres Service
- Prometheus HTTP Metrics
- TypeScript Config
- Inlang i18n Settings
- Web CI Job
- app.go
- Generated OpenAPI Types
- DB Pool Metrics Collector
- Vitest Component Tests
- Go CI Job
- CI Triggers
- livestock/http_integration_test.go
- DecodeJSON
- Prettier Config
- Go Format Hook
- File Guard Hook
- checker
- SvelteKit App Types
- Go Module Root
- Web Package Root
- checker
- Runner
- NewValidator
- Validator
- Service
- Date
- Env
- Service
- checker
- WriteJSON
- Handler
- CreateSpeciesParams
- waterchanges/http_integration_test.go
- MustParse
- Parse
- recurrence_test.go
- NewRouter
- Migrator
- NewRunner
- 5. Initial endpoints
- 2. Core concepts
- ReefCare — Knowledge Base and Reef Intelligence Engine
- dosing/http_integration_test.go
- ReefCare — Security Design
- parser
- Hasher
- ReefCare — Architecture
- ReefCare — User Stories and Acceptance Criteria
- .Perform
- Reef management domain rules
- ReefCare — Open Items
- ReefCare — Product Requirements
- checker
- .Timeline
- ReefCare — Claude Code Project Guide
- Queries
- ReefCare — Database Design
- ListEquipmentRow
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- product-requirements.md
- SMTPSender
- ListLivestockEntriesRow
- Community, Reef Clinic and reputation rules
- identity/jobs_integration_test.go
- PostgreSQL, migrations and sqlc
- config_test.go
- writeEvent
- ParsePageParams
- Handler
- GetLivestockEventByIdempotencyKeyRow
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- GetMaintenanceEventByIdempotencyKeyRow
- jobOrigin
- toSpecies
- CountJobsByStatusRow
- DeactivateTankMaintenancePlansParams
- DeactivateUserParams
- InsertTankEventParams
- ListLivestockEventsParams
- SetWaterChangeMaterializedUntilParams
- SkipWaterChangeEventParams
- UpdateMaintenancePlanParams

## God Nodes (most connected - your core abstractions)
1. `New()` - 89 edges
2. `Decimal` - 77 edges
3. `Actor` - 63 edges
4. `WriteJSON()` - 59 edges
5. `Queries` - 52 edges
6. `InTx()` - 50 edges
7. `DecodeJSON()` - 44 edges
8. `New()` - 42 edges
9. `ActorFrom()` - 42 edges
10. `NewValidator()` - 41 edges

## Surprising Connections (you probably didn't know these)
- `3. Authentication` --references--> `Origin`  [INFERRED]
  docs/api-guidelines.md → internal/audit/audit.go
- `4. Web protections` --references--> `Origin`  [INFERRED]
  docs/security.md → internal/audit/audit.go
- `6. Rate limits (initial)` --references--> `RateLimitKey()`  [INFERRED]
  docs/api-guidelines.md → internal/platform/httpserver/clientip.go
- `Dependency rules` --references--> `Authorizer`  [INFERRED]
  docs/architecture.md → internal/tanks/authz.go
- `Tanks` --references--> `tankID()`  [INFERRED]
  docs/api-guidelines.md → internal/waterchanges/http.go

## Import Cycles
- 3-file cycle: `internal/app/app.go -> internal/dosing/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/equipment/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/livestock/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterchanges/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`

## Communities (142 total, 36 thin omitted)

### Community 0 - "English UI Messages"
Cohesion: 0.01
Nodes (211): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+203 more)

### Community 1 - "Vietnamese UI Messages"
Cohesion: 0.01
Nodes (211): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+203 more)

### Community 2 - "Service"
Cohesion: 0.14
Nodes (14): GetLivestockEntryRow, ListLivestockEventsRow, stored(), checkKey(), entryFromRow(), eventFromRow(), hashIf(), speciesCoralType() (+6 more)

### Community 3 - "New"
Cohesion: 0.12
Nodes (28): Entry, Client, LoginInput, Principal, RegisterInput, Service, Session, User (+20 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.13
Nodes (37): MustURL(), New(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable(), TestHostileInputs(), TestLoginFailuresAreUniformAndRateLimited(), TestPasswordResetRevokesAllSessions() (+29 more)

### Community 5 - "Outbox"
Cohesion: 0.29
Nodes (4): Outbox, randomSuffix(), Message, LogSender

### Community 6 - "Decimal"
Cohesion: 0.04
Nodes (35): CompleteWaterChangeEventParams, CreateAdHocDoseParams, CreateDosingPlanParams, CreateTankParams, CreateWaterChangeEventParams, CreateWaterChangeScheduleParams, InsertDosingOccurrenceParams, InsertWaterChangeOccurrenceParams (+27 more)

### Community 7 - "Typed API Client"
Cohesion: 0.08
Nodes (41): ApiError, Method, ProfilePatch, Result, Schemas, Tank, TankCreate, TankPatch (+33 more)

### Community 8 - "Svelte UI Components"
Cohesion: 0.08
Nodes (32): svelte, svelte@^5.57.1, icons, { kind = 'info', children }, Props, dialog, {
		open = $bindable(false),
		title,
		confirmLabel,
		danger = false,
		busy = false,
		onconfirm,
		children
	}, Props (+24 more)

### Community 10 - "Docker Compose Stack"
Cohesion: 0.05
Nodes (45): build, command, depends_on, environment, ports, condition, environment, healthcheck (+37 more)

### Community 11 - "New"
Cohesion: 0.19
Nodes (7): Product, ProductInput, New(), productFromRow(), IsForeignKeyViolation(), Enqueue(), Job

### Community 12 - "Session and Locale Runtime"
Cohesion: 0.09
Nodes (11): onUnauthenticated(), User, Locale, session, { children }, Locale, localeNames, logoutError (+3 more)

### Community 13 - "Profile Page Form"
Cohesion: 0.07
Nodes (26): describedBy, id, { label, name, options, value = $bindable(''), error, hint }, Option, Props, bio, busy, displayName (+18 more)

### Community 14 - "time.Time"
Cohesion: 0.02
Nodes (75): System, ArchiveTankParams, BuryJobParams, CompleteJobParams, ConsumeAuthTokenParams, ConsumeAuthTokenRow, CreateAuthTokenParams, CreateLivestockEventParams (+67 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.15
Nodes (6): statusRecorder, Handler, client(), PrincipalFrom(), toUserResponse(), recorder

### Community 16 - "Account Security Page"
Cohesion: 0.09
Nodes (21): api(), csrfToken(), allError, changePassword(), confirmDelete, current, delBusy, delError (+13 more)

### Community 17 - "serve"
Cohesion: 0.16
Nodes (11): PoolConfig, migrate(), openDB(), Run(), serve(), Config, NewMigrator(), Open() (+3 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.22
Nodes (9): EncodeCursor(), NewPage(), handleHealthz(), actor(), pathID(), toResponse(), toShareResponse(), Handler (+1 more)

### Community 19 - "models.go"
Cohesion: 0.05
Nodes (34): AuditLog, AuthToken, ClaimJobsParams, ClaimJobsRow, CreateConfigurationParams, CreateConfigurationRow, DosingEvent, DosingPlan (+26 more)

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.16
Nodes (12): GetTankAccessRow, GetTankRow, NotFound(), decide(), Access, Perm, fromRow(), listShares() (+4 more)

### Community 22 - "Frontend Error Handling"
Cohesion: 0.12
Nodes (17): ErrorDetail, errorMessage(), fieldErrors(), fieldMessage(), busy, email, error, fields (+9 more)

### Community 23 - "Tank List and Labels"
Cohesion: 0.12
Nodes (16): Page, statusOptions, typeOptions, experience, experienceLabel(), statuses, statusLabel(), tankTypeLabel() (+8 more)

### Community 24 - "Date Formatting and Tank Detail"
Cohesion: 0.13
Nodes (16): formatDate(), formatDateTime(), formatDecimal(), today(), zoneOrFallback(), actionError, busy, confirmArchive (+8 more)

### Community 25 - "github.com/google/uuid.UUID"
Cohesion: 0.02
Nodes (58): AnonymizeUserParams, ChangeUserPasswordParams, CountLivestockEventsRow, CountTankLivestockRow, CreateDosingProductParams, CreateEquipmentParams, CreateLivestockEntryParams, CreateMaintenancePlanParams (+50 more)

### Community 26 - "sqlc Configuration"
Cohesion: 0.12
Nodes (16): go, emit_empty_slices, emit_pointers_for_null_types, out, overrides, package, sql_package, db_type (+8 more)

### Community 27 - "Frontend Dev Tooling"
Cohesion: 0.12
Nodes (16): openapi-typescript, prettier, prettier-plugin-svelte, svelte-check, @sveltejs/vite-plugin-svelte, @types/node, typescript, devDependencies (+8 more)

### Community 28 - "Tank Sharing Page"
Cohesion: 0.14
Nodes (15): Share, busy, by, error, fields, grant(), id, load() (+7 more)

### Community 29 - "Playwright E2E Tests"
Cohesion: 0.20
Nodes (11): @axe-core/playwright, @playwright/test, emailLink(), password, signIn(), signUp(), t, uniqueEmail() (+3 more)

### Community 30 - "ESLint Configuration"
Cohesion: 0.13
Nodes (13): eslint, eslint-config-prettier, @eslint/js, eslint-plugin-svelte, globals, typescript-eslint, gitignorePath, eslint@^10.4.1 (+5 more)

### Community 31 - "Renovate Config"
Cohesion: 0.14
Nodes (13): config:recommended, helpers:pinGitHubActionDigests, :semanticCommitTypeAll(chore), extends, labels, packageRules, postUpdateOptions, schedule (+5 more)

### Community 32 - "Web Package Manifest"
Cohesion: 0.14
Nodes (13): playwright, vite, playwright@^1.60.0, vite@^8.3.0, imports, #lib, name, typescript (+5 more)

### Community 33 - "NPM Scripts"
Cohesion: 0.14
Nodes (14): scripts, build, check, check:watch, dev, format, gen:api, i18n (+6 more)

### Community 34 - "App Wiring and Config"
Cohesion: 0.10
Nodes (26): App, Deps, Go backend rules, Service, Config, Limits, New(), TestOpenAPIMatchesRoutes() (+18 more)

### Community 35 - "Typed App Errors"
Cohesion: 0.16
Nodes (11): Error, FieldError, Errors, Conflict(), Forbidden(), Invalid(), RateLimited(), Unauthenticated() (+3 more)

### Community 36 - "Load"
Cohesion: 0.45
Nodes (3): parser, Load(), originOf()

### Community 37 - "IsKind"
Cohesion: 0.13
Nodes (19): Kind, TestSettings(), NewHasher(), TestDeriveGivesUpWhenSlotsAreBusy(), TestHashAndVerify(), TestVerifyFlagsOutdatedParams(), TestVerifyRejectsMalformedHashes(), As() (+11 more)

### Community 38 - "CI Workflow Jobs"
Cohesion: 0.18
Nodes (10): concurrency, cancel-in-progress, group, runs-on, timeout-minutes, jobs, docker, name (+2 more)

### Community 39 - "net/http.Handler"
Cohesion: 0.10
Nodes (22): Current status, Options, ClientIP(), ClientIPFrom(), parseHop(), RateLimitKey(), RateLimitNetKey(), remoteAddr() (+14 more)

### Community 40 - "SvelteKit Build Config"
Cohesion: 0.20
Nodes (9): @inlang/paraglide-js, @sveltejs/adapter-static, @sveltejs/kit, @vitest/browser-playwright, @inlang/paraglide-js@^2.18.2, @sveltejs/adapter-static@^4.0.0, @sveltejs/kit@^3.0.0, @vitest/browser-playwright@^4.1.8 (+1 more)

### Community 41 - "Claude Settings Hooks"
Cohesion: 0.22
Nodes (8): hooks, PostToolUse, PreToolUse, permissions, allow, ask, deny, $schema

### Community 42 - "CI Workflow Steps"
Cohesion: 0.39
Nodes (9): steps, steps, steps, if, name, run, uses, working-directory (+1 more)

### Community 43 - "CI Action Inputs"
Cohesion: 0.22
Nodes (9): with, cache, cache-dependency-path, go-version-file, name, node-version, path, persist-credentials (+1 more)

### Community 44 - "Rate Limiter"
Cohesion: 0.18
Nodes (3): Fixed, Limiter, bucket

### Community 45 - "E2E CI Services"
Cohesion: 0.25
Nodes (8): needs, runs-on, services, timeout-minutes, e2e, image, ports, mailpit

### Community 46 - "CI Postgres Service"
Cohesion: 0.25
Nodes (8): POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, env, image, options, ports, postgres

### Community 47 - "Prometheus HTTP Metrics"
Cohesion: 0.18
Nodes (4): Metrics, New(), normalizeMethod(), TestMiddlewareUsesRoutePattern()

### Community 48 - "TypeScript Config"
Cohesion: 0.29
Nodes (6): $app/tsconfig, compilerOptions, strict, types, extends, include

### Community 49 - "Inlang i18n Settings"
Cohesion: 0.29
Nodes (6): baseLocale, locales, modules, plugin.inlang.messageFormat, pathPattern, $schema

### Community 50 - "Web CI Job"
Cohesion: 0.33
Nodes (6): run, web, working-directory, defaults, runs-on, timeout-minutes

### Community 51 - "app.go"
Cohesion: 0.06
Nodes (45): main(), Error, productRequest, createTank, ctxKey, dimsField, ErrorBody, ErrorDetail (+37 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): RFC-3339, components, $defs, operations, paths, webhooks

### Community 54 - "Vitest Component Tests"
Cohesion: 0.40
Nodes (4): vitest, vitest-browser-svelte, vitest@^4.1.8, vitest-browser-svelte@^2.1.1

### Community 55 - "Go CI Job"
Cohesion: 0.50
Nodes (4): runs-on, services, timeout-minutes, go

### Community 56 - "CI Triggers"
Cohesion: 0.50
Nodes (4): on, pull_request, push, branches

### Community 57 - "livestock/http_integration_test.go"
Cohesion: 0.45
Nodes (9): addFish(), count(), items(), setup(), TestAuthorizationArchiveErasure(), TestIdempotencyAndConcurrency(), TestQuantityFollowsEvents(), TestReviewRules() (+1 more)

### Community 58 - "DecodeJSON"
Cohesion: 0.22
Nodes (8): Handler, planResponse, dateString(), toConfig(), toEquipment(), toPlan(), WriteJSON(), PathID()

### Community 67 - "checker"
Cohesion: 0.17
Nodes (5): checker, allDigits(), Parse(), TestParseCanonical(), TestParseRejects()

### Community 68 - "Runner"
Cohesion: 0.10
Nodes (10): 5. Background work, Backoff(), IsPermanent(), Permanent(), randomJitter(), TestBackoff(), TestPermanent(), Runner (+2 more)

### Community 69 - "NewValidator"
Cohesion: 0.16
Nodes (17): Configuration, DeviceInput, Equipment, Plan, PlanInput, Service, GetEquipmentRow, GetMaintenancePlanRow (+9 more)

### Community 70 - "Validator"
Cohesion: 0.09
Nodes (22): PlanPatch, planPatchRequest, DevicePatch, equipmentPatchRequest, PlanPatch, planPatchRequest, ProfilePatch, updateMeRequest (+14 more)

### Community 71 - "Service"
Cohesion: 0.11
Nodes (21): GetWaterChangeEventRow, GetWaterChangeScheduleRow, LockWaterChangeScheduleRow, dateToTime(), estimate(), applyPatch(), decorateEvent(), eventFromRow() (+13 more)

### Community 72 - "Date"
Cohesion: 0.12
Nodes (15): checker, timeToDate(), dateToTime(), localToday(), nextDue(), optTimeToDate(), date(), TestDecorate() (+7 more)

### Community 73 - "Env"
Cohesion: 0.21
Nodes (13): Browser, Env, Response, addDevice(), addPlan(), count(), items(), setup() (+5 more)

### Community 74 - "Service"
Cohesion: 0.25
Nodes (8): PlanInput, dateToTime(), grace(), sameText(), planFromLocked(), planFromRow(), sameDate(), withNext()

### Community 75 - "checker"
Cohesion: 0.23
Nodes (5): MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune()

### Community 76 - "WriteJSON"
Cohesion: 0.18
Nodes (6): Handler, toEvent(), toPlan(), toProduct(), utc(), deviation()

### Community 77 - "Handler"
Cohesion: 0.24
Nodes (5): tankID(), toEstimate(), toEvent(), toSchedule(), Handler

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 80 - "MustParse"
Cohesion: 0.14
Nodes (14): mustD(), TestAmountIsExact(), TestDeviation(), TestGrace(), TestUnitsByForm(), mul(), MustParse(), TestCmpAndSign() (+6 more)

### Community 81 - "Parse"
Cohesion: 0.16
Nodes (14): Rule, Series, ints(), join(), minSpacing(), mondayIndex(), Parse(), syntaxErr() (+6 more)

### Community 82 - "recurrence_test.go"
Cohesion: 0.41
Nodes (15): eq(), fmtAll(), mustLoc(), mustRule(), TestDailyMultipleHoursAndWindow(), TestDSTGapAndOverlap(), TestDSTGapWestOfUTC(), TestDSTKeepsLocalTime() (+7 more)

### Community 83 - "NewRouter"
Cohesion: 0.24
Nodes (13): LimitBody(), TestTypedNilAppErrorIsInternal(), allowedMethods(), NewRouter(), discardLogger(), do(), TestHealthz(), TestMethodNotAllowedSetsAllow() (+5 more)

### Community 85 - "NewRunner"
Cohesion: 0.10
Nodes (43): createUser(), hash32(), issueToken(), newSession(), pgCode(), TestAuditLog(), TestConcurrentTokenIssueLeavesOneLiveToken(), TestConstraints() (+35 more)

### Community 86 - "5. Initial endpoints"
Cohesion: 0.12
Nodes (16): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, 5. Initial endpoints, 6. Rate limits (initial), Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation) (+8 more)

### Community 87 - "2. Core concepts"
Cohesion: 0.13
Nodes (15): Water changes, 2. Core concepts, Community (Release 2), Dosing, Equipment & maintenance, Identity, Knowledge & intelligence, Livestock (+7 more)

### Community 88 - "ReefCare — Knowledge Base and Reef Intelligence Engine"
Cohesion: 0.14
Nodes (14): 10. Feedback loop, 1. Purpose and boundaries, 2. Knowledge base, 3. Pipeline, 4. Output model, 5. MVP rule set (v1.0.0), 6. Safety rules (enforced in validation step), 7. Triggers (+6 more)

### Community 89 - "dosing/http_integration_test.go"
Cohesion: 0.45
Nodes (13): count(), items(), plan(), setup(), TestAdHocIdempotency(), TestAuthorizationArchiveErasure(), TestConcurrentKeyedAdHoc(), TestPlanLifecycle() (+5 more)

### Community 90 - "ReefCare — Security Design"
Cohesion: 0.15
Nodes (12): 10. Secrets and supply chain, 11. Abuse prevention (community), 1. Threat model (summary), 2. Authentication, 4. Web protections, 5. Data access, 6. File uploads (Phase 4 or when photos ship), 7. Intelligence and LLM boundary (+4 more)

### Community 91 - "parser"
Cohesion: 0.35
Nodes (5): itoa(), Dimensions, fields, parser, Tank

### Community 92 - "Hasher"
Cohesion: 0.60
Nodes (3): Hasher, Argon2Params, parseHash()

### Community 93 - "ReefCare — Architecture"
Cohesion: 0.17
Nodes (12): 1. Overview, 2. Technology decisions, 3. Repository layout, 4. Module anatomy, 6. Configuration, 7. Environments, 8. Observability, 9. CI (+4 more)

### Community 94 - "ReefCare — User Stories and Acceptance Criteria"
Cohesion: 0.17
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Perform"
Cohesion: 0.27
Nodes (9): EventFilter, EventInput, ListMaintenanceEventsRow, checkKey(), eventFromRow(), hashIf(), performedSummary(), replay() (+1 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.20
Nodes (10): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 4. Security and identity follow-ups, 6. Repository and tooling (+2 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 100 - ".Timeline"
Cohesion: 0.25
Nodes (6): ListTankEventsParams, ListTankEventsRow, IsCategory(), TimelineCursor, TimelineEvent, TimelineFilter

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.11
Nodes (15): Architecture: modular monolith, Commands, Definition of Done, graphify, Non-negotiable domain rules, ReefCare — Claude Code Project Guide, Roadmap, Stack (unless the repo later establishes a better-justified alternative) (+7 more)

### Community 102 - "Queries"
Cohesion: 0.21
Nodes (10): Queries, sameAmount(), eventFromRow(), recordTimeline(), usableProduct(), record(), recordTimeline(), IsNoRows() (+2 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.18
Nodes (11): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 6. Phase 4 — community (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency (+3 more)

### Community 105 - "feature/SKILL.md"
Cohesion: 0.25
Nodes (7): 1. Inspect, 2. Analyze (write it down briefly in the conversation), 3. Design, 4. Implement, 5. Verify, 6. Review, 7. Summarize

### Community 106 - "InSerializableTx"
Cohesion: 0.39
Nodes (5): TxStarter, TestInSerializableTxRetries(), IsSerializationFailure(), InSerializableTx(), inTx()

### Community 107 - "ReefCare web"
Cohesion: 0.13
Nodes (12): 1. Scope of 1.8, 2. Stack, 3. Security requirements, 4. UX requirements (from .claude/rules/frontend.md), 5. Decisions (accepted 2026-10-30), 6. Implementation notes, ReefCare — Web Frontend Proposal (milestone 1.8), Checks (+4 more)

### Community 108 - "ReefCare — Implementation Roadmap"
Cohesion: 0.17
Nodes (12): Open decisions (need product owner input), Phase 0 — Product and technical design ✅ drafted, awaiting review, Phase 1 — Foundation, Phase 2 — Reef management, Phase 3 — Reef Intelligence, Phase 4 — Community (Release 2), Phase 5 — Production readiness, ReefCare — Implementation Roadmap (+4 more)

### Community 109 - "Reef Intelligence and knowledge base rules"
Cohesion: 0.29
Nodes (6): Knowledge base, Mandatory safety rules, Output model, Pipeline (keep these stages explicit and individually testable), Provider independence, Reef Intelligence and knowledge base rules

### Community 110 - "product-requirements.md"
Cohesion: 0.24
Nodes (3): 1. Bounded contexts (modules), 3. Authorization model, ReefCare — Domain Model

### Community 111 - "SMTPSender"
Cohesion: 0.39
Nodes (5): NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), SMTPConfig, SMTPSender

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 114 - "identity/jobs_integration_test.go"
Cohesion: 0.57
Nodes (7): count(), TestAccountErasedAfterThirtyDays(), TestEmailJobsHoldNoSecrets(), TestEmailSurvivesSMTPOutage(), TestLoginFailureAuditedThroughJob(), TestPurgeExpiredSessionsAndTokens(), waitJobs()

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 116 - "config_test.go"
Cohesion: 0.60
Nodes (5): env(), TestAllowedOrigins(), TestLoadDefaults(), TestLoadReportsAllErrors(), TestLoadValidation()

### Community 117 - "writeEvent"
Cohesion: 0.67
Nodes (3): eventResponse, toEvent(), writeEvent()

### Community 118 - "ParsePageParams"
Cohesion: 0.50
Nodes (3): PageParams, DecodeCursor(), ParsePageParams()

### Community 119 - "Handler"
Cohesion: 0.15
Nodes (13): toEntry(), toEvent(), writeCreated(), BadRequest(), decodeError(), DecodeJSON(), jsonKind(), Hash() (+5 more)

### Community 132 - "jobOrigin"
Cohesion: 0.67
Nodes (3): emailJob, jobOrigin, loginFailedJob

## Knowledge Gaps
- **7 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1023 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **36 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ReefCare — Implementation Roadmap` connect `ReefCare — Implementation Roadmap` to `product-requirements.md`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `English UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._
- **Why does `Phase 1 — Foundation` connect `ReefCare — Implementation Roadmap` to `New`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Should `Vietnamese UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._
- **Why does `InTx()` connect `New` to `Service`, `NewValidator`, `Queries`, `net/http.Handler`, `Service`, `Service`, `New`, `ReefCare — Implementation Roadmap`, `InSerializableTx`, `app.go`, `NewRunner`, `Tank Authorization`, `github.com/google/uuid.UUID`, `.Perform`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Should `Service` be split into smaller, more focused modules?**
  _Cohesion score 0.1380952380952381 - nodes in this community are weakly interconnected._