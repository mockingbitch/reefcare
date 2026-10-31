package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	neturl "net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
)

// RoleEditor is the role that edits and reviews knowledge.
const RoleEditor = "administrator"

// Service implements the knowledge base.
type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
	log   *slog.Logger
}

// NewService wires the service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, log: log}
}

// Actor is the signed-in user.
type Actor struct {
	UserID uuid.UUID
	Origin audit.Origin
}

var (
	errNotEditor       = apperr.Forbidden("forbidden", "Only knowledge editors can do this.")
	errSlugTaken       = apperr.Conflict("slug_taken", "An article with this slug exists.")
	errVersionConflict = apperr.Conflict("version_conflict", "This was changed elsewhere. Reload and try again.")
	errNotDraft        = apperr.Conflict("not_draft", "Only a draft can be changed or submitted.")
	errNotInReview     = apperr.Conflict("not_in_review", "Only a version in review can be published or rejected.")
	errOwnReview       = apperr.Conflict("own_review", "Another editor must review this version.")
	errArticleRetired  = apperr.Conflict("article_retired", "This article was withdrawn.")
	errNotAuthor       = apperr.Forbidden("not_author", "Only the author changes or submits a draft; create a new version instead.")
	errAuthorUnknown   = apperr.Conflict("author_unknown", "The author's account was erased; send this version back instead.")
	errNewerLive       = apperr.Conflict("newer_version_live", "A newer version of this article is already published.")
)

// strength orders claim classes: a recommendation may not claim more than its version.
var strength = map[string]int{"unverified": 0, "community": 1, "expert": 2, "established": 3}

// requireEditor checks the role in the caller's transaction (service-layer authorization).
func requireEditor(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	roles, err := q.ListUserRoles(ctx, userID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, RoleEditor) {
		return errNotEditor
	}
	return nil
}

// --- sources ---

// SourceInput adds a source.
type SourceInput struct {
	Title       string
	Publisher   *string
	URL         *string
	PublishedOn *time.Time
	AccessedOn  *time.Time
}

// CreateSource adds a citable source (editors).
func (s *Service) CreateSource(ctx context.Context, a Actor, in SourceInput) (Source, error) {
	v := apperr.NewValidator()
	c := checker{v}
	title := c.line("title", in.Title, 1, 300)
	pub := c.optLine("publisher", in.Publisher, 200)
	url := c.optLine("url", in.URL, 1000)
	if url != nil && !validURL(*url) {
		v.Add("url", "https_only", nil)
	}
	if url == nil && pub == nil {
		v.Add("url", "url_or_publisher_required", nil)
	}
	now := s.clock.Now()
	for _, d := range []struct {
		f string
		t *time.Time
	}{{"published_on", in.PublishedOn}, {"accessed_on", in.AccessedOn}} {
		if d.t != nil && d.t.After(now) {
			v.Add(d.f, "in_future", nil)
		}
	}
	if err := v.Err(); err != nil {
		return Source{}, err
	}
	var out Source
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		r, err := q.CreateKnowledgeSource(ctx, dbq.CreateKnowledgeSourceParams{
			Title: title, Publisher: pub, Url: url, PublishedOn: in.PublishedOn, AccessedOn: in.AccessedOn, CreatedBy: &a.UserID, Now: now,
		})
		if err != nil {
			return err
		}
		out = Source{ID: r.ID, Title: r.Title, Publisher: r.Publisher, URL: r.Url, PublishedOn: r.PublishedOn, AccessedOn: r.AccessedOn, CreatedAt: r.CreatedAt}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.source_created", TargetType: "knowledge_source", TargetID: &r.ID})
	})
	return out, err
}

// validURL accepts absolute https URLs with a host and no spaces or credentials.
func validURL(raw string) bool {
	u, err := neturl.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && !strings.ContainsAny(raw, " \t")
}

// ListSources lists every source (editors).
func (s *Service) ListSources(ctx context.Context, a Actor) ([]Source, error) {
	q := dbq.New(s.pool)
	if err := requireEditor(ctx, q, a.UserID); err != nil {
		return nil, err
	}
	rows, err := q.ListKnowledgeSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Source, len(rows))
	for i, r := range rows {
		out[i] = Source{ID: r.ID, Title: r.Title, Publisher: r.Publisher, URL: r.Url, PublishedOn: r.PublishedOn, AccessedOn: r.AccessedOn, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// --- articles and versions (editors) ---

// CreateArticle adds an article (editors).
func (s *Service) CreateArticle(ctx context.Context, a Actor, slug, category string) (Article, error) {
	v := apperr.NewValidator()
	c := checker{v}
	slug = c.slug(slug)
	c.oneOf("category", category, categories)
	if err := v.Err(); err != nil {
		return Article{}, err
	}
	var out Article
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		r, err := q.CreateKnowledgeArticle(ctx, dbq.CreateKnowledgeArticleParams{Slug: slug, Category: category, CreatedBy: &a.UserID, Now: s.clock.Now()})
		if database.IsNoRows(err) {
			return errSlugTaken
		}
		if err != nil {
			return err
		}
		out = Article{ID: r.ID, Slug: r.Slug, Category: r.Category, CreatedAt: r.CreatedAt, RetiredAt: r.RetiredAt}
		return audit.Record(ctx, q, r.CreatedAt, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.article_created", TargetType: "knowledge_article", TargetID: &r.ID})
	})
	return out, err
}

// ListArticles is the editors' overview: every article with each locale's newest version.
func (s *Service) ListArticles(ctx context.Context, a Actor) ([]Article, error) {
	q := dbq.New(s.pool)
	if err := requireEditor(ctx, q, a.UserID); err != nil {
		return nil, err
	}
	rows, err := q.ListKnowledgeArticlesAdmin(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Article, len(rows))
	ids := make([]uuid.UUID, len(rows))
	idx := map[uuid.UUID]int{}
	for i, r := range rows {
		out[i] = Article{ID: r.ID, Slug: r.Slug, Category: r.Category, CreatedAt: r.CreatedAt, RetiredAt: r.RetiredAt, Latest: []VersionSummary{}}
		ids[i] = r.ID
		idx[r.ID] = i
	}
	vs, err := q.ListLatestKnowledgeVersions(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range vs {
		i := idx[r.ArticleID]
		out[i].Latest = append(out[i].Latest, VersionSummary{ID: r.ID, Locale: r.Locale, VersionNo: r.VersionNo, Title: r.Title, Status: r.Status, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

// SourceRef cites a source from a version.
type SourceRef struct {
	SourceID uuid.UUID
	Note     *string
}

// VersionInput is a draft's content.
type VersionInput struct {
	Locale     string
	Title      string
	Summary    string
	BodyMD     string
	ClaimClass string
	Parameters []string
	Symptoms   []string
	Structured Structured
	Sources    []SourceRef
}

type parsedVersion struct {
	title, summary, body, claim string
	params, symptoms            []string
	structured                  []byte
	sources                     []SourceRef
}

func parseVersion(v *apperr.Validator, in VersionInput) parsedVersion {
	c := checker{v}
	p := parsedVersion{
		title: c.line("title", in.Title, 1, 200), summary: c.multi("summary", in.Summary, 1000),
		body: c.multi("body_md", in.BodyMD, maxBody), claim: c.oneOf("claim_class", in.ClaimClass, claimClasses),
		params: c.parameters(in.Parameters), symptoms: c.symptoms(in.Symptoms),
	}
	st, raw := c.structured(in.Structured)
	p.structured = raw
	for i, r := range st.Recommendations {
		if claimClasses[r.ClaimClass] && claimClasses[p.claim] && strength[r.ClaimClass] > strength[p.claim] {
			v.Add(fmt.Sprintf("structured.recommendations.%d.claim_class", i), "stronger_than_version", map[string]string{"max": p.claim})
		}
	}
	seen := map[uuid.UUID]bool{}
	if len(in.Sources) > maxListItems {
		v.Add("sources", "too_many", map[string]string{"max": fmt.Sprint(maxListItems)})
	}
	for i, r := range in.Sources {
		if seen[r.SourceID] {
			v.Add(fmt.Sprintf("sources.%d.source_id", i), "duplicate", nil)
			continue
		}
		seen[r.SourceID] = true
		p.sources = append(p.sources, SourceRef{SourceID: r.SourceID, Note: c.optLine(fmt.Sprintf("sources.%d.note", i), r.Note, 300)})
	}
	return p
}

// setSources replaces a draft's sources after checking they exist.
func setSources(ctx context.Context, q *dbq.Queries, v *apperr.Validator, id uuid.UUID, refs []SourceRef) error {
	ids := make([]uuid.UUID, len(refs))
	for i, r := range refs {
		ids[i] = r.SourceID
	}
	if n, err := q.CountKnowledgeSources(ctx, ids); err != nil {
		return err
	} else if int(n) != len(ids) {
		v.Add("sources", "source_not_found", nil)
		return v.Err()
	}
	if err := q.SetKnowledgeVersionSources(ctx, id); err != nil {
		return err
	}
	for _, r := range refs {
		if err := q.AddKnowledgeVersionSource(ctx, dbq.AddKnowledgeVersionSourceParams{VersionID: id, SourceID: r.SourceID, Note: r.Note}); err != nil {
			return err
		}
	}
	return nil
}

// CreateVersion starts a new draft version of an article in a locale (editors).
func (s *Service) CreateVersion(ctx context.Context, a Actor, articleID uuid.UUID, in VersionInput) (Version, error) {
	v := apperr.NewValidator()
	checker{v}.oneOf("locale", in.Locale, locales)
	p := parseVersion(v, in)
	if err := v.Err(); err != nil {
		return Version{}, err
	}
	now := s.clock.Now()
	var id uuid.UUID
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		art, err := q.LockKnowledgeArticle(ctx, articleID) // serializes version numbers
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if art.RetiredAt != nil {
			return errArticleRetired
		}
		no, err := q.NextKnowledgeVersionNo(ctx, dbq.NextKnowledgeVersionNoParams{ArticleID: articleID, Locale: in.Locale})
		if err != nil {
			return err
		}
		id, err = q.CreateKnowledgeVersion(ctx, dbq.CreateKnowledgeVersionParams{
			ArticleID: articleID, Locale: in.Locale, VersionNo: no, Title: p.title, Summary: p.summary, BodyMd: p.body,
			ClaimClass: p.claim, Parameters: p.params, Symptoms: p.symptoms, Structured: p.structured, AuthorID: &a.UserID, Now: now,
		})
		if err != nil {
			return err
		}
		if err := setSources(ctx, q, v, id, p.sources); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.draft_created", TargetType: "knowledge_version", TargetID: &id})
	})
	if err != nil {
		return Version{}, err
	}
	return s.version(ctx, dbq.New(s.pool), id)
}

// UpdateDraft replaces a draft's content (editors; revision required).
func (s *Service) UpdateDraft(ctx context.Context, a Actor, id uuid.UUID, revision *int32, in VersionInput) (Version, error) {
	v := apperr.NewValidator()
	if revision == nil {
		v.Add("revision", "required", nil)
	}
	p := parseVersion(v, in)
	if err := v.Err(); err != nil {
		return Version{}, err
	}
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		cur, err := q.LockKnowledgeVersion(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if cur.Status != StatusDraft {
			return errNotDraft
		}
		if err := authorOfLiveArticle(ctx, q, a, cur); err != nil {
			return err
		}
		if cur.Version != *revision {
			return errVersionConflict
		}
		now := s.clock.Now()
		if _, err := q.UpdateKnowledgeDraft(ctx, dbq.UpdateKnowledgeDraftParams{
			ID: id, Version: cur.Version, Title: p.title, Summary: p.summary, BodyMd: p.body, ClaimClass: p.claim,
			Parameters: p.params, Symptoms: p.symptoms, Structured: p.structured, Now: now,
		}); err != nil {
			return err
		}
		if err := setSources(ctx, q, v, id, p.sources); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.draft_updated", TargetType: "knowledge_version", TargetID: &id})
	})
	if err != nil {
		return Version{}, err
	}
	return s.version(ctx, dbq.New(s.pool), id)
}

// GetVersion returns any version with its sources (editors).
func (s *Service) GetVersion(ctx context.Context, a Actor, id uuid.UUID) (Version, error) {
	q := dbq.New(s.pool)
	if err := requireEditor(ctx, q, a.UserID); err != nil {
		return Version{}, err
	}
	return s.version(ctx, q, id)
}

// ListVersions returns every version of an article (editors).
func (s *Service) ListVersions(ctx context.Context, a Actor, articleID uuid.UUID) ([]Version, error) {
	q := dbq.New(s.pool)
	if err := requireEditor(ctx, q, a.UserID); err != nil {
		return nil, err
	}
	if _, err := q.GetKnowledgeArticle(ctx, articleID); database.IsNoRows(err) {
		return nil, apperr.NotFound()
	} else if err != nil {
		return nil, err
	}
	rows, err := q.ListArticleVersions(ctx, articleID)
	if err != nil {
		return nil, err
	}
	out := make([]Version, len(rows))
	for i, r := range rows {
		out[i] = Version{
			ID: r.ID, ArticleID: r.ArticleID, Locale: r.Locale, VersionNo: r.VersionNo, Title: r.Title, Summary: r.Summary,
			ClaimClass: r.ClaimClass, Status: r.Status, AuthorID: r.AuthorID, SubmittedAt: r.SubmittedAt, ReviewedBy: r.ReviewedBy,
			ReviewedAt: r.ReviewedAt, ReviewNote: r.ReviewNote, PublishedAt: r.PublishedAt, RetiredAt: r.RetiredAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Revision: r.Version,
		}
	}
	return out, nil
}

// Submit sends a draft to review; its content is frozen from now on (editors).
func (s *Service) Submit(ctx context.Context, a Actor, id uuid.UUID) (Version, error) {
	return s.transition(ctx, a, id, func(q *dbq.Queries, cur dbq.LockKnowledgeVersionRow, now time.Time) error {
		if cur.Status != StatusDraft {
			return errNotDraft
		}
		if err := authorOfLiveArticle(ctx, q, a, cur); err != nil {
			return err
		}
		if err := sourcesForClaim(ctx, q, cur); err != nil {
			return err
		}
		if _, err := q.SubmitKnowledgeVersion(ctx, dbq.SubmitKnowledgeVersionParams{ID: id, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.submitted", TargetType: "knowledge_version", TargetID: &id})
	})
}

// Reject sends a version in review back to draft with a note (another editor).
func (s *Service) Reject(ctx context.Context, a Actor, id uuid.UUID, note string) (Version, error) {
	v := apperr.NewValidator()
	n := checker{v}.multi("note", note, 2000)
	if err := v.Err(); err != nil {
		return Version{}, err
	}
	return s.transition(ctx, a, id, func(q *dbq.Queries, cur dbq.LockKnowledgeVersionRow, now time.Time) error {
		if cur.Status != StatusInReview {
			return errNotInReview
		}
		if cur.AuthorID != nil && *cur.AuthorID == a.UserID {
			return errOwnReview
		}
		if _, err := q.RejectKnowledgeVersion(ctx, dbq.RejectKnowledgeVersionParams{ID: id, Reviewer: &a.UserID, Note: &n, Now: now}); err != nil {
			return err
		}
		if err := q.InsertKnowledgeReview(ctx, dbq.InsertKnowledgeReviewParams{VersionID: id, ReviewerID: &a.UserID, Decision: "rejected", Note: &n, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.rejected", TargetType: "knowledge_version", TargetID: &id,
			Metadata: map[string]any{"article_id": cur.ArticleID.String()}})
	})
}

// Publish makes a version in review live, retiring the previous live version of the same
// article and locale (another editor). An 'established' version needs a source.
func (s *Service) Publish(ctx context.Context, a Actor, id uuid.UUID, note *string) (Version, error) {
	v := apperr.NewValidator()
	var n *string
	if note != nil && strings.TrimSpace(*note) != "" {
		t := checker{v}.multi("note", *note, 2000)
		n = &t
	}
	if err := v.Err(); err != nil {
		return Version{}, err
	}
	return s.transition(ctx, a, id, func(q *dbq.Queries, cur dbq.LockKnowledgeVersionRow, now time.Time) error {
		if cur.Status != StatusInReview {
			return errNotInReview
		}
		if cur.AuthorID == nil {
			return errAuthorUnknown // nobody can tell whether the reviewer wrote it
		}
		if *cur.AuthorID == a.UserID {
			return errOwnReview
		}
		art, err := q.LockKnowledgeArticle(ctx, cur.ArticleID)
		if err != nil {
			return err
		}
		if art.RetiredAt != nil {
			return errArticleRetired
		}
		if err := sourcesForClaim(ctx, q, cur); err != nil {
			return err
		}
		if newer, err := q.NewerVersionLive(ctx, dbq.NewerVersionLiveParams{ArticleID: cur.ArticleID, Locale: cur.Locale, VersionNo: cur.VersionNo}); err != nil {
			return err
		} else if newer {
			return errNewerLive
		}
		if err := q.RetirePublishedVersion(ctx, dbq.RetirePublishedVersionParams{ArticleID: cur.ArticleID, Locale: cur.Locale, Now: now}); err != nil {
			return err
		}
		if _, err := q.PublishKnowledgeVersion(ctx, dbq.PublishKnowledgeVersionParams{ID: id, Reviewer: &a.UserID, Note: n, Now: now}); err != nil {
			return err
		}
		if err := q.InsertKnowledgeReview(ctx, dbq.InsertKnowledgeReviewParams{VersionID: id, ReviewerID: &a.UserID, Decision: "published", Note: n, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.published", TargetType: "knowledge_version", TargetID: &id,
			Metadata: map[string]any{"article_id": cur.ArticleID.String()}})
	})
}

func (s *Service) transition(ctx context.Context, a Actor, id uuid.UUID, apply func(*dbq.Queries, dbq.LockKnowledgeVersionRow, time.Time) error) (Version, error) {
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		cur, err := q.LockKnowledgeVersion(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		return apply(q, cur, s.clock.Now())
	})
	if err != nil {
		return Version{}, err
	}
	return s.version(ctx, dbq.New(s.pool), id)
}

// authorOfLiveArticle allows only the author to change or submit a draft, and only while the
// article is live. Another editor's changes go into a new version they author, so the
// four-eyes check compares the reviewer with whoever wrote the content.
func authorOfLiveArticle(ctx context.Context, q *dbq.Queries, a Actor, cur dbq.LockKnowledgeVersionRow) error {
	if cur.AuthorID == nil || *cur.AuthorID != a.UserID {
		return errNotAuthor
	}
	art, err := q.GetKnowledgeArticle(ctx, cur.ArticleID)
	if err != nil {
		return err
	}
	if art.RetiredAt != nil {
		return errArticleRetired
	}
	return nil
}

// sourcesForClaim requires a source for 'established' content (the version or, through the
// strength rule, any of its recommendations).
func sourcesForClaim(ctx context.Context, q *dbq.Queries, cur dbq.LockKnowledgeVersionRow) error {
	if cur.ClaimClass != "established" {
		return nil
	}
	srcs, err := q.ListVersionSources(ctx, []uuid.UUID{cur.ID})
	if err != nil {
		return err
	}
	if len(srcs) == 0 {
		v := apperr.NewValidator()
		v.Add("sources", "source_required", map[string]string{"claim_class": "established"})
		return v.Err()
	}
	return nil
}

// RetireArticle withdraws an article: its live versions are retired and it disappears for
// readers; assessments keep citing the versions they used (editors).
func (s *Service) RetireArticle(ctx context.Context, a Actor, id uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := requireEditor(ctx, q, a.UserID); err != nil {
			return err
		}
		if _, err := q.LockKnowledgeArticle(ctx, id); database.IsNoRows(err) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		now := s.clock.Now()
		if err := q.RetireArticleVersions(ctx, dbq.RetireArticleVersionsParams{ArticleID: id, Now: now}); err != nil {
			return err
		}
		if n, err := q.RetireKnowledgeArticle(ctx, dbq.RetireKnowledgeArticleParams{ID: id, Now: now}); err != nil || n == 0 {
			return err // already withdrawn: nothing to record
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "knowledge.retired", TargetType: "knowledge_article", TargetID: &id})
	})
}

func (s *Service) version(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Version, error) {
	r, err := q.GetKnowledgeVersion(ctx, id)
	if database.IsNoRows(err) {
		return Version{}, apperr.NotFound()
	}
	if err != nil {
		return Version{}, err
	}
	out := Version{
		ID: r.ID, ArticleID: r.ArticleID, Slug: r.Slug, Category: r.Category, Locale: r.Locale, VersionNo: r.VersionNo,
		Title: r.Title, Summary: r.Summary, BodyMD: r.BodyMd, ClaimClass: r.ClaimClass, Parameters: r.Parameters,
		Symptoms: r.Symptoms, Status: r.Status, AuthorID: r.AuthorID, SubmittedAt: r.SubmittedAt, ReviewedBy: r.ReviewedBy,
		ReviewedAt: r.ReviewedAt, ReviewNote: r.ReviewNote, PublishedAt: r.PublishedAt, RetiredAt: r.RetiredAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Revision: r.Version,
	}
	if err := json.Unmarshal(r.Structured, &out.Structured); err != nil {
		return Version{}, fmt.Errorf("decode structured knowledge: %w", err)
	}
	if out.Sources, err = sources(ctx, q, id); err != nil {
		return Version{}, err
	}
	reviews, err := q.ListKnowledgeReviews(ctx, id)
	if err != nil {
		return Version{}, err
	}
	for _, rv := range reviews {
		out.Reviews = append(out.Reviews, Review{ReviewerID: rv.ReviewerID, Decision: rv.Decision, Note: rv.Note, At: rv.CreatedAt})
	}
	return out, nil
}

func sources(ctx context.Context, q *dbq.Queries, id uuid.UUID) ([]Source, error) {
	rows, err := q.ListVersionSources(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	out := make([]Source, len(rows))
	for i, r := range rows {
		out[i] = Source{ID: r.ID, Title: r.Title, Publisher: r.Publisher, URL: r.Url, PublishedOn: r.PublishedOn, AccessedOn: r.AccessedOn, Note: r.Note}
	}
	return out, nil
}

// --- readers ---

// Summary is a published article in a list.
type Summary struct {
	Slug        string
	Category    string
	VersionID   uuid.UUID
	Locale      string
	VersionNo   int32
	Title       string
	Summary     string
	ClaimClass  string
	Parameters  []string
	Symptoms    []string
	PublishedAt time.Time
}

// ListPublished returns live articles in locale (falling back to the other one), optionally
// about some parameters or matching a search.
func (s *Service) ListPublished(ctx context.Context, locale string, parameters []string, search string) ([]Summary, error) {
	v := apperr.NewValidator()
	c := checker{v}
	if !locales[locale] {
		locale = "vi"
	}
	params := c.parameters(parameters)
	if err := v.Err(); err != nil {
		return nil, err
	}
	rows, err := dbq.New(s.pool).ListPublishedKnowledge(ctx, dbq.ListPublishedKnowledgeParams{Parameters: params, Pattern: likePattern(search), Locale: locale})
	if err != nil {
		return nil, err
	}
	out := make([]Summary, len(rows))
	for i, r := range rows {
		out[i] = Summary{Slug: r.Slug, Category: r.Category, VersionID: r.VersionID, Locale: r.Locale, VersionNo: r.VersionNo,
			Title: r.Title, Summary: r.Summary, ClaimClass: r.ClaimClass, Parameters: r.Parameters, Symptoms: r.Symptoms}
		out[i].PublishedAt = r.PublishedAt
	}
	return out, nil
}

// GetPublished returns an article's live version in locale (or the other one).
func (s *Service) GetPublished(ctx context.Context, slug, locale string) (Version, error) {
	if !locales[locale] {
		locale = "vi"
	}
	q := dbq.New(s.pool)
	r, err := q.GetPublishedKnowledgeBySlug(ctx, dbq.GetPublishedKnowledgeBySlugParams{Slug: slug, Locale: locale})
	if database.IsNoRows(err) {
		return Version{}, apperr.NotFound()
	}
	if err != nil {
		return Version{}, err
	}
	out := Version{ID: r.ID, ArticleID: r.ArticleID, Slug: r.Slug, Category: r.Category, Locale: r.Locale, VersionNo: r.VersionNo,
		Title: r.Title, Summary: r.Summary, BodyMD: r.BodyMd, ClaimClass: r.ClaimClass, Parameters: r.Parameters,
		Symptoms: r.Symptoms, Status: StatusPublished, PublishedAt: r.PublishedAt}
	if err := json.Unmarshal(r.Structured, &out.Structured); err != nil {
		return Version{}, fmt.Errorf("decode structured knowledge: %w", err)
	}
	out.Sources, err = sources(ctx, q, r.ID)
	return out, err
}
