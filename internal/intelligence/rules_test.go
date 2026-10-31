package intelligence

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current rules")

func loadFixture(t *testing.T, name string) Snapshot {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// Each fixture with the rules it must (and only may) trigger, in output order.
var expectations = map[string][]string{
	"stable":                  {},
	"stale":                   {"DQ-STALE", "NM-NEXT"},
	"alk_swing":               {"PI-ALK-SWING", "PD-OUT-OF-TARGET", "DM-ADHERENCE", "DQ-STALE", "NM-NEXT"},
	"conflict":                {"PD-OUT-OF-TARGET", "DQ-CONFLICT", "NM-NEXT"},
	"implausible_jump":        {"DQ-FAST-CHANGE", "PD-OUT-OF-TARGET", "NM-NEXT"},
	"out_of_target_confirmed": {"PD-OUT-OF-TARGET"},
	"ammonia_spike":           {"NH-AMMONIA-NITRITE"},
	"losses_after_change":     {"LC-LOSSES"},
	"trend":                   {"PI-TREND", "DQ-STALE", "NM-NEXT"},
	"fish_only_cycling":       {"DQ-STALE", "NM-NEXT"},
	"heater_failure":          {"WB-WELFARE-BOUNDS"},
	"no_targets_extremes":     {"WB-WELFARE-BOUNDS", "WB-WELFARE-BOUNDS", "WB-WELFARE-BOUNDS", "WB-WELFARE-BOUNDS", "DQ-STALE", "NM-NEXT"},
	"fish_in_cycle":           {"NH-AMMONIA-NITRITE", "DQ-STALE", "NM-NEXT"},
	"mass_death_one_event":    {"LC-LOSSES"},
	"ph_day_night":            {},
	"alk_three_hours":         {},
	"conflict_resolved":       {},
	"ammonia_at_threshold":    {"NH-AMMONIA-NITRITE"},
}

func TestEveryFixtureHasExpectations(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if _, ok := expectations[strings.TrimSuffix(filepath.Base(f), ".json")]; !ok {
			t.Errorf("%s has no expectations", f)
		}
	}
	if len(files) != len(expectations) {
		t.Errorf("%d fixtures, %d expectations", len(files), len(expectations))
	}
}

func TestRulesOnFixtures(t *testing.T) {
	for name, want := range expectations {
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, name)
			res := Evaluate(s)
			var got []string
			for _, f := range res.Findings {
				got = append(got, f.RuleID)
			}
			if (len(got) > 0 || len(want) > 0) && !slices.Equal(got, want) {
				t.Errorf("rules = %v, want %v", got, want)
			}
			checkProperties(t, s, res)

			out, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, '\n')
			golden := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, out, 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			exp, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if !bytes.Equal(exp, out) {
				t.Errorf("output differs from %s (run with -update after checking the change):\n%s", golden, out)
			}
		})
	}
}

// checkProperties holds for every assessment (docs/knowledge-engine.md §9).
func checkProperties(t *testing.T, s Snapshot, res Result) {
	t.Helper()
	known := evidenceSet(s)
	if res.RuleVersion != RuleVersion || !res.Provisional {
		t.Errorf("version %q provisional %v", res.RuleVersion, res.Provisional)
	}
	for _, f := range res.Findings {
		for _, o := range f.Observations {
			if len(o.Evidence) == 0 {
				t.Errorf("%s: observation %s without evidence", f.RuleID, o.Text.Key)
			}
			for _, e := range o.Evidence {
				if !known[e] {
					t.Errorf("%s: unknown evidence %v", f.RuleID, e)
				}
			}
		}
		for _, h := range append(f.Hypotheses, f.Alternatives...) {
			if len(h.Supporting) == 0 {
				t.Errorf("%s: hypothesis %s without support", f.RuleID, h.Key)
			}
			if strings.Contains(h.Text.Key, "cause") {
				t.Errorf("%s: causal wording %s", f.RuleID, h.Text.Key)
			}
		}
		if f.Severity == SeverityUrgent {
			urgent := false
			for _, w := range f.SafetyWarnings {
				urgent = urgent || w.Urgent
			}
			if !urgent {
				t.Errorf("%s: urgent finding without an urgent safety warning", f.RuleID)
			}
		}
		if !slices.Contains([]string{ConfidenceNotAssessed, ConfidenceLow, ConfidenceModerate, ConfidenceHigh}, f.Confidence) {
			t.Errorf("%s: confidence %q", f.RuleID, f.Confidence)
		}
	}
}

func TestValidationDropsUnsupportedClaims(t *testing.T) {
	s := loadFixture(t, "stable")
	real := EvidenceRef{Kind: "reading", ID: s.Readings[0].ID}
	fake := EvidenceRef{Kind: "reading", ID: s.TankID} // not a reading of this tank
	res := finish(s, []Finding{
		{RuleID: "X", Severity: SeverityInfo, Confidence: ConfidenceLow,
			Observations: []Observation{{Text: Text{Key: "a"}, Evidence: []EvidenceRef{fake}}, {Text: Text{Key: "b"}, Evidence: []EvidenceRef{real, fake}}},
			Hypotheses: []Hypothesis{
				{Key: "unsupported", Supporting: []EvidenceRef{fake}},
				{Key: "community", ClaimClass: "community", Supporting: []EvidenceRef{real}},
				{Key: "ok", ClaimClass: "expert", Supporting: []EvidenceRef{real}},
			}},
		{RuleID: "Y", Severity: SeverityUrgent, Confidence: ConfidenceNotAssessed,
			SafetyWarnings: []SafetyWarning{{Text: Text{Key: "keep"}, Urgent: true}}},
		{RuleID: "Z", Severity: SeverityInfo, Observations: []Observation{{Text: Text{Key: "gone"}, Evidence: []EvidenceRef{fake}}}},
	})
	if len(res.Findings) != 2 || res.Findings[0].RuleID != "Y" {
		t.Fatalf("findings: %+v", res.Findings)
	}
	x := res.Findings[1]
	if len(x.Observations) != 1 || len(x.Observations[0].Evidence) != 1 {
		t.Errorf("observations: %+v", x.Observations)
	}
	if len(x.Hypotheses) != 1 || x.Hypotheses[0].Key != "ok" || len(x.Alternatives) != 1 || x.Alternatives[0].Key != "community" {
		t.Errorf("hypotheses %+v alternatives %+v", x.Hypotheses, x.Alternatives)
	}
	if res.Confidence != ConfidenceLow {
		t.Errorf("overall confidence %q", res.Confidence)
	}
}

func TestSnapshotLimitIsReported(t *testing.T) {
	s := loadFixture(t, "stable")
	s.Truncated = []string{"readings"}
	res := Evaluate(s)
	if len(res.Findings) != 1 || res.Findings[0].RuleID != "DQ-SNAPSHOT-LIMIT" || len(res.Findings[0].Missing) != 1 {
		t.Fatalf("%+v", res.Findings)
	}
}
