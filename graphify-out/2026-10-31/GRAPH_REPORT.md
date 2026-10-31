# Graph Report - reefcare  (2026-10-31)

## Corpus Check
- 367 files · ~291,814 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 5008 nodes · 14222 edges · 140 communities (107 shown, 33 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 347 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2614df6b`
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
- Service
- helpers.ts
- eslint.config.js
- Renovate Config
- overrides
- NPM Scripts
- log/slog.Logger
- move
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
- app.go
- Generated OpenAPI Types
- New
- ReefCare — Claude Code Project Guide
- Go CI Job
- CI Triggers
- .Perform
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
- encoding/json.RawMessage
- 2. Core concepts
- Decimal
- Date
- Env
- IsNoRows
- checker
- intelligence/metrics.go
- Handler
- waterchanges/http_integration_test.go
- checker
- recurrence_test.go
- waterparameters/http_integration_test.go
- NewRouter
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
- checker
- Finding
- ReefCare — Claude Code Project Guide
- devDependencies
- ReefCare — Database Design
- playwright.config.ts
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- intelligence/metrics_test.go
- package.json
- Handler
- Community, Reef Clinic and reputation rules
- equipment/http_integration_test.go
- PostgreSQL, migrations and sqlc
- time.Time
- checker
- .List
- net/http.ResponseWriter
- .Timeline
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- CreateSpeciesParams
- poolCollector
- writeEvent
- ListPublishedKnowledgeRow
- toPlan
- toEvent
- vitest-browser-svelte

## God Nodes (most connected - your core abstractions)
1. `Decimal` - 136 edges
2. `New()` - 125 edges
3. `WriteJSON()` - 84 edges
4. `api()` - 82 edges
5. `Queries` - 80 edges
6. `Actor` - 72 edges
7. `InTx()` - 65 edges
8. `NewValidator()` - 58 edges
9. `ApiError` - 57 edges
10. `IsNoRows()` - 56 edges

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
- 4-file cycle: `internal/app/app.go -> internal/intelligence/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/intelligence/http_integration_test.go -> internal/waterparameters/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`

## Communities (140 total, 33 thin omitted)

### Community 0 - "en.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 1 - "vi.json"
Cohesion: 0.00
Nodes (500): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+492 more)

### Community 2 - "Service"
Cohesion: 0.14
Nodes (19): GetLivestockEntryRow, ListLivestockEventsRow, likePattern(), stored(), TestLikePattern(), TestStoredTruncatesToMicroseconds(), checkKey(), entryFromRow() (+11 more)

### Community 3 - "New"
Cohesion: 0.10
Nodes (34): Entry, Client, Hasher, LoginInput, Principal, RegisterInput, Service, Session (+26 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.07
Nodes (65): MustURL(), New(), TestAmountIsExact(), TestUnitsByForm(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable(), TestHostileInputs() (+57 more)

### Community 5 - "Outbox"
Cohesion: 0.16
Nodes (9): Outbox, randomSuffix(), Message, NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), LogSender, SMTPConfig (+1 more)

### Community 6 - "Queries"
Cohesion: 0.06
Nodes (39): 3. Background jobs (milestone 2.3 done), Assessment, Cursor, Service, Queries, ForAssessment(), Reminders(), EraseUserData() (+31 more)

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

### Community 11 - "NewValidator"
Cohesion: 0.14
Nodes (21): LockKnowledgeVersionRow, authorOfLiveArticle(), parseVersion(), requireEditor(), setSources(), sources(), sourcesForClaim(), validURL() (+13 more)

### Community 12 - "api"
Cohesion: 0.02
Nodes (118): api(), Notification, Share, User, formatDate(), formatDateTime(), today(), zoneOrFallback() (+110 more)

### Community 13 - "versions/[id]/+page.svelte"
Cohesion: 0.03
Nodes (81): ClaimClass, KnowledgeSource, KnowledgeStructured, KnowledgeVersion, KnowledgeVersionInput, body, busy, claim (+73 more)

### Community 14 - "github.com/google/uuid.UUID"
Cohesion: 0.02
Nodes (102): Assessment, AssessmentEvidence, AssessmentKnowledgeRef, AuditLog, AuthToken, CreateAdHocDoseParams, CreateEquipmentParams, CreateLivestockEntryParams (+94 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.12
Nodes (15): Current status, Handler, client(), NewHandler(), PrincipalFrom(), toUserResponse(), ClientIP(), ClientIPFrom() (+7 more)

### Community 16 - "DoseResolveDialog.svelte"
Cohesion: 0.02
Nodes (115): DoseEvent, DoseUnit, DosingForm, DosingPlan, DosingPlanCreate, DosingPlanPatch, DosingProduct, amount (+107 more)

### Community 17 - "Load"
Cohesion: 0.14
Nodes (9): parser, Migrator, Load(), originOf(), env(), TestAllowedOrigins(), TestLoadDefaults(), TestLoadReportsAllErrors() (+1 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.18
Nodes (8): EncodeCursor(), NewPage(), actor(), pathID(), toResponse(), toShareResponse(), Handler, shareResponse

### Community 19 - "NewPage"
Cohesion: 0.15
Nodes (9): PageParams, userID(), BadRequest(), DecodeCursor(), ParsePageParams(), decodeError(), jsonKind(), parseTime() (+1 more)

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.15
Nodes (13): Phase 1 — Foundation, Done on 2026-10-30 (committed), 3. Authorization, GetTankAccessRow, GetTankRow, decide(), Access, Authorizer (+5 more)

### Community 22 - "Validator"
Cohesion: 0.21
Nodes (6): ProfilePatch, mergeNotifyPrefs(), mergeUnitPrefs(), validateDisplayName(), validTimezone(), Validator

### Community 23 - "[equipmentId]/+page.svelte"
Cohesion: 0.03
Nodes (87): Equipment, EquipmentConfiguration, EquipmentCreate, EquipmentPatch, MaintenancePlan, MaintenancePlanCreate, MaintenancePlanPatch, Page (+79 more)

### Community 24 - "water-changes/+page.svelte"
Cohesion: 0.03
Nodes (86): MaintenanceEvent, WaterChange, WaterChangeEstimate, WaterChangeRecord, executedAt, { estimate }, locale, busy (+78 more)

### Community 25 - "context.Context"
Cohesion: 0.01
Nodes (61): AddAssessmentEvidenceParams, AddAssessmentKnowledgeRefParams, AddKnowledgeVersionSourceParams, AnonymizeUserParams, ArchiveTankParams, BuryJobParams, ChangeUserPasswordParams, ClaimPendingNotificationEmailsParams (+53 more)

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
Cohesion: 0.12
Nodes (17): GetWaterChangeEventRow, GetWaterChangeScheduleRow, LockWaterChangeScheduleRow, applyPatch(), decorateEvent(), eventFromRow(), recordTimeline(), sameText() (+9 more)

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
Nodes (51): App, Deps, PoolConfig, Config, Limits, migrate(), New(), openDB() (+43 more)

### Community 35 - "move"
Cohesion: 0.70
Nodes (5): balanceAt(), before(), insert(), TestInsertKeepsEveryBalance(), move

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
Cohesion: 0.13
Nodes (14): Error, FieldError, Kind, Errors, Conflict(), Forbidden(), Invalid(), IsKind() (+6 more)

### Community 45 - "E2E CI Services"
Cohesion: 0.25
Nodes (8): needs, runs-on, services, timeout-minutes, e2e, image, ports, mailpit

### Community 46 - "CI Postgres Service"
Cohesion: 0.25
Nodes (8): POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, env, image, options, ports, postgres

### Community 47 - "IsKind"
Cohesion: 0.13
Nodes (19): TestSettings(), codes(), TestSlugAndTags(), TestStructured(), As(), TestValidatorCollectsAllFields(), jsonRequest(), TestDecodeJSON() (+11 more)

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
Cohesion: 0.05
Nodes (77): main(), Error, 6. Phase 4 — community (summary), PlanPatch, planPatchRequest, productRequest, DevicePatch, equipmentPatchRequest (+69 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): components, $defs, operations, paths, RFC-3339, webhooks

### Community 53 - "New"
Cohesion: 0.12
Nodes (12): PlanInput, New(), TestPreferences(), wants(), wantsEmail(), IsForeignKeyViolation(), IsNoRows(), Job (+4 more)

### Community 54 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.22
Nodes (9): Architecture: modular monolith, Commands, Definition of Done, graphify, Non-negotiable domain rules, ReefCare — Claude Code Project Guide, Roadmap, Stack (unless the repo later establishes a better-justified alternative) (+1 more)

### Community 55 - "Go CI Job"
Cohesion: 0.50
Nodes (4): runs-on, services, timeout-minutes, go

### Community 56 - "CI Triggers"
Cohesion: 0.50
Nodes (4): on, pull_request, push, branches

### Community 57 - ".Perform"
Cohesion: 0.25
Nodes (10): EventFilter, EventInput, ListMaintenanceEventsRow, checkKey(), eventFromRow(), hashIf(), performedSummary(), replay() (+2 more)

### Community 58 - "net/http.Request"
Cohesion: 0.11
Nodes (24): Handler, Handler, toEvent(), toPlan(), toProduct(), dateString(), toConfig(), toEquipment() (+16 more)

### Community 62 - "MustParse"
Cohesion: 0.10
Nodes (22): Water parameters, 3g. Water parameters (milestones 2.1/2.2 done, values provisional), deviation(), mustD(), TestDeviation(), mul(), MustParse(), TestAddSubRound() (+14 more)

### Community 68 - "Runner"
Cohesion: 0.09
Nodes (8): 5. Background work, Source, IsPermanent(), randomJitter(), TestPermanent(), Runner, Observer, Source

### Community 69 - "encoding/json.RawMessage"
Cohesion: 0.05
Nodes (35): ClaimJobsParams, ClaimJobsRow, CreateAssessmentParams, CreateConfigurationParams, CreateConfigurationRow, CreateKnowledgeVersionParams, EnqueueJobParams, GetAssessmentRow (+27 more)

### Community 70 - "2. Core concepts"
Cohesion: 0.15
Nodes (13): 1. Bounded contexts (modules), 2. Core concepts, 3. Authorization model, Community (Release 2), Dosing, Equipment & maintenance, Identity, Knowledge & intelligence (+5 more)

### Community 71 - "Decimal"
Cohesion: 0.03
Nodes (49): CompleteWaterChangeEventParams, CreateDosingPlanParams, CreateTargetParams, CreateWaterChangeEventParams, CreateWaterChangeScheduleParams, InsertDosingOccurrenceParams, InsertWaterChangeOccurrenceParams, LatestValidReadingsRow (+41 more)

### Community 72 - "Date"
Cohesion: 0.13
Nodes (19): Plan, sameDate(), decorate(), localToday(), nextDue(), optTimeToDate(), date(), TestDecorate() (+11 more)

### Community 73 - "Env"
Cohesion: 0.11
Nodes (25): Intelligence (Phase 3), Browser, Env, Response, count(), findings(), items(), publishKnowledge() (+17 more)

### Community 74 - "IsNoRows"
Cohesion: 0.12
Nodes (16): AdHocInput, PlanInput, Product, ProductInput, ResolveInput, Service, GetDosingEventRow, GetDosingPlanRow (+8 more)

### Community 75 - "checker"
Cohesion: 0.15
Nodes (6): MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune(), checker

### Community 76 - "intelligence/metrics.go"
Cohesion: 0.25
Nodes (17): ParameterMetrics, WaterChangeCadence, Window, abs(), AllMetrics(), byTime(), CadenceFor(), Cycling() (+9 more)

### Community 77 - "Handler"
Cohesion: 0.24
Nodes (4): tankID(), toEvent(), toSchedule(), Handler

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 80 - "checker"
Cohesion: 0.19
Nodes (6): checker, Item, Recommendation, sourceRefRequest, Structured, versionRequest

### Community 81 - "recurrence_test.go"
Cohesion: 0.14
Nodes (28): grace(), TestGrace(), Rule, ints(), join(), minSpacing(), mondayIndex(), Parse() (+20 more)

### Community 82 - "waterparameters/http_integration_test.go"
Cohesion: 0.48
Nodes (14): count(), items(), reading(), readings(), record(), setup(), TestAuthorizationArchiveErasure(), TestCatalogMatchesDatabase() (+6 more)

### Community 83 - "NewRouter"
Cohesion: 0.12
Nodes (23): Options, WriteError(), LimitBody(), Recover(), RequestIDFrom(), IsSafeMethod(), OriginCheck(), TestRetryAfterIsAtLeastOneSecond() (+15 more)

### Community 84 - "Limiter"
Cohesion: 0.11
Nodes (12): DosingAdherence, adherenceEvents(), AdherenceFor(), TestAdherence(), missedDoses(), ruleAdherence(), Fixed, Backoff() (+4 more)

### Community 86 - "5. Initial endpoints"
Cohesion: 0.15
Nodes (13): 5. Initial endpoints, Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation), Dosing, Equipment, maintenance, livestock (Phase 2), Health, Notifications, Tanks (+5 more)

### Community 87 - "rules.go"
Cohesion: 0.23
Nodes (27): bound, evalCtx, rule, tolerance, agreeing(), confidence(), coreParameters(), correctionWarning() (+19 more)

### Community 88 - "ReefCare — Knowledge Base and Reef Intelligence Engine"
Cohesion: 0.13
Nodes (15): 10. Feedback loop, 1. Purpose and boundaries, 2. Knowledge base, 3. Pipeline, 4. Output model, 5. MVP rule set (v1.0.0), 6. Safety rules (enforced in validation step), 7. Triggers (+7 more)

### Community 89 - "dosing/http_integration_test.go"
Cohesion: 0.45
Nodes (13): count(), items(), plan(), setup(), TestAdHocIdempotency(), TestAuthorizationArchiveErasure(), TestConcurrentKeyedAdHoc(), TestPlanLifecycle() (+5 more)

### Community 90 - "ReefCare — Security Design"
Cohesion: 0.15
Nodes (12): 10. Secrets and supply chain, 11. Abuse prevention (community), 1. Threat model (summary), 2. Authentication, 4. Web protections, 5. Data access, 6. File uploads (Phase 4 or when photos ship), 7. Intelligence and LLM boundary (+4 more)

### Community 92 - "Finding"
Cohesion: 0.12
Nodes (13): EquipmentChange, Evaluate(), newContext(), RuleIDs(), checkProperties(), loadFixture(), TestRulesOnFixtures(), TestSnapshotLimitIsReported() (+5 more)

### Community 93 - "ReefCare — Architecture"
Cohesion: 0.17
Nodes (12): 1. Overview, 2. Technology decisions, 3. Repository layout, 4. Module anatomy, 6. Configuration, 7. Environments, 8. Observability, 9. CI (+4 more)

### Community 94 - "ReefCare — User Stories and Acceptance Criteria"
Cohesion: 0.17
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Record"
Cohesion: 0.10
Nodes (21): Lookup(), hashIf(), loadTargets(), replay(), sameTarget(), stored(), Annotation, checker (+13 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.15
Nodes (13): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 3f. Notifications (milestone 2.9 done), 3g. Knowledge base (milestone 3.1 done) (+5 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 100 - "Finding"
Cohesion: 0.36
Nodes (12): assessmentResponse, EvidenceRef, Finding, Hypothesis, KnowledgeRef, MissingItem, Observation, Result (+4 more)

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.13
Nodes (12): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, 6. Rate limits (initial), ReefCare — API Guidelines and Initial Contracts, API binary, Development (+4 more)

### Community 102 - "devDependencies"
Cohesion: 0.15
Nodes (13): openapi-typescript, playwright, svelte-check, @sveltejs/vite-plugin-svelte, @types/node, vite, devDependencies, openapi-typescript@^7.13.0 (+5 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.20
Nodes (10): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency, 9. Seed data (+2 more)

### Community 105 - "feature/SKILL.md"
Cohesion: 0.25
Nodes (7): 1. Inspect, 2. Analyze (write it down briefly in the conversation), 3. Design, 4. Implement, 5. Verify, 6. Review, 7. Summarize

### Community 106 - "InSerializableTx"
Cohesion: 0.39
Nodes (6): TxStarter, TestInSerializableTxRetries(), IsSerializationFailure(), InReadSnapshotTx(), InSerializableTx(), inTx()

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
Cohesion: 0.25
Nodes (9): LivestockSummary, LivestockFor(), d(), eq(), rd(), TestCadenceAndLivestock(), TestParameterMetrics(), TestTargetAtMeasurementTime() (+1 more)

### Community 111 - "package.json"
Cohesion: 0.15
Nodes (12): prettier, prettier-plugin-svelte, typescript, prettier@^3.8.3, prettier-plugin-svelte@^4.1.0, typescript@^6.0.3, imports, #lib (+4 more)

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 114 - "equipment/http_integration_test.go"
Cohesion: 0.58
Nodes (10): addDevice(), addPlan(), count(), items(), setup(), TestAuthorizationArchiveErasure(), TestEquipmentLifecycle(), TestMaintenanceDue() (+2 more)

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 116 - "time.Time"
Cohesion: 0.02
Nodes (112): System, CompleteJobParams, ConsumeAuthTokenParams, ConsumeAuthTokenRow, CreateKnowledgeArticleParams, CreateKnowledgeArticleRow, CreateKnowledgeSourceParams, CreateKnowledgeSourceRow (+104 more)

### Community 118 - ".List"
Cohesion: 0.22
Nodes (5): ListNotificationsParams, ListNotificationsRow, ListUnreadNotificationsParams, ListUnreadNotificationsRow, Cursor

### Community 119 - "net/http.ResponseWriter"
Cohesion: 0.12
Nodes (11): statusRecorder, actor(), date(), nonNil(), toArticle(), toSource(), toVersion(), utc() (+3 more)

### Community 120 - ".Timeline"
Cohesion: 0.22
Nodes (6): ListTankEventsParams, ListTankEventsRow, IsCategory(), TimelineCursor, TimelineEvent, TimelineFilter

### Community 133 - "writeEvent"
Cohesion: 0.67
Nodes (3): eventResponse, toEvent(), writeEvent()

## Knowledge Gaps
- **9 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+4 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 2215 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **33 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `api()` connect `api` to `RecurrenceFields.svelte`, `Typed API Client`, `web_src_lib_paraglide_messages_m`, `water-tests/+page.svelte`, `dosing/+page.svelte`, `versions/[id]/+page.svelte`, `Identity HTTP Handlers`, `DoseResolveDialog.svelte`, `[equipmentId]/+page.svelte`, `water-changes/+page.svelte`, `[entryId]/+page.svelte`?**
  _High betweenness centrality (0.368) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _9 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `en.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `Current status` connect `Identity HTTP Handlers` to `New`, `Runner`, `New`, `Queries`, `net/http.Request`, `api`, `NewRouter`, `Tank Authorization`, `ReefCare — Claude Code Project Guide`, `ReefCare — Security Design`?**
  _High betweenness centrality (0.367) - this node is a cross-community bridge._
- **Should `vi.json` be split into smaller, more focused modules?**
  _Cohesion score 0.003992015968063872 - nodes in this community are weakly interconnected._
- **Why does `svelte` connect `web_src_lib_paraglide_messages_m` to `versions/[id]/+page.svelte`, `package.json`?**
  _High betweenness centrality (0.189) - this node is a cross-community bridge._
- **Should `Service` be split into smaller, more focused modules?**
  _Cohesion score 0.14082503556187767 - nodes in this community are weakly interconnected._