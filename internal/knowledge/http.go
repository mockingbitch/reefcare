package knowledge

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/tanks"
)

// Handler exposes the knowledge base (docs/api-guidelines.md §5 Knowledge).
type Handler struct {
	svc  *Service
	auth func(http.Handler) http.Handler
	log  *slog.Logger
}

// NewHandler returns the handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(svc *Service, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: requireAuth, log: log}
}

// Mount registers the reader routes and the editor routes under /admin/knowledge (the
// service checks the editor role on every call).
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/knowledge", h.listPublished)
		r.Get("/knowledge/{slug}", h.getPublished)
		r.Route("/admin/knowledge", func(r chi.Router) {
			r.Get("/sources", h.listSources)
			r.Post("/sources", h.createSource)
			r.Get("/articles", h.listArticles)
			r.Post("/articles", h.createArticle)
			r.Post("/articles/{articleID}/retire", h.retireArticle)
			r.Get("/articles/{articleID}/versions", h.listVersions)
			r.Post("/articles/{articleID}/versions", h.createVersion)
			r.Get("/versions/{versionID}", h.getVersion)
			r.Patch("/versions/{versionID}", h.updateDraft)
			r.Post("/versions/{versionID}/submit", h.submit)
			r.Post("/versions/{versionID}/publish", h.publish)
			r.Post("/versions/{versionID}/reject", h.reject)
		})
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

func actor(r *http.Request) Actor {
	a := tanks.ActorFrom(r)
	return Actor{UserID: a.UserID, Origin: a.Origin}
}

// --- responses ---

type sourceResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Publisher   *string   `json:"publisher"`
	URL         *string   `json:"url"`
	PublishedOn *string   `json:"published_on"`
	AccessedOn  *string   `json:"accessed_on"`
	Note        *string   `json:"note,omitempty"`
}

func date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.DateOnly)
	return &s
}

func toSource(s Source) sourceResponse {
	return sourceResponse{ID: s.ID, Title: s.Title, Publisher: s.Publisher, URL: s.URL, PublishedOn: date(s.PublishedOn), AccessedOn: date(s.AccessedOn), Note: s.Note}
}

type versionResponse struct {
	ID          uuid.UUID        `json:"id"`
	ArticleID   uuid.UUID        `json:"article_id"`
	Slug        string           `json:"slug,omitempty"`
	Category    string           `json:"category,omitempty"`
	Locale      string           `json:"locale"`
	VersionNo   int32            `json:"version_no"`
	Title       string           `json:"title"`
	Summary     string           `json:"summary"`
	BodyMD      *string          `json:"body_md,omitempty"`
	ClaimClass  string           `json:"claim_class"`
	Parameters  []string         `json:"parameters"`
	Symptoms    []string         `json:"symptoms"`
	Structured  *Structured      `json:"structured,omitempty"`
	Status      string           `json:"status"`
	AuthorID    *uuid.UUID       `json:"author_id,omitempty"`
	ReviewedBy  *uuid.UUID       `json:"reviewed_by,omitempty"`
	ReviewNote  *string          `json:"review_note,omitempty"`
	SubmittedAt *time.Time       `json:"submitted_at,omitempty"`
	ReviewedAt  *time.Time       `json:"reviewed_at,omitempty"`
	PublishedAt *time.Time       `json:"published_at"`
	RetiredAt   *time.Time       `json:"retired_at,omitempty"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
	Revision    *int32           `json:"revision,omitempty"`
	Sources     []sourceResponse `json:"sources,omitempty"`
	Reviews     []reviewResponse `json:"reviews,omitempty"`
}

type reviewResponse struct {
	ReviewerID *uuid.UUID `json:"reviewer_id"`
	Decision   string     `json:"decision"`
	Note       *string    `json:"note"`
	At         time.Time  `json:"at"`
}

func utc(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// toVersion renders a version; editor adds the workflow fields readers don't need.
func toVersion(v Version, full, editor bool) versionResponse {
	out := versionResponse{
		ID: v.ID, ArticleID: v.ArticleID, Slug: v.Slug, Category: v.Category, Locale: v.Locale, VersionNo: v.VersionNo,
		Title: v.Title, Summary: v.Summary, ClaimClass: v.ClaimClass, Parameters: nonNil(v.Parameters),
		Symptoms: nonNil(v.Symptoms), Status: v.Status, PublishedAt: utc(v.PublishedAt),
	}
	if full {
		body, st := v.BodyMD, v.Structured
		out.BodyMD, out.Structured = &body, &st
		out.Sources = make([]sourceResponse, len(v.Sources))
		for i, s := range v.Sources {
			out.Sources[i] = toSource(s)
		}
	}
	if editor {
		rev := v.Revision
		out.AuthorID, out.ReviewedBy, out.ReviewNote, out.Revision = v.AuthorID, v.ReviewedBy, v.ReviewNote, &rev
		out.SubmittedAt, out.ReviewedAt, out.RetiredAt, out.UpdatedAt = utc(v.SubmittedAt), utc(v.ReviewedAt), utc(v.RetiredAt), utc(&v.UpdatedAt)
		for _, r := range v.Reviews {
			out.Reviews = append(out.Reviews, reviewResponse{ReviewerID: r.ReviewerID, Decision: r.Decision, Note: r.Note, At: r.At.UTC()})
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func list[T, R any](items []T, f func(T) R) httpserver.Page[R] {
	out := make([]R, len(items))
	for i, it := range items {
		out[i] = f(it)
	}
	return httpserver.Page[R]{Items: out}
}

// --- readers ---

func (h *Handler) listPublished(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	var params []string
	for _, v := range qs["parameter"] {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				params = append(params, p)
			}
		}
	}
	if len([]rune(qs.Get("q"))) > 100 {
		h.fail(w, r, apperr.BadRequest("invalid_query", "Search is too long.", apperr.FieldError{Field: "q", Code: "length"}))
		return
	}
	items, err := h.svc.ListPublished(r.Context(), qs.Get("locale"), params, qs.Get("q"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, func(s Summary) versionResponse {
		pub := s.PublishedAt
		return versionResponse{ID: s.VersionID, Slug: s.Slug, Category: s.Category, Locale: s.Locale, VersionNo: s.VersionNo,
			Title: s.Title, Summary: s.Summary, ClaimClass: s.ClaimClass, Parameters: nonNil(s.Parameters),
			Symptoms: nonNil(s.Symptoms), Status: StatusPublished, PublishedAt: utc(&pub)}
	}))
}

func (h *Handler) getPublished(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.GetPublished(r.Context(), chi.URLParam(r, "slug"), r.URL.Query().Get("locale"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toVersion(v, true, false))
}

// --- editors ---

func (h *Handler) createSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string  `json:"title"`
		Publisher   *string `json:"publisher"`
		URL         *string `json:"url"`
		PublishedOn *string `json:"published_on"`
		AccessedOn  *string `json:"accessed_on"`
	}
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in := SourceInput{Title: req.Title, Publisher: req.Publisher, URL: req.URL}
	v := apperr.NewValidator()
	for _, d := range []struct {
		f   string
		s   *string
		dst **time.Time
	}{{"published_on", req.PublishedOn, &in.PublishedOn}, {"accessed_on", req.AccessedOn, &in.AccessedOn}} {
		if d.s == nil {
			continue
		}
		t, err := time.Parse(time.DateOnly, *d.s)
		if err != nil {
			v.Add(d.f, "invalid_date", map[string]string{"format": "YYYY-MM-DD"})
			continue
		}
		*d.dst = &t
	}
	if err := v.Err(); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.CreateSource(r.Context(), actor(r), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toSource(s))
}

func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListSources(r.Context(), actor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toSource))
}

type articleResponse struct {
	ID        uuid.UUID         `json:"id"`
	Slug      string            `json:"slug"`
	Category  string            `json:"category"`
	CreatedAt time.Time         `json:"created_at"`
	RetiredAt *time.Time        `json:"retired_at"`
	Latest    []versionOverview `json:"latest"`
}

type versionOverview struct {
	ID        uuid.UUID `json:"id"`
	Locale    string    `json:"locale"`
	VersionNo int32     `json:"version_no"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toArticle(a Article) articleResponse {
	out := articleResponse{ID: a.ID, Slug: a.Slug, Category: a.Category, CreatedAt: a.CreatedAt.UTC(), RetiredAt: utc(a.RetiredAt), Latest: []versionOverview{}}
	for _, v := range a.Latest {
		out.Latest = append(out.Latest, versionOverview{ID: v.ID, Locale: v.Locale, VersionNo: v.VersionNo, Title: v.Title, Status: v.Status, UpdatedAt: v.UpdatedAt.UTC()})
	}
	return out
}

func (h *Handler) createArticle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug     string `json:"slug"`
		Category string `json:"category"`
	}
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.CreateArticle(r.Context(), actor(r), req.Slug, req.Category)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toArticle(a))
}

func (h *Handler) listArticles(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListArticles(r.Context(), actor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toArticle))
}

func (h *Handler) retireArticle(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "articleID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RetireArticle(r.Context(), actor(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sourceRefRequest struct {
	SourceID uuid.UUID `json:"source_id"`
	Note     *string   `json:"note"`
}

type versionRequest struct {
	Revision   *int32             `json:"revision"`
	Locale     string             `json:"locale"`
	Title      string             `json:"title"`
	Summary    string             `json:"summary"`
	BodyMD     string             `json:"body_md"`
	ClaimClass string             `json:"claim_class"`
	Parameters []string           `json:"parameters"`
	Symptoms   []string           `json:"symptoms"`
	Structured Structured         `json:"structured"`
	Sources    []sourceRefRequest `json:"sources"`
}

func (req versionRequest) input() VersionInput {
	in := VersionInput{Locale: req.Locale, Title: req.Title, Summary: req.Summary, BodyMD: req.BodyMD, ClaimClass: req.ClaimClass,
		Parameters: req.Parameters, Symptoms: req.Symptoms, Structured: req.Structured}
	for _, s := range req.Sources {
		in.Sources = append(in.Sources, SourceRef(s))
	}
	return in
}

func (h *Handler) createVersion(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "articleID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req versionRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.CreateVersion(r.Context(), actor(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toVersion(v, true, true))
}

func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "articleID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.ListVersions(r.Context(), actor(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, func(v Version) versionResponse { return toVersion(v, false, true) }))
}

func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "versionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.GetVersion(r.Context(), actor(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toVersion(v, true, true))
}

func (h *Handler) updateDraft(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "versionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req versionRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.UpdateDraft(r.Context(), actor(r), id, req.Revision, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toVersion(v, true, true))
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	h.workflow(w, r, func(id uuid.UUID, note *string) (Version, error) { return h.svc.Submit(r.Context(), actor(r), id) })
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	h.workflow(w, r, func(id uuid.UUID, note *string) (Version, error) {
		return h.svc.Publish(r.Context(), actor(r), id, note)
	})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.workflow(w, r, func(id uuid.UUID, note *string) (Version, error) {
		n := ""
		if note != nil {
			n = *note
		}
		return h.svc.Reject(r.Context(), actor(r), id, n)
	})
}

func (h *Handler) workflow(w http.ResponseWriter, r *http.Request, do func(uuid.UUID, *string) (Version, error)) {
	id, err := tanks.PathID(r, "versionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req struct {
		Note *string `json:"note"`
	}
	if r.ContentLength != 0 {
		if err := httpserver.DecodeJSON(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	v, err := do(id, req.Note)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toVersion(v, true, true))
}
