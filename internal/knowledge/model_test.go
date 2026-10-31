package knowledge

import (
	"encoding/json"
	"strings"
	"testing"

	"reefcare/internal/platform/apperr"
)

func codes(v *apperr.Validator) map[string]string {
	out := map[string]string{}
	if ae, ok := apperr.As(v.Err()); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Code
		}
	}
	return out
}

func TestSlugAndTags(t *testing.T) {
	for slug, ok := range map[string]bool{"alk": true, "alkalinity-stability-2": true, "Alk": false, "a--b": false, "-a": false, "a_b": false, strings.Repeat("a", 81): false} {
		v := apperr.NewValidator()
		checker{v}.slug(slug)
		if v.HasErrors() == ok {
			t.Errorf("slug %q ok=%v", slug, !v.HasErrors())
		}
	}
	v := apperr.NewValidator()
	c := checker{v}
	if got := c.parameters([]string{"alkalinity", "calcium", "alkalinity"}); len(got) != 2 {
		t.Errorf("parameters dedupe: %v", got)
	}
	c.parameters([]string{"kh"})
	c.symptoms([]string{"Tissue Recession"})
	if cs := codes(v); cs["parameters.0"] != "unknown_parameter" || cs["symptoms.0"] != "invalid_tag" {
		t.Errorf("%v", cs)
	}
}

func TestStructured(t *testing.T) {
	v := apperr.NewValidator()
	s, raw := checker{v}.structured(Structured{
		Verifications:   []Item{{Key: "retest", Text: " Re-test "}},
		Recommendations: []Recommendation{{Key: "steady", Text: "Keep dosing steady.", ClaimClass: "expert"}},
	})
	if v.HasErrors() || s.Verifications[0].Text != "Re-test" || s.SafetyWarnings == nil {
		t.Fatalf("%v %+v", codes(v), s)
	}
	var back Structured
	if err := json.Unmarshal(raw, &back); err != nil || len(back.Recommendations) != 1 || !strings.Contains(string(raw), `"safety_warnings":[]`) {
		t.Fatalf("canonical JSON: %s", raw)
	}

	v = apperr.NewValidator()
	many := make([]Item, 21)
	checker{v}.structured(Structured{
		Verifications:     []Item{{Key: "dup", Text: "a"}, {Key: "dup", Text: "b"}, {Key: "x", Text: "line\nbreak"}},
		Recommendations:   []Recommendation{{Key: "r", Text: "x", ClaimClass: "hearsay"}},
		Contraindications: many,
	})
	cs := codes(v)
	if cs["structured.verifications.1.key"] != "duplicate" || cs["structured.verifications.2.key"] != "invalid_key" ||
		cs["structured.verifications.2.text"] != "invalid_characters" || cs["structured.recommendations.0.claim_class"] != "unsupported" ||
		cs["structured.contraindications"] != "too_many" {
		t.Fatalf("%v", cs)
	}
}

func TestLikePattern(t *testing.T) {
	if got := likePattern(" 50%_KH "); got != `%50\%\_kh%` {
		t.Fatal(got)
	}
}
