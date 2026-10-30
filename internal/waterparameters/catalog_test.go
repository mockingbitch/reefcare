package waterparameters

import (
	"testing"

	"reefcare/internal/platform/decimal"
)

func convert(t *testing.T, param, unit, value string) string {
	t.Helper()
	p, ok := Lookup(param)
	if !ok {
		t.Fatalf("unknown parameter %s", param)
	}
	u, ok := p.Unit(unit)
	if !ok {
		t.Fatalf("%s: unknown unit %s", param, unit)
	}
	got, err := p.ToCanonical(u, decimal.MustParse(value))
	if err != nil {
		t.Fatalf("%s %s %s: %v", param, value, unit, err)
	}
	return got.String()
}

func TestConversions(t *testing.T) {
	tests := []struct{ param, unit, in, want string }{
		{"alkalinity", "dKH", "8.3", "8.3"},
		{"alkalinity", "meq/L", "3", "8.41"},      // × 2.804
		{"alkalinity", "ppm CaCO3", "150", "8.4"}, // 150 / 17.848 = 8.4043… → 8.40
		{"temperature", "F", "77", "25"},          // (77 − 32) × 5/9
		{"temperature", "F", "78.8", "26"},        //
		{"nitrate", "mg/L NO3-N", "1", "4.43"},    // × 4.427 → 2 places
		{"phosphate", "mg/L PO4-P", "0.01", "0.0307"},
		{"phosphate", "ppb PO4-P", "10", "0.0307"}, // ULR phosphorus checkers
		{"phosphate", "ppb PO4", "30", "0.03"},
		{"ammonia", "mg/L NH3-N", "0.25", "0.3"},
		{"ammonia", "mg/L NH4+", "1", "0.94"},
		{"nitrite", "ppb NO2-N", "10", "0.033"},
		{"nitrite", "mg/L NO2-N", "0.1", "0.329"},
		{"calcium", "ppm", "420", "420"},
		{"salinity", "PSU", "35", "35"},
	}
	for _, tt := range tests {
		if got := convert(t, tt.param, tt.unit, tt.in); got != tt.want {
			t.Errorf("%s %s %s → %s, want %s", tt.param, tt.in, tt.unit, got, tt.want)
		}
	}
}

func TestSpecificGravityEstimate(t *testing.T) {
	// EOS-80 at 25 °C: 35 ppt seawater has SG(25/25) ≈ 1.0264.
	for in, want := range map[string]string{"1.0264": "35", "1.025": "33.2", "1.000": "0"} {
		if got := convert(t, "salinity", "SG", in); got != want {
			t.Errorf("SG %s → %s ppt, want %s", in, got, want)
		}
	}
	p, _ := Lookup("salinity")
	u, _ := p.Unit("SG")
	if !u.Estimate {
		t.Error("SG conversions must be flagged as estimates")
	}
	if _, err := p.ToCanonical(u, decimal.MustParse("1.2")); err == nil {
		t.Error("SG beyond the seawater range must be rejected")
	}
}

func TestCatalogIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Catalog {
		if seen[p.Code] {
			t.Errorf("duplicate code %s", p.Code)
		}
		seen[p.Code] = true
		if p.HardMin.Cmp(p.HardMax) >= 0 {
			t.Errorf("%s: hard_min must be below hard_max", p.Code)
		}
		if _, ok := p.Unit(p.CanonicalUnit); !ok {
			t.Errorf("%s: the canonical unit %s must be an accepted input unit", p.Code, p.CanonicalUnit)
		}
	}
}

func TestUnitMistakeWarnings(t *testing.T) {
	sal, _ := Lookup("salinity")
	if w := sal.Warnings("ppt", decimal.MustParse("1.026"), decimal.MustParse("1")); len(w) != 1 {
		t.Errorf("SG typed as ppt should warn, got %v", w)
	}
	if w := sal.Warnings("ppt", decimal.MustParse("35"), decimal.MustParse("35")); len(w) != 0 {
		t.Errorf("normal salinity must not warn, got %v", w)
	}
	po4, _ := Lookup("phosphate")
	if w := po4.Warnings("mg/L PO4", decimal.MustParse("15"), decimal.MustParse("15")); len(w) != 1 {
		t.Errorf("15 mg/L PO4 should warn, got %v", w)
	}
}
