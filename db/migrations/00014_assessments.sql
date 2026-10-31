-- +goose Up
-- Provisional assessments (docs/knowledge-engine.md §3–§7, milestone 3.4). An assessment is
-- immutable: it stores the snapshot it was computed from, the rule and knowledge versions,
-- and the validated result, so it can always be reproduced and explained. It can only be
-- retracted (with a reason), never edited.
SET LOCAL lock_timeout = '5s';

CREATE TABLE assessments (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id           uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    trigger           text NOT NULL CHECK (trigger IN ('manual', 'water_test', 'scheduled')),
    -- The water test that triggered it (trigger = water_test).
    source_id         uuid,
    status            text NOT NULL DEFAULT 'generated' CHECK (status IN ('generated', 'retracted')),
    result            jsonb NOT NULL CHECK (jsonb_typeof(result) = 'object' AND pg_column_size(result) <= 262144),
    finding_count     int NOT NULL CHECK (finding_count >= 0),
    max_severity      text CHECK (max_severity IN ('info', 'notice', 'warning', 'urgent')),
    confidence_label  text NOT NULL CHECK (confidence_label IN ('not_assessed', 'low', 'moderate', 'high')),
    input_snapshot    jsonb NOT NULL CHECK (jsonb_typeof(input_snapshot) = 'object'
                                            AND pg_column_size(input_snapshot) <= 4194304),
    snapshot_version  int NOT NULL CHECK (snapshot_version >= 1),
    rule_version      text NOT NULL CHECK (char_length(rule_version) BETWEEN 1 AND 20),
    knowledge_version text NOT NULL CHECK (char_length(knowledge_version) BETWEEN 1 AND 80),
    provisional       boolean NOT NULL, -- rule thresholds not yet confirmed by the domain reviewer
    -- The requester's locale, preferred when citing knowledge (each reference carries the
    -- locale actually cited; an article may only exist in the other one).
    locale            text NOT NULL CHECK (locale IN ('vi', 'en')),
    requested_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    retracted_at      timestamptz,
    retract_reason    text CHECK (char_length(retract_reason) BETWEEN 1 AND 500),
    created_at        timestamptz NOT NULL DEFAULT now(),
    CHECK ((trigger = 'water_test') = (source_id IS NOT NULL)),
    CHECK ((finding_count = 0) = (max_severity IS NULL)),
    CHECK ((status = 'retracted') = (retracted_at IS NOT NULL)),
    CHECK ((status = 'retracted') = (retract_reason IS NOT NULL))
);
CREATE INDEX assessments_by_tank ON assessments (tank_id, created_at DESC, id DESC);
-- One automatic assessment per water test (a retried job inserts nothing).
CREATE UNIQUE INDEX assessments_per_test ON assessments (source_id) WHERE trigger = 'water_test';

-- Every fact an assessment cites, so "which assessments used this reading?" is answerable.
CREATE TABLE assessment_evidence (
    assessment_id uuid NOT NULL REFERENCES assessments (id) ON DELETE RESTRICT,
    kind          text NOT NULL CHECK (kind IN ('reading', 'dosing_event', 'water_change', 'equipment_change',
                                                'livestock_event')),
    ref_id        uuid NOT NULL,
    PRIMARY KEY (assessment_id, kind, ref_id)
);
CREATE INDEX assessment_evidence_by_ref ON assessment_evidence (kind, ref_id);

-- The exact knowledge versions an assessment cites (RESTRICT: versions are never deleted).
CREATE TABLE assessment_knowledge_refs (
    assessment_id uuid NOT NULL REFERENCES assessments (id) ON DELETE RESTRICT,
    version_id    uuid NOT NULL REFERENCES knowledge_article_versions (id) ON DELETE RESTRICT,
    PRIMARY KEY (assessment_id, version_id)
);
CREATE INDEX assessment_knowledge_refs_by_version ON assessment_knowledge_refs (version_id);
CREATE INDEX assessments_automatic ON assessments (tank_id, created_at) WHERE trigger <> 'manual';

-- Snapshot queries: the newest readings of a tank across parameters, and configuration changes
-- by tank (equipment_configurations had no tank index).
CREATE INDEX readings_recent ON water_test_readings (tank_id, measured_at DESC, id DESC) WHERE status = 'valid';
CREATE INDEX equipment_configurations_by_tank ON equipment_configurations (tank_id, effective_at);

-- +goose StatementBegin
-- Assessments are immutable; the only change is retraction (and the requester becoming NULL
-- when the account is erased).
CREATE FUNCTION assessments_immutable() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF (NEW.id, NEW.tank_id, NEW.trigger, NEW.source_id, NEW.result, NEW.finding_count, NEW.max_severity,
        NEW.confidence_label, NEW.input_snapshot, NEW.snapshot_version, NEW.rule_version, NEW.knowledge_version,
        NEW.provisional, NEW.locale, NEW.created_at)
       IS DISTINCT FROM
       (OLD.id, OLD.tank_id, OLD.trigger, OLD.source_id, OLD.result, OLD.finding_count, OLD.max_severity,
        OLD.confidence_label, OLD.input_snapshot, OLD.snapshot_version, OLD.rule_version, OLD.knowledge_version,
        OLD.provisional, OLD.locale, OLD.created_at)
       OR (NEW.requested_by IS DISTINCT FROM OLD.requested_by AND NEW.requested_by IS NOT NULL)
       OR (OLD.status = 'retracted' AND (NEW.status, NEW.retracted_at, NEW.retract_reason)
                                        IS DISTINCT FROM (OLD.status, OLD.retracted_at, OLD.retract_reason))
    THEN
        RAISE EXCEPTION 'assessments are immutable (only retraction is allowed)' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER assessments_immutable
    BEFORE UPDATE ON assessments
    FOR EACH ROW EXECUTE FUNCTION assessments_immutable();

-- +goose StatementBegin
CREATE FUNCTION assessment_children_immutable() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    RAISE EXCEPTION '% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

-- DELETE stays possible: account erasure removes them with their assessment.
CREATE TRIGGER assessment_evidence_immutable
    BEFORE UPDATE ON assessment_evidence
    FOR EACH ROW EXECUTE FUNCTION assessment_children_immutable();
CREATE TRIGGER assessment_knowledge_refs_immutable
    BEFORE UPDATE ON assessment_knowledge_refs
    FOR EACH ROW EXECUTE FUNCTION assessment_children_immutable();

-- +goose Down
DROP INDEX equipment_configurations_by_tank;
DROP INDEX readings_recent;
DROP TABLE assessment_knowledge_refs;
DROP TABLE assessment_evidence;
DROP TABLE assessments;
DROP FUNCTION assessment_children_immutable();
DROP FUNCTION assessments_immutable();
