package intelligence

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/textcheck"
)

// Optional LLM narrative (docs/knowledge-engine.md §8, milestone 3.6). A generator may only
// rephrase the deterministic assessment: its output is untrusted, parsed into this typed
// model and validated — it can't add evidence, cite knowledge that wasn't cited, or trigger
// anything. Deterministic assessments never depend on it: with no generator, or when it
// fails, the assessment is stored unchanged with "narrative": null.

// NarrativeGenerator turns an assessment into plain-language wording.
type NarrativeGenerator interface {
	Name() string
	Narrate(ctx context.Context, in NarrativeInput) (NarrativeOutput, error)
}

// NarrativeInput is all a generator sees: the validated findings and excerpts of the
// knowledge they cite. No tank name, user or free text from the keeper.
type NarrativeInput struct {
	Locale    string             `json:"locale"`
	Findings  []Finding          `json:"findings"`
	Knowledge []KnowledgeExcerpt `json:"knowledge"`
}

// KnowledgeExcerpt is a cited knowledge version.
type KnowledgeExcerpt struct {
	VersionID uuid.UUID `json:"version_id"`
	Title     string    `json:"title"`
}

// NarrativeOutput is what a generator returns, before validation.
type NarrativeOutput struct {
	Summary      string               `json:"summary"`
	Explanations []Explanation        `json:"explanations"`
	Hypotheses   []ProposedHypothesis `json:"hypotheses"`
}

// Explanation rephrases one finding.
type Explanation struct {
	FindingIndex int    `json:"finding_index"`
	Text         string `json:"text"`
}

// ProposedHypothesis is a generator's suggestion. It must cite facts the finding already
// cites; it is shown as unverified, never merged into the findings.
type ProposedHypothesis struct {
	FindingIndex        int           `json:"finding_index"`
	Text                string        `json:"text"`
	Supporting          []EvidenceRef `json:"supporting"`
	KnowledgeVersionIDs []uuid.UUID   `json:"knowledge_version_ids,omitempty"`
}

// Narrative is the validated wording stored with an assessment.
type Narrative struct {
	Provider     string               `json:"provider"`
	Summary      string               `json:"summary"`
	Explanations []Explanation        `json:"explanations"`
	Hypotheses   []ProposedHypothesis `json:"hypotheses"`
}

const (
	maxNarrativeSummary = 1000
	maxNarrativeText    = 600
	maxNarrativeItems   = 30
)

// narrativeInput builds the generator's input from a result.
func narrativeInput(res Result, locale string) NarrativeInput {
	in := NarrativeInput{Locale: locale, Findings: res.Findings}
	seen := map[uuid.UUID]bool{}
	for _, f := range res.Findings {
		for _, k := range f.KnowledgeRefs {
			if !seen[k.VersionID] {
				seen[k.VersionID] = true
				in.Knowledge = append(in.Knowledge, KnowledgeExcerpt{VersionID: k.VersionID, Title: k.Title})
			}
		}
	}
	return in
}

// validNarrativeText trims and checks a generated sentence; "" when unusable.
func validNarrativeText(s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if t == "" || utf8.RuneCountInString(t) > maxLen || !textcheck.MultiLine(t) {
		return ""
	}
	return t
}

// ValidateNarrative keeps only what the deterministic result backs (pipeline step 8 for LLM
// output): explanations of existing findings, and hypotheses citing facts that finding
// already cites and knowledge it already references. Returns nil when nothing survives.
func ValidateNarrative(provider string, out NarrativeOutput, res Result) *Narrative {
	n := &Narrative{Provider: provider, Summary: validNarrativeText(out.Summary, maxNarrativeSummary), Explanations: []Explanation{}, Hypotheses: []ProposedHypothesis{}}
	inRange := func(i int) bool { return i >= 0 && i < len(res.Findings) }
	for _, e := range out.Explanations {
		if len(n.Explanations) == maxNarrativeItems {
			break
		}
		if t := validNarrativeText(e.Text, maxNarrativeText); t != "" && inRange(e.FindingIndex) {
			n.Explanations = append(n.Explanations, Explanation{FindingIndex: e.FindingIndex, Text: t})
		}
	}
	for _, h := range out.Hypotheses {
		if len(n.Hypotheses) == maxNarrativeItems || !inRange(h.FindingIndex) {
			continue
		}
		f := res.Findings[h.FindingIndex]
		cited := map[EvidenceRef]bool{}
		for _, o := range f.Observations {
			for _, e := range o.Evidence {
				cited[e] = true
			}
		}
		for _, x := range append(f.Hypotheses, f.Alternatives...) {
			for _, e := range append(x.Supporting, x.Contradicting...) {
				cited[e] = true
			}
		}
		refs := map[uuid.UUID]bool{}
		for _, k := range f.KnowledgeRefs {
			refs[k.VersionID] = true
		}
		t := validNarrativeText(h.Text, maxNarrativeText)
		ok := t != "" && len(h.Supporting) > 0
		for _, e := range h.Supporting {
			ok = ok && cited[e]
		}
		for _, k := range h.KnowledgeVersionIDs {
			ok = ok && refs[k]
		}
		if ok {
			n.Hypotheses = append(n.Hypotheses, ProposedHypothesis{FindingIndex: h.FindingIndex, Text: t, Supporting: h.Supporting, KnowledgeVersionIDs: h.KnowledgeVersionIDs})
		}
	}
	if n.Summary == "" && len(n.Explanations) == 0 && len(n.Hypotheses) == 0 {
		return nil
	}
	return n
}

// guardedNarrator bounds a generator: a timeout per call, no retries, and a circuit breaker
// that stops calling it for a while after consecutive failures.
type guardedNarrator struct {
	gen       NarrativeGenerator
	clock     clock.Clock
	log       *slog.Logger
	timeout   time.Duration
	threshold int
	cooldown  time.Duration

	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

func newGuardedNarrator(gen NarrativeGenerator, clk clock.Clock, log *slog.Logger) *guardedNarrator {
	return &guardedNarrator{gen: gen, clock: clk, log: log, timeout: 10 * time.Second, threshold: 5, cooldown: 5 * time.Minute}
}

// narrate returns validated wording, or nil (no generator output is ever required).
func (g *guardedNarrator) narrate(ctx context.Context, res Result, locale string) *Narrative {
	if len(res.Findings) == 0 {
		return nil
	}
	g.mu.Lock()
	open := g.clock.Now().Before(g.openUntil)
	g.mu.Unlock()
	if open {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	out, err := g.gen.Narrate(cctx, narrativeInput(res, locale))
	g.mu.Lock()
	defer g.mu.Unlock()
	if err != nil {
		g.failures++
		if g.failures >= g.threshold {
			g.openUntil = g.clock.Now().Add(g.cooldown)
			g.failures = 0
		}
		// The error text may echo provider responses: log the provider and kind only.
		g.log.Warn("narrative generator failed; assessment stored without it", "provider", g.gen.Name(), "timeout", cctx.Err() != nil)
		return nil
	}
	g.failures = 0
	return ValidateNarrative(g.gen.Name(), out, res)
}
