-- All time comparisons and timestamps use @now from the application clock.

-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, locale, timezone, created_at, updated_at)
VALUES (@email, @password_hash, @display_name, @locale, @timezone, @now, @now)
RETURNING id, created_at;

-- name: GetUserCredentialsByEmail :one
-- Login lookup: only the columns needed to verify a password and gate access.
-- auth_epoch must be passed to CreateSession unchanged.
SELECT id, password_hash, auth_epoch, status, email_verified_at
FROM users
WHERE email = @email;

-- name: GetUserByID :one
SELECT id, email, email_verified_at, display_name, username, bio, experience_level,
       locale, timezone, unit_prefs, notification_prefs, status, created_at, updated_at, version
FROM users
WHERE id = @id;

-- name: GetUserCredentialsByID :one
SELECT id, email, password_hash, auth_epoch, status
FROM users
WHERE id = @id;

-- name: ChangeUserPassword :one
-- A real credential change (reset or change-password). Bumping auth_epoch invalidates
-- every existing session, including ones created by logins racing this update.
-- Returns the new epoch so the session performing a change-password can be kept.
UPDATE users
SET password_hash = @password_hash, auth_epoch = auth_epoch + 1, updated_at = @now
WHERE id = @id
RETURNING auth_epoch;

-- name: RotateSession :one
-- Gives a live session a new token and CSRF secret under the current auth epoch (after a
-- password change), so a previously copied token stops working while the user stays
-- signed in. Returns the unchanged absolute expiry for the new cookie.
UPDATE user_sessions
SET token_hash = @token_hash, csrf_secret = @csrf_secret, auth_epoch = @auth_epoch
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL
RETURNING expires_at;

-- name: RehashUserPassword :execrows
-- Transparent hash-parameter upgrade after a successful login. Applies only if the hash
-- is still the one that was verified, so it can never undo a concurrent password change.
UPDATE users
SET password_hash = @new_hash, updated_at = @now
WHERE id = @id AND password_hash = @old_hash;

-- name: MarkEmailVerified :execrows
UPDATE users
SET email_verified_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = @id AND email_verified_at IS NULL;

-- name: ListUserRoles :many
SELECT role FROM user_roles WHERE user_id = @user_id ORDER BY role;

-- name: CreateSession :one
INSERT INTO user_sessions (user_id, auth_epoch, token_hash, csrf_secret, created_at, last_seen_at,
                           idle_expires_at, expires_at, user_agent, ip_prefix)
VALUES (@user_id, @auth_epoch, @token_hash, @csrf_secret, @now, @now,
        @idle_expires_at, @expires_at, @user_agent, @ip_prefix)
RETURNING id;

-- name: GetActiveSessionByTokenHash :one
-- Resolves a cookie token to a live session. A session created under an older
-- auth_epoch (password since changed) never resolves. Status is returned, not filtered,
-- so the caller can distinguish suspended accounts.
SELECT s.id, s.user_id, s.csrf_secret, s.last_seen_at, s.idle_expires_at, s.expires_at,
       u.status AS user_status, u.email_verified_at
FROM user_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.auth_epoch = u.auth_epoch
  AND s.revoked_at IS NULL
  AND s.expires_at > @now
  AND s.idle_expires_at > @now;

-- name: TouchSession :execrows
-- Slides the idle timeout of a still-live session; never revives an expired one and
-- never extends past the absolute expiry. Callers throttle this to avoid a write per request.
UPDATE user_sessions
SET last_seen_at = @now, idle_expires_at = LEAST(@idle_expires_at, expires_at)
WHERE id = @id
  AND revoked_at IS NULL
  AND idle_expires_at > @now
  AND expires_at > @now;

-- name: RevokeSession :execrows
-- Scoped to the owner so a session id from another user is a no-op (0 rows → 404).
UPDATE user_sessions SET revoked_at = sqlc.arg(now)::timestamptz
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeOtherUserSessions :execrows
-- Revokes every live session of the user except keep_id (pass NULL to revoke all).
UPDATE user_sessions SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = @user_id
  AND revoked_at IS NULL
  AND expires_at > @now
  AND (sqlc.narg(keep_id)::uuid IS NULL OR id <> sqlc.narg(keep_id)::uuid);

-- name: RevokeLiveAuthTokens :exec
-- Run in the same transaction before CreateAuthToken, and after credential changes.
UPDATE auth_tokens SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL AND revoked_at IS NULL;

-- name: CreateAuthToken :exec
-- Fails with a unique violation (auth_tokens_one_live) if another live token of the
-- same purpose was issued concurrently; the service treats that as "already sent".
INSERT INTO auth_tokens (user_id, purpose, token_hash, email, created_at, expires_at)
VALUES (@user_id, @purpose, @token_hash, @email, @now, @expires_at);

-- name: ConsumeAuthToken :one
-- Atomically marks a live token used and returns its user with their status. A second
-- call with the same token returns no rows, and so does a token sent to an address the
-- account no longer has (callers roll back, so such a token stays unusable until it
-- expires; it can never succeed). Run inside the transaction that
-- performs the token's effect so a failure rolls the consumption back.
WITH consumed AS (
    UPDATE auth_tokens
    SET used_at = sqlc.arg(now)::timestamptz
    WHERE token_hash = @token_hash
      AND purpose = @purpose
      AND used_at IS NULL
      AND revoked_at IS NULL
      AND expires_at > @now
    RETURNING user_id, email
)
SELECT u.id AS user_id, u.status, u.email_verified_at
FROM consumed c
JOIN users u ON u.id = c.user_id AND u.email = c.email;

-- name: UpdateUserProfile :execrows
-- Optimistic lock: 0 rows means the version is stale (or the account is gone).
UPDATE users
SET display_name = @display_name, username = sqlc.narg(username), bio = sqlc.narg(bio),
    experience_level = sqlc.narg(experience_level), locale = @locale, timezone = @timezone,
    unit_prefs = @unit_prefs, notification_prefs = @notification_prefs,
    updated_at = @now, version = version + 1
WHERE id = @id AND version = @version AND status = 'active';

-- name: DeactivateUser :execrows
-- Account deletion request (docs/security.md §9). Bumping auth_epoch invalidates every
-- session atomically, including logins racing this update.
UPDATE users
SET status = 'deactivated', deactivated_at = sqlc.arg(now)::timestamptz, auth_epoch = auth_epoch + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = @id AND status = 'active';

-- name: RevokeAllLiveAuthTokens :exec
UPDATE auth_tokens SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = @user_id AND used_at IS NULL AND revoked_at IS NULL;

-- name: FindShareGranteeByEmail :one
-- Only active accounts that proved control of their address can receive shares, so an
-- account squatting on someone else's unverified email never gets their tanks.
SELECT id FROM users WHERE email = @email AND status = 'active' AND email_verified_at IS NOT NULL;

-- name: FindShareGranteeByUsername :one
SELECT id FROM users
WHERE username = sqlc.arg(username)::citext AND status = 'active' AND email_verified_at IS NOT NULL;

-- name: LockUserRow :exec
-- Serializes per-user checks such as "at most N tanks" against concurrent creates.
SELECT id FROM users WHERE id = @id FOR NO KEY UPDATE;

-- name: GetUserForMail :one
-- What email jobs need at send time; erased accounts are never mailed.
SELECT id, email, email_verified_at, locale, status FROM users WHERE id = @id AND erased_at IS NULL;

-- name: PurgeSessions :execrows
-- Sessions that can never resolve again (absolute or idle expiry passed) and revoked ones
-- after a grace period. Bounded batch.
DELETE FROM user_sessions
WHERE id = ANY (ARRAY(
    SELECT x.id FROM user_sessions x
    WHERE x.expires_at < @now OR x.idle_expires_at < @now OR x.revoked_at < sqlc.arg(revoked_before)::timestamptz
    LIMIT @batch
));

-- name: PurgeAuthTokens :execrows
DELETE FROM auth_tokens
WHERE id = ANY (ARRAY(SELECT x.id FROM auth_tokens x WHERE x.expires_at < @expired_before LIMIT @batch));

-- name: ListUsersDueForErasure :many
-- Keyset pagination on (deactivated_at, id): pass the last row of the previous page, or
-- NULLs for the first page.
SELECT id, deactivated_at FROM users
WHERE status = 'deactivated' AND erased_at IS NULL
  AND deactivated_at <= sqlc.arg(deactivated_before)::timestamptz
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (deactivated_at, id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY deactivated_at, id
LIMIT @batch;

-- name: LockUserForErasure :one
-- Locks the row; no rows unless the account is still deactivated, not yet erased, and its
-- grace period has passed (re-checked here: the account may have changed since enqueue).
SELECT id FROM users
WHERE id = @id AND status = 'deactivated' AND erased_at IS NULL
  AND deactivated_at <= sqlc.arg(deactivated_before)::timestamptz
FOR UPDATE;

-- name: DeleteUserCredentials :exec
-- Sessions, email-link tokens and roles of an account being erased.
WITH s AS (DELETE FROM user_sessions WHERE user_id = @user_id),
     t AS (DELETE FROM auth_tokens WHERE user_id = @user_id)
DELETE FROM user_roles WHERE user_roles.user_id = @user_id;

-- name: AnonymizeUser :execrows
-- Replaces every personal field. The placeholder "erased:<id>" has no "@", so it is not an
-- address anyone can register (which would make this UPDATE fail on the unique email and
-- block the erasure) and nothing can be mailed to it. The password becomes unusable and
-- the original address is free to register again.
UPDATE users
SET email = 'erased:' || id::text, email_verified_at = NULL, password_hash = '!',
    auth_epoch = auth_epoch + 1, display_name = 'Deleted user', username = NULL, bio = NULL,
    experience_level = NULL, locale = 'vi', timezone = 'Asia/Ho_Chi_Minh',
    unit_prefs = '{}', notification_prefs = '{}', version = version + 1,
    erased_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE id = @id AND status = 'deactivated' AND erased_at IS NULL;

-- name: DeleteUserJobs :exec
-- Queued work about an account being erased (its payloads hold the user id and request
-- origin). The running erasure job itself is kept: the runner records its outcome.
DELETE FROM jobs
WHERE payload->>'user_id' = sqlc.arg(user_id)::uuid::text AND id <> @current_job_id AND status <> 'running';

-- name: GrantUserRole :execrows
INSERT INTO user_roles (user_id, role, granted_by, granted_at) VALUES (@user_id, @role, NULL, @now)
ON CONFLICT (user_id, role) DO NOTHING;

-- name: RevokeUserRole :execrows
DELETE FROM user_roles WHERE user_id = @user_id AND role = @role;
