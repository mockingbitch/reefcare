-- Equipment and maintenance. Tank-owned rows are read or written only after
-- tanks.Authorizer allowed the caller on tank_id (own-id routes resolve tank_id first).

-- name: CreateEquipment :one
INSERT INTO tank_equipment (tank_id, category, name, manufacturer, model, serial_number, purchased_on, warranty_until,
                            installed_at, notes, created_by, created_at, updated_at)
VALUES (@tank_id, @category, @name, sqlc.narg(manufacturer), sqlc.narg(model), sqlc.narg(serial_number),
        sqlc.narg(purchased_on), sqlc.narg(warranty_until), @installed_at, sqlc.narg(notes), @created_by, @now, @now)
RETURNING id;

-- name: CountTankEquipment :one
SELECT count(*) FROM tank_equipment WHERE tank_id = @tank_id;

-- name: GetEquipment :one
SELECT e.id, e.tank_id, e.category, e.name, e.manufacturer, e.model, e.serial_number, e.purchased_on,
       e.warranty_until, e.installed_at, e.removed_at, e.removal_reason, e.notes, e.created_at, e.updated_at, e.version
FROM tank_equipment e
WHERE e.id = @id;

-- name: ListEquipment :many
-- The tank's equipment, newest installation first; removed devices only on request.
SELECT e.id, e.tank_id, e.category, e.name, e.manufacturer, e.model, e.serial_number, e.purchased_on,
       e.warranty_until, e.installed_at, e.removed_at, e.removal_reason, e.notes, e.created_at, e.updated_at, e.version
FROM tank_equipment e
WHERE e.tank_id = @tank_id AND (sqlc.arg(include_removed)::boolean OR e.removed_at IS NULL)
ORDER BY e.installed_at DESC, e.id DESC;

-- name: ListCurrentConfigurations :many
-- The latest configuration of each given device (one query for a whole list).
SELECT DISTINCT ON (equipment_id) id, equipment_id, tank_id, version_no, settings, effective_at, note, created_at
FROM equipment_configurations
WHERE equipment_id = ANY (sqlc.arg(equipment_ids)::uuid[])
ORDER BY equipment_id, version_no DESC;

-- name: GetEquipmentTank :one
SELECT tank_id FROM tank_equipment WHERE id = @id;

-- name: LockEquipment :one
-- NO KEY UPDATE (id and tank_id never change) so FK checks from new events don't block on it.
SELECT id, tank_id, installed_at, removed_at, version FROM tank_equipment WHERE id = @id FOR NO KEY UPDATE;

-- name: ShareLockEquipment :one
-- Taken when a plan or event is linked to the device, so a concurrent removal (NO KEY
-- UPDATE) either sees the link and deactivates the plan, or is seen as removed here.
-- Lock order everywhere: tank, equipment, plan.
SELECT id, tank_id, installed_at, removed_at FROM tank_equipment WHERE id = @id FOR SHARE;

-- name: UpdateEquipment :execrows
UPDATE tank_equipment
SET category = @category, name = @name, manufacturer = sqlc.narg(manufacturer), model = sqlc.narg(model),
    serial_number = sqlc.narg(serial_number), purchased_on = sqlc.narg(purchased_on),
    warranty_until = sqlc.narg(warranty_until), notes = sqlc.narg(notes),
    updated_at = @now, version = version + 1
WHERE id = @id AND version = @version AND removed_at IS NULL;

-- name: RemoveEquipment :execrows
UPDATE tank_equipment
SET removed_at = sqlc.arg(removed_at)::timestamptz, removal_reason = sqlc.narg(removal_reason), updated_at = @now,
    version = version + 1
WHERE id = @id AND removed_at IS NULL;

-- name: NextConfigurationVersion :one
-- Run with the equipment row locked (LockEquipment) so concurrent writers serialize. The
-- latest effective_at (epoch when there is none) keeps version order = timeline order.
SELECT (COALESCE(max(version_no), 0) + 1)::integer AS version_no,
       COALESCE(max(effective_at), 'epoch')::timestamptz AS latest_effective_at
FROM equipment_configurations WHERE equipment_id = @equipment_id;

-- name: CreateConfiguration :one
INSERT INTO equipment_configurations (equipment_id, tank_id, version_no, settings, effective_at, note, created_by,
                                      created_at)
VALUES (@equipment_id, @tank_id, @version_no, @settings, @effective_at, sqlc.narg(note), @created_by, @now)
RETURNING id, equipment_id, tank_id, version_no, settings, effective_at, note, created_at;

-- name: ListConfigurations :many
SELECT id, equipment_id, tank_id, version_no, settings, effective_at, note, created_at
FROM equipment_configurations
WHERE equipment_id = @equipment_id
ORDER BY version_no DESC;

-- name: CreateMaintenancePlan :one
INSERT INTO maintenance_plans (tank_id, equipment_id, title, interval_days, timezone, starts_on, next_due_on, notes,
                               created_by, created_at, updated_at)
VALUES (@tank_id, sqlc.narg(equipment_id), @title, @interval_days, @timezone, @starts_on, @starts_on, sqlc.narg(notes),
        @created_by, @now, @now)
RETURNING id;

-- name: GetMaintenancePlan :one
SELECT p.id, p.tank_id, p.equipment_id, e.name AS equipment_name, p.title, p.interval_days, p.timezone, p.starts_on,
       p.last_performed_at, p.next_due_on, p.active, p.notes, p.created_at, p.updated_at, p.version
FROM maintenance_plans p
LEFT JOIN tank_equipment e ON e.id = p.equipment_id
WHERE p.id = @id;

-- name: ListMaintenancePlans :many
-- Active plans first, soonest due first.
SELECT p.id, p.tank_id, p.equipment_id, e.name AS equipment_name, p.title, p.interval_days, p.timezone, p.starts_on,
       p.last_performed_at, p.next_due_on, p.active, p.notes, p.created_at, p.updated_at, p.version
FROM maintenance_plans p
LEFT JOIN tank_equipment e ON e.id = p.equipment_id
WHERE p.tank_id = @tank_id
ORDER BY p.active DESC, p.next_due_on, p.id;

-- name: CountTankMaintenancePlans :one
SELECT count(*) FROM maintenance_plans WHERE tank_id = @tank_id;

-- name: LockMaintenancePlan :one
SELECT id, tank_id, equipment_id, title, interval_days, timezone, starts_on, last_performed_at, next_due_on, active,
       notes, version
FROM maintenance_plans WHERE id = @id FOR UPDATE;

-- name: UpdateMaintenancePlan :execrows
UPDATE maintenance_plans
SET equipment_id = sqlc.narg(equipment_id), title = @title, interval_days = @interval_days, timezone = @timezone,
    starts_on = @starts_on, next_due_on = @next_due_on, active = @active, notes = sqlc.narg(notes),
    last_performed_at = sqlc.narg(last_performed_at), updated_at = @now, version = version + 1
WHERE id = @id AND version = @version;

-- name: DeactivateTankMaintenancePlans :exec
UPDATE maintenance_plans SET active = false, updated_at = @now, version = version + 1
WHERE tank_id = @tank_id AND active;

-- name: DeactivateEquipmentMaintenancePlans :exec
-- A removed device's plans stop being due.
UPDATE maintenance_plans SET active = false, updated_at = @now, version = version + 1
WHERE equipment_id = sqlc.arg(equipment_id)::uuid AND active;

-- name: CreateMaintenanceEvent :one
-- With an idempotency key, a retry inserts nothing and returns no row.
INSERT INTO maintenance_events (tank_id, plan_id, equipment_id, title, performed_at, notes, recorded_by,
                                idempotency_key, request_hash, created_at)
VALUES (@tank_id, sqlc.narg(plan_id), sqlc.narg(equipment_id), @title, @performed_at, sqlc.narg(notes), @recorded_by,
        sqlc.narg(idempotency_key), sqlc.narg(request_hash), @now)
ON CONFLICT (recorded_by, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id, tank_id, plan_id, equipment_id, title, performed_at, notes, created_at;

-- name: ListMaintenanceEvents :many
-- Keyset on (performed_at, id) descending; the first-page cursor is a sentinel.
SELECT id, tank_id, plan_id, equipment_id, title, performed_at, notes, created_at
FROM maintenance_events
WHERE tank_id = @tank_id
  AND (sqlc.narg(plan_id)::uuid IS NULL OR plan_id = sqlc.narg(plan_id)::uuid)
  AND (sqlc.narg(equipment_id)::uuid IS NULL OR equipment_id = sqlc.narg(equipment_id)::uuid)
  AND (performed_at, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY performed_at DESC, id DESC
LIMIT @row_limit;

-- name: GetMaintenanceEventByIdempotencyKey :one
SELECT id, tank_id, plan_id, equipment_id, title, performed_at, notes, created_at, request_hash
FROM maintenance_events WHERE recorded_by = @recorded_by AND idempotency_key = @idempotency_key;

-- name: LatestPlanPerformance :one
-- The most recent performance of a plan (the due date follows the latest, even when an
-- older one is recorded later). Only called right after inserting an event of the plan, so
-- max() is never NULL.
SELECT max(performed_at)::timestamptz FROM maintenance_events WHERE plan_id = @plan_id;

-- name: LockOwnerTanksForErasure :exec
-- Account erasure: lock the owner's tanks first (a plain statement — a SELECT in an
-- unreferenced CTE would never run), so shared writers, which take the tank lock first,
-- can't add rows meanwhile. Then delete in FK order: events, plans, configurations,
-- equipment; all before tanks.EraseUserData.
SELECT id FROM tanks WHERE owner_id = @owner_id ORDER BY id FOR UPDATE;

-- name: DeleteOwnerMaintenance :exec
DELETE FROM maintenance_events WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerMaintenancePlans :exec
DELETE FROM maintenance_plans WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerConfigurations :exec
DELETE FROM equipment_configurations
WHERE equipment_id IN (SELECT e.id FROM tank_equipment e JOIN tanks t ON t.id = e.tank_id WHERE t.owner_id = @owner_id);

-- name: DeleteOwnerEquipment :exec
DELETE FROM tank_equipment WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: ListMaintenanceForReminders :many
-- Reminder source: active plans due in [since, until] (dates; the service applies each
-- plan's zone), keyset on (next_due_on, id).
SELECT p.id, p.tank_id, t.owner_id, t.name AS tank_name, p.title, p.timezone, p.next_due_on, e.name AS equipment_name
FROM maintenance_plans p
JOIN tanks t ON t.id = p.tank_id
LEFT JOIN tank_equipment e ON e.id = p.equipment_id
WHERE p.active AND t.status <> 'archived'
  AND p.next_due_on >= sqlc.arg(since)::date AND p.next_due_on <= sqlc.arg(until)::date
  AND (p.next_due_on, p.id) > (sqlc.arg(after_on)::date, sqlc.arg(after_id)::uuid)
ORDER BY p.next_due_on, p.id
LIMIT @batch;

-- name: ListEquipmentChangesForAssessment :many
-- Assessment snapshot: devices installed or removed and configuration versions since @since.
SELECT e.id, 'installed'::text AS kind, e.category, e.installed_at AS at
FROM tank_equipment e
WHERE e.tank_id = @tank_id AND e.installed_at >= sqlc.arg(since)::timestamptz AND e.installed_at <= sqlc.arg(until)::timestamptz
UNION ALL
SELECT e.id, 'removed'::text, e.category, e.removed_at::timestamptz
FROM tank_equipment e
WHERE e.tank_id = @tank_id AND e.removed_at >= sqlc.arg(since)::timestamptz AND e.removed_at <= sqlc.arg(until)::timestamptz
UNION ALL
SELECT c.id, 'configuration_changed'::text, e.category, c.effective_at
FROM equipment_configurations c JOIN tank_equipment e ON e.id = c.equipment_id
WHERE c.tank_id = @tank_id AND c.effective_at >= sqlc.arg(since)::timestamptz AND c.effective_at <= sqlc.arg(until)::timestamptz
ORDER BY 4 DESC, 1 DESC
LIMIT sqlc.arg(row_limit)::int;
