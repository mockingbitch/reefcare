# ReefCare — User Stories and Acceptance Criteria

Status: **Draft for review** (Phase 0)

Format: `US-<area>-<n>` · priority **M** = MVP, **R2** = Release 2, **L** = later.
Acceptance criteria (AC) are written so they can become API or integration tests.

## Identity

**US-ID-1 (M) Register.** As a visitor I register with email, password, display name, language and
timezone.
- AC1: Password ≥ 10 characters and not in a common-password list; otherwise 422 `weak_password`.
- AC2: Email is normalized (trim, lowercase) and unique case-insensitively; duplicate returns the
  same 202 response as success (no account enumeration) and sends an "already registered" email.
- AC3: A verification email with a single-use token (expires in 24 h) is sent.
- AC4: Unverified accounts can manage their own tanks but cannot post in the community.

**US-ID-2 (M) Login / logout.**
- AC1: Valid credentials create a session and set an `HttpOnly; Secure; SameSite=Lax` cookie.
- AC2: Invalid credentials return 401 `invalid_credentials` with identical timing and body whether
  the email exists or not.
- AC3: More than 10 failed attempts per email or 30 per IP in 15 minutes returns 429.
- AC4: Logout revokes the server-side session; the cookie no longer authenticates.

**US-ID-3 (M) Password reset.**
- AC1: Request always returns 202. Token single-use, expires in 1 h, stored hashed.
- AC2: Successful reset revokes all other sessions and is written to the audit log.

**US-ID-4 (M) Profile and preferences.** Update display name, bio, avatar (R2), experience level,
language, timezone, unit preferences, notification preferences.
- AC1: Timezone must be a valid IANA name. Changing it never changes stored timestamps.

**US-ID-5 (M) Delete account.**
- AC1: Requires re-authentication. Account is deactivated immediately; personal data is erased or
  anonymized within 30 days per [security.md](security.md) §Privacy.

## Tanks

**US-TK-1 (M) Create tank.** Name, type, display volume, optional sump volume, estimated system
volume, setup date, cycling status, salt mix, water source, description.
- AC1: Volumes are positive; system volume ≥ display volume is *not* enforced (display volume may
  include rock displacement), but the UI warns when system < display.
- AC2: New tanks are private. The creator becomes owner.
- AC3: Creating a tank emits a `tank.created` timeline event.

**US-TK-2 (M) List and view tanks.** Only owned and shared tanks are listed.
- AC1: Requesting a tank the caller cannot access returns 404 (not 403).

**US-TK-3 (M) Update / archive tank.** Optimistic concurrency via `version`.
- AC1: Stale `version` returns 409 `version_conflict`.
- AC2: A tank with history can only be archived, not hard-deleted.

**US-TK-4 (M) Share tank read-only with another user** by email/username.
- AC1: Grantee sees the tank and its data but cannot modify anything (403 on writes after access
  is established via read).
- AC2: Owner can revoke; revocation takes effect immediately.

## Water parameters

**US-WP-1 (M) Record a water test** containing one or more parameter readings measured at the same
time.
- AC1: Each reading stores value, unit as entered, canonical value, method, kit/device, source.
- AC2: `measured_at` may be in the past; it may not be more than 5 minutes in the future.
- AC3: Physically impossible values (outside hard bounds) → 422 with per-field details.
- AC4: Values outside the tank's target range are accepted and flagged `outside_target`.
- AC5: Units not allowed for the parameter → 422 `unsupported_unit`.
- AC6: Supplying the same `Idempotency-Key` twice creates one water test.

**US-WP-2 (M) Correct a reading.**
- AC1: Correction creates a new reading that supersedes the original; the original remains
  retrievable with status `superseded` and a reason.
- AC2: Charts and assessments use only non-superseded readings.

**US-WP-3 (M) View trends** for one or more parameters in a time range.
- AC1: Response includes points (canonical unit), target band(s) valid over the range, and
  annotations for dosing, water-change and equipment events in the same range.
- AC2: Max range per request 2 years; downsampling not required in MVP.

**US-WP-4 (M) Set targets** per parameter.
- AC1: min ≤ ideal ≤ max; values within hard bounds.
- AC2: Updating targets creates a new target version; old readings are compared against the target
  that was valid when measured.

**US-WP-5 (M) See freshness.** Dashboard shows, per parameter, last measured time and a stale flag
when older than the parameter's freshness window (user-configurable, defaults per parameter).

## Dosing

**US-DO-1 (M) Create dosing plan** with product, amount, unit, schedule (RRULE subset), timezone,
start/end, delivery method, target parameters.
- AC1: Schedule must produce at least one occurrence; max 48 occurrences per day.
- AC2: Plan creation emits a `dosing.plan_created` timeline event.

**US-DO-2 (M) Record dosing.** Confirm an occurrence (planned vs actual amount) or record an
unscheduled dose.
- AC1: Status one of `completed`, `partial`, `skipped`, `failed`; skipped/failed require a reason.
- AC2: An occurrence can be resolved once; a second resolution returns 409 unless it is an explicit
  correction.
- AC3: Occurrences not resolved within the grace period become `missed` — never `completed`.

**US-DO-3 (M) Pause / resume / end a plan.** Future unresolved occurrences are cancelled on
pause/end and regenerated on resume.

**US-DO-4 (M) Track stock.** When product container size is recorded, consumption decrements
remaining stock; low-stock notification at user threshold.

## Water changes

**US-WC-1 (M) Schedule water changes** (one-time or recurring) with planned volume.
- AC1: Response includes estimated % of system volume and the assumption text.

**US-WC-2 (M) Complete or skip a water change** with actual volume, salt, target/measured
salinity and temperature, notes.
- AC1: Overdue occurrences are shown as overdue until completed or skipped by the user.
- AC2: Completion emits a timeline event and can link pre/post water tests.

## Equipment & maintenance

**US-EQ-1 (M) Add equipment** (category, manufacturer, model, name, serial, dates, specs).
**US-EQ-2 (M) Record configuration change.** Creates a new configuration version and a
`equipment.configuration_changed` timeline event; previous versions remain visible.
**US-EQ-3 (M) Maintenance plans and events** with intervals and overdue indicators.

## Livestock

**US-LS-1 (M) Add livestock** (common/scientific name, category, quantity, date, source, location).
- AC1: Creates an `added` event; current quantity = sum of event deltas.
**US-LS-2 (M) Record livestock event** (death, transfer out, sale, correction, health observation).
- AC1: Quantity can never go below 0 (409 `quantity_underflow`).
**US-LS-3 (M) Reconcile count.** Owner enters actual count; system records a `reconciliation` event
with the delta and note; history is not modified.

## Timeline

**US-TL-1 (M) View timeline** filtered by categories and time range, newest first, cursor-paginated.
- AC1: Every item shows occurred time (user TZ), category, summary, source, actor.

## Notifications

**US-NT-1 (M) Receive reminders** for upcoming/missed dosing, upcoming/overdue water changes and
maintenance, stale measurements, threshold breaches, new assessments.
- AC1: The same reminder for the same occurrence is never delivered twice.
- AC2: User can disable each category; email delivery is opt-in.

## Intelligence

**US-AI-1 (M) Request an assessment** for a tank.
- AC1: Returns a stored assessment with observations, hypotheses (each with supporting, missing
  and contradictory evidence), recommended verifications, safety warnings, knowledge references,
  confidence label, rule version, input snapshot.
- AC2: When data is stale or missing, the assessment says so and recommends tests instead of
  drawing conclusions.
- AC3: Every knowledge reference resolves to a published knowledge article version.
- AC4: Rate limited to 10 on-demand assessments per tank per day.

**US-AI-2 (M) Give feedback** on an assessment (useful / not useful / wrong + outcome note).

**US-AI-3 (M) Automatic assessment** after a new water test triggers a deviation or instability
rule; user receives a notification.

## Community (R2)

**US-CM-1 Create a Reef Clinic case** from a tank, choosing which data categories and photos to share.
- AC1: Case stores an immutable snapshot; later tank edits don't change it.
**US-CM-2 Answer, comment, vote.** One vote per user per target; no self-votes.
**US-CM-3 Mark helpful answer; update case status and outcome.** Inconclusive closure allowed.
**US-CM-4 Reputation.** Points from ledger events; reward for an event is granted at most once.
**US-CM-5 Report content; moderator actions** are audited and reversible.
