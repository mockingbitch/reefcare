# Graph Report - reefcare  (2026-10-30)

## Corpus Check
- 215 files · ~185,730 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 6, .graphify-bak 1, .example 1)

## Summary
- 2905 nodes · 8516 edges · 170 communities (103 shown, 67 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 270 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `41a1435b`
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
- NewPage
- Golangci-lint Config
- Tank Authorization
- Validator
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
- livestock/model.go
- Load
- testing.T
- CI Workflow Jobs
- net/http.Handler
- SvelteKit Build Config
- Claude Settings Hooks
- CI Workflow Steps
- CI Action Inputs
- time.Duration
- E2E CI Services
- CI Postgres Service
- Prometheus HTTP Metrics
- TypeScript Config
- Inlang i18n Settings
- Web CI Job
- app.go
- Generated OpenAPI Types
- DB Pool Metrics Collector
- ReefCare — Claude Code Project Guide
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
- IsNoRows
- checker
- WriteJSON
- Handler
- CreateSpeciesParams
- waterchanges/http_integration_test.go
- .List
- Parse
- recurrence_test.go
- NewRouter
- Migrator
- notifications/http_integration_test.go
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
- DosingProduct
- ReefCare — Database Design
- ListEquipmentRow
- feature/SKILL.md
- InSerializableTx
- ReefCare web
- ReefCare — Implementation Roadmap
- Reef Intelligence and knowledge base rules
- Options
- ListTanksForUserRow
- ListLivestockEntriesRow
- Community, Reef Clinic and reputation rules
- identity/jobs_integration_test.go
- PostgreSQL, migrations and sqlc
- Error
- ConsumeAuthTokenRow
- BadRequest
- Handler
- GetLivestockEventByIdempotencyKeyRow
- api.md
- rules/frontend.md
- rules/security.md
- testing.md
- GetMaintenanceEventByIdempotencyKeyRow
- net/netip.Prefix
- toSpecies
- CountJobsByStatusRow
- DeactivateTankMaintenancePlansParams
- DeactivateUserParams
- CreateMaintenanceEventParams
- ListLivestockEventsParams
- SetWaterChangeMaterializedUntilParams
- ListDosingEventsRow
- UpdateMaintenancePlanParams
- ListMaintenanceForRemindersParams
- ListMissedDosesForRemindersRow
- ListUsersDueForErasureParams
- ListWaterChangeEventsRow
- MarkMissedDosingEventsRow
- permanentError
- ArchiveTankParams
- ChangeUserPasswordParams
- ClaimPendingNotificationEmailsParams
- CreateAuthTokenParams
- CreateTankShareParams
- DeleteFutureWaterChangeOccurrencesParams
- GetUserCredentialsByEmailRow
- ListMaintenanceEventsParams
- MarkAllNotificationsReadParams
- MarkEmailVerifiedParams
- RehashUserPasswordParams
- RetryJobParams
- RevokeAllLiveAuthTokensParams
- RevokeLiveAuthTokensParams
- RevokeOtherUserSessionsParams
- RevokeSessionParams
- .RotateSession
- SetDosingMaterializedUntilParams
- TouchSessionParams
- UpdateEquipmentParams
- UpdateLivestockEntryParams
- userResponse

## God Nodes (most connected - your core abstractions)
1. `New()` - 98 edges
2. `Decimal` - 79 edges
3. `Actor` - 63 edges
4. `WriteJSON()` - 61 edges
5. `Queries` - 56 edges
6. `InTx()` - 52 edges
7. `New()` - 45 edges
8. `DecodeJSON()` - 44 edges
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
- `Done on 2026-10-30 (uncommitted)` --references--> `Authorizer`  [INFERRED]
  docs/open-items.md → internal/tanks/authz.go

## Import Cycles
- 3-file cycle: `internal/app/app.go -> internal/dosing/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/equipment/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/livestock/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 3-file cycle: `internal/app/app.go -> internal/waterchanges/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/dosing/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/equipment/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/livestock/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/http.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/reminders.go -> internal/notifications/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`
- 4-file cycle: `internal/app/app.go -> internal/waterchanges/service.go -> internal/tanks/http_integration_test.go -> internal/apptest/apptest.go -> internal/app/app.go`

## Communities (170 total, 67 thin omitted)

### Community 0 - "English UI Messages"
Cohesion: 0.01
Nodes (211): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+203 more)

### Community 1 - "Vietnamese UI Messages"
Cohesion: 0.01
Nodes (211): access_owner, access_viewer, app_name, archive_confirm, archive_confirm_body, archive_confirm_title, archived_notice, back (+203 more)

### Community 2 - "Service"
Cohesion: 0.21
Nodes (13): Queries, ListLivestockEventsRow, stored(), checkKey(), entryFromRow(), eventFromRow(), hashIf(), recordTimeline() (+5 more)

### Community 3 - "New"
Cohesion: 0.13
Nodes (25): Client, Principal, Service, Session, User, Origin, Record(), enqueueEmail() (+17 more)

### Community 4 - "Identity Integration Tests"
Cohesion: 0.13
Nodes (32): MustURL(), New(), TestChangePasswordKeepsCurrentSessionOnly(), TestCrossSiteRequestsRejected(), TestDuplicateRegistrationIsIndistinguishable(), TestHostileInputs(), TestLoginFailuresAreUniformAndRateLimited(), TestPasswordResetRevokesAllSessions() (+24 more)

### Community 5 - "Outbox"
Cohesion: 0.17
Nodes (8): Outbox, Message, NewSMTPSender(), mustAddr(), TestRenderNormalizesLineEndings(), LogSender, SMTPConfig, SMTPSender

### Community 6 - "Decimal"
Cohesion: 0.03
Nodes (42): CompleteWaterChangeEventParams, CreateAdHocDoseParams, CreateDosingPlanParams, CreateTankParams, CreateWaterChangeEventParams, CreateWaterChangeScheduleParams, GetTankParams, InsertDosingOccurrenceParams (+34 more)

### Community 7 - "Typed API Client"
Cohesion: 0.08
Nodes (41): ApiError, Method, ProfilePatch, Result, Schemas, Tank, TankCreate, TankPatch (+33 more)

### Community 8 - "Svelte UI Components"
Cohesion: 0.08
Nodes (31): svelte, icons, { kind = 'info', children }, Props, dialog, {
		open = $bindable(false),
		title,
		confirmLabel,
		danger = false,
		busy = false,
		onconfirm,
		children
	}, Props, titleId (+23 more)

### Community 10 - "Docker Compose Stack"
Cohesion: 0.05
Nodes (45): build, command, depends_on, environment, ports, condition, environment, healthcheck (+37 more)

### Community 11 - "New"
Cohesion: 0.14
Nodes (14): Phase 1 — Foundation, Product, New(), TestInTxRollsBackOnError(), productFromRow(), TestPreferences(), wants(), wantsEmail() (+6 more)

### Community 12 - "Session and Locale Runtime"
Cohesion: 0.09
Nodes (12): @sveltejs/kit, onUnauthenticated(), User, Locale, session, { children }, Locale, localeNames (+4 more)

### Community 13 - "Profile Page Form"
Cohesion: 0.07
Nodes (26): describedBy, id, { label, name, options, value = $bindable(''), error, hint }, Option, Props, bio, busy, displayName (+18 more)

### Community 14 - "time.Time"
Cohesion: 0.02
Nodes (88): System, AuthToken, BuryJobParams, ClaimJobsParams, ClaimJobsRow, CompleteJobParams, CreateConfigurationParams, CreateConfigurationRow (+80 more)

### Community 15 - "Identity HTTP Handlers"
Cohesion: 0.19
Nodes (7): Handler, LoginInput, RegisterInput, client(), PrincipalFrom(), toUserResponse(), DecodeJSON()

### Community 16 - "Account Security Page"
Cohesion: 0.06
Nodes (38): api(), csrfToken(), ErrorDetail, errorMessage(), fieldErrors(), fieldMessage(), allError, changePassword() (+30 more)

### Community 17 - "serve"
Cohesion: 0.16
Nodes (11): PoolConfig, migrate(), openDB(), Run(), serve(), Config, NewMigrator(), Open() (+3 more)

### Community 18 - "Tank HTTP Handlers"
Cohesion: 0.26
Nodes (7): actor(), pathID(), toResponse(), toShareResponse(), Handler, ShareInput, shareResponse

### Community 19 - "NewPage"
Cohesion: 0.26
Nodes (5): NewHandler(), userID(), EncodeCursor(), NewPage(), Handler

### Community 20 - "Golangci-lint Config"
Cohesion: 0.10
Nodes (20): presets, rules, formatters, enable, settings, local-prefixes, linters, default (+12 more)

### Community 21 - "Tank Authorization"
Cohesion: 0.13
Nodes (18): Kind, Errors, GetTankAccessRow, Conflict(), NotFound(), RateLimited(), TestWithCauseIsFoundThroughWrapping(), TestWriteAppError() (+10 more)

### Community 22 - "Validator"
Cohesion: 0.21
Nodes (6): ProfilePatch, mergeNotifyPrefs(), mergeUnitPrefs(), validateDisplayName(), validTimezone(), Validator

### Community 23 - "Tank List and Labels"
Cohesion: 0.12
Nodes (16): Page, statusOptions, typeOptions, experience, experienceLabel(), statuses, statusLabel(), tankTypeLabel() (+8 more)

### Community 24 - "Date Formatting and Tank Detail"
Cohesion: 0.13
Nodes (16): formatDate(), formatDateTime(), formatDecimal(), today(), zoneOrFallback(), actionError, busy, confirmArchive (+8 more)

### Community 25 - "github.com/google/uuid.UUID"
Cohesion: 0.02
Nodes (57): Entry, AnonymizeUserParams, CountLivestockEventsRow, CountTankLivestockRow, CreateEquipmentParams, CreateLivestockEntryParams, CreateMaintenancePlanParams, DeactivateEquipmentMaintenancePlansParams (+49 more)

### Community 26 - "sqlc Configuration"
Cohesion: 0.12
Nodes (16): go, emit_empty_slices, emit_pointers_for_null_types, out, overrides, package, sql_package, db_type (+8 more)

### Community 27 - "Frontend Dev Tooling"
Cohesion: 0.12
Nodes (16): openapi-typescript, playwright, svelte-check, @sveltejs/vite-plugin-svelte, typescript, vite, devDependencies, openapi-typescript@^7.13.0 (+8 more)

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
Cohesion: 0.12
Nodes (15): prettier, prettier-plugin-svelte, @types/node, prettier@^3.8.3, prettier-plugin-svelte@^4.1.0, @types/node@^24, imports, #lib (+7 more)

### Community 33 - "NPM Scripts"
Cohesion: 0.14
Nodes (14): scripts, build, check, check:watch, dev, format, gen:api, i18n (+6 more)

### Community 34 - "App Wiring and Config"
Cohesion: 0.11
Nodes (27): Deps, Go backend rules, Config, Limits, New(), TestOpenAPIMatchesRoutes(), NewHandler(), NewService() (+19 more)

### Community 35 - "livestock/model.go"
Cohesion: 0.29
Nodes (9): balanceAt(), before(), insert(), likePattern(), TestInsertKeepsEveryBalance(), TestLikePattern(), Event, move (+1 more)

### Community 36 - "Load"
Cohesion: 0.28
Nodes (8): parser, Load(), originOf(), env(), TestAllowedOrigins(), TestLoadDefaults(), TestLoadReportsAllErrors(), TestLoadValidation()

### Community 37 - "testing.T"
Cohesion: 0.06
Nodes (54): createUser(), hash32(), issueToken(), newSession(), TestConcurrentTokenIssueLeavesOneLiveToken(), TestConsumeAuthTokenIsSingleUseUnderConcurrency(), TestExpiredAndSupersededTokensAreRejected(), TestPasswordChangeInvalidatesRacingLogin() (+46 more)

### Community 38 - "CI Workflow Jobs"
Cohesion: 0.18
Nodes (10): concurrency, cancel-in-progress, group, runs-on, timeout-minutes, jobs, docker, name (+2 more)

### Community 39 - "net/http.Handler"
Cohesion: 0.14
Nodes (18): Current status, ClientIP(), ClientIPFrom(), parseHop(), RateLimitKey(), RateLimitNetKey(), remoteAddr(), TruncatedIP() (+10 more)

### Community 40 - "SvelteKit Build Config"
Cohesion: 0.15
Nodes (11): @inlang/paraglide-js, @sveltejs/adapter-static, vitest, @vitest/browser-playwright, vitest-browser-svelte, @inlang/paraglide-js@^2.18.2, @sveltejs/adapter-static@^4.0.0, vitest@^4.1.8 (+3 more)

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
Cohesion: 0.09
Nodes (12): Error, FieldError, Forbidden(), Invalid(), Unauthenticated(), Unavailable(), Fixed, Backoff() (+4 more)

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
Cohesion: 0.05
Nodes (71): main(), productRequest, createTank, ctxKey, dimsField, ErrorBody, ErrorDetail, fieldError (+63 more)

### Community 52 - "Generated OpenAPI Types"
Cohesion: 0.29
Nodes (6): RFC-3339, components, $defs, operations, paths, webhooks

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

### Community 58 - "DecodeJSON"
Cohesion: 0.16
Nodes (12): eventResponse, Handler, PlanInput, planResponse, dateString(), toConfig(), toEquipment(), toEvent() (+4 more)

### Community 68 - "Runner"
Cohesion: 0.10
Nodes (8): App, 5. Background work, IsPermanent(), Permanent(), randomJitter(), TestPermanent(), Runner, Observer

### Community 69 - "NewValidator"
Cohesion: 0.17
Nodes (11): Configuration, Equipment, Service, ListConfigurationsRow, attachConfigs(), configFromRow(), equipmentFromRow(), planFromRow() (+3 more)

### Community 70 - "Validator"
Cohesion: 0.11
Nodes (17): PlanPatch, planPatchRequest, DevicePatch, equipmentPatchRequest, PlanPatch, planPatchRequest, updateMeRequest, Field (+9 more)

### Community 71 - "Service"
Cohesion: 0.15
Nodes (14): GetWaterChangeEventRow, applyPatch(), decorateEvent(), eventFromRow(), recordTimeline(), sameText(), scheduleFromLocked(), scheduleFromRow() (+6 more)

### Community 72 - "Date"
Cohesion: 0.13
Nodes (20): Plan, timeToDate(), sameDate(), dateToTime(), decorate(), localToday(), nextDue(), optDateToTime() (+12 more)

### Community 73 - "Env"
Cohesion: 0.21
Nodes (13): Browser, Env, Response, addDevice(), addPlan(), count(), items(), setup() (+5 more)

### Community 74 - "IsNoRows"
Cohesion: 0.21
Nodes (9): Service, sameText(), eventFromRow(), planFromLocked(), planFromRow(), recordTimeline(), usableProduct(), withNext() (+1 more)

### Community 75 - "checker"
Cohesion: 0.16
Nodes (6): checker, MultiLine(), SingleLine(), TestMultiLine(), TestSingleLine(), unsafeRune()

### Community 76 - "WriteJSON"
Cohesion: 0.21
Nodes (6): Handler, ProductInput, toEvent(), toPlan(), toProduct(), WriteJSON()

### Community 77 - "Handler"
Cohesion: 0.22
Nodes (6): tankID(), toEstimate(), toEvent(), toSchedule(), Handler, ScheduleInput

### Community 79 - "waterchanges/http_integration_test.go"
Cohesion: 0.38
Nodes (17): count(), items(), schedule(), setup(), TestAuthorization(), TestCompleteAndSkip(), TestConcurrentIdenticalCompletionsAreIdempotent(), TestDateBounds() (+9 more)

### Community 80 - ".List"
Cohesion: 0.22
Nodes (5): ListNotificationsParams, ListNotificationsRow, ListUnreadNotificationsParams, ListUnreadNotificationsRow, Cursor

### Community 81 - "Parse"
Cohesion: 0.13
Nodes (15): grace(), TestGrace(), Rule, Series, ints(), join(), minSpacing(), mondayIndex() (+7 more)

### Community 82 - "recurrence_test.go"
Cohesion: 0.41
Nodes (15): eq(), fmtAll(), mustLoc(), mustRule(), TestDailyMultipleHoursAndWindow(), TestDSTGapAndOverlap(), TestDSTGapWestOfUTC(), TestDSTKeepsLocalTime() (+7 more)

### Community 83 - "NewRouter"
Cohesion: 0.22
Nodes (14): LimitBody(), TestRetryAfterIsAtLeastOneSecond(), TestTypedNilAppErrorIsInternal(), allowedMethods(), NewRouter(), discardLogger(), do(), TestHealthz() (+6 more)

### Community 85 - "notifications/http_integration_test.go"
Cohesion: 0.54
Nodes (7): count(), items(), kinds(), patchMe(), TestEmailFailureAndQuietHours(), TestReadingAndErasure(), TestReminders()

### Community 86 - "5. Initial endpoints"
Cohesion: 0.14
Nodes (14): 5. Initial endpoints, Auth & profile, Community and reputation (Phase 4, contracts to be detailed before implementation), Dosing, Equipment, maintenance, livestock (Phase 2), Health, Intelligence (Phase 3), Notifications (+6 more)

### Community 87 - "2. Core concepts"
Cohesion: 0.14
Nodes (14): 1. Bounded contexts (modules), 2. Core concepts, 3. Authorization model, Community (Release 2), Dosing, Equipment & maintenance, Identity, Knowledge & intelligence (+6 more)

### Community 88 - "ReefCare — Knowledge Base and Reef Intelligence Engine"
Cohesion: 0.14
Nodes (14): 10. Feedback loop, 1. Purpose and boundaries, 2. Knowledge base, 3. Pipeline, 4. Output model, 5. MVP rule set (v1.0.0), 6. Safety rules (enforced in validation step), 7. Triggers (+6 more)

### Community 89 - "dosing/http_integration_test.go"
Cohesion: 0.45
Nodes (13): count(), items(), plan(), setup(), TestAdHocIdempotency(), TestAuthorizationArchiveErasure(), TestConcurrentKeyedAdHoc(), TestPlanLifecycle() (+5 more)

### Community 90 - "ReefCare — Security Design"
Cohesion: 0.14
Nodes (13): 10. Secrets and supply chain, 11. Abuse prevention (community), 1. Threat model (summary), 2. Authentication, 3. Authorization, 4. Web protections, 5. Data access, 6. File uploads (Phase 4 or when photos ship) (+5 more)

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
Cohesion: 0.12
Nodes (12): Community (R2), Dosing, Equipment & maintenance, Identity, Intelligence, Livestock, Notifications, ReefCare — User Stories and Acceptance Criteria (+4 more)

### Community 95 - ".Perform"
Cohesion: 0.26
Nodes (10): EventFilter, EventInput, ListMaintenanceEventsRow, checkKey(), eventFromRow(), hashIf(), performedSummary(), record() (+2 more)

### Community 96 - "Reef management domain rules"
Cohesion: 0.20
Nodes (9): Dosing, Equipment, Livestock, Notifications and jobs, Reef management domain rules, Tanks, Timeline, Water changes and maintenance (+1 more)

### Community 97 - "ReefCare — Open Items"
Cohesion: 0.17
Nodes (12): 1. Must do before launch, 2. Unfinished Phase 1 milestones, 3b. Water changes (milestone 2.5 done), 3c. Dosing (milestone 2.4 done), 3d. Equipment & maintenance (milestone 2.6 done), 3e. Livestock (milestone 2.7 done), 3f. Notifications (milestone 2.9 done), 4. Security and identity follow-ups (+4 more)

### Community 98 - "ReefCare — Product Requirements"
Cohesion: 0.13
Nodes (15): 10. Open product decisions, 1. Problem, 2. Product vision, 3. Target users and personas, 4. Market and localization, 5.1 MVP (Release 1) — "Reliable reef log with honest insights", 5.2 Release 2 — Community, 5.3 Later (+7 more)

### Community 100 - ".Timeline"
Cohesion: 0.25
Nodes (6): ListTankEventsParams, ListTankEventsRow, IsCategory(), TimelineCursor, TimelineEvent, TimelineFilter

### Community 101 - "ReefCare — Claude Code Project Guide"
Cohesion: 0.13
Nodes (12): 1. Conventions, 2. Errors, 3. Authentication, 4. Authorization notation, 6. Rate limits (initial), ReefCare — API Guidelines and Initial Contracts, API binary, Development (+4 more)

### Community 103 - "ReefCare — Database Design"
Cohesion: 0.18
Nodes (11): 1. Conventions, 2. ER diagram (MVP + Release 2), 3. DDL sketch — Phase 1 (identity, tanks, audit, jobs), 4. DDL sketch — Phase 2 (reef management), 5. Phase 3 — knowledge and intelligence (summary), 6. Phase 4 — community (summary), 7. Important query patterns and indexes, 8. Concurrency and idempotency (+3 more)

### Community 105 - "feature/SKILL.md"
Cohesion: 0.25
Nodes (7): 1. Inspect, 2. Analyze (write it down briefly in the conversation), 3. Design, 4. Implement, 5. Verify, 6. Review, 7. Summarize

### Community 106 - "InSerializableTx"
Cohesion: 0.48
Nodes (4): TxStarter, IsSerializationFailure(), InSerializableTx(), inTx()

### Community 107 - "ReefCare web"
Cohesion: 0.13
Nodes (12): 1. Scope of 1.8, 2. Stack, 3. Security requirements, 4. UX requirements (from .claude/rules/frontend.md), 5. Decisions (accepted 2026-10-30), 6. Implementation notes, ReefCare — Web Frontend Proposal (milestone 1.8), Checks (+4 more)

### Community 108 - "ReefCare — Implementation Roadmap"
Cohesion: 0.25
Nodes (8): Open decisions (need product owner input), Phase 0 — Product and technical design ✅ drafted, awaiting review, Phase 2 — Reef management, Phase 3 — Reef Intelligence, Phase 4 — Community (Release 2), Phase 5 — Production readiness, ReefCare — Implementation Roadmap, Risks

### Community 109 - "Reef Intelligence and knowledge base rules"
Cohesion: 0.29
Nodes (6): Knowledge base, Mandatory safety rules, Output model, Pipeline (keep these stages explicit and individually testable), Provider independence, Reef Intelligence and knowledge base rules

### Community 110 - "Options"
Cohesion: 0.40
Nodes (3): Options, ReadinessCheck, handleReadyz()

### Community 111 - "ListTanksForUserRow"
Cohesion: 0.50
Nodes (3): ListTanksForUserParams, ListTanksForUserRow, 5. Platform and data follow-ups

### Community 113 - "Community, Reef Clinic and reputation rules"
Cohesion: 0.33
Nodes (5): Cases, Community, Reef Clinic and reputation rules, Moderation, Reputation, Votes

### Community 114 - "identity/jobs_integration_test.go"
Cohesion: 0.57
Nodes (7): count(), TestAccountErasedAfterThirtyDays(), TestEmailJobsHoldNoSecrets(), TestEmailSurvivesSMTPOutage(), TestLoginFailureAuditedThroughJob(), TestPurgeExpiredSessionsAndTokens(), waitJobs()

### Community 115 - "PostgreSQL, migrations and sqlc"
Cohesion: 0.40
Nodes (4): Migrations (Goose), PostgreSQL, migrations and sqlc, Schema, sqlc

### Community 118 - "BadRequest"
Cohesion: 0.25
Nodes (6): PageParams, BadRequest(), DecodeCursor(), ParsePageParams(), decodeError(), jsonKind()

### Community 119 - "Handler"
Cohesion: 0.12
Nodes (12): statusRecorder, toEntry(), toEvent(), writeCreated(), handleHealthz(), Hash(), Key(), TestKeyAndHash() (+4 more)

### Community 132 - "net/netip.Prefix"
Cohesion: 0.20
Nodes (7): AuditLog, CreateSessionParams, InsertAuditLogParams, UserSession, emailJob, jobOrigin, loginFailedJob

### Community 135 - "DeactivateTankMaintenancePlansParams"
Cohesion: 0.09
Nodes (6): DeactivateTankMaintenancePlansParams, InsertTankEventParams, MarkNotificationReadParams, RemoveEquipmentParams, SetLivestockQuantityParams, SkipTankScheduledDosesParams

## Knowledge Gaps
- **7 isolated node(s):** `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)`, `helpers:pinGitHubActionDigests`, `reefcare/web` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1028 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **67 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `InTx()` connect `New` to `Service`, `New`, `NewValidator`, `testing.T`, `net/http.Handler`, `Service`, `IsNoRows`, `InSerializableTx`, `app.go`, `Tank Authorization`, `github.com/google/uuid.UUID`, `.Perform`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **What connects `reefcare`, `config:recommended`, `:semanticCommitTypeAll(chore)` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `English UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._
- **Why does `Current status` connect `net/http.Handler` to `New`, `Identity HTTP Handlers`, `app.go`, `Tank Authorization`, `ReefCare — Claude Code Project Guide`, `ReefCare — Security Design`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Should `Vietnamese UI Messages` be split into smaller, more focused modules?**
  _Cohesion score 0.009433962264150943 - nodes in this community are weakly interconnected._
- **Why does `ReefCare — Claude Code Project Guide` connect `ReefCare — Claude Code Project Guide` to `ReefCare — Claude Code Project Guide`, `net/http.Handler`?**
  _High betweenness centrality (0.017) - this node is a cross-community bridge._
- **Should `New` be split into smaller, more focused modules?**
  _Cohesion score 0.12684989429175475 - nodes in this community are weakly interconnected._