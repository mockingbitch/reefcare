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
INSERT INTO auth_tokens (user_id, purpose, token_hash, created_at, expires_at)
VALUES (@user_id, @purpose, @token_hash, @now, @expires_at);

-- name: ConsumeAuthToken :one
-- Atomically marks a live token used and returns its user with their status. A second
-- call with the same token returns no rows. Run inside the transaction that performs the
-- token's effect so a failure rolls the consumption back.
WITH consumed AS (
    UPDATE auth_tokens
    SET used_at = sqlc.arg(now)::timestamptz
    WHERE token_hash = @token_hash
      AND purpose = @purpose
      AND used_at IS NULL
      AND revoked_at IS NULL
      AND expires_at > @now
    RETURNING user_id
)
SELECT u.id AS user_id, u.status, u.email_verified_at
FROM consumed c
JOIN users u ON u.id = c.user_id;
