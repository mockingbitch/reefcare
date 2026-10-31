package intelligence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/platform/clock"
)

type fakeGen struct {
	out   NarrativeOutput
	err   error
	calls int
	slow  bool
	seen  NarrativeInput
}

func (f *fakeGen) Name() string { return "fake" }

func (f *fakeGen) Narrate(ctx context.Context, in NarrativeInput) (NarrativeOutput, error) {
	f.calls++
	f.seen = in
	if f.slow {
		<-ctx.Done()
		return NarrativeOutput{}, ctx.Err()
	}
	return f.out, f.err
}

func sampleResult(t *testing.T) Result {
	t.Helper()
	res := Evaluate(loadFixture(t, "out_of_target_confirmed"))
	res.Findings[0].KnowledgeRefs = []KnowledgeRef{{Slug: "nitrate-basics", VersionID: uuid.New(), Title: "Nitrate"}}
	return res
}

func TestValidateNarrative(t *testing.T) {
	res := sampleResult(t)
	f := res.Findings[0]
	cited := f.Observations[0].Evidence[0]
	ref := f.KnowledgeRefs[0].VersionID
	n := ValidateNarrative("fake", NarrativeOutput{
		Summary: "  Nitrate is above your target.  ",
		Explanations: []Explanation{
			{FindingIndex: 0, Text: "Two kits agree nitrate is high."},
			{FindingIndex: 7, Text: "No such finding."},
			{FindingIndex: 0, Text: "bad\u202etext"},
		},
		Hypotheses: []ProposedHypothesis{
			{FindingIndex: 0, Text: "Feeding may have increased.", Supporting: []EvidenceRef{cited}, KnowledgeVersionIDs: []uuid.UUID{ref}},
			{FindingIndex: 0, Text: "Invented evidence.", Supporting: []EvidenceRef{{Kind: "reading", ID: uuid.New()}}},
			{FindingIndex: 0, Text: "Invented knowledge.", Supporting: []EvidenceRef{cited}, KnowledgeVersionIDs: []uuid.UUID{uuid.New()}},
			{FindingIndex: 0, Text: "No evidence at all."},
		},
	}, res)
	if n == nil || n.Summary != "Nitrate is above your target." || len(n.Explanations) != 1 || len(n.Hypotheses) != 1 || n.Provider != "fake" {
		t.Fatalf("%+v", n)
	}
	if ValidateNarrative("fake", NarrativeOutput{Explanations: []Explanation{{FindingIndex: 3, Text: "x"}}}, res) != nil {
		t.Fatal("nothing valid → nil")
	}
}

func TestGuardedNarrator(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.NewFixed(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC))
	res := sampleResult(t)

	ok := &fakeGen{out: NarrativeOutput{Summary: "Fine."}}
	g := newGuardedNarrator(ok, clk, log)
	if n := g.narrate(context.Background(), res, "vi"); n == nil || n.Summary != "Fine." {
		t.Fatalf("%+v", n)
	}
	if ok.seen.Locale != "vi" || len(ok.seen.Findings) != 1 || len(ok.seen.Knowledge) != 1 {
		t.Fatalf("input: %+v", ok.seen)
	}
	if g.narrate(context.Background(), Result{}, "vi") != nil || ok.calls != 1 {
		t.Fatal("no findings: no call")
	}

	// Failures: nil each time; after 5 the breaker stops calling for the cooldown.
	bad := &fakeGen{err: errors.New("provider says: secret details")}
	g = newGuardedNarrator(bad, clk, log)
	for range 7 {
		if g.narrate(context.Background(), res, "vi") != nil {
			t.Fatal("failure must give nil")
		}
	}
	if bad.calls != 5 {
		t.Fatalf("calls while open = %d", bad.calls)
	}
	clk.Advance(6 * time.Minute)
	g.narrate(context.Background(), res, "vi")
	if bad.calls != 6 {
		t.Fatalf("breaker did not close: %d", bad.calls)
	}

	// A hung provider is cut off by the timeout.
	slow := &fakeGen{slow: true}
	g = newGuardedNarrator(slow, clk, log)
	g.timeout = 20 * time.Millisecond
	start := time.Now()
	if g.narrate(context.Background(), res, "vi") != nil || time.Since(start) > time.Second {
		t.Fatal("timeout")
	}
}
