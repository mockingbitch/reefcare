-- +goose Up
-- Species, livestock entries and their events (docs/domain-model.md "Livestock",
-- US-LS-1..3, milestone 2.7).
SET LOCAL lock_timeout = '5s';

-- Reference species: curated (owner_id NULL, seeded only after the D5 domain review) or a
-- keeper's own, usable only on their tanks. Species are never edited.
CREATE TABLE species (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    owner_id        uuid REFERENCES users (id) ON DELETE RESTRICT, -- NULL: curated catalog entry
    category        text NOT NULL CHECK (category IN ('fish', 'coral', 'invertebrate', 'algae', 'other')),
    coral_type      text CHECK (coral_type IN ('sps', 'lps', 'soft', 'non_photosynthetic')),
    common_name_vi  text CHECK (char_length(common_name_vi) BETWEEN 1 AND 100),
    common_name_en  text CHECK (char_length(common_name_en) BETWEEN 1 AND 100),
    scientific_name text CHECK (char_length(scientific_name) BETWEEN 1 AND 150),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (coral_type IS NULL OR category = 'coral'),
    CHECK (num_nonnulls(common_name_vi, common_name_en, scientific_name) > 0),
    UNIQUE (id, category) -- target of the entries' FK: an entry's category matches its species
);
CREATE INDEX species_by_owner ON species (owner_id, id);

-- A group of individuals of one kind in one tank. current_quantity caches the sum of the
-- entry's event deltas; the service keeps it (and every running balance in event order)
-- within 0..100000 under a row lock on the entry. A group that reaches 0 is "gone" but never
-- deleted. Counting convention: a colony, a frag plug or a rock of zoanthids is 1.
CREATE TABLE livestock_entries (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id           uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    species_id        uuid,
    category          text NOT NULL CHECK (category IN ('fish', 'coral', 'invertebrate', 'algae', 'other')),
    coral_type        text CHECK (coral_type IN ('sps', 'lps', 'soft', 'non_photosynthetic')),
    display_name      text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 100),
    form              text CHECK (form IN ('individual', 'group', 'frag', 'colony')),
    quarantine_status text CHECK (quarantine_status IN ('in_quarantine', 'completed')),
    location_note     text CHECK (char_length(location_note) BETWEEN 1 AND 200),
    source_note       text CHECK (char_length(source_note) BETWEEN 1 AND 200),
    notes             text CHECK (char_length(notes) BETWEEN 1 AND 2000),
    current_quantity  int NOT NULL CHECK (current_quantity BETWEEN 0 AND 100000),
    added_at          timestamptz NOT NULL, -- occurred_at of the 'added' event that created it
    created_by        uuid REFERENCES users (id) ON DELETE SET NULL, -- users are anonymized, not deleted
    -- Retried creates (Idempotency-Key header), as for maintenance events.
    idempotency_key   text CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    request_hash      bytea CHECK (octet_length(request_hash) = 32),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    version           int NOT NULL DEFAULT 1, -- covers the description; quantity changes only via events
    CHECK (coral_type IS NULL OR category = 'coral'),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK (idempotency_key IS NULL OR created_by IS NOT NULL),
    FOREIGN KEY (species_id, category) REFERENCES species (id, category) ON DELETE RESTRICT,
    UNIQUE (id, tank_id) -- target of the events' composite FK: events stay on the entry's tank
);
CREATE INDEX livestock_entries_by_tank ON livestock_entries (tank_id, added_at DESC, id DESC);
CREATE INDEX livestock_entries_by_species ON livestock_entries (species_id) WHERE species_id IS NOT NULL;
CREATE UNIQUE INDEX livestock_entries_idempotency ON livestock_entries (created_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Append-only (trigger below): history is never edited. A wrong count is corrected with a
-- 'reconciliation' event (actual count − balance at that time). Observations carry a note and
-- no quantity. 'missing' is an unseen loss (fate unknown), never a death; 'found' undoes it.
-- 'propagated' is growth inside the system (fragging, splitting, reproduction), unlike 'added'
-- (an arrival from outside) and 'transferred_in' (from another tank).
CREATE TABLE livestock_events (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    entry_id        uuid NOT NULL,
    tank_id         uuid NOT NULL,
    event_type      text NOT NULL CHECK (event_type IN ('added', 'transferred_in', 'propagated', 'found', 'died',
                                                        'missing', 'transferred_out', 'sold', 'reconciliation',
                                                        'health_observation', 'treatment')),
    quantity_delta  int NOT NULL CHECK (quantity_delta BETWEEN -100000 AND 100000),
    occurred_at     timestamptz NOT NULL,
    note            text CHECK (char_length(note) BETWEEN 1 AND 2000),
    recorded_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    idempotency_key text CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    request_hash    bytea CHECK (octet_length(request_hash) = 32),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (event_type IN ('added', 'transferred_in', 'propagated', 'found') AND quantity_delta > 0
        OR event_type IN ('died', 'missing', 'transferred_out', 'sold') AND quantity_delta < 0
        OR event_type = 'reconciliation' AND quantity_delta <> 0
        OR event_type IN ('health_observation', 'treatment') AND quantity_delta = 0),
    CHECK (event_type NOT IN ('health_observation', 'treatment') OR note IS NOT NULL),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK (idempotency_key IS NULL OR recorded_by IS NOT NULL),
    FOREIGN KEY (entry_id, tank_id) REFERENCES livestock_entries (id, tank_id) ON DELETE RESTRICT
);
CREATE INDEX livestock_events_by_entry ON livestock_events (entry_id, occurred_at DESC, id DESC);
-- Tank-wide history (erasure today, intelligence rules later).
CREATE INDEX livestock_events_by_tank ON livestock_events (tank_id, occurred_at DESC, id DESC);
CREATE UNIQUE INDEX livestock_events_idempotency ON livestock_events (recorded_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION livestock_events_append_only() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    -- recorded_by may become NULL when the recorder's account is erased (ON DELETE SET NULL).
    IF (NEW.id, NEW.entry_id, NEW.tank_id, NEW.event_type, NEW.quantity_delta, NEW.occurred_at, NEW.note,
        NEW.idempotency_key, NEW.request_hash, NEW.created_at)
       IS NOT DISTINCT FROM
       (OLD.id, OLD.entry_id, OLD.tank_id, OLD.event_type, OLD.quantity_delta, OLD.occurred_at, OLD.note,
        OLD.idempotency_key, OLD.request_hash, OLD.created_at)
       AND NEW.recorded_by IS NULL
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'livestock_events is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

-- DELETE stays allowed: account erasure removes the erased keeper's history.
CREATE TRIGGER livestock_events_append_only
    BEFORE UPDATE ON livestock_events
    FOR EACH ROW EXECUTE FUNCTION livestock_events_append_only();

-- +goose Down
DROP TABLE livestock_events;
DROP FUNCTION livestock_events_append_only();
DROP TABLE livestock_entries;
DROP TABLE species;
