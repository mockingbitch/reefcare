package intelligence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/knowledge"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/ratelimit"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
	"reefcare/internal/waterparameters"
)

// HistoryWindow is how much history an assessment looks at.
const HistoryWindow = 90 * day

// Source fills one module's part of a snapshot from its own tables (dosing, water changes,
// equipment, livestock register one each). The caller has authorized the tank.
type Source func(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since time.Time, s *Snapshot) error

// Service runs the assessment pipeline (docs/knowledge-engine.md §3): authorize, load the
// snapshot, evaluate the rules, resolve knowledge, validate and persist.
type Service struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	authz   tanks.Authorizer
	log     *slog.Logger
	sources []Source
	manual  *ratelimit.Limiter
}

// NewService wires the service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, log: log, manual: ratelimit.New(20, time.Hour, clk)}
}

// AddSource registers a snapshot source. Call during wiring.
func (s *Service) AddSource(src Source) { s.sources = append(s.sources, src) }

// RegisterJobs registers the after-test assessment.
func (s *Service) RegisterJobs(r *jobs.Runner) {
	r.Handle(waterparameters.JobAssessAfterTest, s.runAfterTest)
}

var (
	errNotFound = apperr.NotFound()
	errLimited  = func(retry time.Duration) error { return apperr.RateLimited(retry) }
)

// Assessment is a stored assessment.
type Assessment struct {
	ID               uuid.UUID
	TankID           uuid.UUID
	Trigger          string
	SourceID         *uuid.UUID
	Status           string
	FindingCount     int32
	MaxSeverity      *string
	Confidence       string
	SnapshotVersion  int32
	RuleVersion      string
	KnowledgeVersion string
	Provisional      bool
	Locale           string
	RetractedAt      *time.Time
	RetractReason    *string
	CreatedAt        time.Time
	Result           *Result   // nil in lists
	MyFeedback       *Feedback // the caller's feedback (single assessments only)
}

// Limits on automatic assessments (one per tank per window, assessing the latest state; at
// most maxAutomaticPerDay per tank) and on the stored snapshot.
const (
	maxAutomaticPerDay = 24
	maxSnapshotBytes   = 3_500_000
)

var errSnapshotTooLarge = apperr.Conflict("snapshot_too_large", "This tank has too much recent data to assess at once.")

// Assess runs a manual assessment of the tank (owner; rate limited) and returns it.
func (s *Service) Assess(ctx context.Context, a tanks.Actor, tankID uuid.UUID) (Assessment, error) {
	if ok, retry := s.manual.Allow(a.UserID.String()); !ok {
		return Assessment{}, errLimited(retry)
	}
	// Authorize before loading anything (owner, tank not archived).
	if err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := s.authz.Require(ctx, dbq.New(tx), a.UserID, tankID, tanks.PermWrite)
		return err
	}); err != nil {
		return Assessment{}, err
	}
	id, err := s.run(ctx, tankID, "manual", nil, &a.UserID, func(q *dbq.Queries) (bool, error) {
		_, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite) // still the owner, still open
		return err == nil, err
	})
	if err != nil {
		return Assessment{}, err
	}
	return s.load(ctx, dbq.New(s.pool), *id, true)
}

// runAfterTest assesses a tank after new water tests (docs/knowledge-engine.md §7). Water tests
// enqueue it once per tank and time bucket, to run at the end of the bucket, so a burst of
// tests gives one assessment of the latest state.
func (s *Service) runAfterTest(ctx context.Context, j jobs.Job) error {
	var p waterparameters.AssessAfterTest
	if err := j.Decode(&p); err != nil {
		return err
	}
	now := s.clock.Now()
	var skip bool
	if err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tank, err := q.GetTankForAssessment(ctx, p.TankID)
		if database.IsNoRows(err) || (err == nil && tank.Status == tanks.StatusArchived) {
			skip = true // erased or archived since
			return nil
		}
		if err != nil {
			return err
		}
		n, err := q.CountRecentAutomaticAssessments(ctx, dbq.CountRecentAutomaticAssessmentsParams{TankID: p.TankID, Since: now.Add(-24 * time.Hour)})
		skip = n >= maxAutomaticPerDay
		return err
	}); err != nil || skip {
		return err
	}
	_, err := s.run(ctx, p.TankID, "water_test", &p.WaterTestID, &p.UserID, func(q *dbq.Queries) (bool, error) {
		status, err := q.LockTankForAssessment(ctx, p.TankID) // an archive or erasure waits for us, or is seen
		if database.IsNoRows(err) || (err == nil && status == tanks.StatusArchived) {
			return false, nil
		}
		return err == nil, err
	})
	if errors.Is(err, errSnapshotTooLarge) {
		return jobs.Permanent(err)
	}
	return err
}

// run is the pipeline: load a consistent snapshot (read-only, repeatable read), evaluate it,
// then — in a short transaction that re-checks the tank with `allow` — resolve knowledge and
// persist. It returns the new assessment's id, or nil when nothing was stored (the test was
// already assessed, or allow declined).
func (s *Service) run(ctx context.Context, tankID uuid.UUID, trigger string, sourceID, requester *uuid.UUID, allow func(*dbq.Queries) (bool, error)) (*uuid.UUID, error) {
	now := s.clock.Now()
	var snap Snapshot
	if err := database.InReadSnapshotTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		snap, err = s.Snapshot(ctx, dbq.New(tx), tankID, now)
		return err
	}); err != nil {
		return nil, err
	}
	res := Evaluate(snap)
	snapshot, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	if len(snapshot) > maxSnapshotBytes {
		return nil, errSnapshotTooLarge
	}
	var id *uuid.UUID
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if ok, err := allow(q); err != nil || !ok {
			return err
		}
		var err error
		id, err = s.persist(ctx, q, tankID, trigger, sourceID, requester, now, res, snapshot)
		return err
	})
	return id, err
}

// persist stores an evaluated assessment with its evidence, knowledge references and timeline
// event (one transaction).
func (s *Service) persist(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, trigger string, sourceID, requester *uuid.UUID, now time.Time, res Result, snapshot []byte) (*uuid.UUID, error) {
	locale := "vi"
	if requester != nil {
		if l, err := q.GetUserLocale(ctx, *requester); err == nil {
			locale = l
		} else if !database.IsNoRows(err) {
			return nil, err
		}
	}
	knowledgeVersion, err := attachKnowledge(ctx, q, &res, locale)
	if err != nil {
		return nil, err
	}
	result, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("encode assessment: %w", err)
	}
	maxSev := maxSeverity(res.Findings)
	id, err := q.CreateAssessment(ctx, dbq.CreateAssessmentParams{
		TankID: tankID, Trigger: trigger, SourceID: sourceID, Result: result, FindingCount: int32(len(res.Findings)), //nolint:gosec // bounded by the rule count
		MaxSeverity: maxSev, ConfidenceLabel: res.Confidence, InputSnapshot: snapshot, SnapshotVersion: SnapshotVersion,
		RuleVersion: res.RuleVersion, KnowledgeVersion: knowledgeVersion, Provisional: res.Provisional, Locale: locale,
		RequestedBy: requester, Now: now,
	})
	if database.IsNoRows(err) {
		return nil, nil // this water test was already assessed
	}
	if err != nil {
		return nil, err
	}
	kinds, refs := evidenceRows(res)
	if len(kinds) > 0 {
		if err := q.AddAssessmentEvidence(ctx, dbq.AddAssessmentEvidenceParams{AssessmentID: id, Kinds: kinds, RefIds: refs}); err != nil {
			return nil, err
		}
	}
	for _, f := range res.Findings {
		for _, k := range f.KnowledgeRefs {
			if err := q.AddAssessmentKnowledgeRef(ctx, dbq.AddAssessmentKnowledgeRefParams{AssessmentID: id, VersionID: k.VersionID}); err != nil {
				return nil, err
			}
		}
	}
	summary := map[string]any{"findings": len(res.Findings), "confidence": res.Confidence, "trigger": trigger}
	if maxSev != nil {
		summary["max_severity"] = *maxSev
	}
	if _, err := timeline.Record(ctx, q, now, timeline.Event{
		TankID: tankID, OccurredAt: now, Category: timeline.CategoryAssessment, EventType: "assessment.created",
		SourceType: "assessment", SourceID: id, ActorID: requester, Summary: summary,
	}); err != nil {
		return nil, err
	}
	return &id, nil
}

// Snapshot loads the tank's last HistoryWindow of data. Call only after authorization.
func (s *Service) Snapshot(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, now time.Time) (Snapshot, error) {
	tank, err := q.GetTankForAssessment(ctx, tankID)
	if database.IsNoRows(err) {
		return Snapshot{}, errNotFound
	}
	if err != nil {
		return Snapshot{}, err
	}
	since := now.Add(-HistoryWindow)
	snap := Snapshot{TankID: tankID, TankType: tank.TankType, TankStatus: tank.Status, SetupDate: tank.SetupDate, Now: now}
	rs, ts, err := waterparameters.ForAssessment(ctx, q, tankID, since, now, LimitReadings)
	if err != nil {
		return Snapshot{}, err
	}
	snap.MarkTruncated("readings", len(rs), LimitReadings)
	for _, r := range rs {
		snap.Readings = append(snap.Readings, Reading{ID: r.ID, Parameter: r.Parameter, MeasuredAt: r.MeasuredAt, Value: r.Value,
			Estimate: r.Estimate, Method: r.Method, TestKit: r.TestKit, Unit: canonicalUnit(r.Parameter)})
	}
	for _, t := range ts {
		snap.Targets = append(snap.Targets, Target{Parameter: t.Parameter, Min: t.Min, Max: t.Max, FreshnessHours: t.FreshnessHours, ValidFrom: t.ValidFrom, ValidTo: t.ValidTo})
	}
	for _, src := range s.sources {
		if err := src(ctx, q, tankID, since, &snap); err != nil {
			return Snapshot{}, err
		}
	}
	return snap, nil
}

func canonicalUnit(param string) string {
	if p, ok := waterparameters.Lookup(param); ok {
		return p.CanonicalUnit
	}
	return ""
}

// attachKnowledge resolves each finding's knowledge slugs to published versions (missing ones
// are dropped, never invented) and copies the articles' safety warnings into the finding.
// It returns the knowledge version: a hash of the cited version ids.
func attachKnowledge(ctx context.Context, q *dbq.Queries, res *Result, locale string) (string, error) {
	var slugs []string
	for _, f := range res.Findings {
		slugs = append(slugs, f.KnowledgeSlugs...)
	}
	cited, err := knowledge.PublishedBySlug(ctx, q, slugs, locale)
	if err != nil {
		return "", err
	}
	var ids []string
	for i := range res.Findings {
		f := &res.Findings[i]
		for _, slug := range f.KnowledgeSlugs {
			c, ok := cited[slug]
			if !ok {
				continue
			}
			f.KnowledgeRefs = append(f.KnowledgeRefs, KnowledgeRef{Slug: c.Slug, VersionID: c.VersionID, Title: c.Title, Locale: c.Locale})
			for _, w := range c.SafetyWarnings {
				f.SafetyWarnings = append(f.SafetyWarnings, SafetyWarning{Text: Text{Key: "knowledge.safety_warning", Params: map[string]string{"text": w.Text, "slug": c.Slug}}})
			}
			ids = append(ids, c.VersionID.String())
		}
	}
	if len(ids) == 0 {
		return "none", nil
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	sum := sha256.New()
	for _, id := range ids {
		sum.Write([]byte(id))
	}
	return "k1:" + hex.EncodeToString(sum.Sum(nil))[:32], nil
}

func maxSeverity(fs []Finding) *string {
	rank := map[string]int{SeverityInfo: 0, SeverityNotice: 1, SeverityWarning: 2, SeverityUrgent: 3}
	var out *string
	for _, f := range fs {
		if out == nil || rank[f.Severity] > rank[*out] {
			sev := f.Severity
			out = &sev
		}
	}
	return out
}

func evidenceRows(res Result) ([]string, []uuid.UUID) {
	seen := map[EvidenceRef]bool{}
	var kinds []string
	var ids []uuid.UUID
	add := func(refs []EvidenceRef) {
		for _, r := range refs {
			if !seen[r] {
				seen[r] = true
				kinds, ids = append(kinds, r.Kind), append(ids, r.ID)
			}
		}
	}
	for _, f := range res.Findings {
		for _, o := range f.Observations {
			add(o.Evidence)
		}
		for _, h := range append(f.Hypotheses, f.Alternatives...) {
			add(h.Supporting)
			add(h.Contradicting)
		}
	}
	return kinds, ids
}

// --- reading ---

// Cursor is the keyset of the assessment list.
type Cursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// List returns the tank's assessments, newest first, without results (owner or viewer).
func (s *Service) List(ctx context.Context, a tanks.Actor, tankID uuid.UUID, limit int32, cur *Cursor) ([]Assessment, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	p := dbq.ListAssessmentsParams{TankID: tankID, CursorAt: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), CursorID: uuid.Max, RowLimit: limit}
	if cur != nil {
		p.CursorAt, p.CursorID = cur.At, cur.ID
	}
	rows, err := q.ListAssessments(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]Assessment, len(rows))
	for i, r := range rows {
		out[i] = Assessment{ID: r.ID, TankID: r.TankID, Trigger: r.Trigger, SourceID: r.SourceID, Status: r.Status,
			FindingCount: r.FindingCount, MaxSeverity: r.MaxSeverity, Confidence: r.ConfidenceLabel, RuleVersion: r.RuleVersion,
			Provisional: r.Provisional, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// Get returns one assessment with its result (anyone who may read its tank).
func (s *Service) Get(ctx context.Context, a tanks.Actor, id uuid.UUID) (Assessment, error) {
	q := dbq.New(s.pool)
	tankID, err := q.GetAssessmentTank(ctx, id)
	if database.IsNoRows(err) {
		return Assessment{}, errNotFound
	}
	if err != nil {
		return Assessment{}, err
	}
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return Assessment{}, err
	}
	out, err := s.load(ctx, q, id, true)
	if err != nil {
		return Assessment{}, err
	}
	out.MyFeedback, err = s.myFeedback(ctx, q, a.UserID, id)
	return out, err
}

func (s *Service) load(ctx context.Context, q *dbq.Queries, id uuid.UUID, withResult bool) (Assessment, error) {
	r, err := q.GetAssessment(ctx, id)
	if database.IsNoRows(err) {
		return Assessment{}, errNotFound
	}
	if err != nil {
		return Assessment{}, err
	}
	out := Assessment{ID: r.ID, TankID: r.TankID, Trigger: r.Trigger, SourceID: r.SourceID, Status: r.Status,
		FindingCount: r.FindingCount, MaxSeverity: r.MaxSeverity, Confidence: r.ConfidenceLabel, SnapshotVersion: r.SnapshotVersion,
		RuleVersion: r.RuleVersion, KnowledgeVersion: r.KnowledgeVersion, Provisional: r.Provisional, Locale: r.Locale,
		RetractedAt: r.RetractedAt, RetractReason: r.RetractReason, CreatedAt: r.CreatedAt}
	if withResult {
		var res Result
		if err := json.Unmarshal(r.Result, &res); err != nil {
			return Assessment{}, fmt.Errorf("decode assessment result: %w", err)
		}
		out.Result = &res
	}
	return out, nil
}

// EraseUserData deletes the assessments of the erased account's tanks (identity.Eraser); it
// must run before tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.DeleteUserAssessmentFeedback(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerAssessmentFeedback(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerAssessmentEvidence(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerAssessmentRefs(ctx, userID); err != nil {
		return err
	}
	return q.DeleteOwnerAssessments(ctx, userID)
}
