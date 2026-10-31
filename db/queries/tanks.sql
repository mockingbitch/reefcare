-- Tanks, shares and timeline events. Every query that returns tank data is either
-- scoped to the caller (user_id) or only runs after tanks.Authorizer allowed access.
-- numeric columns map to decimal.Decimal (exact, canonical text form).

-- name: GetTankAccess :one
-- One row per existing tank; viewer is true when the user holds a share. Tanks of
-- deactivated owners are hidden from everyone but the owner (docs/security.md §9).
SELECT t.owner_id, t.status, o.status AS owner_status,
       EXISTS (SELECT 1 FROM tank_shares s WHERE s.tank_id = t.id AND s.user_id = @user_id) AS viewer
FROM tanks t
JOIN users o ON o.id = t.owner_id
WHERE t.id = @tank_id;

-- name: GetTankAccessForWrite :one
-- GetTankAccess for a check that guards a write in the caller's transaction: FOR SHARE
-- keeps the tank's status (e.g. archived) from changing until that transaction ends.
SELECT t.owner_id, t.status, o.status AS owner_status,
       EXISTS (SELECT 1 FROM tank_shares s WHERE s.tank_id = t.id AND s.user_id = @user_id) AS viewer
FROM tanks t
JOIN users o ON o.id = t.owner_id
WHERE t.id = @tank_id
FOR SHARE OF t;

-- name: CreateTank :one
INSERT INTO tanks (owner_id, name, description, tank_type, status, length_cm, width_cm, height_cm,
                   display_volume_l, sump_volume_l, system_volume_l, setup_date, water_source, salt_mix,
                   created_at, updated_at)
VALUES (@owner_id, @name, sqlc.narg(description), @tank_type, @status,
        sqlc.narg(length_cm), sqlc.narg(width_cm), sqlc.narg(height_cm),
        sqlc.arg(display_volume_l), sqlc.narg(sump_volume_l),
        sqlc.arg(system_volume_l),
        sqlc.narg(setup_date), sqlc.narg(water_source), sqlc.narg(salt_mix), @now, @now)
RETURNING id;

-- name: CountOwnedTanks :one
SELECT count(*) FROM tanks WHERE owner_id = @owner_id;

-- name: GetTank :one
-- access is 'owner' or 'viewer' for user_id; call only after authorization.
SELECT t.id, t.owner_id,
       CASE WHEN t.owner_id = @user_id THEN 'owner' ELSE 'viewer' END::text AS access,
       t.name, t.description, t.tank_type, t.status,
       t.length_cm, t.width_cm,
       t.height_cm,
       t.display_volume_l, t.sump_volume_l,
       t.system_volume_l,
       t.setup_date, t.water_source, t.salt_mix, t.archived_at, t.created_at, t.updated_at, t.version
FROM tanks t
WHERE t.id = @id;

-- name: ListTanksForUser :many
-- Owned tanks, plus tanks shared with the user when include_shared. Keyset pagination on
-- (created_at, id) descending; pass both cursor values or neither. The two branches are
-- separate so each uses its own index (tanks_by_owner; tank_shares_by_user + PK) instead
-- of an OR that forces a full scan.
SELECT x.id, x.owner_id, x.access, x.name, x.description, x.tank_type, x.status, x.length_cm, x.width_cm,
       x.height_cm, x.display_volume_l, x.sump_volume_l, x.system_volume_l, x.setup_date, x.water_source,
       x.salt_mix, x.archived_at, x.created_at, x.updated_at, x.version
FROM (
    (SELECT t.id, t.owner_id, 'owner'::text AS access,
             t.name, t.description, t.tank_type, t.status, t.length_cm, t.width_cm, t.height_cm,
             t.display_volume_l, t.sump_volume_l, t.system_volume_l,
             t.setup_date, t.water_source, t.salt_mix, t.archived_at, t.created_at, t.updated_at, t.version
     FROM tanks t
     WHERE t.owner_id = @user_id
       AND (sqlc.arg(include_archived)::boolean OR t.status <> 'archived')
       AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
            OR (t.created_at, t.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
     ORDER BY t.created_at DESC, t.id DESC
     LIMIT @row_limit)
    UNION ALL
    (SELECT t.id, t.owner_id, 'viewer'::text AS access,
             t.name, t.description, t.tank_type, t.status, t.length_cm, t.width_cm, t.height_cm,
             t.display_volume_l, t.sump_volume_l, t.system_volume_l,
             t.setup_date, t.water_source, t.salt_mix, t.archived_at, t.created_at, t.updated_at, t.version
     FROM tank_shares s
     JOIN tanks t ON t.id = s.tank_id
     JOIN users o ON o.id = t.owner_id
     WHERE sqlc.arg(include_shared)::boolean
       AND s.user_id = @user_id
       AND o.status <> 'deactivated'
       AND (sqlc.arg(include_archived)::boolean OR t.status <> 'archived')
       AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
            OR (t.created_at, t.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
     ORDER BY t.created_at DESC, t.id DESC
     LIMIT @row_limit)
) x
ORDER BY x.created_at DESC, x.id DESC
LIMIT @row_limit;

-- name: UpdateTank :execrows
-- Optimistic lock; never touches archived tanks or the archived status.
UPDATE tanks
SET name = @name, description = sqlc.narg(description), tank_type = @tank_type, status = @status,
    length_cm = sqlc.narg(length_cm), width_cm = sqlc.narg(width_cm),
    height_cm = sqlc.narg(height_cm),
    display_volume_l = sqlc.arg(display_volume_l),
    sump_volume_l = sqlc.narg(sump_volume_l),
    system_volume_l = sqlc.arg(system_volume_l),
    setup_date = sqlc.narg(setup_date), water_source = sqlc.narg(water_source), salt_mix = sqlc.narg(salt_mix),
    updated_at = @now, version = version + 1
WHERE id = @id AND version = @version AND status <> 'archived';

-- name: ArchiveTank :execrows
UPDATE tanks
SET status = 'archived', archived_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz,
    version = version + 1
WHERE id = @id AND status <> 'archived';

-- name: ListTankShares :many
-- Grantee emails are deliberately not returned: a share made by username must not
-- reveal the grantee's address to the owner. Deactivated grantees are hidden at once
-- (docs/security.md §9); their rows go with the account erasure.
SELECT s.user_id, u.display_name, u.username, s.permission, s.created_at
FROM tank_shares s
JOIN users u ON u.id = s.user_id
WHERE s.tank_id = @tank_id AND u.status <> 'deactivated'
ORDER BY s.created_at, s.user_id;

-- name: CountTankShares :one
SELECT count(*) FROM tank_shares s JOIN users u ON u.id = s.user_id
WHERE s.tank_id = @tank_id AND u.status <> 'deactivated';

-- name: CreateTankShare :execrows
-- 0 rows: the user already holds a share.
INSERT INTO tank_shares (tank_id, user_id, permission, granted_by, created_at)
VALUES (@tank_id, @user_id, 'viewer', @granted_by, @now)
ON CONFLICT (tank_id, user_id) DO NOTHING;

-- name: DeleteTankShare :execrows
DELETE FROM tank_shares WHERE tank_id = @tank_id AND user_id = @user_id;

-- name: InsertTankEvent :execrows
-- Idempotent: recording the same (source, event_type) twice is a no-op (0 rows).
INSERT INTO tank_events (tank_id, occurred_at, category, event_type, source_type, source_id, actor_id, summary, created_at)
VALUES (@tank_id, @occurred_at, @category, @event_type, @source_type, @source_id, @actor_id,
        COALESCE(sqlc.narg(summary)::jsonb, '{}'), @now)
ON CONFLICT (source_type, source_id, event_type) DO NOTHING;

-- name: ListTankEvents :many
-- The tank's timeline, newest first: keyset on (occurred_at, id) from the cursor down to
-- from_at. The service folds the exclusive "to" bound into the cursor as (to, uuid.Nil): a
-- separate occurred_at < to condition would become the btree start key and make deep pages
-- walk down from the newest event. Bounds are always concrete values (no "IS NULL OR"), so
-- generic plans use tank_events_feed too. Only called after Authorizer allowed the caller.
SELECT e.id, e.occurred_at, e.category, e.event_type, e.source_type, e.source_id, e.actor_id, u.display_name AS actor_name,
       e.summary
FROM tank_events e
LEFT JOIN users u ON u.id = e.actor_id
WHERE e.tank_id = @tank_id
  AND e.occurred_at >= sqlc.arg(from_at)::timestamptz
  AND (e.occurred_at, e.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT @row_limit;

-- name: ListTankEventsByCategory :many
-- ListTankEvents restricted to some categories: one ordered, LIMITed scan of
-- tank_events_cat per category, merged. A plain "category = ANY(...)" sorts every match or,
-- under a generic plan, filter-scans the whole tank. The actor join runs after the LIMIT.
SELECT e.id, e.occurred_at, e.category, e.event_type, e.source_type, e.source_id, e.actor_id, u.display_name AS actor_name,
       e.summary
FROM (
    SELECT x.id, x.occurred_at, x.category, x.event_type, x.source_type, x.source_id, x.actor_id, x.summary
    FROM unnest(sqlc.arg(categories)::text[]) AS c (cat)
    CROSS JOIN LATERAL (
        SELECT t.id, t.occurred_at, t.category, t.event_type, t.source_type, t.source_id, t.actor_id, t.summary
        FROM tank_events t
        WHERE t.tank_id = sqlc.arg(tank_id)::uuid AND t.category = c.cat
          AND t.occurred_at >= sqlc.arg(from_at)::timestamptz
          AND (t.occurred_at, t.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
        ORDER BY t.occurred_at DESC, t.id DESC
        LIMIT sqlc.arg(row_limit)::integer
    ) x
    ORDER BY x.occurred_at DESC, x.id DESC
    LIMIT sqlc.arg(row_limit)::integer
) e
LEFT JOIN users u ON u.id = e.actor_id
ORDER BY e.occurred_at DESC, e.id DESC;

-- name: LockTankRow :exec
-- Taken first by workflows that change the tank row or check per-tank limits ("at most
-- N shares"), so concurrent writers queue instead of deadlocking on a FOR SHARE upgrade.
SELECT id FROM tanks WHERE id = @id FOR NO KEY UPDATE;

-- name: DeleteOwnerTankData :exec
-- Account erasure: every tank of the owner with its children, in FK order (tank_events and
-- tanks are RESTRICT on purpose so nothing else deletes them). Add new tank child tables here.
-- Locking the owned tanks makes concurrent child inserts (whose FK check needs KEY SHARE)
-- wait, so none can slip in before DeleteOwnerTanks.
WITH owned AS (SELECT id FROM tanks WHERE owner_id = @owner_id FOR UPDATE),
     ev AS (DELETE FROM tank_events WHERE tank_id IN (SELECT id FROM owned)),
     sh AS (DELETE FROM tank_shares WHERE tank_id IN (SELECT id FROM owned))
SELECT 1;

-- name: DeleteOwnerTanks :exec
DELETE FROM tanks WHERE owner_id = @owner_id;

-- name: DeleteGranteeShares :exec
DELETE FROM tank_shares WHERE user_id = @user_id;

-- name: GetTankForAssessment :one
SELECT tank_type, status, setup_date FROM tanks WHERE id = @id;

-- name: LockTankForAssessment :one
-- Taken before storing an automatic assessment: an archive or erasure of the tank waits, or
-- is seen.
SELECT status FROM tanks WHERE id = @id FOR SHARE;
