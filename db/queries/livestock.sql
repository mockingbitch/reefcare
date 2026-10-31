-- Livestock. Tank-owned rows are read or written only after tanks.Authorizer allowed the
-- caller on tank_id; species are visible to their owner (or to everyone when curated).

-- name: SearchSpecies :many
-- Curated and the caller's own species. pattern is a LIKE pattern built (and escaped) by the
-- service, '%' for everything. The catalog is small, so a scan is fine (open item LS4).
SELECT id, owner_id, category, coral_type, common_name_vi, common_name_en, scientific_name, created_at
FROM species
WHERE (owner_id IS NULL OR owner_id = @user_id)
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text)
  AND (lower(coalesce(common_name_vi, '')) LIKE sqlc.arg(pattern)::text
       OR lower(coalesce(common_name_en, '')) LIKE sqlc.arg(pattern)::text
       OR lower(coalesce(scientific_name, '')) LIKE sqlc.arg(pattern)::text)
ORDER BY owner_id NULLS FIRST, lower(coalesce(common_name_vi, common_name_en, scientific_name)), id
LIMIT @row_limit;

-- name: CountOwnSpecies :one
SELECT count(*) FROM species WHERE owner_id = @owner_id;

-- name: CreateSpecies :one
INSERT INTO species (owner_id, category, coral_type, common_name_vi, common_name_en, scientific_name, created_at)
VALUES (@owner_id, @category, sqlc.narg(coral_type), sqlc.narg(common_name_vi), sqlc.narg(common_name_en),
        sqlc.narg(scientific_name), @now)
RETURNING id, owner_id, category, coral_type, common_name_vi, common_name_en, scientific_name, created_at;

-- name: GetSpecies :one
SELECT id, owner_id, category, coral_type, common_name_vi, common_name_en, scientific_name, created_at
FROM species WHERE id = @id;

-- name: CountTankLivestock :one
-- Present groups (the working limit) and all entries ever (a hard cap: entries are never deleted).
SELECT count(*) FILTER (WHERE current_quantity > 0) AS present, count(*) AS total
FROM livestock_entries WHERE tank_id = @tank_id;

-- name: CreateLivestockEntry :one
-- With an idempotency key, a retry inserts nothing and returns no row.
INSERT INTO livestock_entries (tank_id, species_id, category, coral_type, display_name, form, quarantine_status,
                               location_note, source_note, notes, current_quantity, added_at, created_by,
                               idempotency_key, request_hash, created_at, updated_at)
VALUES (@tank_id, sqlc.narg(species_id), @category, sqlc.narg(coral_type), @display_name, sqlc.narg(form),
        sqlc.narg(quarantine_status), sqlc.narg(location_note),
        sqlc.narg(source_note), sqlc.narg(notes), @current_quantity, @added_at, @created_by,
        sqlc.narg(idempotency_key), sqlc.narg(request_hash), @now, @now)
ON CONFLICT (created_by, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id;

-- name: GetLivestockEntryByIdempotencyKey :one
SELECT id, tank_id, request_hash FROM livestock_entries
WHERE created_by = @created_by AND idempotency_key = @idempotency_key;

-- name: GetLivestockEntry :one
SELECT l.id, l.tank_id, l.species_id, l.category, l.coral_type, l.display_name, l.form, l.quarantine_status,
       l.location_note, l.source_note, l.notes, l.current_quantity, l.added_at, l.created_at, l.updated_at, l.version,
       s.common_name_vi, s.common_name_en, s.scientific_name
FROM livestock_entries l
LEFT JOIN species s ON s.id = l.species_id
WHERE l.id = @id;

-- name: ListLivestockEntries :many
-- The tank's livestock, newest first; groups that reached 0 only on request.
SELECT l.id, l.tank_id, l.species_id, l.category, l.coral_type, l.display_name, l.form, l.quarantine_status,
       l.location_note, l.source_note, l.notes, l.current_quantity, l.added_at, l.created_at, l.updated_at, l.version,
       s.common_name_vi, s.common_name_en, s.scientific_name
FROM livestock_entries l
LEFT JOIN species s ON s.id = l.species_id
WHERE l.tank_id = @tank_id AND (sqlc.arg(include_gone)::boolean OR l.current_quantity > 0)
ORDER BY l.added_at DESC, l.id DESC;

-- name: GetLivestockEntryTank :one
SELECT tank_id FROM livestock_entries WHERE id = @id;

-- name: LockLivestockEntry :one
-- Serializes events of one entry. NO KEY UPDATE: id and tank_id never change.
SELECT id, tank_id, current_quantity, added_at, version
FROM livestock_entries WHERE id = @id FOR NO KEY UPDATE;

-- name: UpdateLivestockEntry :execrows
UPDATE livestock_entries
SET species_id = sqlc.narg(species_id), category = @category, coral_type = sqlc.narg(coral_type),
    display_name = @display_name, form = sqlc.narg(form), quarantine_status = sqlc.narg(quarantine_status),
    location_note = sqlc.narg(location_note), source_note = sqlc.narg(source_note), notes = sqlc.narg(notes),
    updated_at = @now, version = version + 1
WHERE id = @id AND version = @version;

-- name: SetLivestockQuantity :exec
-- Run with the entry locked, after the service checked every running balance. version is
-- not bumped: it covers the description, which a concurrent PATCH may change.
UPDATE livestock_entries SET current_quantity = @current_quantity, updated_at = @now
WHERE id = @id;

-- name: LivestockMoves :many
-- Every quantity change of an entry in event order, to check that no running balance goes
-- below zero when an event is inserted (possibly backdated). Run with the entry locked.
SELECT id, occurred_at, quantity_delta FROM livestock_events
WHERE entry_id = @entry_id AND quantity_delta <> 0
ORDER BY occurred_at, id;

-- name: CountLivestockEvents :one
-- Quantity moves (bounded so LivestockMoves stays cheap) and observations, capped separately
-- so many observations never block a death or a reconciliation.
SELECT count(*) FILTER (WHERE quantity_delta <> 0) AS moves, count(*) FILTER (WHERE quantity_delta = 0) AS observations
FROM livestock_events WHERE entry_id = @entry_id;

-- name: CreateLivestockEvent :one
-- With an idempotency key, a retry inserts nothing and returns no row.
INSERT INTO livestock_events (id, entry_id, tank_id, event_type, quantity_delta, occurred_at, note, recorded_by,
                              idempotency_key, request_hash, created_at)
VALUES (@id, @entry_id, @tank_id, @event_type, @quantity_delta, @occurred_at, sqlc.narg(note), @recorded_by,
        sqlc.narg(idempotency_key), sqlc.narg(request_hash), @now)
ON CONFLICT (recorded_by, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id, entry_id, tank_id, event_type, quantity_delta, occurred_at, note, created_at;

-- name: GetLivestockEventByIdempotencyKey :one
SELECT id, entry_id, tank_id, event_type, quantity_delta, occurred_at, note, created_at, request_hash
FROM livestock_events WHERE recorded_by = @recorded_by AND idempotency_key = @idempotency_key;

-- name: ListLivestockEvents :many
-- Keyset on (occurred_at, id) descending; the first-page cursor is a sentinel.
SELECT id, entry_id, tank_id, event_type, quantity_delta, occurred_at, note, created_at
FROM livestock_events
WHERE entry_id = @entry_id AND (occurred_at, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY occurred_at DESC, id DESC
LIMIT @row_limit;

-- name: LockOwnerTanksForLivestockErasure :exec
-- Account erasure: lock the owner's tanks first (a plain statement — a SELECT in an
-- unreferenced CTE would never run) so shared writers can't add rows meanwhile. Then delete
-- events, entries and the owner's species; all before tanks.EraseUserData.
SELECT id FROM tanks WHERE owner_id = @owner_id ORDER BY id FOR UPDATE;

-- name: DeleteOwnerLivestockEvents :exec
DELETE FROM livestock_events WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerLivestockEntries :exec
DELETE FROM livestock_entries WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DetachOwnerSpecies :exec
-- Private species are only usable on their owner's tanks; should another entry ever point at
-- one, it loses the reference instead of blocking the erasure.
UPDATE livestock_entries SET species_id = NULL
WHERE species_id IN (SELECT id FROM species WHERE owner_id = @owner_id);

-- name: DeleteOwnerSpecies :exec
DELETE FROM species WHERE owner_id = @owner_id;

-- name: ListLivestockEventsForAssessment :many
SELECT id, entry_id, event_type, quantity_delta, occurred_at
FROM livestock_events
WHERE tank_id = @tank_id AND occurred_at >= sqlc.arg(since)::timestamptz AND occurred_at <= sqlc.arg(until)::timestamptz
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountLivestockPresent :one
SELECT COALESCE(sum(current_quantity), 0)::integer FROM livestock_entries WHERE tank_id = @tank_id;
