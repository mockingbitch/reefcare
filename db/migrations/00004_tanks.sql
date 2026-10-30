-- +goose Up
-- Tanks, read-only shares and the unified tank timeline (docs/database-design.md §3–4).

CREATE TABLE tanks (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    owner_id         uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    name             text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description      text CHECK (char_length(description) <= 4000),
    tank_type        text NOT NULL CHECK (tank_type IN ('fish_only', 'fowlr', 'mixed_reef', 'sps_dominant',
                                                        'lps_dominant', 'soft_coral', 'custom')),
    -- 'archived' is terminal and only reachable through the archive action.
    status           text NOT NULL DEFAULT 'cycling' CHECK (status IN ('cycling', 'active', 'paused', 'archived')),
    length_cm        numeric(7, 1) CHECK (length_cm > 0),
    width_cm         numeric(7, 1) CHECK (width_cm > 0),
    height_cm        numeric(7, 1) CHECK (height_cm > 0),
    -- Display volume and total system volume are both explicit: neither is derived from
    -- the other (display volume may include rock displacement; US-TK-1 AC1).
    display_volume_l numeric(9, 2) NOT NULL CHECK (display_volume_l > 0),
    sump_volume_l    numeric(9, 2) CHECK (sump_volume_l >= 0),
    system_volume_l  numeric(9, 2) NOT NULL CHECK (system_volume_l > 0),
    setup_date       date,
    water_source     text CHECK (char_length(water_source) <= 100),
    salt_mix         text CHECK (char_length(salt_mix) <= 100),
    archived_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    version          int NOT NULL DEFAULT 1,
    CHECK ((status = 'archived') = (archived_at IS NOT NULL)),
    -- Dimensions are entered as a set or not at all.
    CHECK ((length_cm IS NULL) = (width_cm IS NULL) AND (width_cm IS NULL) = (height_cm IS NULL))
);
CREATE INDEX tanks_by_owner ON tanks (owner_id, created_at DESC, id DESC);

-- Shares have no meaning without the tank or the grantee, so both cascade.
CREATE TABLE tank_shares (
    tank_id    uuid NOT NULL REFERENCES tanks (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    permission text NOT NULL DEFAULT 'viewer' CHECK (permission IN ('viewer')), -- decision D4
    granted_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tank_id, user_id)
);
CREATE INDEX tank_shares_by_user ON tank_shares (user_id, tank_id); -- covers the shared-tanks list

-- Chronological projection of everything that happened to a tank. source_type/source_id
-- point at the row that caused the event (no FK: several source tables); events are
-- written in the same transaction as their source, and the UNIQUE key makes
-- re-recording the same fact a no-op. Rule: source_id names the fact itself (a status
-- change row, a configuration version, a dosing event), never a parent entity that can
-- produce the same event type twice — the second occurrence would be dropped.
CREATE TABLE tank_events (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id     uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    occurred_at timestamptz NOT NULL,
    category    text NOT NULL CHECK (category IN ('measurement', 'dosing', 'water_change', 'equipment', 'maintenance',
                                                  'livestock', 'symptom', 'treatment', 'milestone', 'note', 'assessment')),
    event_type  text NOT NULL CHECK (event_type ~ '^[a-z_]+(\.[a-z_]+)+$'),
    source_type text NOT NULL CHECK (source_type ~ '^[a-z_]+$'),
    source_id   uuid NOT NULL,
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    -- i18n params only, render-ready; bounded so a bug can't bloat the feed.
    summary     jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(summary) = 'object' AND pg_column_size(summary) <= 4096),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_type, source_id, event_type)
);
CREATE INDEX tank_events_feed ON tank_events (tank_id, occurred_at DESC, id DESC);
CREATE INDEX tank_events_cat ON tank_events (tank_id, category, occurred_at DESC, id DESC);

-- +goose Down
DROP TABLE tank_events;
DROP TABLE tank_shares;
DROP TABLE tanks;
