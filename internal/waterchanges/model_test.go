package waterchanges

import (
	"testing"
	"time"

	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/recurrence"
)

func TestOverdue(t *testing.T) {
	at := time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)
	cases := []struct {
		status string
		now    time.Time
		want   bool
	}{
		{EventScheduled, at.Add(23 * time.Hour), false}, // same-day completion is on time
		{EventScheduled, at.Add(24 * time.Hour), false}, // grace is inclusive
		{EventScheduled, at.Add(24*time.Hour + time.Second), true},
		{EventScheduled, at.Add(-time.Hour), false},
		{EventCompleted, at.Add(48 * time.Hour), false},
		{EventSkipped, at.Add(48 * time.Hour), false},
	}
	after := at.Add(OverdueGrace)
	for _, c := range cases {
		if got := overdue(c.status, &after, c.now); got != c.want {
			t.Errorf("%s at +%v: %v, want %v", c.status, c.now.Sub(at), got, c.want)
		}
	}
	if overdue(EventScheduled, nil, at) {
		t.Error("no scheduled time: never overdue")
	}
}

func TestEstimate(t *testing.T) {
	v := decimal.MustParse("10")
	e := estimate(&v, decimal.MustParse("72"), BasisPlanned)
	if e == nil || e.FractionOfSystem.String() != "0.139" || len(e.Assumptions) != 3 || len(e.Flags) != 0 || e.Basis != BasisPlanned {
		t.Fatalf("estimate: %+v", e)
	}
	if estimate(nil, decimal.MustParse("72"), BasisActual) != nil || estimate(&v, decimal.MustParse("0"), BasisActual) != nil {
		t.Fatal("no estimate without both volumes")
	}
	big := decimal.MustParse("100")
	if e := estimate(&big, decimal.MustParse("72"), BasisActual); e.FractionOfSystem.String() != "1.389" || e.Flags[0] != FlagExceedsSystem {
		t.Fatalf("more than the system volume is reported as is, not clamped, and flagged: %+v", e)
	}
	half := decimal.MustParse("40")
	if e := estimate(&half, decimal.MustParse("72"), BasisActual); len(e.Flags) != 1 || e.Flags[0] != FlagLargeChange {
		t.Fatalf("large change flag: %+v", e)
	}
}

func TestGrace(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"FREQ=WEEKLY":               24 * time.Hour,
		"FREQ=DAILY":                12 * time.Hour,
		"FREQ=DAILY;BYHOUR=8,20":    6 * time.Hour,
		"FREQ=MONTHLY;BYMONTHDAY=1": 24 * time.Hour,
	} {
		r, err := recurrence.Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := grace(r); got != want {
			t.Errorf("%s: grace %v, want %v", in, got, want)
		}
	}
}
