-- name: ListParameterTypes :many
SELECT code, canonical_unit, display_order FROM parameter_types ORDER BY display_order;

-- name: GetWaterTestByIdempotencyKey :one
SELECT id, tank_id, request_hash FROM water_tests
WHERE recorded_by = @recorded_by AND idempotency_key = @idempotency_key;

-- name: CreateWaterTest :one
-- Returns no row when a concurrent request already used the same Idempotency-Key.
INSERT INTO water_tests (tank_id, measured_at, source, notes, recorded_by, idempotency_key, request_hash, created_at)
VALUES (@tank_id, @measured_at, 'manual', @notes, @recorded_by, @idempotency_key, @request_hash, @now)
ON CONFLICT (recorded_by, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id;

-- name: CreateReading :one
INSERT INTO water_test_readings (water_test_id, tank_id, parameter, measured_at, input_value, input_unit,
                                 canonical_value, estimate, method, test_kit, supersedes_id, correction_reason,
                                 recorded_by, created_at)
VALUES (@water_test_id, @tank_id, @parameter, @measured_at, @input_value, @input_unit, @canonical_value,
        @estimate, @method, @test_kit, @supersedes_id, @correction_reason, @recorded_by, @now)
RETURNING id;

-- name: GetWaterTest :one
SELECT id, tank_id, measured_at, source, notes, created_at FROM water_tests WHERE id = @id;

-- name: GetReadingTank :one
SELECT tank_id FROM water_test_readings WHERE id = @id;

-- name: LockReading :one
SELECT id, water_test_id, tank_id, parameter, measured_at, status, method, test_kit
FROM water_test_readings WHERE id = @id FOR UPDATE;

-- name: SupersedeReading :execrows
UPDATE water_test_readings SET status = 'superseded' WHERE id = @id AND status = 'valid';

-- name: ListReadingsForTests :many
-- Every reading of the tests, superseded ones included (the history of corrections).
SELECT id, water_test_id, parameter, measured_at, input_value, input_unit, canonical_value, estimate, method,
       test_kit, status, supersedes_id, correction_reason, created_at
FROM water_test_readings
WHERE water_test_id = ANY(sqlc.arg(test_ids)::uuid[])
ORDER BY created_at, id;

-- name: ListWaterTests :many
-- Newest first; keyset on (measured_at, id). Every bound is a non-null parameter (the
-- service passes ±infinity, a max uuid, and '' for "any parameter") so it stays an index
-- condition under generic plans.
SELECT t.id, t.tank_id, t.measured_at, t.source, t.notes, t.created_at
FROM water_tests t
WHERE t.tank_id = @tank_id
  AND t.measured_at >= sqlc.arg(from_at)::timestamptz
  AND t.measured_at < sqlc.arg(to_at)::timestamptz
  AND (t.measured_at, t.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  AND (sqlc.arg(parameter)::text = '' OR EXISTS (
        SELECT 1 FROM water_test_readings r
        WHERE r.water_test_id = t.id AND r.parameter = sqlc.arg(parameter)::text AND r.status = 'valid'))
ORDER BY t.measured_at DESC, t.id DESC
LIMIT @row_limit;

-- name: ListValidReadingsInRange :many
-- Trend points: per parameter, the newest @per_parameter valid readings in [from, to),
-- newest first (the service reverses them). One index range scan per parameter, so a
-- dense series never crowds out another or its own recent end.
SELECT r.id, r.parameter, r.measured_at, r.canonical_value, r.estimate
FROM unnest(sqlc.arg(parameters)::text[]) AS p (code)
CROSS JOIN LATERAL (
    SELECT w.id, w.parameter, w.measured_at, w.canonical_value, w.estimate
    FROM water_test_readings w
    WHERE w.tank_id = @tank_id AND w.status = 'valid' AND w.parameter = p.code
      AND w.measured_at >= sqlc.arg(from_at)::timestamptz AND w.measured_at < sqlc.arg(to_at)::timestamptz
    ORDER BY w.measured_at DESC, w.id DESC
    LIMIT sqlc.arg(per_parameter)::int
) r;

-- name: LatestValidReadings :many
-- The newest valid reading of every parameter measured on the tank: one index probe per
-- catalog parameter (DISTINCT ON would walk the tank's whole history).
SELECT r.id, r.parameter, r.measured_at, r.input_value, r.input_unit, r.canonical_value, r.estimate
FROM parameter_types pt
CROSS JOIN LATERAL (
    SELECT w.id, w.parameter, w.measured_at, w.input_value, w.input_unit, w.canonical_value, w.estimate
    FROM water_test_readings w
    WHERE w.tank_id = @tank_id AND w.status = 'valid' AND w.parameter = pt.code
    ORDER BY w.measured_at DESC, w.id DESC
    LIMIT 1
) r;

-- name: ListTargetHistory :many
-- Every target version of the tank (a handful per parameter), oldest first per parameter.
SELECT id, parameter, min_value, ideal_value, max_value, freshness_hours, valid_from, valid_to, created_at
FROM parameter_targets
WHERE tank_id = @tank_id
ORDER BY parameter, valid_from, created_at;

-- name: CloseTarget :execrows
UPDATE parameter_targets SET valid_to = sqlc.arg(now)::timestamptz WHERE tank_id = @tank_id AND parameter = @parameter AND valid_to IS NULL;

-- name: CreateTarget :exec
INSERT INTO parameter_targets (tank_id, parameter, min_value, ideal_value, max_value, freshness_hours, valid_from,
                               created_by, created_at)
VALUES (@tank_id, @parameter, @min_value, @ideal_value, @max_value, @freshness_hours, @now, @created_by, @now);

-- name: ListTankEventsForAnnotations :many
-- Trend annotations: dosing, water-change and equipment events in [from, to), newest first
-- (the service reverses them), so a cap drops the oldest.
SELECT id, occurred_at, category, event_type, summary
FROM tank_events
WHERE tank_id = @tank_id AND category = ANY(sqlc.arg(categories)::text[])
  AND occurred_at >= sqlc.arg(from_at)::timestamptz AND occurred_at < sqlc.arg(to_at)::timestamptz
ORDER BY occurred_at DESC, id DESC
LIMIT @row_limit;

-- name: DeleteOwnerReadings :exec
DELETE FROM water_test_readings WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerWaterTests :exec
DELETE FROM water_tests WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: DeleteOwnerTargets :exec
DELETE FROM parameter_targets WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: ListReadingsForAssessment :many
-- Assessment snapshot: the newest valid readings in [since, until] (readings_recent), newest
-- first; the caller reverses them and reports a full page as truncated.
SELECT id, parameter, measured_at, canonical_value, estimate, method, test_kit
FROM water_test_readings
WHERE tank_id = @tank_id AND status = 'valid'
  AND measured_at >= sqlc.arg(since)::timestamptz AND measured_at <= sqlc.arg(until)::timestamptz
ORDER BY measured_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;
