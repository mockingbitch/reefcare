package waterparameters

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/decimal"
)

// AssessmentReading is a valid reading in canonical units, for the intelligence snapshot.
type AssessmentReading struct {
	ID         uuid.UUID
	Parameter  string
	MeasuredAt time.Time
	Value      decimal.Decimal
	Estimate   bool
	Method     string
	TestKit    string
}

// AssessmentTarget is one target version.
type AssessmentTarget struct {
	Parameter      string
	Min, Max       *decimal.Decimal
	FreshnessHours *int32
	ValidFrom      time.Time
	ValidTo        *time.Time
}

// ForAssessment returns the newest `limit` valid readings in [since, until], oldest first,
// and the whole target history. The caller has authorized the tank.
func ForAssessment(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since, until time.Time, limit int) ([]AssessmentReading, []AssessmentTarget, error) {
	rows, err := q.ListReadingsForAssessment(ctx, dbq.ListReadingsForAssessmentParams{TankID: tankID, Since: since, Until: until, RowLimit: int32(limit)}) //nolint:gosec // a small constant
	if err != nil {
		return nil, nil, err
	}
	rs := make([]AssessmentReading, len(rows))
	for i := range rows {
		r := rows[len(rows)-1-i] // newest first → oldest first
		rs[i] = AssessmentReading{ID: r.ID, Parameter: r.Parameter, MeasuredAt: r.MeasuredAt, Value: r.CanonicalValue, Estimate: r.Estimate}
		if r.Method != nil {
			rs[i].Method = *r.Method
		}
		if r.TestKit != nil {
			rs[i].TestKit = *r.TestKit
		}
	}
	trows, err := q.ListTargetHistory(ctx, tankID)
	if err != nil {
		return nil, nil, err
	}
	ts := make([]AssessmentTarget, len(trows))
	for i, r := range trows {
		ts[i] = AssessmentTarget{Parameter: r.Parameter, Min: r.MinValue, Max: r.MaxValue, FreshnessHours: r.FreshnessHours, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo}
	}
	return rs, ts, nil
}

// JobAssessAfterTest is the job kind enqueued for every new water test; package intelligence
// handles it (docs/knowledge-engine.md §7).
const JobAssessAfterTest = "intelligence.assess_after_test"

// AssessDebounce groups the tests of one tank into one automatic assessment.
const AssessDebounce = 10 * time.Minute

// AssessAfterTest is its payload: ids only.
type AssessAfterTest struct {
	TankID      uuid.UUID `json:"tank_id"`
	WaterTestID uuid.UUID `json:"water_test_id"`
	UserID      uuid.UUID `json:"user_id"`
}
