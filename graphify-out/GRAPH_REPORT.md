# Graph Report - reefcare  (2026-10-31)

## Corpus Check
- 374 files · ~298,594 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 5094 nodes · 14509 edges · 143 communities (111 shown, 32 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 349 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `0d01b858`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- en.json
- vi.json
- Service
- New
- Identity Integration Tests
- Outbox
- Queries
- Typed API Client
- web_src_lib_paraglide_messages_m
- water-tests/+page.svelte
- Docker Compose Stack
- [assessmentId]/+page.svelte
- api
- versions/[id]/+page.svelte
- go_pkg_time
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
- github.com/google/uuid.UUID
- sqlc Configuration
- [entryId]/+page.svelte
- Service
- helpers.ts
- eslint.config.js
- Renovate Config
- overrides
- NPM Scripts
- log/slog.Logger
- apperr.go
- RecurrenceFields.svelte
- New
- CI Workflow Jobs
- TankDashboard.svelte
- mail.go
- Claude Settings Hooks
- CI Workflow Steps
- CI Action Inputs
- recurrence.go
- E2E CI Services
- CI Postgres Service
- Error
- TypeScript Config
- Inlang i18n Settings
- Web CI Job
- app.go
- Generated OpenAPI Types
- notifications/model_test.go
- ReefCare — Claude Code Project Guide
- Go CI Job
- CI Triggers
- WriteJSON
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
- Field
- 2. Core concepts
- Decimal
- Date
- Env
- IsNoRows
- checker
- intelligence/metrics.go
- Handler
- Hasher
- waterchanges/http_integration_test.go
- checker
- recurrence_test.go
- waterparameters/http_integration_test.go
- testing.T
- Limiter
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
- livestock/http_integration_test.go
- Finding
- ReefCare — Claude Code Project Guide
- package.json
- ReefCare — Database Design
- playwright.config.ts
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- intelligence/metrics_test.go
- notifications/http_integration_test.go
- Handler
- Community, Reef Clinic and reputation rules
- checker
- PostgreSQL, migrations and sqlc
- time.Time
- checker
- 5. Background work
- net/http.ResponseWriter
- ReefCare — Web Frontend Proposal (milestone 1.8)
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- toEvent
- poolCollector
- ReefCare — API Guidelines and Initial Contracts
- createTank
- Error
- BadRequest
- permanentError
- System
- CountJobsByStatusRow
- maxSeverity
- TestInput

## God Nodes (most connected - your core abstractions)
1. `Decimal` - 136 edges
2. `New()` - 127 edges
3. `WriteJSON()` - 86 edges
4. `api()` - 85 edges
5. `Queries` - 82 edges
6. `Actor` - 75 edges
7. `InTx()` - 67 edges
8. `NewValidator()` - 60 edges
9. `ApiError` - 59 edges
10. `IsNoRows()` - 58 edges

## Surprising Connections (you probably didn't know these)
- `3. Authentication` --references--> `Origin`  [INFERRED]
  docs/api-guidelines.md → internal/audit/audit.go
- `4. Web protections` --references--> `Origin`  [INFERRED]
  docs/security.md → internal/audit/audit.go
- `Dependency rules` --references--> `Authorizer`  [INFERRED]
  docs/architecture.md → internal/tanks/authz.go
- `Done on 2026-10-30 (committed)` --references--> `Authorizer`  [INFERRED]
  docs/open-items.md → internal/tanks/authz.go
- `3. Authorization` --references--> `Authorizer`  [INFERRED]
  docs/security.md → internal/tanks/authz.go

## Import Cycles
- 3-file cycle: `internal/app/app.go -> internal/dosing/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/equipment/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/intelligence/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/knowledge/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/livestock/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/scheduling/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterchanges/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterparameters/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/assessment.go -> internal/intelligence/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/assessment.go -> internal/intelligence/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/intelligence/feedback.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/intelligence/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`

## Communities (143 total, 32 thin omitted)

### Community 0 - "en.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 1 - "vi.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 2 - "Service"
Cohesion: 0.13
Nodes (20): ListLivestockEventsParams, GetLivestockEntryRow, ListLivestockEventsRow, Species, stored(), TestStoredTruncatesToMicroseconds(), checkKey(), entryFromRow() (+12 more)

### Community 3 - "New"
Cohesion: 0.05
Nodes (65): Entry, PlanInput, PlanPatch, Client, emailJob, jobOrigin, loginFailedJob, Principal (+57 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.10
Nodes (37): MustURL(), New(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable(), TestHostileInputs(), TestLoginFailuresAreUniformAndRateLimited(), TestPasswordResetRevokesAllSessions() (+29 more)

### Community 5 - "Outbox"
Cohesion: 0.16
Nodes (9): Outbox, randomSuffix(), Message, NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), LogSender, SMTPConfig (+1 more)

### Community 6 - "Queries"
Cohesion: 0.08
Nodes (22): DBTX, Assessment, Cursor, Feedback, Service, Queries, ForAssessment(), Reminders() (+14 more)

### Community 7 - "Typed API Client"
Cohesion: 0.04
Nodes (71): Tank, TankCreate, TankPatch, fields, {
		form = $bindable(),
		error,
		busy,
		submitLabel,
		timeZone,
		onsubmit,
		oncancel
	}, localProblems, Props, statusOptions (+63 more)

### Community 8 - "web_src_lib_paraglide_messages_m"
Cohesion: 0.03
Nodes (79): svelte, vitest, ApiError, csrfToken(), KnowledgeArticle, icons, { kind = 'info', children }, Props (+71 more)

### Community 9 - "water-tests/+page.svelte"
Cohesion: 0.02
Nodes (132): LatestReading, ParameterType, Reading, Target, TargetInput, Trends, WaterTest, WaterTestCreate (+124 more)

### Community 10 - "Docker Compose Stack"
Cohesion: 0.05
Nodes (45): build, command, depends_on, environment, ports, condition, environment, healthcheck (+37 more)

### Community 11 - "[assessmentId]/+page.svelte"
Cohesion: 0.05
Nodes (47): Assessment, AssessmentFeedback, Finding, I18nText, assessmentText(), confidenceLabel(), evidenceCount(), kinds (+39 more)

### Community 12 - "api"
Cohesion: 0.02
Nodes (109): @sveltejs/kit, api(), Notification, Share, User, formatDate(), formatDateTime(), today() (+101 more)

### Community 13 - "versions/[id]/+page.svelte"
Cohesion: 0.03
Nodes (81): ClaimClass, KnowledgeSource, KnowledgeStructured, KnowledgeVersion, KnowledgeVersionInput, body, busy, claim (+73 more)

### Community 14 - "go_pkg_time"
Cohesion: 0.23
Nodes (15): AssessmentEvidence, AssessmentKnowledgeRef, CountLivestockEventsRow, CountTankLivestockRow, GetPublishedKnowledgeBySlugParams, KnowledgeVersionSource, ParameterType, eraseJob (+7 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.11
Nodes (18): Current status, 6. Rate limits (initial), Handler, LoginInput, client(), NewHandler(), PrincipalFrom(), toUserResponse() (+10 more)

### Community 16 - "DoseResolveDialog.svelte"
Cohesion: 0.02
Nodes (140): DoseEvent, DoseUnit, DosingForm, DosingPlan, DosingPlanCreate, DosingPlanPatch, DosingProduct, amount (+132 more)

### Community 17 - "Load"
Cohesion: 0.08
Nodes (21): parser, Migrator, PoolConfig, migrate(), openDB(), roles(), Run(), serve() (+13 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.18
Nodes (8): NewPage(), actor(), pathID(), toResponse(), toShareResponse(), Handler, ShareInput, shareResponse

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.12
Nodes (17): GetTankAccessParams, GetTankParams, GetTankAccessRow, GetTankRow, decide(), Access, Authorizer, Perm (+9 more)

### Community 22 - "Validator"
Cohesion: 0.19
Nodes (7): FieldError, ProfilePatch, mergeNotifyPrefs(), mergeUnitPrefs(), validateDisplayName(), validTimezone(), Validator

### Community 23 - "[equipmentId]/+page.svelte"
Cohesion: 0.03
Nodes (92): Equipment, EquipmentConfiguration, EquipmentCreate, EquipmentPatch, MaintenancePlan, MaintenancePlanCreate, MaintenancePlanPatch, Page (+84 more)

### Community 24 - "water-changes/+page.svelte"
Cohesion: 0.03
Nodes (86): MaintenanceEvent, WaterChange, WaterChangeEstimate, WaterChangeRecord, executedAt, { estimate }, locale, busy (+78 more)

### Community 25 - "github.com/google/uuid.UUID"
Cohesion: 0.01
Nodes (148): AddAssessmentEvidenceParams, AddAssessmentKnowledgeRefParams, AddKnowledgeVersionSourceParams, AnonymizeUserParams, ArchiveTankParams, ChangeUserPasswordParams, ClaimPendingNotificationEmailsParams, CloseTargetParams (+140 more)

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

### Community 28 - "Service"
Cohesion: 0.15
Nodes (15): GetWaterChangeEventRow, GetWaterChangeScheduleRow, dateToTime(), decorateEvent(), eventFromRow(), recordTimeline(), sameText(), scheduleFromLocked() (+7 more)

### Community 29 - "helpers.ts"
Cohesion: 0.15
Nodes (12): @axe-core/playwright, @playwright/test, globalSetup(), storageState, emailLink(), password, signIn(), signUp() (+4 more)

### Community 30 - "eslint.config.js"
Cohesion: 0.13
Nodes (13): eslint, eslint-config-prettier, @eslint/js, eslint-plugin-svelte, globals, typescript-eslint, gitignorePath, eslint@^10.4.1 (+5 more)

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
Cohesion: 0.06
Nodes (50): App, Deps, Go backend rules, Options, Config, Limits, New(), NewHandler() (+42 more)

### Community 35 - "apperr.go"
Cohesion: 0.16
Nodes (11): likePattern(), TestLikePattern(), balanceAt(), before(), insert(), likePattern(), TestInsertKeepsEveryBalance(), TestLikePattern() (+3 more)

### Community 36 - "RecurrenceFields.svelte"
Cohesion: 0.05
Nodes (64): ErrorDetail, KnowledgeCategory, Method, onUnauthenticated(), ProfilePatch, RecurrencePreview, Result, Schemas (+56 more)

### Community 37 - "New"
Cohesion: 0.16
Nodes (27): NewFixed(), TestMigrationsUpDownUp(), New(), Spec, enqueue(), jobRow(), quiet(), TestCleanupDeletesOldFinishedJobs() (+19 more)

### Community 38 - "CI Workflow Jobs"
Cohesion: 0.18
Nodes (10): concurrency, cancel-in-progress, group, runs-on, timeout-minutes, jobs, docker, name (+2 more)

### Community 39 - "TankDashboard.svelte"
Cohesion: 0.07
Nodes (29): TimelineEvent, allLoaded, attentionCount, base, doseAttention, doseNext, equipment, failed (+21 more)

### Community 41 - "Claude Settings Hooks"
Cohesion: 0.22
Nodes (8): hooks, PostToolUse, PreToolUse, permissions, allow, ask, deny, $schema

### Community 42 - "CI Workflow Steps"
Cohesion: 0.39
Nodes (9): steps, steps, steps, if, name, run, uses, working-directory (+1 more)

### Community 43 - "CI Action Inputs"
Cohesion: 0.22
Nodes (9): with, cache, cache-dependency-path, go-version-file, name, node-version, path, persist-credentials (+1 more)

### Community 44 - "recurrence.go"
Cohesion: 0.14
Nodes (17): mustD(), TestAmountIsExact(), TestDeviation(), TestGrace(), TestUnitsByForm(), Rule, ints(), join() (+9 more)

### Community 45 - "E2E CI Services"
Cohesion: 0.25
Nodes (8): needs, runs-on, services, timeout-minutes, e2e, image, ports, mailpit

### Community 46 - "CI Postgres Service"
Cohesion: 0.25
Nodes (8): POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, env, image, options, ports, postgres

### Community 47 - "Error"
Cohesion: 0.08
Nodes (30): Error, Kind, Errors, TestSettings(), As(), Forbidden(), Invalid(), IsKind() (+22 more)

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
Nodes (53): planRequest, productRequest, planRequest, ctxKey, ErrorBody, ErrorDetail, fieldError, PageParams (+45 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): components, $defs, operations, paths, RFC-3339, webhooks

### Community 53 - "notifications/model_test.go"
Cohesion: 0.22
Nodes (8): emailBody(), mayEmail(), quietHours(), TestEmailBody(), TestEmailLimits(), TestPreferences(), TestQuietHours(), wants()

### Community 54 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.22
Nodes (9): Architecture: modular monolith, Commands, Definition of Done, graphify, Non-negotiable domain rules, ReefCare — Claude Code Project Guide, Roadmap, Stack (unless the repo later establishes a better-justified alternative) (+1 more)

### Community 55 - "Go CI Job"
Cohesion: 0.50
Nodes (4): runs-on, services, timeout-minutes, go

### Community 56 - "CI Triggers"
Cohesion: 0.50
Nodes (4): on, pull_request, push, branches

### Community 57 - "WriteJSON"
Cohesion: 0.22
Nodes (5): Handler, toPlan(), toProduct(), WriteJSON(), handleHealthz()

### Community 58 - "net/http.Request"
Cohesion: 0.12
Nodes (16): eventResponse, Handler, planResponse, toConfig(), toEquipment(), toEvent(), toPlan(), writeEvent() (+8 more)

### Community 62 - "MustParse"
Cohesion: 0.13
Nodes (19): Water parameters, 3g. Water parameters (milestones 2.1/2.2 done, values provisional), mul(), MustParse(), TestAddSubRound(), TestCmpAndSign(), TestDigitsAndFits(), TestDivRound() (+11 more)

### Community 68 - "Runner"
Cohesion: 0.11
Nodes (8): Backoff(), IsPermanent(), randomJitter(), TestBackoff(), TestPermanent(), Runner, Observer, periodic

### Community 69 - "Field"
Cohesion: 0.12
Nodes (13): PlanPatch, planPatchRequest, DevicePatch, equipmentPatchRequest, planPatchRequest, updateMeRequest, Field, Of() (+5 more)

### Community 70 - "2. Core concepts"
Cohesion: 0.15
Nodes (13): 1. Bounded contexts (modules), 2. Core concepts, 3. Authorization model, Community (Release 2), Dosing, Equipment & maintenance, Identity, Knowledge & intelligence (+5 more)

### Community 71 - "Decimal"
Cohesion: 0.06
Nodes (37): DosingEvent, DosingPlan, ParameterTarget, Tank, WaterChangeEvent, WaterChangeSchedule, WaterTestReading, bound (+29 more)

### Community 72 - "Date"
Cohesion: 0.05
Nodes (47): ListMaintenanceEventsParams, Event, Plan, ProductRef, Configuration, DeviceInput, Equipment, Event (+39 more)

### Community 73 - "Env"
Cohesion: 0.12
Nodes (26): Browser, Env, Response, addDevice(), addPlan(), count(), items(), setup() (+18 more)

### Community 74 - "IsNoRows"
Cohesion: 0.14
Nodes (16): PlanInput, Product, ProductInput, Service, GetDosingEventRow, GetDosingPlanRow, DosingProduct, dateToTime() (+8 more)

### Community 75 - "checker"
Cohesion: 0.31
Nodes (5): MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune()

### Community 76 - "intelligence/metrics.go"
Cohesion: 0.22
Nodes (17): LivestockSummary, ParameterMetrics, WaterChangeCadence, Window, AllMetrics(), byTime(), CadenceFor(), Cycling() (+9 more)

### Community 77 - "Handler"
Cohesion: 0.22
Nodes (5): tankID(), toEstimate(), toEvent(), toSchedule(), Handler

### Community 78 - "Hasher"
Cohesion: 0.27
Nodes (8): Hasher, Argon2Params, NewHasher(), parseHash(), TestDeriveGivesUpWhenSlotsAreBusy(), TestHashAndVerify(), TestVerifyFlagsOutdatedParams(), TestVerifyRejectsMalformedHashes()

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 80 - "checker"
Cohesion: 0.19
Nodes (6): checker, Item, Recommendation, sourceRefRequest, Structured, versionRequest

### Community 81 - "recurrence_test.go"
Cohesion: 0.39
Nodes (15): eq(), fmtAll(), mustLoc(), mustRule(), TestDailyMultipleHoursAndWindow(), TestDSTGapAndOverlap(), TestDSTGapWestOfUTC(), TestDSTKeepsLocalTime() (+7 more)

### Community 82 - "waterparameters/http_integration_test.go"
Cohesion: 0.48
Nodes (14): count(), items(), reading(), readings(), record(), setup(), TestAuthorizationArchiveErasure(), TestCatalogMatchesDatabase() (+6 more)

### Community 83 - "testing.T"
Cohesion: 0.09
Nodes (42): TestOpenAPIMatchesRoutes(), createUser(), hash32(), issueToken(), newSession(), pgCode(), TestAuditLog(), TestConcurrentTokenIssueLeavesOneLiveToken() (+34 more)

### Community 84 - "Limiter"
Cohesion: 0.10
Nodes (8): DosingAdherence, adherenceEvents(), AdherenceFor(), missedDoses(), Fixed, Limiter, bucket, checker

### Community 86 - "5. Initial endpoints"
Cohesion: 0.14
Nodes (14): 5. Initial endpoints, Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation), Dosing, Equipment, maintenance, livestock (Phase 2), Health, Intelligence (Phase 3), Notifications (+6 more)

### Community 87 - "rules.go"
Cohesion: 0.25
Nodes (27): evalCtx, rule, itoa(), agreeing(), confidence(), coreParameters(), correctionWarning(), deviationSeverity() (+19 more)

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
Cohesion: 0.23
Nodes (10): itoa(), CreateInput, createRequest, Dimensions, DimensionsInput, fields, parser, Patch (+2 more)

### Community 92 - "Finding"
Cohesion: 0.12
Nodes (12): Evaluate(), newContext(), RuleIDs(), checkProperties(), loadFixture(), TestRulesOnFixtures(), TestSnapshotLimitIsReported(), TestValidationDropsUnsupportedClaims() (+4 more)

### Community 93 - "ReefCare — Architecture"
Cohesion: 0.17
Nodes (12): 1. Overview, 2. Technology decisions, 3. Repository layout, 4. Module anatomy, 6. Configuration, 7. Environments, 8. Observability, 9. CI (+4 more)

### Community 94 - "ReefCare — User Stories and Acceptance Criteria"
Cohesion: 0.17
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Record"
Cohesion: 0.14
Nodes (16): TestValid(), Valid(), Lookup(), hashIf(), loadTargets(), replay(), sameTarget(), stored() (+8 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.14
Nodes (14): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 3f. Notifications (milestone 2.9 done), 3g. Knowledge base (milestone 3.1 done) (+6 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 99 - "livestock/http_integration_test.go"
Cohesion: 0.45
Nodes (9): addFish(), count(), items(), setup(), TestAuthorizationArchiveErasure(), TestIdempotencyAndConcurrency(), TestQuantityFollowsEvents(), TestReviewRules() (+1 more)

### Community 100 - "Finding"
Cohesion: 0.47
Nodes (10): EvidenceRef, Finding, Hypothesis, KnowledgeRef, MissingItem, Observation, Result, SafetyWarning (+2 more)

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.22
Nodes (6): API binary, Development, Endpoints so far, Quick start, ReefCare, Requirements

### Community 102 - "package.json"
Cohesion: 0.07
Nodes (36): @inlang/paraglide-js, openapi-typescript, playwright, prettier, prettier-plugin-svelte, svelte-check, @sveltejs/adapter-static, @sveltejs/vite-plugin-svelte (+28 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.18
Nodes (11): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 6. Phase 4 — community (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency (+3 more)

### Community 105 - "feature/SKILL.md"
Cohesion: 0.25
Nodes (7): 1. Inspect, 2. Analyze (write it down briefly in the conversation), 3. Design, 4. Implement, 5. Verify, 6. Review, 7. Summarize

### Community 106 - "InSerializableTx"
Cohesion: 0.39
Nodes (6): TxStarter, TestInSerializableTxRetries(), IsSerializationFailure(), InReadSnapshotTx(), InSerializableTx(), inTx()

### Community 107 - "ReefCare web"
Cohesion: 0.25
Nodes (5): Checks, Develop, Layout, ReefCare web, Security notes

### Community 108 - "ReefCare — Implementation Roadmap"
Cohesion: 0.22
Nodes (9): Open decisions (need product owner input), Phase 0 — Product and technical design ✅ drafted, awaiting review, Phase 1 — Foundation, Phase 2 — Reef management, Phase 3 — Reef Intelligence, Phase 4 — Community (Release 2), Phase 5 — Production readiness, ReefCare — Implementation Roadmap (+1 more)

### Community 109 - "Reef Intelligence and knowledge base rules"
Cohesion: 0.29
Nodes (6): Knowledge base, Mandatory safety rules, Output model, Pipeline (keep these stages explicit and individually testable), Provider independence, Reef Intelligence and knowledge base rules

### Community 110 - "intelligence/metrics_test.go"
Cohesion: 0.42
Nodes (8): d(), eq(), rd(), TestAdherence(), TestCadenceAndLivestock(), TestParameterMetrics(), TestTargetAtMeasurementTime(), dp()

### Community 111 - "notifications/http_integration_test.go"
Cohesion: 0.54
Nodes (7): count(), items(), kinds(), patchMe(), TestEmailFailureAndQuietHours(), TestReadingAndErasure(), TestReminders()

### Community 112 - "Handler"
Cohesion: 0.20
Nodes (5): assessmentResponse, feedbackResponse, Handler, toFeedback(), toResponse()

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 116 - "time.Time"
Cohesion: 0.01
Nodes (165): Assessment, AssessmentFeedback, AuditLog, AuthToken, BuryJobParams, ClaimJobsParams, ClaimJobsRow, CompleteJobParams (+157 more)

### Community 118 - "5. Background work"
Cohesion: 0.29
Nodes (3): 5. Background work, Source, Source

### Community 119 - "net/http.ResponseWriter"
Cohesion: 0.13
Nodes (11): statusRecorder, actor(), date(), nonNil(), toArticle(), toSource(), toVersion(), utc() (+3 more)

### Community 120 - "ReefCare — Web Frontend Proposal (milestone 1.8)"
Cohesion: 0.29
Nodes (7): 1. Scope of 1.8, 2. Stack, 3. Security requirements, 4. UX requirements (from .claude/rules/frontend.md), 5. Decisions (accepted 2026-10-30), 6. Implementation notes, ReefCare — Web Frontend Proposal (milestone 1.8)

### Community 125 - "toEvent"
Cohesion: 0.29
Nodes (6): eventResponse, planResponse, productRefJSON, toEvent(), utc(), deviation()

### Community 133 - "ReefCare — API Guidelines and Initial Contracts"
Cohesion: 0.40
Nodes (5): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, ReefCare — API Guidelines and Initial Contracts

### Community 134 - "createTank"
Cohesion: 0.40
Nodes (3): createTank, dimsField, textField

### Community 137 - "BadRequest"
Cohesion: 0.13
Nodes (11): toEntry(), toEvent(), writeCreated(), BadRequest(), decodeError(), jsonKind(), Hash(), Key() (+3 more)

## Knowledge Gaps
- **9 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+4 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 2254 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Current status` connect `Identity HTTP Handlers` to `log/slog.Logger`, `New`, `New`, `Queries`, `api`, `testing.T`, `Tank Authorization`, `5. Background work`, `ReefCare — Claude Code Project Guide`, `ReefCare — Security Design`?**
  _High betweenness centrality (0.374) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _9 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `en.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `api()` connect `api` to `RecurrenceFields.svelte`, `TankDashboard.svelte`, `web_src_lib_paraglide_messages_m`, `water-tests/+page.svelte`, `Typed API Client`, `[assessmentId]/+page.svelte`, `versions/[id]/+page.svelte`, `Identity HTTP Handlers`, `DoseResolveDialog.svelte`, `[equipmentId]/+page.svelte`, `water-changes/+page.svelte`, `[entryId]/+page.svelte`?**
  _High betweenness centrality (0.372) - this node is a cross-community bridge._
- **Should `vi.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `svelte` connect `web_src_lib_paraglide_messages_m` to `versions/[id]/+page.svelte`, `package.json`?**
  _High betweenness centrality (0.184) - this node is a cross-community bridge._
- **Should `Service` be split into smaller, more focused modules?**
  _Cohesion score 0.13205128205128205 - nodes in this community are weakly interconnected._