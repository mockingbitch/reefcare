-- +goose Up
-- Feedback on assessments (docs/knowledge-engine.md §10, milestone 3.5). Reviewers use "wrong"
-- feedback to adjust rules (a new rule_version) or knowledge (a new article version); past
-- assessments are never rewritten.
SET LOCAL lock_timeout = '5s';

CREATE TABLE assessment_feedback (
    assessment_id uuid NOT NULL REFERENCES assessments (id) ON DELETE RESTRICT,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT, -- erasure deletes it
    rating        text NOT NULL CHECK (rating IN ('useful', 'not_useful', 'wrong')),
    -- What happened afterwards, in the keeper's words (e.g. "re-tested: kit was expired").
    outcome_note  text CHECK (char_length(outcome_note) BETWEEN 1 AND 2000),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (assessment_id, user_id)
);
CREATE INDEX assessment_feedback_by_user ON assessment_feedback (user_id);
CREATE INDEX assessment_feedback_wrong ON assessment_feedback (created_at) WHERE rating = 'wrong';

-- +goose Down
DROP TABLE assessment_feedback;
