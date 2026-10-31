-- Assessments. Every query is called only after tanks.Authorizer allowed the caller on the
-- assessment's tank, or from a background job for that tank.

-- name: CreateAssessment :one
-- A retried job for the same water test inserts nothing (assessments_per_test) and gets no row.
INSERT INTO assessments (tank_id, trigger, source_id, result, finding_count, max_severity, confidence_label,
                         input_snapshot, snapshot_version, rule_version, knowledge_version, provisional, locale,
                         requested_by, created_at)
VALUES (@tank_id, @trigger, sqlc.narg(source_id), @result, @finding_count, sqlc.narg(max_severity), @confidence_label,
        @input_snapshot, @snapshot_version, @rule_version, @knowledge_version, @provisional, @locale,
        sqlc.narg(requested_by), @now)
ON CONFLICT (source_id) WHERE trigger = 'water_test' DO NOTHING
RETURNING id;

-- name: AddAssessmentEvidence :exec
INSERT INTO assessment_evidence (assessment_id, kind, ref_id)
SELECT @assessment_id, unnest(sqlc.arg(kinds)::text[]), unnest(sqlc.arg(ref_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: AddAssessmentKnowledgeRef :exec
INSERT INTO assessment_knowledge_refs (assessment_id, version_id) VALUES (@assessment_id, @version_id)
ON CONFLICT DO NOTHING;

-- name: GetAssessment :one
SELECT id, tank_id, trigger, source_id, status, result, finding_count, max_severity, confidence_label,
       snapshot_version, rule_version, knowledge_version, provisional, locale, requested_by, retracted_at,
       retract_reason, created_at
FROM assessments WHERE id = @id;

-- name: GetAssessmentTank :one
SELECT tank_id FROM assessments WHERE id = @id;

-- name: ListAssessments :many
-- The tank's assessments, newest first (keyset on created_at, id); without results.
SELECT id, tank_id, trigger, source_id, status, finding_count, max_severity, confidence_label, rule_version,
       provisional, created_at
FROM assessments
WHERE tank_id = @tank_id
  AND (created_at, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: GetAssessmentSnapshot :one
SELECT input_snapshot FROM assessments WHERE id = @id;

-- name: CountRecentAutomaticAssessments :one
SELECT count(*) FROM assessments
WHERE tank_id = @tank_id AND trigger <> 'manual' AND created_at >= sqlc.arg(since)::timestamptz;

-- name: GetUserLocale :one
SELECT locale FROM users WHERE id = @id;

-- name: DeleteOwnerAssessmentEvidence :exec
DELETE FROM assessment_evidence
WHERE assessment_id IN (SELECT a.id FROM assessments a JOIN tanks t ON t.id = a.tank_id WHERE t.owner_id = @owner_id);

-- name: DeleteOwnerAssessmentRefs :exec
DELETE FROM assessment_knowledge_refs
WHERE assessment_id IN (SELECT a.id FROM assessments a JOIN tanks t ON t.id = a.tank_id WHERE t.owner_id = @owner_id);

-- name: DeleteOwnerAssessments :exec
DELETE FROM assessments WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);

-- name: UpsertAssessmentFeedback :exec
INSERT INTO assessment_feedback (assessment_id, user_id, rating, outcome_note, created_at, updated_at)
VALUES (@assessment_id, @user_id, @rating, sqlc.narg(outcome_note), @now, @now)
ON CONFLICT (assessment_id, user_id) DO UPDATE
SET rating = EXCLUDED.rating, outcome_note = EXCLUDED.outcome_note, updated_at = EXCLUDED.updated_at;

-- name: GetAssessmentFeedback :one
SELECT rating, outcome_note, updated_at FROM assessment_feedback WHERE assessment_id = @assessment_id AND user_id = @user_id;

-- name: LockAssessment :one
SELECT tank_id, status FROM assessments WHERE id = @id FOR NO KEY UPDATE;

-- name: RetractAssessment :execrows
UPDATE assessments SET status = 'retracted', retracted_at = sqlc.arg(now)::timestamptz, retract_reason = @reason
WHERE id = @id AND status = 'generated';

-- name: DeleteUserAssessmentFeedback :exec
-- Account erasure: the user's own feedback, and any on the user's tanks' assessments.
DELETE FROM assessment_feedback WHERE user_id = @user_id;

-- name: DeleteOwnerAssessmentFeedback :exec
DELETE FROM assessment_feedback
WHERE assessment_id IN (SELECT a.id FROM assessments a JOIN tanks t ON t.id = a.tank_id WHERE t.owner_id = @owner_id);
