# Graph Report - reefcare  (2026-10-31)

## Corpus Check
- 355 files · ~281,267 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 4898 nodes · 13516 edges · 129 communities (104 shown, 25 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 335 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `35b6ead9`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- en.json
- vi.json
- Service
- New
- Identity Integration Tests
- Outbox
- Parse
- Typed API Client
- Alert.svelte
- water-tests/+page.svelte
- Docker Compose Stack
- NewValidator
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
- ref_app
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
- Decimal
- Date
- Env
- IsNoRows
- checker
- intelligence/metrics.go
- Handler
- Handler
- waterchanges/http_integration_test.go
- checker
- recurrence_test.go
- waterparameters/http_integration_test.go
- NewRouter
- Limiter
- product-requirements.md
- 5. Initial endpoints
- rules.go
- ReefCare — Knowledge Base and Reef Intelligence Engine
- dosing/http_integration_test.go
- ReefCare — Security Design
- parser
- Finding
- ReefCare — Architecture
- ReefCare — User Stories and Acceptance Criteria
- .Record
- Reef management domain rules
- ReefCare — Open Items
- ReefCare — Product Requirements
- checker
- Current status
- ReefCare — Claude Code Project Guide
- ReefCare — Database Design
- playwright.config.ts
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- intelligence/metrics_test.go
- ParsePageParams
- Observer
- Community, Reef Clinic and reputation rules
- PostgreSQL, migrations and sqlc
- app.go
- net/http.ResponseWriter
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- CountJobsByStatusRow
- permanentError

## God Nodes (most connected - your core abstractions)
1. `Decimal` - 132 edges
2. `New()` - 119 edges
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
- `4. Web protections` --references--> `Origin`  [INFERRED]
  docs/security.md → internal/audit/audit.go
- `6. Rate limits (initial)` --references--> `RateLimitKey()`  [INFERRED]
  docs/api-guidelines.md → internal/platform/httpserver/clientip.go
- `Dependency rules` --references--> `Authorizer`  [INFERRED]
  docs/architecture.md → internal/tanks/authz.go
- `Done on 2026-10-30 (committed)` --references--> `Authorizer`  [INFERRED]
  docs/open-items.md → internal/tanks/authz.go

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

## Communities (129 total, 25 thin omitted)

### Community 0 - "en.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 1 - "vi.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 2 - "Service"
Cohesion: 0.09
Nodes (27): ListLivestockEventsParams, Queries, GetLivestockEntryRow, ListLivestockEventsRow, EraseUserData(), OnTankArchived(), EraseUserData(), OnTankArchived() (+19 more)

### Community 3 - "New"
Cohesion: 0.07
Nodes (41): Entry, Client, Hasher, LoginInput, Principal, RegisterInput, Service, Session (+33 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.06
Nodes (68): MustURL(), New(), TestAmountIsExact(), TestUnitsByForm(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable(), TestHostileInputs() (+60 more)

### Community 5 - "Outbox"
Cohesion: 0.16
Nodes (9): Outbox, randomSuffix(), Message, NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), LogSender, SMTPConfig (+1 more)

### Community 6 - "Parse"
Cohesion: 0.12
Nodes (5): allDigits(), canonical(), Parse(), TestParseCanonical(), TestParseRejects()

### Community 7 - "Typed API Client"
Cohesion: 0.03
Nodes (82): DosingPlanCreate, DosingPlanPatch, KnowledgeCategory, Method, onUnauthenticated(), ProfilePatch, Result, Schemas (+74 more)

### Community 8 - "Alert.svelte"
Cohesion: 0.05
Nodes (41): svelte, svelte@^5.57.1, csrfToken(), icons, { kind = 'info', children }, Props, dialog, {
		open = $bindable(false),
		title,
		confirmLabel,
		danger = false,
		busy = false,
		onconfirm,
		children
	} (+33 more)

### Community 9 - "water-tests/+page.svelte"
Cohesion: 0.02
Nodes (152): LatestReading, ParameterType, Reading, Target, TargetInput, TimelineEvent, WaterTest, WaterTestCreate (+144 more)

### Community 10 - "Docker Compose Stack"
Cohesion: 0.05
Nodes (45): build, command, depends_on, environment, ports, condition, environment, healthcheck (+37 more)

### Community 11 - "NewValidator"
Cohesion: 0.11
Nodes (25): LockKnowledgeVersionRow, authorOfLiveArticle(), parseVersion(), requireEditor(), setSources(), sources(), sourcesForClaim(), validURL() (+17 more)

### Community 12 - "api"
Cohesion: 0.03
Nodes (74): api(), Notification, doseText(), formatDate(), formatDateTime(), today(), zoneOrFallback(), notificationSection() (+66 more)

### Community 13 - "versions/[id]/+page.svelte"
Cohesion: 0.02
Nodes (108): ApiError, ClaimClass, ErrorDetail, KnowledgeArticle, KnowledgeSource, KnowledgeStructured, KnowledgeVersion, KnowledgeVersionInput (+100 more)

### Community 14 - "github.com/google/uuid.UUID"
Cohesion: 0.01
Nodes (240): AuditLog, AuthToken, CompleteJobParams, ConsumeAuthTokenParams, ConsumeAuthTokenRow, CreateAdHocDoseParams, CreateConfigurationParams, CreateConfigurationRow (+232 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.15
Nodes (7): statusRecorder, Handler, client(), PrincipalFrom(), toUserResponse(), WriteAppError(), recorder

### Community 16 - "DoseResolveDialog.svelte"
Cohesion: 0.02
Nodes (113): DoseEvent, DoseUnit, DosingForm, DosingPlan, DosingProduct, amount, busy, confirmOver (+105 more)

### Community 17 - "Load"
Cohesion: 0.07
Nodes (24): parser, Migrator, PoolConfig, migrate(), openDB(), roles(), Run(), serve() (+16 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.17
Nodes (10): EncodeCursor(), NewPage(), actor(), NewHandler(), pathID(), toResponse(), toShareResponse(), Handler (+2 more)

### Community 19 - "NewPage"
Cohesion: 0.33
Nodes (3): NewHandler(), userID(), Handler

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.11
Nodes (19): GetTankAccessParams, GetTankParams, Phase 1 — Foundation, GetTankAccessRow, GetTankRow, NotFound(), decide(), Access (+11 more)

### Community 22 - "Validator"
Cohesion: 0.19
Nodes (7): ProfilePatch, mergeNotifyPrefs(), mergeUnitPrefs(), validateDisplayName(), validTimezone(), speciesCoralType(), Validator

### Community 23 - "[equipmentId]/+page.svelte"
Cohesion: 0.03
Nodes (105): Equipment, EquipmentConfiguration, EquipmentCreate, EquipmentPatch, MaintenancePlan, MaintenancePlanCreate, MaintenancePlanPatch, Page (+97 more)

### Community 24 - "water-changes/+page.svelte"
Cohesion: 0.03
Nodes (76): vitest, vitest@^4.1.8, MaintenanceEvent, WaterChangeEstimate, executedAt, { estimate }, locale, busy (+68 more)

### Community 25 - "context.Context"
Cohesion: 0.01
Nodes (55): AddKnowledgeVersionSourceParams, AnonymizeUserParams, ArchiveTankParams, BuryJobParams, ChangeUserPasswordParams, ClaimJobsParams, ClaimJobsRow, ClaimPendingNotificationEmailsParams (+47 more)

### Community 26 - "sqlc Configuration"
Cohesion: 0.12
Nodes (16): go, emit_empty_slices, emit_pointers_for_null_types, out, overrides, package, sql_package, db_type (+8 more)

### Community 27 - "[entryId]/+page.svelte"
Cohesion: 0.03
Nodes (112): LivestockEntry, LivestockEntryCreate, LivestockEntryPatch, LivestockEvent, Species, fe, {
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
	}, locale (+104 more)

### Community 28 - "go_pkg_time"
Cohesion: 0.32
Nodes (3): Handler, emailCopy, recipient

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
Cohesion: 0.10
Nodes (28): Deps, Go backend rules, Config, Limits, New(), TestOpenAPIMatchesRoutes(), NewService(), NewHandler() (+20 more)

### Community 35 - "waterparameters/service.go"
Cohesion: 0.11
Nodes (15): likePattern(), TestLikePattern(), balanceAt(), before(), insert(), likePattern(), TestInsertKeepsEveryBalance(), TestLikePattern() (+7 more)

### Community 36 - "RecurrenceFields.svelte"
Cohesion: 0.06
Nodes (51): RecurrencePreview, WaterChangeSchedule, WaterChangeScheduleCreate, WaterChangeSchedulePatch, { form = $bindable(), startsOn, endsOn, timeZone, error }, intervalLabel, legendId, locale (+43 more)

### Community 37 - "New"
Cohesion: 0.10
Nodes (41): createUser(), hash32(), issueToken(), newSession(), pgCode(), TestAuditLog(), TestConcurrentTokenIssueLeavesOneLiveToken(), TestConstraints() (+33 more)

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
Cohesion: 0.16
Nodes (11): Error, FieldError, Errors, Conflict(), Forbidden(), Invalid(), RateLimited(), Unauthenticated() (+3 more)

### Community 45 - "E2E CI Services"
Cohesion: 0.25
Nodes (8): needs, runs-on, services, timeout-minutes, e2e, image, ports, mailpit

### Community 46 - "CI Postgres Service"
Cohesion: 0.25
Nodes (8): POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, env, image, options, ports, postgres

### Community 47 - "IsKind"
Cohesion: 0.10
Nodes (25): Kind, TestSettings(), codes(), TestSlugAndTags(), TestStructured(), As(), IsKind(), TestValidatorCollectsAllFields() (+17 more)

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
Nodes (42): productRequest, createTank, ctxKey, dimsField, ErrorBody, ErrorDetail, fieldError, textField (+34 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): components, $defs, operations, paths, RFC-3339, webhooks

### Community 53 - "ref_app"
Cohesion: 0.03
Nodes (58): Share, Trends, User, Locale, session, bio, busy, displayName (+50 more)

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
Cohesion: 0.07
Nodes (31): Handler, ProductInput, Handler, planResponse, NewHandler(), toEvent(), toPlan(), toProduct() (+23 more)

### Community 62 - "MustParse"
Cohesion: 0.12
Nodes (17): Water parameters, 3g. Water parameters (milestones 2.1/2.2 done, values provisional), mustD(), TestDeviation(), deviationSeverity(), MustParse(), TestAddSubRound(), TestCmpAndSign() (+9 more)

### Community 68 - "Runner"
Cohesion: 0.13
Nodes (6): App, 5. Background work, randomJitter(), Runner, periodic, Source

### Community 69 - "Service"
Cohesion: 0.09
Nodes (26): ListMaintenanceEventsParams, Configuration, DeviceInput, Equipment, EventFilter, EventInput, PlanInput, PlanPatch (+18 more)

### Community 70 - "Validator"
Cohesion: 0.12
Nodes (15): PlanPatch, planPatchRequest, DevicePatch, equipmentPatchRequest, planPatchRequest, updateMeRequest, Field, Of() (+7 more)

### Community 71 - "Decimal"
Cohesion: 0.05
Nodes (41): CompleteWaterChangeEventParams, CreateTargetParams, InsertDosingOccurrenceParams, InsertWaterChangeOccurrenceParams, ResolveDosingEventParams, UpdateDosingPlanParams, UpdateTankParams, UpdateWaterChangeScheduleParams (+33 more)

### Community 72 - "Date"
Cohesion: 0.12
Nodes (22): Plan, timeToDate(), sameDate(), dateToTime(), decorate(), localToday(), nextDue(), optDateToTime() (+14 more)

### Community 73 - "Env"
Cohesion: 0.15
Nodes (20): Browser, Env, Response, addDevice(), addPlan(), count(), items(), setup() (+12 more)

### Community 74 - "IsNoRows"
Cohesion: 0.10
Nodes (18): AdHocInput, PlanInput, Product, ResolveInput, Service, GetDosingEventRow, GetDosingPlanRow, LockDosingPlanRow (+10 more)

### Community 75 - "checker"
Cohesion: 0.10
Nodes (7): checker, MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune(), checker

### Community 76 - "intelligence/metrics.go"
Cohesion: 0.13
Nodes (24): DosingAdherence, LivestockSummary, ParameterMetrics, Snapshot, WaterChangeCadence, Window, abs(), adherenceEvents() (+16 more)

### Community 77 - "Handler"
Cohesion: 0.21
Nodes (6): NewHandler(), tankID(), toEstimate(), toEvent(), toSchedule(), Handler

### Community 78 - "Handler"
Cohesion: 0.15
Nodes (8): NewHandler(), parseTime(), toReading(), toTarget(), toTest(), writeTargets(), CorrectionInput, Handler

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 81 - "recurrence_test.go"
Cohesion: 0.14
Nodes (28): grace(), TestGrace(), Rule, ints(), join(), minSpacing(), mondayIndex(), Parse() (+20 more)

### Community 82 - "waterparameters/http_integration_test.go"
Cohesion: 0.48
Nodes (14): count(), items(), reading(), readings(), record(), setup(), TestAuthorizationArchiveErasure(), TestCatalogMatchesDatabase() (+6 more)

### Community 83 - "NewRouter"
Cohesion: 0.08
Nodes (27): Options, WriteError(), AccessLog(), LimitBody(), Recover(), RequestIDFrom(), SecurityHeaders(), IsSafeMethod() (+19 more)

### Community 84 - "Limiter"
Cohesion: 0.18
Nodes (3): Fixed, Limiter, bucket

### Community 85 - "product-requirements.md"
Cohesion: 0.24
Nodes (3): 1. Bounded contexts (modules), 3. Authorization model, ReefCare — Domain Model

### Community 86 - "5. Initial endpoints"
Cohesion: 0.08
Nodes (24): 5. Initial endpoints, Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation), Dosing, Equipment, maintenance, livestock (Phase 2), Health, Intelligence (Phase 3), Notifications (+16 more)

### Community 87 - "rules.go"
Cohesion: 0.24
Nodes (26): bound, context, rule, tolerance, byTime(), agreeing(), confidence(), coreParameters() (+18 more)

### Community 88 - "ReefCare — Knowledge Base and Reef Intelligence Engine"
Cohesion: 0.13
Nodes (15): 10. Feedback loop, 1. Purpose and boundaries, 2. Knowledge base, 3. Pipeline, 4. Output model, 5. MVP rule set (v1.0.0), 6. Safety rules (enforced in validation step), 7. Triggers (+7 more)

### Community 89 - "dosing/http_integration_test.go"
Cohesion: 0.45
Nodes (13): count(), items(), plan(), setup(), TestAdHocIdempotency(), TestAuthorizationArchiveErasure(), TestConcurrentKeyedAdHoc(), TestPlanLifecycle() (+5 more)

### Community 90 - "ReefCare — Security Design"
Cohesion: 0.14
Nodes (13): 10. Secrets and supply chain, 11. Abuse prevention (community), 1. Threat model (summary), 2. Authentication, 3. Authorization, 4. Web protections, 5. Data access, 6. File uploads (Phase 4 or when photos ship) (+5 more)

### Community 91 - "parser"
Cohesion: 0.35
Nodes (5): itoa(), Dimensions, fields, parser, Tank

### Community 92 - "Finding"
Cohesion: 0.15
Nodes (20): EvidenceRef, Finding, Hypothesis, KnowledgeRef, MissingItem, Observation, Result, SafetyWarning (+12 more)

### Community 93 - "ReefCare — Architecture"
Cohesion: 0.17
Nodes (12): 1. Overview, 2. Technology decisions, 3. Repository layout, 4. Module anatomy, 6. Configuration, 7. Environments, 8. Observability, 9. CI (+4 more)

### Community 94 - "ReefCare — User Stories and Acceptance Criteria"
Cohesion: 0.17
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Record"
Cohesion: 0.11
Nodes (19): Valid(), Lookup(), hashIf(), loadTargets(), replay(), sameTarget(), stored(), checker (+11 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.14
Nodes (14): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 3f. Notifications (milestone 2.9 done), 3g. Knowledge base (milestone 3.1 done) (+6 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 100 - "Current status"
Cohesion: 0.24
Nodes (10): Current status, ClientIP(), ClientIPFrom(), parseHop(), RateLimitKey(), RateLimitNetKey(), remoteAddr(), TruncatedIP() (+2 more)

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.13
Nodes (12): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, 6. Rate limits (initial), ReefCare — API Guidelines and Initial Contracts, API binary, Development (+4 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.17
Nodes (12): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 6. Phase 4 — community (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency (+4 more)

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
Cohesion: 0.25
Nodes (8): Open decisions (need product owner input), Phase 0 — Product and technical design ✅ drafted, awaiting review, Phase 2 — Reef management, Phase 3 — Reef Intelligence, Phase 4 — Community (Release 2), Phase 5 — Production readiness, ReefCare — Implementation Roadmap, Risks

### Community 109 - "Reef Intelligence and knowledge base rules"
Cohesion: 0.29
Nodes (6): Knowledge base, Mandatory safety rules, Output model, Pipeline (keep these stages explicit and individually testable), Provider independence, Reef Intelligence and knowledge base rules

### Community 110 - "intelligence/metrics_test.go"
Cohesion: 0.42
Nodes (8): d(), eq(), rd(), TestAdherence(), TestCadenceAndLivestock(), TestParameterMetrics(), TestTargetAtMeasurementTime(), dp()

### Community 111 - "ParsePageParams"
Cohesion: 0.50
Nodes (3): PageParams, DecodeCursor(), ParsePageParams()

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 116 - "app.go"
Cohesion: 0.06
Nodes (17): System, main(), Error, emailTemplate, emailBody(), mayEmail(), quietHours(), TestEmailBody() (+9 more)

### Community 119 - "net/http.ResponseWriter"
Cohesion: 0.17
Nodes (10): actor(), date(), nonNil(), toArticle(), toSource(), toVersion(), utc(), handleHealthz() (+2 more)

## Knowledge Gaps
- **9 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+4 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 2214 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **25 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Current status` connect `Current status` to `New`, `net/http.Request`, `New`, `api`, `Identity HTTP Handlers`, `Tank Authorization`, `ReefCare — Claude Code Project Guide`, `ReefCare — Security Design`?**
  _High betweenness centrality (0.373) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _9 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `en.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `api()` connect `api` to `Current status`, `RecurrenceFields.svelte`, `Typed API Client`, `Alert.svelte`, `water-tests/+page.svelte`, `dosing/+page.svelte`, `versions/[id]/+page.svelte`, `DoseResolveDialog.svelte`, `ref_app`, `[equipmentId]/+page.svelte`, `water-changes/+page.svelte`, `[entryId]/+page.svelte`?**
  _High betweenness centrality (0.364) - this node is a cross-community bridge._
- **Should `vi.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `svelte` connect `Alert.svelte` to `versions/[id]/+page.svelte`, `package.json`?**
  _High betweenness centrality (0.189) - this node is a cross-community bridge._
- **Should `Service` be split into smaller, more focused modules?**
  _Cohesion score 0.08888888888888889 - nodes in this community are weakly interconnected._