-- +goose Up
-- Equipment with versioned configurations, and interval-based maintenance
-- (docs/domain-model.md "Equipment & maintenance", US-EQ-1..3, milestone 2.6).
SET LOCAL lock_timeout = '5s';

-- One physical device on a tank. Several of the same model are allowed. Removal is a date,
-- never a delete: history (configurations, maintenance, timeline) keeps pointing here.
CREATE TABLE tank_equipment (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id        uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    category       text NOT NULL CHECK (category IN ('lighting', 'return_pump', 'flow_pump', 'skimmer', 'heater',
                                                     'chiller', 'dosing_pump', 'ato', 'reactor', 'uv', 'controller',
                                                     'filter', 'sensor', 'other')),
    name           text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    manufacturer   text CHECK (char_length(manufacturer) <= 100),
    model          text CHECK (char_length(model) <= 100),
    serial_number  text CHECK (char_length(serial_number) <= 100),
    purchased_on   date,
    warranty_until date,
    installed_at   timestamptz NOT NULL,
    removed_at     timestamptz,
    removal_reason text CHECK (char_length(removal_reason) BETWEEN 1 AND 500),
    notes          text CHECK (char_length(notes) <= 2000),
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL, -- users are anonymized, not deleted
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    version        int NOT NULL DEFAULT 1,
    CHECK (removed_at IS NULL OR removed_at >= installed_at),
    CHECK (removal_reason IS NULL OR removed_at IS NOT NULL),
    CHECK (warranty_until IS NULL OR purchased_on IS NULL OR warranty_until >= purchased_on),
    UNIQUE (id, tank_id) -- target of composite FKs: children stay on the equipment's tank
);
CREATE INDEX tank_equipment_by_tank ON tank_equipment (tank_id, installed_at DESC, id DESC);

-- Immutable configuration versions (light schedule, pump speed, …). The current one is the
-- highest version_no; earlier ones stay visible (US-EQ-2). settings is free-form metadata.
CREATE TABLE equipment_configurations (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    equipment_id uuid NOT NULL,
    tank_id      uuid NOT NULL,
    version_no   int NOT NULL CHECK (version_no >= 1),
    settings     jsonb NOT NULL CHECK (jsonb_typeof(settings) = 'object' AND pg_column_size(settings) <= 8192),
    effective_at timestamptz NOT NULL,
    note         text CHECK (char_length(note) <= 500),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (equipment_id, tank_id) REFERENCES tank_equipment (id, tank_id) ON DELETE RESTRICT,
    UNIQUE (equipment_id, version_no)
);

-- A recurring task. Due = last performed (or the start) + interval_days: maintenance is
-- usually "every N days since the last time", not on fixed calendar dates. Overdue is
-- derived from next_due_on, never stored as a status.
CREATE TABLE maintenance_plans (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id           uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    equipment_id      uuid,
    title             text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 100),
    interval_days     int NOT NULL CHECK (interval_days BETWEEN 1 AND 730),
    -- Due dates are calendar dates in this zone: due on D means "some time on day D"; the
    -- plan is overdue once D has passed there.
    timezone          text NOT NULL CHECK (char_length(timezone) <= 64),
    starts_on         date NOT NULL,
    last_performed_at timestamptz,
    next_due_on       date NOT NULL, -- local date of the last performance + interval, or starts_on
    active            boolean NOT NULL DEFAULT true,
    notes             text CHECK (char_length(notes) <= 2000),
    created_by        uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    version           int NOT NULL DEFAULT 1,
    FOREIGN KEY (equipment_id, tank_id) REFERENCES tank_equipment (id, tank_id) ON DELETE RESTRICT,
    UNIQUE (id, tank_id)
);
CREATE INDEX maintenance_plans_by_tank ON maintenance_plans (tank_id, next_due_on, id);
CREATE INDEX maintenance_plans_by_equipment ON maintenance_plans (equipment_id) WHERE equipment_id IS NOT NULL;

-- Something that was done. History: never edited by normal flows.
CREATE TABLE maintenance_events (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id      uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    plan_id      uuid,
    equipment_id uuid,
    title        text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 100),
    performed_at timestamptz NOT NULL,
    notes        text CHECK (char_length(notes) <= 2000),
    recorded_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Retried creates (Idempotency-Key header): unique per recorder, with a hash of the
    -- request so a key reused for a different request is rejected.
    idempotency_key text CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    request_hash    bytea CHECK (octet_length(request_hash) = 32),
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK (idempotency_key IS NULL OR recorded_by IS NOT NULL),
    FOREIGN KEY (plan_id, tank_id) REFERENCES maintenance_plans (id, tank_id) ON DELETE RESTRICT,
    FOREIGN KEY (equipment_id, tank_id) REFERENCES tank_equipment (id, tank_id) ON DELETE RESTRICT
);
CREATE INDEX maintenance_events_by_tank ON maintenance_events (tank_id, performed_at DESC, id DESC);
CREATE INDEX maintenance_events_by_plan ON maintenance_events (plan_id, performed_at DESC, id DESC)
    WHERE plan_id IS NOT NULL;
CREATE INDEX maintenance_events_by_equipment ON maintenance_events (equipment_id, performed_at DESC, id DESC)
    WHERE equipment_id IS NOT NULL;
CREATE UNIQUE INDEX maintenance_events_idempotency ON maintenance_events (recorded_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP TABLE maintenance_events;
DROP TABLE maintenance_plans;
DROP TABLE equipment_configurations;
DROP TABLE tank_equipment;
