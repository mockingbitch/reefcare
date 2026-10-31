package intelligence

import (
	"slices"

	"github.com/google/uuid"
)

// finish is pipeline step 8 (docs/knowledge-engine.md §3): every evidence reference must
// name a fact of this tank's snapshot; observations without evidence and hypotheses without
// supporting evidence are dropped; hypotheses from community or unverified knowledge become
// labelled alternatives; safety warnings are never removed. Findings are ordered by
// severity, then rule order.
func finish(s Snapshot, fs []Finding) Result {
	known := evidenceSet(s)
	valid := func(refs []EvidenceRef) []EvidenceRef {
		out := []EvidenceRef{}
		for _, r := range refs {
			if known[r] {
				out = append(out, r)
			}
		}
		return out
	}
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		obs := []Observation{}
		for _, o := range f.Observations {
			if o.Evidence = valid(o.Evidence); len(o.Evidence) > 0 {
				obs = append(obs, o)
			}
		}
		f.Observations = obs
		hyp, alt := []Hypothesis{}, []Hypothesis{}
		for _, h := range append(f.Hypotheses, f.Alternatives...) {
			h.Supporting, h.Contradicting = valid(h.Supporting), valid(h.Contradicting)
			if h.Missing == nil {
				h.Missing = []MissingItem{}
			}
			if len(h.Supporting) == 0 {
				continue // a hypothesis needs at least one supporting fact
			}
			// Allow-list: only rule-derived links ("") and established/expert knowledge are
			// hypotheses; anything else is a labelled alternative.
			switch h.ClaimClass {
			case "", "established", "expert":
				hyp = append(hyp, h)
			default:
				alt = append(alt, h)
			}
		}
		f.Hypotheses, f.Alternatives = hyp, alt
		// A finding that lost every fact it was about (but asked for nothing) says nothing.
		if len(f.Observations) == 0 && len(f.Missing) == 0 && len(f.Verifications) == 0 && len(f.SafetyWarnings) == 0 {
			continue
		}
		for _, l := range []*[]MissingItem{&f.Missing} {
			if *l == nil {
				*l = []MissingItem{}
			}
		}
		if f.Verifications == nil {
			f.Verifications = []Verification{}
		}
		if f.SafetyWarnings == nil {
			f.SafetyWarnings = []SafetyWarning{}
		}
		if f.KnowledgeRefs == nil {
			f.KnowledgeRefs = []KnowledgeRef{}
		}
		out = append(out, f)
	}
	order := map[string]int{}
	for i, id := range RuleIDs() {
		order[id] = i
	}
	rank := map[string]int{SeverityUrgent: 0, SeverityWarning: 1, SeverityNotice: 2, SeverityInfo: 3}
	slices.SortStableFunc(out, func(a, b Finding) int {
		if d := rank[a.Severity] - rank[b.Severity]; d != 0 {
			return d
		}
		return order[a.RuleID] - order[b.RuleID]
	})
	return Result{Findings: out, RuleVersion: RuleVersion, Provisional: Provisional, Confidence: overallConfidence(out)}
}

// overallConfidence is the lowest confidence among assessed findings.
func overallConfidence(fs []Finding) string {
	level := map[string]int{ConfidenceLow: 0, ConfidenceModerate: 1, ConfidenceHigh: 2}
	best := ConfidenceNotAssessed
	for _, f := range fs {
		l, ok := level[f.Confidence]
		if !ok {
			continue
		}
		if best == ConfidenceNotAssessed || l < level[best] {
			best = f.Confidence
		}
	}
	return best
}

func evidenceSet(s Snapshot) map[EvidenceRef]bool {
	m := map[EvidenceRef]bool{}
	add := func(kind string, id uuid.UUID) { m[EvidenceRef{Kind: kind, ID: id}] = true }
	for _, r := range s.Readings {
		add("reading", r.ID)
	}
	for _, e := range s.DosingEvents {
		add("dosing_event", e.ID)
	}
	for _, w := range s.WaterChanges {
		add("water_change", w.ID)
	}
	for _, e := range s.EquipmentChanges {
		add("equipment_change", e.ID)
	}
	for _, e := range s.LivestockEvents {
		add("livestock_event", e.ID)
	}
	return m
}
