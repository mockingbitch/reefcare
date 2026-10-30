-- Dosing. Tank-owned rows are read or written only after tanks.Authorizer allowed the
-- caller on tank_id; products are visible to their owner (or to everyone when curated).

-- name: ListDosingProducts :many
-- The curated catalog first, then the caller's own products.
SELECT id, owner_id, name, brand, form, default_unit, notes, created_at, updated_at
FROM dosing_products
WHERE owner_id IS NULL OR owner_id = @user_id
ORDER BY owner_id NULLS FIRST, lower(name), id;

-- name: CountOwnDosingProducts :one
SELECT count(*) FROM dosing_products WHERE owner_id = @owner_id;

-- name: CreateDosingProduct :one
INSERT INTO dosing_products (owner_id, name, brand, form, default_unit, notes, created_at, updated_at)
VALUES (@owner_id, @name, sqlc.narg(brand), @form, @default_unit, sqlc.narg(notes), @now, @now)
RETURNING id, owner_id, name, brand, form, default_unit, notes, created_at, updated_at;

-- name: GetDosingProduct :one
SELECT id, owner_id, name, brand, form, default_unit, notes, created_at, updated_at
FROM dosing_products WHERE id = @id;

-- name: CountTankDosingPlans :one
SELECT count(*) FROM dosing_plans WHERE tank_id = @tank_id;

-- name: CreateDosingPlan :one
INSERT INTO dosing_plans (tank_id, product_id, amount, amount_unit, rrule, timezone, starts_on, ends_on,
                          delivery_method, notes, created_by, created_at, updated_at)
VALUES (@tank_id, @product_id, @amount, @amount_unit, @rrule, @timezone, @starts_on, sqlc.narg(ends_on),
        @delivery_method, sqlc.narg(notes), @created_by, @now, @now)
RETURNING id;

-- name: GetDosingPlan :one
SELECT p.id, p.tank_id, p.product_id, d.name AS product_name, d.brand AS product_brand, d.form AS product_form,
       p.amount, p.amount_unit, p.rrule, p.timezone, p.starts_on, p.ends_on, p.delivery_method, p.status, p.notes,
       p.materialized_until, p.created_at, p.updated_at, p.version
FROM dosing_plans p
JOIN dosing_products d ON d.id = p.product_id
WHERE p.id = @id;

-- name: ListDosingPlans :many
SELECT p.id, p.tank_id, p.product_id, d.name AS product_name, d.brand AS product_brand, d.form AS product_form,
       p.amount, p.amount_unit, p.rrule, p.timezone, p.starts_on, p.ends_on, p.delivery_method, p.status, p.notes,
       p.materialized_until, p.created_at, p.updated_at, p.version
FROM dosing_plans p
JOIN dosing_products d ON d.id = p.product_id
WHERE p.tank_id = @tank_id
ORDER BY p.created_at, p.id;

-- name: LockDosingPlan :one
-- Serializes materialization and edits of one plan; returns the tank and owner states so
-- the job can skip plans whose tank was archived or owner deactivated after listing.
SELECT p.id, p.tank_id, p.product_id, p.amount, p.amount_unit, p.rrule, p.timezone, p.starts_on, p.ends_on,
       p.delivery_method, p.status, p.notes, p.materialized_until, p.version,
       t.status AS tank_status, o.status AS owner_status
FROM dosing_plans p
JOIN tanks t ON t.id = p.tank_id
JOIN users o ON o.id = t.owner_id
WHERE p.id = @id
FOR UPDATE OF p;

-- name: UpdateDosingPlan :execrows
UPDATE dosing_plans
SET amount = @amount, amount_unit = @amount_unit, rrule = @rrule, timezone = @timezone, starts_on = @starts_on,
    ends_on = sqlc.narg(ends_on), delivery_method = @delivery_method, notes = sqlc.narg(notes), status = @status,
    materialized_until = sqlc.narg(materialized_until), updated_at = @now, version = version + 1
WHERE id = @id AND version = @version;

-- name: SetDosingMaterializedUntil :exec
UPDATE dosing_plans SET materialized_until = sqlc.arg(until)::timestamptz WHERE id = @id;

-- name: ListDosingPlansToMaterialize :many
SELECT p.id
FROM dosing_plans p
JOIN tanks t ON t.id = p.tank_id
JOIN users o ON o.id = t.owner_id
WHERE p.status = 'active' AND t.status <> 'archived' AND o.status = 'active'
  AND (p.materialized_until IS NULL OR p.materialized_until < sqlc.arg(horizon)::timestamptz)
  AND p.id > sqlc.arg(after_id)::uuid
ORDER BY p.id
LIMIT @batch;

-- name: EndTankDosingPlans :many
-- Archiving a tank completes its plans; returns them so upcoming occurrences can go.
UPDATE dosing_plans
SET status = 'completed', materialized_until = NULL, updated_at = @now, version = version + 1
WHERE tank_id = @tank_id AND status <> 'completed'
RETURNING id;

-- name: InsertDosingOccurrence :execrows
-- Idempotent: an occurrence that already exists (in any status) is left alone.
INSERT INTO dosing_events (tank_id, plan_id, product_id, scheduled_for, missed_after, planned_amount, amount_unit,
                           status, created_at, updated_at)
VALUES (@tank_id, sqlc.arg(plan_id)::uuid, @product_id, sqlc.arg(scheduled_for)::timestamptz,
        sqlc.arg(missed_after)::timestamptz, sqlc.arg(planned_amount)::numeric, @amount_unit, 'scheduled', @now, @now)
ON CONFLICT (plan_id, scheduled_for) WHERE plan_id IS NOT NULL DO NOTHING;

-- name: DeleteFutureDosingOccurrences :execrows
-- Occurrences nobody acted on that are still scheduled after `after` (callers pass now
-- minus the old grace, so a dose still inside its grace goes too and can't be doubled by
-- the new timing); missed and resolved ones are history and stay.
DELETE FROM dosing_events
WHERE plan_id = sqlc.arg(plan_id)::uuid AND status = 'scheduled' AND scheduled_for > sqlc.arg(after)::timestamptz;

-- name: CreateAdHocDose :one
-- A dose given without a plan. With an idempotency key, a retry returns no row and the
-- caller looks the first one up by key.
INSERT INTO dosing_events (tank_id, product_id, actual_amount, actual_source, amount_unit, status, executed_at, notes,
                           reported_by, resolved_at, recorded_by, idempotency_key, request_hash, created_at, updated_at)
VALUES (@tank_id, @product_id, sqlc.arg(actual_amount)::numeric, 'entered', @amount_unit, 'completed',
        sqlc.arg(executed_at)::timestamptz, sqlc.narg(notes), 'manual', sqlc.arg(now)::timestamptz, @recorded_by,
        sqlc.narg(idempotency_key), sqlc.narg(request_hash), sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz)
ON CONFLICT (recorded_by, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id;

-- name: GetDosingEventByIdempotencyKey :one
SELECT id, tank_id, request_hash FROM dosing_events WHERE recorded_by = @recorded_by AND idempotency_key = @idempotency_key;

-- name: GetDosingEvent :one
SELECT e.id, e.tank_id, e.plan_id, e.product_id, d.name AS product_name, e.scheduled_for, e.missed_after,
       e.planned_amount, e.actual_amount, e.actual_source, e.amount_unit, e.status, e.executed_at, e.reason,
       e.reason_code, e.notes, e.reported_by,
       e.missed_at, e.resolved_at, e.created_at, e.updated_at, e.version
FROM dosing_events e
JOIN dosing_products d ON d.id = e.product_id
WHERE e.id = @id;

-- name: LockDosingEvent :one
SELECT id, tank_id, plan_id, status, planned_amount, actual_amount, amount_unit, executed_at, reason, notes,
       scheduled_for, missed_after
FROM dosing_events WHERE id = @id FOR UPDATE;

-- name: GetDosingEventTank :one
SELECT tank_id FROM dosing_events WHERE id = @id;

-- name: ListDosingEvents :many
-- History of a tank: event time is executed_at when dosed, else scheduled_for. Bounds and
-- the first-page cursor are sentinels (year 1 / 9999-12-31, max uuid) so they stay index
-- conditions under generic plans.
SELECT e.id, e.tank_id, e.plan_id, e.product_id, d.name AS product_name, e.scheduled_for, e.missed_after,
       e.planned_amount, e.actual_amount, e.actual_source, e.amount_unit, e.status, e.executed_at, e.reason,
       e.reason_code, e.notes, e.reported_by,
       e.missed_at, e.resolved_at, e.created_at, e.updated_at, e.version
FROM dosing_events e
JOIN dosing_products d ON d.id = e.product_id
WHERE e.tank_id = @tank_id
  AND e.status = ANY (sqlc.arg(statuses)::text[])
  AND COALESCE(e.executed_at, e.scheduled_for) >= sqlc.arg(from_at)::timestamptz
  AND COALESCE(e.executed_at, e.scheduled_for) < sqlc.arg(to_at)::timestamptz
  AND (COALESCE(e.executed_at, e.scheduled_for), e.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY COALESCE(e.executed_at, e.scheduled_for) DESC, e.id DESC
LIMIT @row_limit;

-- name: ResolveDosingEvent :execrows
-- The keeper records what happened to a scheduled, missed or unconfirmed dose.
UPDATE dosing_events
SET status = @status, actual_amount = sqlc.narg(actual_amount), actual_source = sqlc.narg(actual_source),
    executed_at = sqlc.narg(executed_at), reason = sqlc.narg(reason), notes = sqlc.narg(notes),
    reported_by = 'manual', recorded_by = @recorded_by,
    resolved_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, version = version + 1
WHERE id = @id AND status IN ('scheduled', 'missed', 'unconfirmed');

-- name: MarkMissedDosingEvents :many
-- Scheduler only: occurrences still scheduled after missed_after become 'missed' (manual
-- plans) or 'unconfirmed' (pump plans). Only tanks of active owners. Bounded batch; SKIP
-- LOCKED so a keeper resolving one at the same moment wins.
UPDATE dosing_events e
SET status = CASE WHEN p.delivery_method = 'dosing_pump' THEN 'unconfirmed' ELSE 'missed' END,
    missed_at = CASE WHEN p.delivery_method = 'dosing_pump' THEN NULL ELSE sqlc.arg(now)::timestamptz END,
    updated_at = sqlc.arg(now)::timestamptz, version = e.version + 1
FROM dosing_plans p
WHERE p.id = e.plan_id
  AND e.id = ANY (ARRAY(
    SELECT x.id FROM dosing_events x
    JOIN tanks t ON t.id = x.tank_id
    JOIN users o ON o.id = t.owner_id
    WHERE x.status = 'scheduled' AND x.missed_after < sqlc.arg(now)::timestamptz AND o.status = 'active'
    ORDER BY x.missed_after
    LIMIT @batch
    FOR UPDATE OF x SKIP LOCKED
  ))
RETURNING e.id, e.tank_id, e.scheduled_for, e.status;

-- name: SkipTankScheduledDoses :execrows
-- Archiving a tank resolves its still-scheduled doses as skipped with the system reason
-- code 'tank_archived'. Missed doses stay missed (history).
UPDATE dosing_events
SET status = 'skipped', reason_code = 'tank_archived', recorded_by = @recorded_by,
    resolved_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, version = version + 1
WHERE tank_id = @tank_id AND status = 'scheduled';

-- name: LockOwnerDosingPlans :exec
-- Account erasure: holding the plan locks keeps the materialize job from adding
-- occurrences between the deletes below (same lock order as LockOwnerWaterChangeSchedules).
SELECT p.id FROM dosing_plans p JOIN tanks t ON t.id = p.tank_id
WHERE t.owner_id = @owner_id
ORDER BY p.id
FOR UPDATE OF p;

-- name: DeleteOwnerDosingEvents :exec
DELETE FROM dosing_events WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerDosingPlans :exec
DELETE FROM dosing_plans WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerDosingProducts :exec
-- Private products are only usable on the owner's own tanks, whose rows are gone by now.
DELETE FROM dosing_products WHERE owner_id = @owner_id;
