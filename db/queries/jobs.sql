-- Durable job queue (internal/platform/jobs). All times come from the application clock.

-- name: EnqueueJob :execrows
-- 0 rows: a job with the same (kind, dedupe_key) already exists. completed inserts the row
-- as already succeeded (never claimed): used where the write must happen for uniform
-- timing but there is nothing to do.
INSERT INTO jobs (kind, dedupe_key, payload, run_at, max_attempts, status, finished_at, created_at, updated_at)
VALUES (@kind, sqlc.narg(dedupe_key), @payload, @run_at, @max_attempts,
        CASE WHEN sqlc.arg(completed)::boolean THEN 'succeeded' ELSE 'pending' END,
        CASE WHEN sqlc.arg(completed)::boolean THEN sqlc.arg(now)::timestamptz END,
        sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz)
ON CONFLICT (kind, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING;

-- name: ReviveDeadJob :execrows
-- Gives a buried job with this dedupe key a fresh set of attempts (for work that must
-- eventually happen, such as account erasure).
UPDATE jobs
SET status = 'pending', attempts = 0, run_at = sqlc.arg(now)::timestamptz, finished_at = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE kind = @kind AND dedupe_key = @dedupe_key AND status = 'dead';

-- name: ClaimJobs :many
-- Takes due pending jobs of the given kinds (ordered scan of jobs_ready). Expired leases
-- are turned back into pending jobs by RequeueExpiredJobs, so this stays pending-only and
-- fast however large the backlog. SKIP LOCKED lets any number of workers poll at once.
WITH next AS (
    SELECT id
    FROM jobs
    WHERE status = 'pending' AND run_at <= @now
      AND kind = ANY (sqlc.arg(kinds)::text[])
    ORDER BY run_at, id
    LIMIT @batch
    FOR UPDATE SKIP LOCKED
)
UPDATE jobs j
SET status = 'running', attempts = j.attempts + 1, locked_by = sqlc.arg(worker)::text, locked_until = sqlc.arg(lease_until)::timestamptz, updated_at = @now
FROM next
WHERE j.id = next.id
RETURNING j.id, j.kind, j.payload, j.attempts, j.max_attempts;

-- name: RequeueExpiredJobs :execrows
-- A running job whose lease passed belongs to a crashed or stuck worker: make it pending
-- again. Clearing locked_by fences the old worker out of recording an outcome.
UPDATE jobs
SET status = 'pending', locked_by = NULL, locked_until = NULL, updated_at = @now
WHERE status = 'running' AND locked_until < @now AND attempts < max_attempts;

-- name: BuryExpiredJobs :execrows
-- A lease that expired on the final attempt means the worker died while running it; the
-- job is not retried again.
UPDATE jobs
SET status = 'dead', locked_by = NULL, locked_until = NULL, finished_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz,
    last_error = 'lease expired on the final attempt'
WHERE status = 'running' AND locked_until < @now AND attempts >= max_attempts;

-- name: CompleteJob :execrows
-- Fenced by worker and attempt (each claim increments attempts): 0 rows means the lease
-- was lost and a later claim, even by the same process, owns the job.
UPDATE jobs
SET status = 'succeeded', locked_by = NULL, locked_until = NULL, finished_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, last_error = NULL
WHERE id = @id AND status = 'running' AND locked_by = sqlc.arg(worker)::text AND attempts = @attempt;

-- name: RetryJob :execrows
UPDATE jobs
SET status = 'pending', locked_by = NULL, locked_until = NULL, run_at = @run_at, updated_at = @now, last_error = sqlc.arg(last_error)::text
WHERE id = @id AND status = 'running' AND locked_by = sqlc.arg(worker)::text AND attempts = @attempt;

-- name: BuryJob :execrows
UPDATE jobs
SET status = 'dead', locked_by = NULL, locked_until = NULL, finished_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz, last_error = sqlc.arg(last_error)::text
WHERE id = @id AND status = 'running' AND locked_by = sqlc.arg(worker)::text AND attempts = @attempt;

-- name: DeleteFinishedJobs :execrows
-- Bounded batch; callers loop until it returns fewer than batch rows.
-- = ANY(ARRAY(subquery)) keeps a primary-key lookup under pgx's generic plans, where
-- IN (SELECT … LIMIT $n) degrades to a full scan.
DELETE FROM jobs
WHERE id = ANY (ARRAY(
    SELECT x.id FROM jobs x
    WHERE (x.status = 'succeeded' AND x.finished_at < sqlc.arg(succeeded_before)::timestamptz)
       OR (x.status = 'dead' AND x.finished_at < sqlc.arg(dead_before)::timestamptz)
    LIMIT @batch
));

-- name: CountJobsByStatus :many
SELECT kind, status, count(*) AS jobs FROM jobs WHERE status IN ('pending', 'running', 'dead') GROUP BY kind, status;
