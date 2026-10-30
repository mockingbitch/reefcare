-- +goose Up
-- Tokens remember the address they were sent to, so a verification link mailed to
-- address A can never verify address B after an email change (open item S3).
-- Runs in one transaction holding ACCESS EXCLUSIVE on auth_tokens (small table: tokens
-- live ≤ 24 h). The lock timeout makes it fail fast instead of stalling sign-ins behind a
-- long transaction. Deploy migrations before the new binary: old binaries cannot insert
-- tokens once email is NOT NULL.
SET LOCAL lock_timeout = '5s';
ALTER TABLE auth_tokens ADD COLUMN email citext;
UPDATE auth_tokens t SET email = u.email FROM users u WHERE u.id = t.user_id;
ALTER TABLE auth_tokens ALTER COLUMN email SET NOT NULL;

-- +goose Down
ALTER TABLE auth_tokens DROP COLUMN email;
