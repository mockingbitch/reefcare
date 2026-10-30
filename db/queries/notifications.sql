-- Notifications. Every query is scoped to the recipient (user_id = the caller) or runs in a
-- background job for one recipient.

-- name: GetNotificationRecipients :many
-- What the scan needs to decide per recipient: active, verified address, preferences.
SELECT id, status, (email_verified_at IS NOT NULL)::boolean AS email_verified, notification_prefs
FROM users WHERE id = ANY (sqlc.arg(ids)::uuid[]) AND erased_at IS NULL;

-- name: InsertNotification :one
-- Idempotent: the same (user, dedupe_key) is created once; a repeat returns no row.
INSERT INTO notifications (user_id, tank_id, category, kind, dedupe_key, payload, occurred_at, email_status, created_at)
VALUES (@user_id, sqlc.narg(tank_id), @category, @kind, @dedupe_key, @payload, @occurred_at, sqlc.narg(email_status), @now)
ON CONFLICT (user_id, dedupe_key) DO NOTHING
RETURNING id;

-- name: ListNotifications :many
-- The caller's notifications, newest first (keyset on created_at, id; notifications_by_user).
SELECT id, tank_id, category, kind, payload, occurred_at, read_at, created_at
FROM notifications
WHERE user_id = @user_id
  AND (created_at, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: ListUnreadNotifications :many
-- ListNotifications for unread ones only: a separate statement so generic plans use
-- notifications_unread (a "NOT $flag OR read_at IS NULL" filter would walk the whole history).
SELECT id, tank_id, category, kind, payload, occurred_at, read_at, created_at
FROM notifications
WHERE user_id = @user_id AND read_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: CountUnreadNotifications :one
-- Capped so a huge backlog stays cheap; clients show "99+".
SELECT count(*) FROM (SELECT 1 FROM notifications WHERE user_id = @user_id AND read_at IS NULL LIMIT 100) x;

-- name: MarkNotificationRead :execrows
-- Idempotent: an already-read notification keeps its first read_at.
UPDATE notifications SET read_at = COALESCE(read_at, sqlc.arg(now)::timestamptz) WHERE id = @id AND user_id = @user_id;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = sqlc.arg(now)::timestamptz
WHERE user_id = @user_id AND read_at IS NULL AND created_at <= @now;

-- name: GetUserForNotificationEmail :one
-- At send time: erased, deactivated or unverified accounts are never mailed.
SELECT id, email, (email_verified_at IS NOT NULL)::boolean AS email_verified, locale, timezone, status, notification_prefs
FROM users WHERE id = @id AND erased_at IS NULL;

-- name: ClaimPendingNotificationEmails :many
-- Marks every pending reminder of the recipient as sent and returns their categories (the
-- email lists counts only). Run in the transaction that sends: a failed send rolls back.
-- A concurrent job for the same user waits on the row locks, then finds nothing pending.
UPDATE notifications SET email_status = 'sent', emailed_at = sqlc.arg(now)::timestamptz
WHERE user_id = @user_id AND email_status = 'pending'
RETURNING category;

-- name: RecentNotificationEmails :one
-- Reminder emails sent to the user since @since (one emailed_at per email) and the last one.
SELECT count(DISTINCT emailed_at)::integer AS sent, COALESCE(max(emailed_at), 'epoch')::timestamptz AS last_sent
FROM notifications WHERE user_id = @user_id AND emailed_at >= sqlc.arg(since)::timestamptz;

-- name: ListUsersWithPendingEmail :many
-- Recipients with reminders waiting for email (the scan queues a job for each).
SELECT DISTINCT user_id FROM notifications WHERE email_status = 'pending' LIMIT 5000;

-- name: SkipPendingNotificationEmails :exec
-- The recipient opted out or can't be mailed: nothing pending is sent later.
UPDATE notifications SET email_status = 'skipped' WHERE user_id = @user_id AND email_status = 'pending';

-- name: PurgeReadNotifications :execrows
-- Retention: read notifications after 90 days (notifications_read); bounded batch.
DELETE FROM notifications
WHERE id IN (SELECT id FROM notifications WHERE read_at < sqlc.arg(before)::timestamptz ORDER BY read_at LIMIT 5000);

-- name: PurgeOldNotifications :execrows
-- Retention: every notification, read or not, after 180 days (notifications_created).
DELETE FROM notifications
WHERE id IN (SELECT id FROM notifications WHERE created_at < sqlc.arg(before)::timestamptz ORDER BY created_at LIMIT 5000);

-- name: DeleteUserNotifications :exec
-- Account erasure: the user's notifications (notifications_by_user).
DELETE FROM notifications WHERE user_id = @user_id;

-- name: DeleteTankOwnerNotifications :exec
-- Account erasure: any notification about the user's tanks (defensive: only owners are
-- notified today; notifications_by_tank).
DELETE FROM notifications WHERE tank_id IN (SELECT id FROM tanks WHERE owner_id = @owner_id);
