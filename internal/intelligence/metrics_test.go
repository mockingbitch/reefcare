package intelligence

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/platform/decimal"
)

var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func d(s string) decimal.Decimal { return decimal.MustParse(s) }

func rd(param string, daysAgo float64, value string) Reading {
	return Reading{ID: uuid.New(), Parameter: param, MeasuredAt: now.Add(-time.Duration(daysAgo * float64(day))), Value: d(value)}
}

func eq(t *testing.T, name string, got *decimal.Decimal, want string) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Errorf("%s = %s, want nil", name, got)
	case want != "" && (got == nil || got.Cmp(d(want)) != 0):
		t.Errorf("%s = %v, want %s", name, got, want)
	}
}

func TestParameterMetrics(t *testing.T) {
	s := Snapshot{Now: now, Readings: []Reading{
		rd("alkalinity", 20, "8.0"),
		rd("alkalinity", 6, "8.4"),
		rd("alkalinity", 4, "8.2"),
		rd("alkalinity", 2, "7.6"),
		rd("alkalinity", 0.5, "7.4"),
		rd("calcium", 1, "420"),
		rd("alkalinity", -1, "9"), // in the future: ignored
	}, Targets: []Target{{Parameter: "alkalinity", Min: dp("7.8"), Max: dp("8.6"), ValidFrom: now.Add(-60 * day)}}}

	m := MetricsFor(s, "alkalinity")
	if m.Count != 5 || m.Latest.Value.Cmp(d("7.4")) != 0 || m.Previous.Value.Cmp(d("7.6")) != 0 {
		t.Fatalf("latest/previous: %+v", m)
	}
	if !m.Fresh || m.FreshnessHours != 7*24 || m.Age != 12*time.Hour || m.Unit != "dKH" {
		t.Errorf("freshness: %+v", m)
	}
	if m.TargetStatus != "below" {
		t.Errorf("target status %q", m.TargetStatus)
	}
	eq(t, "delta", m.DeltaFromPrevious, "-0.2")
	eq(t, "delta/day", m.DeltaPerDay, "-0.133333")
	eq(t, "7d min", m.Window7.Min, "7.4")
	eq(t, "7d max", m.Window7.Max, "8.4")
	eq(t, "7d range", m.Window7.Range, "1")
	eq(t, "7d mean", m.Window7.Mean, "7.9")
	if m.Window7.Count != 4 || m.Window30.Count != 5 {
		t.Errorf("window counts %d %d", m.Window7.Count, m.Window30.Count)
	}
	eq(t, "max step", m.MaxStepChange, "0.6")
	if m.SlopePerDay == nil || m.SlopePerDay.Sign() >= 0 || m.SlopePoints != 5 {
		t.Errorf("slope %v over %d", m.SlopePerDay, m.SlopePoints)
	}

	// A perfect line: 8, 8.5, 9 at day 0, 1, 2 → slope 0.5/day over 2 days.
	line := Snapshot{Now: now, Readings: []Reading{rd("calcium", 2, "8"), rd("calcium", 1, "8.5"), rd("calcium", 0, "9")}}
	lm := MetricsFor(line, "calcium")
	eq(t, "slope", lm.SlopePerDay, "0.5")
	eq(t, "span", lm.SlopeSpan, "2")

	// Too few points or none.
	if MetricsFor(Snapshot{Now: now, Readings: []Reading{rd("ph", 1, "8.1"), rd("ph", 0, "8.2")}}, "ph").SlopePerDay != nil {
		t.Error("slope needs 3 points")
	}
	empty := MetricsFor(s, "nitrate")
	if empty.Latest != nil || empty.Fresh || empty.Count != 0 || empty.FreshnessHours != 14*24 {
		t.Errorf("no readings: %+v", empty)
	}
	stale := MetricsFor(Snapshot{Now: now, Readings: []Reading{rd("ph", 8, "8.1")}}, "ph")
	if stale.Fresh {
		t.Error("8-day-old pH is stale")
	}
	// A target's freshness_hours overrides the catalog.
	h := int32(24)
	custom := MetricsFor(Snapshot{Now: now, Readings: []Reading{rd("ph", 2, "8.1")},
		Targets: []Target{{Parameter: "ph", FreshnessHours: &h, ValidFrom: now.Add(-day * 30)}}}, "ph")
	if custom.Fresh || custom.FreshnessHours != 24 || custom.TargetStatus != "" {
		t.Errorf("custom freshness: %+v", custom)
	}
}

func TestTargetAtMeasurementTime(t *testing.T) {
	changed := now.Add(-3 * day)
	s := Snapshot{Now: now, Readings: []Reading{rd("calcium", 5, "390")}, Targets: []Target{
		{Parameter: "calcium", Min: dp("380"), Max: dp("450"), ValidFrom: now.Add(-30 * day), ValidTo: &changed},
		{Parameter: "calcium", Min: dp("400"), Max: dp("450"), ValidFrom: changed},
	}}
	if m := MetricsFor(s, "calcium"); m.TargetStatus != "within" {
		t.Errorf("status uses the target valid when measured, got %q", m.TargetStatus)
	}
}

func TestAdherence(t *testing.T) {
	manual, pump := uuid.New(), uuid.New()
	at := func(h float64) *time.Time { v := now.Add(-time.Duration(h * float64(time.Hour))); return &v }
	s := Snapshot{Now: now,
		DosingPlans: []DosingPlan{{ID: manual, Delivery: "manual", Status: "active"}, {ID: pump, Delivery: "dosing_pump", Status: "active"}},
		DosingEvents: []DosingEvent{
			{PlanID: &manual, Status: "completed", ScheduledFor: at(48)},
			{PlanID: &manual, Status: "partial", ScheduledFor: at(36)},
			{PlanID: &manual, Status: "missed", ScheduledFor: at(24)},
			{PlanID: &manual, Status: "scheduled", ScheduledFor: at(1)},
			{PlanID: &manual, Status: "scheduled", ScheduledFor: at(-5)},     // future
			{PlanID: &manual, Status: "completed", ScheduledFor: at(24 * 9)}, // outside the window
			{PlanID: &pump, Status: "unconfirmed", ScheduledFor: at(10)},
			{Status: "completed", ExecutedAt: at(3)}, // ad hoc
		}}
	a := AdherenceFor(s, 7*day)
	if a.Due != 4 || a.Completed != 2 || a.Missed != 1 || a.Open != 1 {
		t.Fatalf("%+v", a)
	}
	eq(t, "ratio", a.Ratio, "0.6667")
	if AdherenceFor(Snapshot{Now: now}, 7*day).Ratio != nil {
		t.Error("no ratio without resolved doses")
	}
}

func TestCadenceAndLivestock(t *testing.T) {
	s := Snapshot{Now: now,
		WaterChanges: []WaterChange{
			{CompletedAt: now.Add(-3 * day), VolumeL: d("10"), SystemVolumeL: d("100")},
			{CompletedAt: now.Add(-10 * day), VolumeL: d("15"), SystemVolumeL: d("100")},
			{CompletedAt: now.Add(-40 * day), VolumeL: d("50"), SystemVolumeL: d("100")},
		},
		LivestockEvents: []LivestockEvent{
			{Type: "died", QuantityDelta: -2, OccurredAt: now.Add(-day)},
			{Type: "missing", QuantityDelta: -1, OccurredAt: now.Add(-2 * day)},
			{Type: "sold", QuantityDelta: -3, OccurredAt: now.Add(-2 * day)},
			{Type: "reconciliation", QuantityDelta: -5, OccurredAt: now.Add(-day)},
			{Type: "reconciliation", QuantityDelta: 2, OccurredAt: now.Add(-day)},
			{Type: "added", QuantityDelta: 1, OccurredAt: now.Add(-3 * day)},
			{Type: "propagated", QuantityDelta: 4, OccurredAt: now.Add(-3 * day)},
			{Type: "died", QuantityDelta: -1, OccurredAt: now.Add(-20 * day)},
		}}
	c := CadenceFor(s)
	if c.Count30 != 2 || !c.LastAt.Equal(now.Add(-3*day)) {
		t.Fatalf("%+v", c)
	}
	eq(t, "since last", c.DaysSinceLast, "3")
	eq(t, "replaced", c.ReplacedFraction30, "0.25")

	l := LivestockFor(s, 7*day)
	if l.Deaths != 1 || l.DeathCount != 2 || l.Missing != 1 || l.CountCorrections != 5 || l.Arrivals != 1 {
		t.Fatalf("%+v", l)
	}
}
