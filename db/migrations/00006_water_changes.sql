-- +goose Up
-- Water-change schedules and occurrences (docs/domain-model.md, US-WC-1..2).
SET LOCAL lock_timeout = '5s';

CREATE TABLE water_change_schedules (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id            uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    rrule              text NOT NULL CHECK (char_length(rrule) <= 200), -- canonical recurrence subset
    timezone           text NOT NULL CHECK (char_length(timezone) <= 64), -- IANA, validated by the service
    starts_on          date NOT NULL,
    ends_on            date,
    planned_volume_l   numeric(9, 2) NOT NULL CHECK (planned_volume_l > 0),
    salt_mix           text CHECK (char_length(salt_mix) <= 100),
    notes              text CHECK (char_length(notes) <= 2000),
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'ended')),
    -- Occurrences exist up to here; the materialize job extends it.
    materialized_until timestamptz,
    -- Users are anonymized, never deleted, on erasure: the id stays (SET NULL never fires).
    created_by         uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    version            int NOT NULL DEFAULT 1,
    CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX water_change_schedules_by_tank ON water_change_schedules (tank_id, created_at, id);
CREATE INDEX water_change_schedules_to_materialize ON water_change_schedules (materialized_until NULLS FIRST, id)
    WHERE status = 'active';

-- One row per planned or performed water change. "Overdue" is not a status: a scheduled
-- row is overdue once overdue_after has passed (its time plus a grace of min(24 h, half
-- the schedule's spacing)), and it stays so until the keeper completes or skips it — it
-- is never completed automatically.
CREATE TABLE water_change_events (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id          uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    schedule_id      uuid REFERENCES water_change_schedules (id) ON DELETE RESTRICT,
    scheduled_for    timestamptz,
    overdue_after    timestamptz,
    planned_volume_l numeric(9, 2) CHECK (planned_volume_l > 0),
    status           text NOT NULL CHECK (status IN ('scheduled', 'completed', 'skipped')),
    completed_at     timestamptz,
    actual_volume_l  numeric(9, 2) CHECK (actual_volume_l > 0),
    -- The tank's system volume when the change was completed, so its estimate never
    -- changes when the keeper later corrects the tank volume (history is not rewritten).
    system_volume_l  numeric(9, 2) CHECK (system_volume_l > 0),
    salt_mix         text CHECK (char_length(salt_mix) <= 100),
    notes            text CHECK (char_length(notes) <= 2000),
    skip_reason      text CHECK (char_length(skip_reason) BETWEEN 1 AND 500),
    recorded_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at      timestamptz, -- when the keeper completed/skipped it (≠ completed_at, the water-change time)
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    version          int NOT NULL DEFAULT 1,
    CHECK (schedule_id IS NULL OR scheduled_for IS NOT NULL),
    -- Every row has an event time: completed rows completed_at, the others scheduled_for.
    CHECK (status = 'completed' OR scheduled_for IS NOT NULL),
    CHECK ((scheduled_for IS NULL) = (overdue_after IS NULL)),
    CHECK (overdue_after IS NULL OR overdue_after >= scheduled_for),
    CHECK (status <> 'scheduled' OR planned_volume_l IS NOT NULL),
    CHECK ((status = 'completed') = (completed_at IS NOT NULL)),
    CHECK ((status = 'completed') = (actual_volume_l IS NOT NULL)),
    CHECK ((status = 'completed') = (system_volume_l IS NOT NULL)),
    CHECK ((status = 'skipped') = (skip_reason IS NOT NULL)),
    CHECK ((status IN ('completed', 'skipped')) = (resolved_at IS NOT NULL))
);
CREATE UNIQUE INDEX water_change_occurrence ON water_change_events (schedule_id, scheduled_for)
    WHERE schedule_id IS NOT NULL;
-- List/filter by tank in time order (completed_at for done changes, scheduled_for otherwise).
CREATE INDEX water_change_events_by_tank ON water_change_events (tank_id, (COALESCE(completed_at, scheduled_for)) DESC, id DESC);
-- Overdue / upcoming lists (status = 'scheduled' only) without walking the history.
CREATE INDEX water_change_events_scheduled ON water_change_events (tank_id, scheduled_for DESC, id DESC)
    WHERE status = 'scheduled';

-- +goose Down
DROP TABLE water_change_events;
DROP TABLE water_change_schedules;
