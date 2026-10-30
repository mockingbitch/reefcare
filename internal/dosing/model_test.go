package dosing

import (
	"testing"
	"time"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/recurrence"
)

func TestGrace(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"FREQ=DAILY":                       MissedGrace,
		"FREQ=DAILY;BYHOUR=8,20":           6 * time.Hour,
		"FREQ=DAILY;BYHOUR=0,4,8,12,16,20": 2 * time.Hour,
		"FREQ=WEEKLY;BYDAY=MO,WE,FR":       MissedGrace,
	} {
		r, err := recurrence.Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := grace(r); got != want {
			t.Errorf("%s: %v, want %v", in, got, want)
		}
	}
}

func TestUnitsByForm(t *testing.T) {
	cases := []struct {
		unit, form string
		ok         bool
	}{
		{"ml", "liquid", true}, {"drops", "liquid", true}, {"g", "liquid", false},
		{"g", "powder", true}, {"ml", "powder", true}, {"drops", "powder", false}, {"tablets", "tablet", true}, {"ml", "tablet", false},
		{"mg", "other", true},
	}
	for _, c := range cases {
		v := apperr.NewValidator()
		checker{v}.unitForForm("u", c.unit, c.form)
		if v.HasErrors() == c.ok {
			t.Errorf("%s for %s: ok=%v", c.unit, c.form, !v.HasErrors())
		}
	}
}

func TestAmountIsExact(t *testing.T) {
	v := apperr.NewValidator()
	c := checker{v}
	if d := c.amount("a", "0.0005"); v.HasErrors() || d.String() != "0.0005" {
		t.Fatalf("4 decimals must be kept exactly: %v %v", d, v.Err())
	}
	c.amount("a", "0.00005")
	if !v.HasErrors() {
		t.Fatal("a fifth decimal is rejected, never rounded")
	}
}

func TestDeviation(t *testing.T) {
	p, more, less := mustD("5"), mustD("7"), mustD("4")
	if deviation(&p, &more) != "over" || deviation(&p, &less) != "under" || deviation(&p, &p) != "" || deviation(nil, &p) != "" {
		t.Fatal("deviation")
	}
}

func mustD(s string) decimal.Decimal { return decimal.MustParse(s) }
