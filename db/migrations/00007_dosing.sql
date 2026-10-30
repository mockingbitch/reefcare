-- +goose Up
-- Dosing products, plans and events (docs/domain-model.md "Dosing", milestone 2.4).
SET LOCAL lock_timeout = '5s';

-- A product is what is dosed. No concentration is stored or inferred from it
-- (.claude/rules/reef-domain.md): plans and events record amounts only. Powders are often
-- dissolved and dosed by volume, so powder products accept ml/l ("prepared solution").
CREATE TABLE dosing_products (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    owner_id     uuid REFERENCES users (id) ON DELETE RESTRICT, -- NULL: curated catalog entry
    name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    brand        text CHECK (char_length(brand) <= 100),
    form         text NOT NULL CHECK (form IN ('liquid', 'powder', 'tablet', 'other')),
    default_unit text NOT NULL CHECK (default_unit IN ('ml', 'l', 'drops', 'g', 'mg', 'tablets')),
    notes        text CHECK (char_length(notes) <= 2000),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX dosing_products_by_owner ON dosing_products (owner_id, lower(name), id);

CREATE TABLE dosing_plans (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id            uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    product_id         uuid NOT NULL REFERENCES dosing_products (id) ON DELETE RESTRICT,
    amount             numeric(12, 4) NOT NULL CHECK (amount > 0),
    amount_unit        text NOT NULL CHECK (amount_unit IN ('ml', 'l', 'drops', 'g', 'mg', 'tablets')),
    rrule              text NOT NULL CHECK (char_length(rrule) <= 200),
    timezone           text NOT NULL CHECK (char_length(timezone) <= 64),
    starts_on          date NOT NULL,
    ends_on            date,
    -- Recorded only: ReefCare never controls a pump. Pump occurrences become
    -- 'unconfirmed' (not 'missed') when nobody confirms them.
    delivery_method    text NOT NULL CHECK (delivery_method IN ('manual', 'dosing_pump')),
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed')),
    notes              text CHECK (char_length(notes) <= 2000),
    materialized_until timestamptz,
    created_by         uuid REFERENCES users (id) ON DELETE SET NULL, -- users are anonymized, not deleted
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    version            int NOT NULL DEFAULT 1,
    CHECK (ends_on IS NULL OR ends_on >= starts_on),
    -- Target of the composite FK below: an occurrence always belongs to its plan's tank
    -- and product (neither changes after creation).
    UNIQUE (id, tank_id, product_id)
);
CREATE INDEX dosing_plans_by_tank ON dosing_plans (tank_id, created_at, id);
CREATE INDEX dosing_plans_by_product ON dosing_plans (product_id);
CREATE INDEX dosing_plans_active ON dosing_plans (id) WHERE status = 'active'; -- materialize job keyset

-- One planned occurrence or one ad-hoc dose. Planned and actual amounts are separate.
-- When a scheduled occurrence passes missed_after without being recorded, the scheduler
-- sets 'missed' (manual plans) or 'unconfirmed' (pump plans: ReefCare cannot know whether
-- the pump delivered). Neither is ever completed automatically; the keeper may resolve
-- them later, and missed_at keeps that history.
CREATE TABLE dosing_events (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    tank_id         uuid NOT NULL REFERENCES tanks (id) ON DELETE RESTRICT,
    plan_id         uuid,
    product_id      uuid NOT NULL REFERENCES dosing_products (id) ON DELETE RESTRICT,
    scheduled_for   timestamptz,
    missed_after    timestamptz,
    planned_amount  numeric(12, 4) CHECK (planned_amount > 0),
    actual_amount   numeric(12, 4) CHECK (actual_amount > 0),
    -- 'entered' by the keeper, or 'planned_default' when a completion took the plan's amount.
    actual_source   text CHECK (actual_source IN ('entered', 'planned_default')),
    amount_unit     text NOT NULL CHECK (amount_unit IN ('ml', 'l', 'drops', 'g', 'mg', 'tablets')),
    status          text NOT NULL CHECK (status IN ('scheduled', 'completed', 'partial', 'skipped', 'failed',
                                                    'missed', 'unconfirmed')),
    executed_at     timestamptz,
    reason          text CHECK (char_length(reason) BETWEEN 1 AND 500), -- the keeper's words
    -- System reasons, kept apart from free text so they can't be confused with it.
    reason_code     text CHECK (reason_code IN ('tank_archived', 'plan_paused', 'plan_changed')),
    notes           text CHECK (char_length(notes) <= 2000),
    reported_by     text CHECK (reported_by IN ('manual', 'device')),
    missed_at       timestamptz,
    resolved_at     timestamptz,
    recorded_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Retried ad-hoc creates (Idempotency-Key header): unique per recorder, with a hash of
    -- the request so a reused key with a different body is rejected.
    idempotency_key text CHECK (char_length(idempotency_key) BETWEEN 8 AND 128),
    request_hash    bytea CHECK (octet_length(request_hash) = 32),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    version         int NOT NULL DEFAULT 1,
    FOREIGN KEY (plan_id, tank_id, product_id) REFERENCES dosing_plans (id, tank_id, product_id) ON DELETE RESTRICT,
    CHECK (plan_id IS NULL OR scheduled_for IS NOT NULL),
    CHECK ((scheduled_for IS NULL) = (missed_after IS NULL)),
    CHECK (missed_after IS NULL OR missed_after >= scheduled_for),
    CHECK (status NOT IN ('scheduled', 'missed', 'unconfirmed') OR (scheduled_for IS NOT NULL AND planned_amount IS NOT NULL)),
    -- Every row has an event time: executed_at when dosed, else scheduled_for.
    CHECK (executed_at IS NOT NULL OR scheduled_for IS NOT NULL),
    CHECK ((status IN ('completed', 'partial')) = (executed_at IS NOT NULL)),
    CHECK (status NOT IN ('completed', 'partial') OR actual_amount IS NOT NULL),
    CHECK (status IN ('completed', 'partial', 'failed') OR actual_amount IS NULL), -- failed may record what went in
    CHECK ((actual_amount IS NULL) = (actual_source IS NULL)),
    CHECK (status <> 'partial' OR (planned_amount IS NOT NULL AND actual_amount < planned_amount)),
    CHECK ((status IN ('skipped', 'failed')) = (reason IS NOT NULL OR reason_code IS NOT NULL)),
    CHECK ((status IN ('completed', 'partial', 'skipped', 'failed')) = (resolved_at IS NOT NULL)),
    CHECK (status <> 'missed' OR missed_at IS NOT NULL),
    CHECK (missed_at IS NULL OR scheduled_for IS NOT NULL),
    CHECK ((idempotency_key IS NULL) = (request_hash IS NULL)),
    CHECK (idempotency_key IS NULL OR (plan_id IS NULL AND recorded_by IS NOT NULL))
);
CREATE UNIQUE INDEX dosing_occurrence ON dosing_events (plan_id, scheduled_for) WHERE plan_id IS NOT NULL;
CREATE UNIQUE INDEX dosing_events_idempotency ON dosing_events (recorded_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX dosing_events_by_tank ON dosing_events (tank_id, (COALESCE(executed_at, scheduled_for)) DESC, id DESC);
CREATE INDEX dosing_events_by_product ON dosing_events (product_id); -- RESTRICT checks on product delete
-- Scheduler: due occurrences to mark missed/unconfirmed.
CREATE INDEX dosing_events_due ON dosing_events (missed_after) WHERE status = 'scheduled';

-- +goose Down
DROP TABLE dosing_events;
DROP TABLE dosing_plans;
DROP TABLE dosing_products;
