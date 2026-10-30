-- +goose Up
-- Parameter catalog, water tests with readings and corrections, and per-tank targets
-- (docs/domain-model.md "Water parameters", US-WP-1..5, milestones 2.1/2.2).
SET LOCAL lock_timeout = '5s';

-- System parameters. Units, conversions, hard bounds and freshness live in Go
-- (internal/waterparameters/catalog.go, provisional until decision D5); this table anchors
-- the foreign keys and must list the same codes (checked by an integration test).
CREATE TABLE parameter_types (
    code           text PRIMARY KEY CHECK (code ~ '^[a-z_]{2,40}$'),
    canonical_unit text NOT NULL CHECK (char_length(canonical_unit) BETWEEN 1 AND 20),
    display_order  int NOT NULL UNIQUE
);
INSERT INTO parameter_types (code, canonical_unit, display_order) VALUES
    ('alkalinity', 'dKH', 10), ('calcium', 'mg/L', 20), ('magnesium', 'mg/L', 30), ('ph', 'pH', 40),
    ('temperature', 'C', 50), ('salinity', 'ppt', 60), ('nitrate', 'mg/L NO3', 70),
    ('phosphate', 'mg/L PO4', 80), ('ammonia', 'mg/L NH3', 90), ('nitrite', 'mg/L NO2', 100);

-- One testing session: one or more readings measured at the same time.
CREATE TABLE water_tests (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id         uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    measured_at     timestamptz NOT NULL,
    source          text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'imported', 'sensor')),
    notes           text CHECK (char_length(notes) BETWEEN 1 AND 2000),
    recorded_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Retried creates (Idempotency-Key header), as for livestock entries.
    idempotency_key text CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    request_hash    bytea CHECK (octet_length(request_hash) = 32),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK (idempotency_key IS NULL OR recorded_by IS NOT NULL),
    UNIQUE (id, tank_id) -- target of the readings' composite FK
);
CREATE INDEX water_tests_by_tank ON water_tests (tank_id, measured_at DESC, id DESC);
CREATE UNIQUE INDEX water_tests_idempotency ON water_tests (recorded_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- A reading keeps the value and unit as entered and the canonical value used for charts and
-- comparisons. Readings are never edited: a correction inserts a new reading that supersedes
-- the old one (which stays, with status 'superseded'). At most one valid reading per
-- parameter per test.
CREATE TABLE water_test_readings (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    water_test_id     uuid NOT NULL,
    tank_id           uuid NOT NULL,
    parameter         text NOT NULL REFERENCES parameter_types (code) ON DELETE RESTRICT,
    measured_at       timestamptz NOT NULL, -- copied from the test for the trend index
    input_value       numeric(12, 4) NOT NULL,
    input_unit        text NOT NULL CHECK (char_length(input_unit) BETWEEN 1 AND 20),
    canonical_value   numeric(12, 4) NOT NULL,
    estimate          boolean NOT NULL DEFAULT false, -- converted with an assumption (e.g. SG at 25 °C)
    method            text CHECK (method IN ('titration', 'colorimetric', 'photometer', 'refractometer',
                                             'hydrometer', 'probe', 'icp', 'other')),
    test_kit          text CHECK (char_length(test_kit) BETWEEN 1 AND 100),
    status            text NOT NULL DEFAULT 'valid' CHECK (status IN ('valid', 'superseded')),
    -- NO ACTION (checked at statement end) so erasing an account can delete a reading and its
    -- correction in one statement; nothing else deletes readings.
    supersedes_id     uuid UNIQUE REFERENCES water_test_readings (id),
    correction_reason text CHECK (char_length(correction_reason) BETWEEN 1 AND 500),
    recorded_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CHECK ((supersedes_id IS NULL) = (correction_reason IS NULL)),
    FOREIGN KEY (water_test_id, tank_id) REFERENCES water_tests (id, tank_id) ON DELETE RESTRICT
);
CREATE INDEX readings_trend ON water_test_readings (tank_id, parameter, measured_at DESC, id DESC)
    WHERE status = 'valid';
CREATE INDEX readings_by_test ON water_test_readings (water_test_id);
CREATE UNIQUE INDEX readings_one_valid_per_parameter ON water_test_readings (water_test_id, parameter)
    WHERE status = 'valid';

-- Readings are append-only except: valid → superseded (a correction), and recorded_by
-- becoming NULL when the recorder's account is erased.
-- +goose StatementBegin
CREATE FUNCTION water_test_readings_append_only() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF (NEW.id, NEW.water_test_id, NEW.tank_id, NEW.parameter, NEW.measured_at, NEW.input_value, NEW.input_unit,
        NEW.canonical_value, NEW.estimate, NEW.method, NEW.test_kit, NEW.supersedes_id, NEW.correction_reason,
        NEW.created_at)
       IS NOT DISTINCT FROM
       (OLD.id, OLD.water_test_id, OLD.tank_id, OLD.parameter, OLD.measured_at, OLD.input_value, OLD.input_unit,
        OLD.canonical_value, OLD.estimate, OLD.method, OLD.test_kit, OLD.supersedes_id, OLD.correction_reason,
        OLD.created_at)
       AND (NEW.status = OLD.status OR (OLD.status = 'valid' AND NEW.status = 'superseded'))
       AND (NEW.recorded_by IS NOT DISTINCT FROM OLD.recorded_by OR NEW.recorded_by IS NULL)
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'water_test_readings is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER water_test_readings_append_only
    BEFORE UPDATE ON water_test_readings
    FOR EACH ROW EXECUTE FUNCTION water_test_readings_append_only();

-- Tests are immutable too (readings copy measured_at for the trend index, so it must never
-- drift); only recorded_by may become NULL on account erasure.
-- +goose StatementBegin
CREATE FUNCTION water_tests_immutable() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF (NEW.id, NEW.tank_id, NEW.measured_at, NEW.source, NEW.notes, NEW.idempotency_key, NEW.request_hash,
        NEW.created_at)
       IS NOT DISTINCT FROM
       (OLD.id, OLD.tank_id, OLD.measured_at, OLD.source, OLD.notes, OLD.idempotency_key, OLD.request_hash,
        OLD.created_at)
       AND (NEW.recorded_by IS NOT DISTINCT FROM OLD.recorded_by OR NEW.recorded_by IS NULL)
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'water_tests rows are immutable' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER water_tests_immutable
    BEFORE UPDATE ON water_tests
    FOR EACH ROW EXECUTE FUNCTION water_tests_immutable();

-- Targets are versioned: changing one closes the current row (valid_to) and inserts a new
-- one, so each reading is compared with the target that was valid when it was measured.
CREATE TABLE parameter_targets (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id         uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    parameter       text NOT NULL REFERENCES parameter_types (code) ON DELETE RESTRICT,
    min_value       numeric(12, 4),
    ideal_value     numeric(12, 4),
    max_value       numeric(12, 4),
    freshness_hours int CHECK (freshness_hours BETWEEN 1 AND 8760), -- NULL: the catalog default
    valid_from      timestamptz NOT NULL,
    valid_to        timestamptz,
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (min_value IS NULL OR max_value IS NULL OR min_value <= max_value),
    CHECK (ideal_value IS NULL OR ((min_value IS NULL OR ideal_value >= min_value)
                                   AND (max_value IS NULL OR ideal_value <= max_value))),
    CHECK (num_nonnulls(min_value, ideal_value, max_value, freshness_hours) > 0),
    CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
CREATE UNIQUE INDEX parameter_targets_current ON parameter_targets (tank_id, parameter) WHERE valid_to IS NULL;
CREATE INDEX parameter_targets_history ON parameter_targets (tank_id, parameter, valid_from DESC);

-- Target versions are history: the only changes are closing the current one (valid_to set
-- once) and created_by becoming NULL on account erasure.
-- +goose StatementBegin
CREATE FUNCTION parameter_targets_versioned() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF (NEW.id, NEW.tank_id, NEW.parameter, NEW.min_value, NEW.ideal_value, NEW.max_value, NEW.freshness_hours,
        NEW.valid_from, NEW.created_at)
       IS NOT DISTINCT FROM
       (OLD.id, OLD.tank_id, OLD.parameter, OLD.min_value, OLD.ideal_value, OLD.max_value, OLD.freshness_hours,
        OLD.valid_from, OLD.created_at)
       AND (NEW.valid_to IS NOT DISTINCT FROM OLD.valid_to OR OLD.valid_to IS NULL)
       AND (NEW.created_by IS NOT DISTINCT FROM OLD.created_by OR NEW.created_by IS NULL)
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'parameter_targets versions are immutable' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER parameter_targets_versioned
    BEFORE UPDATE ON parameter_targets
    FOR EACH ROW EXECUTE FUNCTION parameter_targets_versioned();

-- +goose Down
DROP TABLE parameter_targets;
DROP FUNCTION parameter_targets_versioned();
DROP TABLE water_test_readings;
DROP FUNCTION water_test_readings_append_only();
DROP TABLE water_tests;
DROP FUNCTION water_tests_immutable();
DROP TABLE parameter_types;
