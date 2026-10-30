-- +goose Up
-- In-app notifications with opt-in email delivery (US-NT-1, milestone 2.9).
SET LOCAL lock_timeout = '5s';

-- One reminder for one user. dedupe_key names the occurrence it is about (e.g.
-- 'dosing.missed:<event id>', 'maintenance.overdue:<plan id>:<due date>'), so the same
-- reminder is never created — or delivered — twice (AC1). payload holds render-ready
-- parameters only (names, times, amounts the recipient can already see); clients localize
-- by kind. Notifications are a convenience copy: deleting one never touches the tank's data.
CREATE TABLE notifications (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT, -- erasure deletes them explicitly
    tank_id      uuid REFERENCES tanks (id) ON DELETE RESTRICT,
    category     text NOT NULL CHECK (category IN ('dosing', 'water_change', 'maintenance')),
    kind         text NOT NULL CHECK (kind ~ '^[a-z_]+\.[a-z_]+$'),
    dedupe_key   text NOT NULL CHECK (char_length(dedupe_key) BETWEEN 1 AND 200),
    payload      jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(payload) = 'object' AND pg_column_size(payload) <= 2048),
    occurred_at  timestamptz NOT NULL, -- the moment the reminder is about (due, missed, …)
    read_at      timestamptz,
    -- NULL: not emailed (opted out or unverified address when created).
    email_status text CHECK (email_status IN ('pending', 'sent', 'skipped')),
    emailed_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((email_status IS NOT DISTINCT FROM 'sent') = (emailed_at IS NOT NULL)),
    CHECK (kind LIKE category || '.%'),
    UNIQUE (user_id, dedupe_key)
);
CREATE INDEX notifications_by_user ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread ON notifications (user_id, created_at DESC, id DESC) WHERE read_at IS NULL;
CREATE INDEX notifications_email_pending ON notifications (user_id) WHERE email_status = 'pending';
CREATE INDEX notifications_by_tank ON notifications (tank_id) WHERE tank_id IS NOT NULL; -- erasure, tank RESTRICT checks
CREATE INDEX notifications_read ON notifications (read_at) WHERE read_at IS NOT NULL;   -- retention purge
CREATE INDEX notifications_created ON notifications (created_at);                        -- retention purge

-- Reminder sources scan recent occurrences only (bounded windows), through these indexes.
-- (x, id) matches their keyset. Built inside the migration transaction (writes to these
-- tables wait, ~0.5 s per 3M dosing events): acceptable before launch; later migrations on big
-- tables use a NO TRANSACTION migration with CREATE INDEX CONCURRENTLY.
CREATE INDEX dosing_events_missed ON dosing_events (missed_at, id) WHERE status = 'missed';
CREATE INDEX water_change_events_due ON water_change_events (overdue_after, id) WHERE status = 'scheduled';
CREATE INDEX maintenance_plans_due ON maintenance_plans (next_due_on, id) WHERE active;

-- +goose Down
DROP INDEX maintenance_plans_due;
DROP INDEX water_change_events_due;
DROP INDEX dosing_events_missed;
DROP TABLE notifications;
