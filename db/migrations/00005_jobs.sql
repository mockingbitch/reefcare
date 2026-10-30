-- +goose Up
-- Durable background work (docs/architecture.md §5, decision D3: in-house queue).
-- Fail fast instead of queueing behind a long transaction while users is locked below.
SET LOCAL lock_timeout = '5s';

CREATE TABLE jobs (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind         text NOT NULL CHECK (kind ~ '^[a-z_]+(\.[a-z_]+)+$'),
    -- Optional idempotency key: enqueuing the same (kind, dedupe_key) twice is a no-op for
    -- as long as the first row exists (periodic tasks use one key per time bucket).
    dedupe_key   text CHECK (char_length(dedupe_key) BETWEEN 1 AND 200),
    -- Never secrets: tokens are created when the job runs, not stored here.
    payload      jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(payload) = 'object' AND pg_column_size(payload) <= 8192),
    run_at       timestamptz NOT NULL,
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'dead')),
    attempts     int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts int NOT NULL DEFAULT 10 CHECK (max_attempts BETWEEN 1 AND 100),
    -- Lease: a running job whose locked_until has passed belongs to a crashed worker and
    -- is claimed again. locked_by fences completion so a late worker cannot overwrite.
    locked_by    text CHECK (char_length(locked_by) <= 100),
    locked_until timestamptz,
    last_error   text CHECK (char_length(last_error) <= 1000), -- safe summary, never PII
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    finished_at  timestamptz,
    CHECK ((status = 'running') = (locked_until IS NOT NULL AND locked_by IS NOT NULL)),
    -- A pending job always has an attempt left, so it can never get stuck unclaimable.
    CHECK (attempts <= max_attempts),
    CHECK (status <> 'pending' OR attempts < max_attempts),
    CHECK ((status IN ('succeeded', 'dead')) = (finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX jobs_dedupe ON jobs (kind, dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX jobs_ready ON jobs (run_at) WHERE status = 'pending';
CREATE INDEX jobs_leased ON jobs (locked_until) WHERE status = 'running';
CREATE INDEX jobs_finished ON jobs (finished_at) WHERE finished_at IS NOT NULL; -- cleanup
-- Status and lease updates touch indexed columns (no HOT updates): vacuum the queue often.
ALTER TABLE jobs SET (autovacuum_vacuum_scale_factor = 0.01, autovacuum_analyze_scale_factor = 0.02);

-- Account erasure (docs/security.md §9): the users row is kept, anonymized, because the
-- audit log (and later the reputation ledger) reference it.
ALTER TABLE users ADD COLUMN erased_at timestamptz;
-- The new column is NULL everywhere, so validating this CHECK is a quick scan (it runs
-- under the ADD COLUMN lock; split into NOT VALID + a later VALIDATE once users is large).
ALTER TABLE users ADD CONSTRAINT users_erased_only_deactivated
    CHECK (erased_at IS NULL OR status = 'deactivated');
-- Plain (not CONCURRENTLY) index builds are fine before launch; later index changes on
-- users/user_sessions go in a separate non-transactional migration (CONCURRENTLY).
CREATE INDEX users_pending_erasure ON users (deactivated_at, id)
    WHERE status = 'deactivated' AND erased_at IS NULL;

-- Purge of revoked sessions (expired ones use user_sessions_expires_at).
CREATE INDEX user_sessions_revoked_at ON user_sessions (revoked_at) WHERE revoked_at IS NOT NULL;

-- +goose Down
DROP INDEX user_sessions_revoked_at;
DROP INDEX users_pending_erasure;
ALTER TABLE users DROP CONSTRAINT users_erased_only_deactivated;
ALTER TABLE users DROP COLUMN erased_at;
DROP TABLE jobs;
