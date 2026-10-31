# Graph Report - reefcare  (2026-10-31)

## Corpus Check
- 312 files · ~265,403 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 4782 nodes · 13095 edges · 132 communities (100 shown, 32 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 299 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `85527d9d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- en.json
- vi.json
- Service
- Service
- Identity Integration Tests
- Outbox
- Decimal
- Typed API Client
- web_src_lib_paraglide_messages_m
- water-tests/+page.svelte
- Docker Compose Stack
- New
- api
- versions/[id]/+page.svelte
- github.com/google/uuid.UUID
- Identity HTTP Handlers
- DoseResolveDialog.svelte
- Load
- Tank HTTP Handlers
- NewPage
- Golangci-lint Config
- Tank Authorization
- Validator
- [equipmentId]/+page.svelte
- water-changes/+page.svelte
- context.Context
- sqlc Configuration
- [entryId]/+page.svelte
- go_pkg_time
- helpers.ts
- package.json
- Renovate Config
- overrides
- NPM Scripts
- log/slog.Logger
- waterparameters/service.go
- RecurrenceFields.svelte
- New
- CI Workflow Jobs
- dosing/+page.svelte
- SvelteKit Build Config
- Claude Settings Hooks
- CI Workflow Steps
- CI Action Inputs
- time.Duration
- E2E CI Services
- CI Postgres Service
- IsKind
- TypeScript Config
- Inlang i18n Settings
- Web CI Job
- tanks/http.go
- Generated OpenAPI Types
- DB Pool Metrics Collector
- ReefCare — Claude Code Project Guide
- Go CI Job
- CI Triggers
- livestock/http_integration_test.go
- net/http.Request
- Prettier Config
- Go Format Hook
- File Guard Hook
- MustParse
- SvelteKit App Types
- Go Module Root
- Web Package Root
- checker
- Runner
- Service
- Validator
- Service
- Date
- Env
- IsNoRows
- checker
- .Register
- Handler
- Handler
- waterchanges/http_integration_test.go
- checker
- recurrence_test.go
- waterparameters/http_integration_test.go
- NewRouter
- Limiter
- 5. Initial endpoints
- 2. Core concepts
- ReefCare — Knowledge Base and Reef Intelligence Engine
- dosing/http_integration_test.go
- ReefCare — Security Design
- parser
- .Login
- ReefCare — Architecture
- ReefCare — User Stories and Acceptance Criteria
- .Record
- Reef management domain rules
- ReefCare — Open Items
- ReefCare — Product Requirements
- checker
- Plan
- ReefCare — Claude Code Project Guide
- ReefCare — Database Design
- playwright.config.ts
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- toPlan
- Community, Reef Clinic and reputation rules
- toEvent
- PostgreSQL, migrations and sqlc
- app.go
- System
- decodeError
- net/http.ResponseWriter
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- CountJobsByStatusRow
- permanentError

## God Nodes (most connected - your core abstractions)
1. `New()` - 119 edges
2. `Decimal` - 113 edges
3. `api()` - 82 edges
4. `WriteJSON()` - 81 edges
5. `Actor` - 69 edges
6. `Queries` - 68 edges
7. `InTx()` - 62 edges
8. `NewValidator()` - 58 edges
9. `ApiError` - 57 edges
10. `DecodeJSON()` - 52 edges

## Surprising Connections (you probably didn't know these)
- `3. Authentication` --references--> `Origin`  [INFERRED]
  docs/api-guidelines.md → internal/audit/audit.go
- `6. Rate limits (initial)` --references--> `RateLimitKey()`  [INFERRED]
  docs/api-guidelines.md → internal/platform/httpserver/clientip.go
- `Dependency rules` --references--> `Authorizer`  [INFERRED]
  docs/architecture.md → internal/tanks/authz.go
- `Tanks` --references--> `tankID()`  [INFERRED]
  docs/api-guidelines.md → internal/waterchanges/http.go
- `4. Web protections` --references--> `Origin`  [INFERRED]
  docs/security.md → internal/audit/audit.go

## Import Cycles
- 3-file cycle: `internal/app/app.go -> internal/dosing/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/equipment/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/knowledge/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/livestock/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/scheduling/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterchanges/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterparameters/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/knowledge/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/knowledge/model.go -> internal/waterparameters/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`

## Communities (132 total, 32 thin omitted)

### Community 0 - "en.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 1 - "vi.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 2 - "Service"
Cohesion: 0.08
Nodes (32): ListLivestockEventsParams, Queries, GetLivestockEntryRow, ListLivestockEventsRow, EraseUserData(), OnTankArchived(), EraseUserData(), OnTankArchived() (+24 more)

### Community 3 - "Service"
Cohesion: 0.15
Nodes (13): Eraser, Service, Session, User, passwordProblem(), TestPasswordPolicy(), TestTokens(), decodePrefs() (+5 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.09
Nodes (56): TestOpenAPIMatchesRoutes(), MustURL(), New(), TestAmountIsExact(), TestUnitsByForm(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable() (+48 more)

### Community 5 - "Outbox"
Cohesion: 0.15
Nodes (9): Outbox, randomSuffix(), Message, NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), LogSender, SMTPConfig (+1 more)

### Community 6 - "Decimal"
Cohesion: 0.07
Nodes (29): eventResponse, planResponse, productRefJSON, deviation(), sameAmount(), mustD(), TestDeviation(), allDigits() (+21 more)

### Community 7 - "Typed API Client"
Cohesion: 0.04
Nodes (69): KnowledgeCategory, Method, onUnauthenticated(), ProfilePatch, Result, Schemas, Tank, TankCreate (+61 more)

### Community 8 - "web_src_lib_paraglide_messages_m"
Cohesion: 0.03
Nodes (90): svelte, vitest, svelte@^5.57.1, vitest@^4.1.8, ApiError, csrfToken(), ErrorDetail, KnowledgeArticle (+82 more)

### Community 9 - "water-tests/+page.svelte"
Cohesion: 0.02
Nodes (162): Go backend rules, LatestReading, ParameterType, Reading, Target, TimelineEvent, Trends, WaterTest (+154 more)

### Community 10 - "Docker Compose Stack"
Cohesion: 0.05
Nodes (45): build, command, depends_on, environment, ports, condition, environment, healthcheck (+37 more)

### Community 11 - "New"
Cohesion: 0.08
Nodes (30): Entry, AdHocInput, PlanInput, Record(), New(), GetDosingEventRow, GetMaintenancePlanRow, LockKnowledgeVersionRow (+22 more)

### Community 12 - "api"
Cohesion: 0.02
Nodes (118): api(), Notification, Share, User, formatDate(), formatDateTime(), today(), zoneOrFallback() (+110 more)

### Community 13 - "versions/[id]/+page.svelte"
Cohesion: 0.03
Nodes (81): ClaimClass, KnowledgeSource, KnowledgeStructured, KnowledgeVersion, KnowledgeVersionInput, body, busy, claim (+73 more)

### Community 14 - "github.com/google/uuid.UUID"
Cohesion: 0.01
Nodes (244): ArchiveTankParams, AuditLog, AuthToken, ClaimJobsParams, ClaimJobsRow, ConsumeAuthTokenParams, ConsumeAuthTokenRow, CreateAdHocDoseParams (+236 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.11
Nodes (16): Current status, Handler, client(), NewHandler(), PrincipalFrom(), toUserResponse(), ClientIP(), ClientIPFrom() (+8 more)

### Community 16 - "DoseResolveDialog.svelte"
Cohesion: 0.02
Nodes (115): DoseEvent, DoseUnit, DosingForm, DosingPlan, DosingPlanCreate, DosingPlanPatch, DosingProduct, amount (+107 more)

### Community 17 - "Load"
Cohesion: 0.08
Nodes (21): parser, Migrator, PoolConfig, migrate(), openDB(), roles(), Run(), serve() (+13 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.18
Nodes (8): EncodeCursor(), NewPage(), actor(), pathID(), toResponse(), toShareResponse(), Handler, shareResponse

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.10
Nodes (21): GetTankAccessParams, GetTankParams, Done on 2026-10-30 (committed), 3. Authorization, GetTankAccessRow, GetTankRow, recordTimeline(), decide() (+13 more)

### Community 22 - "Validator"
Cohesion: 0.27
Nodes (4): mergeNotifyPrefs(), mergeUnitPrefs(), validateDisplayName(), Validator

### Community 23 - "[equipmentId]/+page.svelte"
Cohesion: 0.03
Nodes (87): Equipment, EquipmentConfiguration, EquipmentCreate, EquipmentPatch, MaintenancePlan, MaintenancePlanCreate, MaintenancePlanPatch, Page (+79 more)

### Community 24 - "water-changes/+page.svelte"
Cohesion: 0.03
Nodes (86): MaintenanceEvent, WaterChange, WaterChangeEstimate, WaterChangeRecord, executedAt, { estimate }, locale, busy (+78 more)

### Community 25 - "context.Context"
Cohesion: 0.01
Nodes (54): AddKnowledgeVersionSourceParams, AnonymizeUserParams, BuryJobParams, ChangeUserPasswordParams, ClaimPendingNotificationEmailsParams, CloseTargetParams, CompleteJobParams, CompleteWaterChangeEventParams (+46 more)

### Community 26 - "sqlc Configuration"
Cohesion: 0.12
Nodes (16): go, emit_empty_slices, emit_pointers_for_null_types, out, overrides, package, sql_package, db_type (+8 more)

### Community 27 - "[entryId]/+page.svelte"
Cohesion: 0.03
Nodes (113): LivestockEntry, LivestockEntryCreate, LivestockEntryPatch, LivestockEvent, Species, fe, {
		form = $bindable(),
		creating,
		speciesLabel = '',
		addedAt = $bindable(''),
		timeZone,
		error,
		busy,
		submitLabel,
		onsubmit,
		oncancel
	}, locale (+105 more)

### Community 28 - "go_pkg_time"
Cohesion: 0.28
Nodes (5): IsPermanent(), TestPermanent(), Handler, emailCopy, recipient

### Community 29 - "helpers.ts"
Cohesion: 0.15
Nodes (12): @axe-core/playwright, @playwright/test, globalSetup(), storageState, emailLink(), password, signIn(), signUp() (+4 more)

### Community 30 - "package.json"
Cohesion: 0.07
Nodes (40): eslint, eslint-config-prettier, @eslint/js, eslint-plugin-svelte, globals, openapi-typescript, playwright, prettier (+32 more)

### Community 31 - "Renovate Config"
Cohesion: 0.14
Nodes (13): config:recommended, helpers:pinGitHubActionDigests, :semanticCommitTypeAll(chore), extends, labels, packageRules, postUpdateOptions, schedule (+5 more)

### Community 32 - "overrides"
Cohesion: 0.67
Nodes (3): typescript, overrides, openapi-typescript

### Community 33 - "NPM Scripts"
Cohesion: 0.14
Nodes (14): scripts, build, check, check:watch, dev, format, gen:api, i18n (+6 more)

### Community 34 - "log/slog.Logger"
Cohesion: 0.07
Nodes (38): App, Deps, Options, Limits, New(), NewHandler(), NewService(), NewHandler() (+30 more)

### Community 35 - "waterparameters/service.go"
Cohesion: 0.13
Nodes (12): likePattern(), TestLikePattern(), balanceAt(), before(), insert(), likePattern(), TestInsertKeepsEveryBalance(), TestLikePattern() (+4 more)

### Community 36 - "RecurrenceFields.svelte"
Cohesion: 0.06
Nodes (52): RecurrencePreview, WaterChangeSchedule, WaterChangeScheduleCreate, WaterChangeSchedulePatch, { form = $bindable(), startsOn, endsOn, timeZone, error }, intervalLabel, legendId, locale (+44 more)

### Community 37 - "New"
Cohesion: 0.10
Nodes (42): createUser(), hash32(), issueToken(), newSession(), pgCode(), TestAuditLog(), TestConcurrentTokenIssueLeavesOneLiveToken(), TestConstraints() (+34 more)

### Community 38 - "CI Workflow Jobs"
Cohesion: 0.18
Nodes (10): concurrency, cancel-in-progress, group, runs-on, timeout-minutes, jobs, docker, name (+2 more)

### Community 39 - "dosing/+page.svelte"
Cohesion: 0.08
Nodes (25): actionError, attention, base, busy, canEdit, completeOpen, completing, history (+17 more)

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

### Community 44 - "time.Duration"
Cohesion: 0.10
Nodes (20): Error, FieldError, 4. Module anatomy, Authorization, Dependency rules, Errors, Transactions, Config (+12 more)

### Community 45 - "E2E CI Services"
Cohesion: 0.25
Nodes (8): needs, runs-on, services, timeout-minutes, e2e, image, ports, mailpit

### Community 46 - "CI Postgres Service"
Cohesion: 0.25
Nodes (8): POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, env, image, options, ports, postgres

### Community 47 - "IsKind"
Cohesion: 0.10
Nodes (26): Kind, TestSettings(), TestDeriveGivesUpWhenSlotsAreBusy(), codes(), TestSlugAndTags(), TestStructured(), As(), IsKind() (+18 more)

### Community 48 - "TypeScript Config"
Cohesion: 0.29
Nodes (6): $app/tsconfig, compilerOptions, strict, types, extends, include

### Community 49 - "Inlang i18n Settings"
Cohesion: 0.29
Nodes (6): baseLocale, locales, modules, plugin.inlang.messageFormat, pathPattern, $schema

### Community 50 - "Web CI Job"
Cohesion: 0.33
Nodes (6): run, web, working-directory, defaults, runs-on, timeout-minutes

### Community 51 - "tanks/http.go"
Cohesion: 0.08
Nodes (45): productRequest, createTank, ctxKey, dimsField, ErrorBody, ErrorDetail, fieldError, PageParams (+37 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): components, $defs, operations, paths, RFC-3339, webhooks

### Community 54 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.22
Nodes (9): Architecture: modular monolith, Commands, Definition of Done, graphify, Non-negotiable domain rules, ReefCare — Claude Code Project Guide, Roadmap, Stack (unless the repo later establishes a better-justified alternative) (+1 more)

### Community 55 - "Go CI Job"
Cohesion: 0.50
Nodes (4): runs-on, services, timeout-minutes, go

### Community 56 - "CI Triggers"
Cohesion: 0.50
Nodes (4): on, pull_request, push, branches

### Community 57 - "livestock/http_integration_test.go"
Cohesion: 0.45
Nodes (9): addFish(), count(), items(), setup(), TestAuthorizationArchiveErasure(), TestIdempotencyAndConcurrency(), TestQuantityFollowsEvents(), TestReviewRules() (+1 more)

### Community 58 - "net/http.Request"
Cohesion: 0.12
Nodes (23): Handler, eventResponse, Handler, toEvent(), toPlan(), toProduct(), dateString(), toConfig() (+15 more)

### Community 62 - "MustParse"
Cohesion: 0.11
Nodes (21): Water parameters, 3g. Water parameters (milestones 2.1/2.2 done, values provisional), mul(), MustParse(), TestAddSubRound(), TestCmpAndSign(), TestDigitsAndFits(), TestDivRound() (+13 more)

### Community 68 - "Runner"
Cohesion: 0.14
Nodes (5): 5. Background work, randomJitter(), Runner, Observer, Source

### Community 69 - "Service"
Cohesion: 0.11
Nodes (21): ListMaintenanceEventsParams, Configuration, DeviceInput, Equipment, EventFilter, EventInput, Service, GetEquipmentRow (+13 more)

### Community 70 - "Validator"
Cohesion: 0.10
Nodes (20): 6. Phase 4 — community (summary), PlanPatch, planPatchRequest, DevicePatch, equipmentPatchRequest, PlanPatch, planPatchRequest, ProfilePatch (+12 more)

### Community 71 - "Service"
Cohesion: 0.11
Nodes (19): GetWaterChangeEventRow, GetWaterChangeScheduleRow, LockWaterChangeScheduleRow, dateToTime(), applyPatch(), decorateEvent(), eventFromRow(), recordTimeline() (+11 more)

### Community 72 - "Date"
Cohesion: 0.11
Nodes (22): Plan, dateToTime(), timeToDate(), sameDate(), dateToTime(), decorate(), localToday(), nextDue() (+14 more)

### Community 73 - "Env"
Cohesion: 0.16
Nodes (18): Browser, Env, Response, addDevice(), addPlan(), count(), items(), setup() (+10 more)

### Community 74 - "IsNoRows"
Cohesion: 0.16
Nodes (11): PlanInput, Product, ProductInput, Service, GetDosingPlanRow, LockDosingPlanRow, sameText(), planFromLocked() (+3 more)

### Community 75 - "checker"
Cohesion: 0.11
Nodes (7): checker, MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune(), checker

### Community 76 - ".Register"
Cohesion: 0.18
Nodes (13): 4. Web protections, Client, Principal, RegisterInput, Origin, enqueueEmail(), originOf(), allow() (+5 more)

### Community 77 - "Handler"
Cohesion: 0.23
Nodes (5): tankID(), toEstimate(), toEvent(), toSchedule(), Handler

### Community 78 - "Handler"
Cohesion: 0.17
Nodes (6): parseTime(), toReading(), toTarget(), toTest(), writeTargets(), Handler

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 80 - "checker"
Cohesion: 0.16
Nodes (10): parseVersion(), checker, Item, parsedVersion, Recommendation, SourceRef, sourceRefRequest, Structured (+2 more)

### Community 81 - "recurrence_test.go"
Cohesion: 0.14
Nodes (28): grace(), TestGrace(), Rule, ints(), join(), minSpacing(), mondayIndex(), Parse() (+20 more)

### Community 82 - "waterparameters/http_integration_test.go"
Cohesion: 0.48
Nodes (14): count(), items(), reading(), readings(), record(), setup(), TestAuthorizationArchiveErasure(), TestCatalogMatchesDatabase() (+6 more)

### Community 83 - "NewRouter"
Cohesion: 0.16
Nodes (19): WriteError(), LimitBody(), Recover(), RequestIDFrom(), IsSafeMethod(), OriginCheck(), TestTypedNilAppErrorIsInternal(), WriteAppError() (+11 more)

### Community 84 - "Limiter"
Cohesion: 0.18
Nodes (3): Fixed, Limiter, bucket

### Community 86 - "5. Initial endpoints"
Cohesion: 0.14
Nodes (14): 5. Initial endpoints, Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation), Dosing, Equipment, maintenance, livestock (Phase 2), Health, Intelligence (Phase 3), Notifications (+6 more)

### Community 87 - "2. Core concepts"
Cohesion: 0.15
Nodes (13): 1. Bounded contexts (modules), 2. Core concepts, 3. Authorization model, Community (Release 2), Dosing, Equipment & maintenance, Identity, Knowledge & intelligence (+5 more)

### Community 88 - "ReefCare — Knowledge Base and Reef Intelligence Engine"
Cohesion: 0.14
Nodes (14): 10. Feedback loop, 1. Purpose and boundaries, 2. Knowledge base, 3. Pipeline, 4. Output model, 5. MVP rule set (v1.0.0), 6. Safety rules (enforced in validation step), 7. Triggers (+6 more)

### Community 89 - "dosing/http_integration_test.go"
Cohesion: 0.45
Nodes (13): count(), items(), plan(), setup(), TestAdHocIdempotency(), TestAuthorizationArchiveErasure(), TestConcurrentKeyedAdHoc(), TestPlanLifecycle() (+5 more)

### Community 90 - "ReefCare — Security Design"
Cohesion: 0.20
Nodes (10): 10. Secrets and supply chain, 11. Abuse prevention (community), 1. Threat model (summary), 2. Authentication, 5. Data access, 6. File uploads (Phase 4 or when photos ship), 7. Intelligence and LLM boundary, 8. Logging and audit (+2 more)

### Community 91 - "parser"
Cohesion: 0.35
Nodes (5): itoa(), Dimensions, fields, parser, Tank

### Community 92 - ".Login"
Cohesion: 0.23
Nodes (9): Hasher, LoginInput, Argon2Params, NewHasher(), parseHash(), TestHashAndVerify(), TestVerifyFlagsOutdatedParams(), TestVerifyRejectsMalformedHashes() (+1 more)

### Community 93 - "ReefCare — Architecture"
Cohesion: 0.25
Nodes (8): 1. Overview, 2. Technology decisions, 3. Repository layout, 6. Configuration, 7. Environments, 8. Observability, 9. CI, ReefCare — Architecture

### Community 94 - "ReefCare — User Stories and Acceptance Criteria"
Cohesion: 0.17
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Record"
Cohesion: 0.10
Nodes (21): TestValid(), Valid(), hashIf(), loadTargets(), replay(), sameTarget(), stored(), Annotation (+13 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.17
Nodes (12): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 3f. Notifications (milestone 2.9 done), 3g. Knowledge base (milestone 3.1 done) (+4 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 100 - "Plan"
Cohesion: 0.50
Nodes (3): Event, Plan, ProductRef

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.13
Nodes (12): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, 6. Rate limits (initial), ReefCare — API Guidelines and Initial Contracts, API binary, Development (+4 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.22
Nodes (9): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency, 9. Seed data (+1 more)

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
Cohesion: 0.22
Nodes (9): Open decisions (need product owner input), Phase 0 — Product and technical design ✅ drafted, awaiting review, Phase 1 — Foundation, Phase 2 — Reef management, Phase 3 — Reef Intelligence, Phase 4 — Community (Release 2), Phase 5 — Production readiness, ReefCare — Implementation Roadmap (+1 more)

### Community 109 - "Reef Intelligence and knowledge base rules"
Cohesion: 0.29
Nodes (6): Knowledge base, Mandatory safety rules, Output model, Pipeline (keep these stages explicit and individually testable), Provider independence, Reef Intelligence and knowledge base rules

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 116 - "app.go"
Cohesion: 0.07
Nodes (12): main(), Error, emailTemplate, emailBody(), mayEmail(), quietHours(), TestEmailBody(), TestEmailLimits() (+4 more)

### Community 119 - "net/http.ResponseWriter"
Cohesion: 0.13
Nodes (11): statusRecorder, actor(), date(), nonNil(), toArticle(), toSource(), toVersion(), utc() (+3 more)

## Knowledge Gaps
- **9 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+4 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 2199 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `api()` connect `api` to `RecurrenceFields.svelte`, `Typed API Client`, `web_src_lib_paraglide_messages_m`, `water-tests/+page.svelte`, `dosing/+page.svelte`, `versions/[id]/+page.svelte`, `Identity HTTP Handlers`, `DoseResolveDialog.svelte`, `[equipmentId]/+page.svelte`, `water-changes/+page.svelte`, `[entryId]/+page.svelte`?**
  _High betweenness centrality (0.347) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _9 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `en.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `Current status` connect `Identity HTTP Handlers` to `Service`, `New`, `New`, `api`, `NewRouter`, `Tank Authorization`, `ReefCare — Claude Code Project Guide`, `net/http.Request`?**
  _High betweenness centrality (0.346) - this node is a cross-community bridge._
- **Should `vi.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `svelte` connect `web_src_lib_paraglide_messages_m` to `versions/[id]/+page.svelte`, `package.json`?**
  _High betweenness centrality (0.177) - this node is a cross-community bridge._
- **Should `Service` be split into smaller, more focused modules?**
  _Cohesion score 0.08082706766917293 - nodes in this community are weakly interconnected._