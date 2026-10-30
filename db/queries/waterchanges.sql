-- Water changes. Every query runs after tanks.Authorizer allowed the caller on tank_id
-- (resources reached by their own id resolve tank_id first).

-- name: GetTankSystemVolume :one
SELECT system_volume_l FROM tanks WHERE id = @id;

-- name: CreateWaterChangeSchedule :one
INSERT INTO water_change_schedules (tank_id, rrule, timezone, starts_on, ends_on, planned_volume_l, salt_mix, notes,
                                    created_by, created_at, updated_at)
VALUES (@tank_id, @rrule, @timezone, @starts_on, sqlc.narg(ends_on), @planned_volume_l, sqlc.narg(salt_mix),
        sqlc.narg(notes), @created_by, @now, @now)
RETURNING id;

-- name: GetWaterChangeSchedule :one
SELECT id, tank_id, rrule, timezone, starts_on, ends_on, planned_volume_l, salt_mix, notes, status,
       materialized_until, created_at, updated_at, version
FROM water_change_schedules
WHERE id = @id;

-- name: LockWaterChangeSchedule :one
-- Serializes materialization and edits of one schedule. The tank and owner states are
-- read under the same statement so the job can skip schedules whose tank was archived
-- (or owner deactivated) after it listed them.
SELECT s.id, s.tank_id, s.rrule, s.timezone, s.starts_on, s.ends_on, s.planned_volume_l, s.salt_mix, s.notes,
       s.status, s.materialized_until, s.created_at, s.updated_at, s.version,
       t.status AS tank_status, o.status AS owner_status
FROM water_change_schedules s
JOIN tanks t ON t.id = s.tank_id
JOIN users o ON o.id = t.owner_id
WHERE s.id = @id
FOR UPDATE OF s;

-- name: ListWaterChangeSchedules :many
SELECT id, tank_id, rrule, timezone, starts_on, ends_on, planned_volume_l, salt_mix, notes, status,
       materialized_until, created_at, updated_at, version
FROM water_change_schedules
WHERE tank_id = @tank_id
ORDER BY created_at, id;

-- name: UpdateWaterChangeSchedule :execrows
UPDATE water_change_schedules
SET rrule = @rrule, timezone = @timezone, starts_on = @starts_on, ends_on = sqlc.narg(ends_on),
    planned_volume_l = @planned_volume_l, salt_mix = sqlc.narg(salt_mix), notes = sqlc.narg(notes),
    status = @status, materialized_until = sqlc.narg(materialized_until), updated_at = @now, version = version + 1
WHERE id = @id AND version = @version;

-- name: SetWaterChangeMaterializedUntil :exec
UPDATE water_change_schedules SET materialized_until = sqlc.arg(until)::timestamptz WHERE id = @id;

-- name: ListWaterChangeSchedulesToMaterialize :many
-- Active schedules of non-archived tanks of active owners whose occurrences don't reach
-- the horizon, least materialized first; keyset on id for batches within one run.
SELECT s.id
FROM water_change_schedules s
JOIN tanks t ON t.id = s.tank_id
JOIN users o ON o.id = t.owner_id
WHERE s.status = 'active' AND t.status <> 'archived' AND o.status = 'active'
  AND (s.materialized_until IS NULL OR s.materialized_until < sqlc.arg(horizon)::timestamptz)
  AND s.id > sqlc.arg(after_id)::uuid
ORDER BY s.id
LIMIT @batch;

-- name: EndTankWaterChangeSchedules :many
-- Archiving a tank ends its schedules; returns them so their upcoming occurrences can be
-- removed.
UPDATE water_change_schedules
SET status = 'ended', materialized_until = NULL, updated_at = @now, version = version + 1
WHERE tank_id = @tank_id AND status <> 'ended'
RETURNING id;

-- name: InsertWaterChangeOccurrence :execrows
-- Idempotent: an occurrence that already exists (in any status) is left alone.
INSERT INTO water_change_events (tank_id, schedule_id, scheduled_for, overdue_after, planned_volume_l, status,
                                 salt_mix, created_at, updated_at)
VALUES (@tank_id, sqlc.arg(schedule_id)::uuid, sqlc.arg(scheduled_for)::timestamptz,
        sqlc.arg(overdue_after)::timestamptz, sqlc.arg(planned_volume_l)::numeric, 'scheduled', sqlc.narg(salt_mix),
        @now, @now)
ON CONFLICT (schedule_id, scheduled_for) WHERE schedule_id IS NOT NULL DO NOTHING;

-- name: DeleteFutureWaterChangeOccurrences :execrows
-- Upcoming occurrences that nobody acted on; overdue and resolved ones are history and stay.
DELETE FROM water_change_events
WHERE schedule_id = sqlc.arg(schedule_id)::uuid AND status = 'scheduled'
  AND scheduled_for > sqlc.arg(after)::timestamptz;

-- name: CreateWaterChangeEvent :one
-- A one-time planned change (status scheduled) or a change recorded after the fact
-- (status completed; the tank's current system volume is captured with it).
INSERT INTO water_change_events (tank_id, scheduled_for, overdue_after, planned_volume_l, status, completed_at,
                                 actual_volume_l, system_volume_l, salt_mix, notes, recorded_by, resolved_at,
                                 created_at, updated_at)
VALUES (@tank_id, sqlc.narg(scheduled_for), sqlc.narg(overdue_after), sqlc.narg(planned_volume_l), @status,
        sqlc.narg(completed_at), sqlc.narg(actual_volume_l),
        CASE WHEN @status::text = 'completed' THEN (SELECT t.system_volume_l FROM tanks t WHERE t.id = @tank_id) END,
        sqlc.narg(salt_mix), sqlc.narg(notes), @recorded_by, sqlc.narg(resolved_at), @now, @now)
RETURNING id;

-- name: GetWaterChangeEvent :one
SELECT id, tank_id, schedule_id, scheduled_for, overdue_after, planned_volume_l, status, completed_at,
       actual_volume_l, system_volume_l, salt_mix, notes, skip_reason, resolved_at, created_at, updated_at, version
FROM water_change_events
WHERE id = @id;

-- name: LockWaterChangeEvent :one
-- Completion and skip lock the row, so concurrent identical requests see each other's
-- result (idempotent) instead of a conflict.
SELECT id, tank_id, schedule_id, scheduled_for, overdue_after, planned_volume_l, status, completed_at,
       actual_volume_l, system_volume_l, salt_mix, notes, skip_reason, resolved_at, created_at, updated_at, version
FROM water_change_events
WHERE id = @id
FOR UPDATE;

-- name: GetWaterChangeEventTank :one
SELECT tank_id FROM water_change_events WHERE id = @id;

-- name: ListWaterChangeEvents :many
-- All statuses or any mix including completed/skipped. Event time is completed_at, else
-- scheduled_for. Every bound is a non-null parameter (the service passes ±infinity and a
-- max uuid for "none") so it stays an index condition under generic plans.
SELECT id, tank_id, schedule_id, scheduled_for, overdue_after, planned_volume_l, status, completed_at,
       actual_volume_l, system_volume_l, salt_mix, notes, skip_reason, resolved_at, created_at, updated_at, version
FROM water_change_events
WHERE tank_id = @tank_id
  AND ((sqlc.arg(want_completed)::boolean AND status = 'completed')
    OR (sqlc.arg(want_skipped)::boolean AND status = 'skipped')
    OR (sqlc.arg(want_overdue)::boolean AND status = 'scheduled' AND overdue_after < sqlc.arg(now)::timestamptz)
    OR (sqlc.arg(want_upcoming)::boolean AND status = 'scheduled' AND overdue_after >= sqlc.arg(now)::timestamptz))
  AND COALESCE(completed_at, scheduled_for) >= sqlc.arg(from_at)::timestamptz
  AND COALESCE(completed_at, scheduled_for) < sqlc.arg(to_at)::timestamptz
  AND (COALESCE(completed_at, scheduled_for), id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY COALESCE(completed_at, scheduled_for) DESC, id DESC
LIMIT @row_limit;

-- name: ListScheduledWaterChangeEvents :many
-- Overdue and/or upcoming only, via water_change_events_scheduled instead of walking the
-- tank's history. Same ordering and cursor as ListWaterChangeEvents (for scheduled rows
-- the event time is scheduled_for).
SELECT id, tank_id, schedule_id, scheduled_for, overdue_after, planned_volume_l, status, completed_at,
       actual_volume_l, system_volume_l, salt_mix, notes, skip_reason, resolved_at, created_at, updated_at, version
FROM water_change_events
WHERE tank_id = @tank_id AND status = 'scheduled'
  AND ((sqlc.arg(want_overdue)::boolean AND overdue_after < sqlc.arg(now)::timestamptz)
    OR (sqlc.arg(want_upcoming)::boolean AND overdue_after >= sqlc.arg(now)::timestamptz))
  AND scheduled_for >= sqlc.arg(from_at)::timestamptz
  AND scheduled_for < sqlc.arg(to_at)::timestamptz
  AND (scheduled_for, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY scheduled_for DESC, id DESC
LIMIT @row_limit;

-- name: CompleteWaterChangeEvent :execrows
UPDATE water_change_events e
SET status = 'completed', completed_at = sqlc.arg(completed_at)::timestamptz,
    actual_volume_l = sqlc.arg(actual_volume_l)::numeric,
    system_volume_l = (SELECT t.system_volume_l FROM tanks t WHERE t.id = e.tank_id),
    salt_mix = sqlc.narg(salt_mix), notes = sqlc.narg(notes), recorded_by = @recorded_by,
    resolved_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, version = e.version + 1
WHERE e.id = @id AND e.status = 'scheduled';

-- name: SkipWaterChangeEvent :execrows
UPDATE water_change_events
SET status = 'skipped', skip_reason = sqlc.arg(skip_reason)::text, recorded_by = @recorded_by,
    resolved_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, version = version + 1
WHERE id = @id AND status = 'scheduled';

-- name: LockOwnerWaterChangeSchedules :exec
-- Account erasure: materialization locks a schedule before adding occurrences, so
-- holding these locks keeps new occurrences from appearing between the deletes below.
-- Lock order (schedules, then events, then tanks in tanks.EraseUserData) must not be
-- changed to tanks-first: the materialize job holds a schedule while its insert takes the
-- tank's KEY SHARE lock.
SELECT s.id FROM water_change_schedules s JOIN tanks t ON t.id = s.tank_id
WHERE t.owner_id = @owner_id
ORDER BY s.id
FOR UPDATE OF s;

-- name: DeleteOwnerWaterChangeEvents :exec
-- Account erasure (runs before tanks.EraseUserData; FKs are RESTRICT).
DELETE FROM water_change_events WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerWaterChangeSchedules :exec
DELETE FROM water_change_schedules WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: SkipTankScheduledWaterChanges :execrows
-- Archiving a tank resolves the changes still open on it (overdue or planned once) as
-- skipped with the system reason code 'tank_archived', recorded by the archiving owner,
-- so nothing stays overdue on a read-only tank.
UPDATE water_change_events
SET status = 'skipped', skip_reason = 'tank_archived', recorded_by = @recorded_by,
    resolved_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, version = version + 1
WHERE tank_id = @tank_id AND status = 'scheduled';

-- name: ListWaterChangesForReminders :many
-- Reminder source: scheduled changes whose overdue_after is in [since, until) (recently
-- overdue, or due soon), keyset on (overdue_after, id).
SELECT e.id, e.tank_id, t.owner_id, t.name AS tank_name, e.scheduled_for::timestamptz AS scheduled_for,
       e.overdue_after::timestamptz AS overdue_after, e.planned_volume_l
FROM water_change_events e
JOIN tanks t ON t.id = e.tank_id
WHERE e.status = 'scheduled' AND t.status <> 'archived'
  AND e.overdue_after >= sqlc.arg(since)::timestamptz AND e.overdue_after < sqlc.arg(until)::timestamptz
  AND (e.overdue_after, e.id) > (sqlc.arg(after_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY e.overdue_after, e.id
LIMIT @batch;
