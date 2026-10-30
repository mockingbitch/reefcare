package waterparameters

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/tanks"
)

// Handler exposes water parameters over HTTP (docs/api-guidelines.md §5 Water parameters).
type Handler struct {
	svc  *Service
	auth func(http.Handler) http.Handler
	log  *slog.Logger
}

// NewHandler returns the handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(svc *Service, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: requireAuth, log: log}
}

// MountTank registers routes under /tanks/{tankID} (pass to tanks.Handler.Mount).
func (h *Handler) MountTank(r chi.Router) {
	r.Get("/water-tests", h.list)
	r.Post("/water-tests", h.record)
	r.Get("/water-tests/latest", h.latest)
	r.Get("/water-tests/trends", h.trends)
	r.Get("/parameter-targets", h.targets)
	r.Put("/parameter-targets", h.replaceTargets)
}

// Mount registers routes addressed by their own id and the catalog.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/parameter-types", h.catalog)
		r.Get("/water-tests/{testID}", h.get)
		r.Post("/readings/{readingID}/corrections", h.correct)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

// --- responses ------------------------------------------------------------------------------

type unitResponse struct {
	Code     string `json:"code"`
	Estimate bool   `json:"estimate"`
}

type parameterResponse struct {
	Code           string          `json:"code"`
	CanonicalUnit  string          `json:"canonical_unit"`
	Units          []unitResponse  `json:"units"`
	HardMin        decimal.Decimal `json:"hard_min"`
	HardMax        decimal.Decimal `json:"hard_max"`
	FreshnessHours int             `json:"freshness_hours"`
	Decimals       int             `json:"decimals"`
}

type readingResponse struct {
	ID               uuid.UUID       `json:"id"`
	Parameter        string          `json:"parameter"`
	MeasuredAt       time.Time       `json:"measured_at"`
	InputValue       decimal.Decimal `json:"input_value"`
	InputUnit        string          `json:"input_unit"`
	CanonicalValue   decimal.Decimal `json:"canonical_value"`
	CanonicalUnit    string          `json:"canonical_unit"`
	Estimate         bool            `json:"estimate"`
	Method           *string         `json:"method"`
	TestKit          *string         `json:"test_kit"`
	Status           string          `json:"status"`
	SupersedesID     *uuid.UUID      `json:"supersedes_id"`
	CorrectionReason *string         `json:"correction_reason"`
	TargetStatus     *string         `json:"target_status"`
	Warnings         []string        `json:"warnings"`
	CreatedAt        time.Time       `json:"created_at"`
}

func toReading(r Reading) readingResponse {
	w := r.Warnings
	if w == nil {
		w = []string{}
	}
	return readingResponse{
		ID: r.ID, Parameter: r.Parameter, MeasuredAt: r.MeasuredAt.UTC(), InputValue: r.InputValue,
		InputUnit: r.InputUnit, CanonicalValue: r.CanonicalValue, CanonicalUnit: r.CanonicalUnit,
		Estimate: r.Estimate, Method: r.Method, TestKit: r.TestKit, Status: r.Status, SupersedesID: r.SupersedesID,
		CorrectionReason: r.CorrectionReason, TargetStatus: r.TargetStatus, Warnings: w, CreatedAt: r.CreatedAt.UTC(),
	}
}

type testResponse struct {
	ID         uuid.UUID         `json:"id"`
	TankID     uuid.UUID         `json:"tank_id"`
	MeasuredAt time.Time         `json:"measured_at"`
	Source     string            `json:"source"`
	Notes      *string           `json:"notes"`
	Readings   []readingResponse `json:"readings"`
	CreatedAt  time.Time         `json:"created_at"`
}

func toTest(t Test) testResponse {
	out := testResponse{
		ID: t.ID, TankID: t.TankID, MeasuredAt: t.MeasuredAt.UTC(), Source: t.Source, Notes: t.Notes,
		Readings: make([]readingResponse, len(t.Readings)), CreatedAt: t.CreatedAt.UTC(),
	}
	for i, r := range t.Readings {
		out.Readings[i] = toReading(r)
	}
	return out
}

type targetResponse struct {
	Parameter      string           `json:"parameter"`
	Min            *decimal.Decimal `json:"min"`
	Ideal          *decimal.Decimal `json:"ideal"`
	Max            *decimal.Decimal `json:"max"`
	FreshnessHours *int32           `json:"freshness_hours"`
	ValidFrom      time.Time        `json:"valid_from"`
	ValidTo        *time.Time       `json:"valid_to"`
}

func toTarget(t Target) targetResponse {
	out := targetResponse{
		Parameter: t.Parameter, Min: t.Min, Ideal: t.Ideal, Max: t.Max, FreshnessHours: t.FreshnessHours,
		ValidFrom: t.ValidFrom.UTC(),
	}
	if t.ValidTo != nil {
		v := t.ValidTo.UTC()
		out.ValidTo = &v
	}
	return out
}

// --- handlers -------------------------------------------------------------------------------

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	items := make([]parameterResponse, len(Catalog))
	for i, p := range Catalog {
		units := make([]unitResponse, len(p.Units))
		for j, u := range p.Units {
			units[j] = unitResponse{Code: u.Code, Estimate: u.Estimate}
		}
		items[i] = parameterResponse{
			Code: p.Code, CanonicalUnit: p.CanonicalUnit, Units: units, HardMin: p.HardMin, HardMax: p.HardMax,
			FreshnessHours: p.FreshnessHours, Decimals: p.Decimals,
		}
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[parameterResponse]{Items: items})
}

type readingRequest struct {
	Parameter string  `json:"parameter"`
	Value     string  `json:"value"`
	Unit      string  `json:"unit"`
	Method    *string `json:"method"`
	TestKit   *string `json:"test_kit"`
}

type recordRequest struct {
	MeasuredAt *time.Time       `json:"measured_at"`
	Notes      *string          `json:"notes"`
	Readings   []readingRequest `json:"readings"`
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req recordRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	hash, err := idempotency.Hash("POST /tanks/"+id.String()+"/water-tests", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	in := TestInput{MeasuredAt: req.MeasuredAt, Notes: req.Notes, Readings: make([]ReadingInput, len(req.Readings))}
	for i, rd := range req.Readings {
		in.Readings[i] = ReadingInput(rd)
	}
	t, created, err := h.svc.Record(r.Context(), tanks.ActorFrom(r), id, in, idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if !created {
		w.Header().Set(idempotency.ReplayedHeader, "true")
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toTest(t))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "testID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.GetTest(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toTest(t))
}

func parseTime(q map[string][]string, name string) (*time.Time, error) {
	v := ""
	if vs := q[name]; len(vs) > 0 {
		v = vs[0]
	}
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, apperr.BadRequest("invalid_query", "Invalid time filter.",
			apperr.FieldError{Field: name, Code: "invalid_datetime", Params: map[string]string{"format": "RFC 3339"}})
	}
	return &t, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[ListCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := ListFilter{Limit: page.FetchLimit(), Cursor: page.Cursor, Parameter: q.Get("parameter")}
	if f.From, err = parseTime(q, "from"); err != nil {
		h.fail(w, r, err)
		return
	}
	if f.To, err = parseTime(q, "to"); err != nil {
		h.fail(w, r, err)
		return
	}
	tests, err := h.svc.List(r.Context(), tanks.ActorFrom(r), id, f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]testResponse, len(tests))
	for i, t := range tests {
		items[i] = toTest(t)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(t testResponse) any {
		return ListCursor{At: t.MeasuredAt, ID: t.ID}
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

type correctionRequest struct {
	Value  string  `json:"value"`
	Unit   string  `json:"unit"`
	Reason *string `json:"reason"`
}

func (h *Handler) correct(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "readingID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req correctionRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	rd, err := h.svc.Correct(r.Context(), tanks.ActorFrom(r), id, CorrectionInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toReading(rd))
}

type latestResponse struct {
	Parameter      string           `json:"parameter"`
	CanonicalUnit  string           `json:"canonical_unit"`
	Reading        *readingResponse `json:"reading"`
	FreshnessHours int              `json:"freshness_hours"`
	Stale          bool             `json:"stale"`
	Target         *targetResponse  `json:"target"`
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	list, err := h.svc.LatestReadings(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]latestResponse, len(list))
	for i, l := range list {
		items[i] = latestResponse{Parameter: l.Parameter, CanonicalUnit: l.CanonicalUnit, FreshnessHours: l.FreshnessHours, Stale: l.Stale}
		if l.Reading != nil {
			rd := toReading(*l.Reading)
			items[i].Reading = &rd
		}
		if l.Target != nil {
			tg := toTarget(*l.Target)
			items[i].Target = &tg
		}
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[latestResponse]{Items: items})
}

type pointResponse struct {
	At        time.Time       `json:"t"`
	Value     decimal.Decimal `json:"v"`
	ReadingID uuid.UUID       `json:"reading_id"`
	Estimate  bool            `json:"estimate"`
}

type seriesResponse struct {
	Parameter     string           `json:"parameter"`
	CanonicalUnit string           `json:"unit"`
	Points        []pointResponse  `json:"points"`
	Targets       []targetResponse `json:"targets"`
	Truncated     bool             `json:"truncated"`
}

type annotationResponse struct {
	At        time.Time       `json:"t"`
	Category  string          `json:"category"`
	EventType string          `json:"event_type"`
	Summary   json.RawMessage `json:"summary"`
}

type trendsResponse struct {
	From                 time.Time            `json:"from"`
	To                   time.Time            `json:"to"`
	Series               []seriesResponse     `json:"series"`
	Annotations          []annotationResponse `json:"annotations"`
	Truncated            bool                 `json:"truncated"`
	AnnotationsTruncated bool                 `json:"annotations_truncated"`
}

func (h *Handler) trends(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	var params []string
	for _, p := range strings.Split(q.Get("parameters"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			params = append(params, p)
		}
	}
	from, err := parseTime(q, "from")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	to, err := parseTime(q, "to")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.TrendsFor(r.Context(), tanks.ActorFrom(r), id, params, from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := trendsResponse{From: t.From.UTC(), To: t.To.UTC(), AnnotationsTruncated: t.AnnotationsTruncated,
		Series: make([]seriesResponse, len(t.Series)), Annotations: make([]annotationResponse, len(t.Annotations))}
	for i, s := range t.Series {
		out.Truncated = out.Truncated || s.Truncated
		sr := seriesResponse{Parameter: s.Parameter, CanonicalUnit: s.CanonicalUnit, Truncated: s.Truncated,
			Points: make([]pointResponse, len(s.Points)), Targets: make([]targetResponse, len(s.Targets))}
		for j, p := range s.Points {
			sr.Points[j] = pointResponse{At: p.At.UTC(), Value: p.Value, ReadingID: p.ReadingID, Estimate: p.Estimate}
		}
		for j, tg := range s.Targets {
			sr.Targets[j] = toTarget(tg)
		}
		out.Series[i] = sr
	}
	for i, a := range t.Annotations {
		summary := a.Summary
		if len(summary) == 0 {
			summary = json.RawMessage(`{}`)
		}
		out.Annotations[i] = annotationResponse{At: a.At.UTC(), Category: a.Category, EventType: a.EventType, Summary: summary}
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

func (h *Handler) targets(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	list, err := h.svc.Targets(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeTargets(w, r, list)
}

func writeTargets(w http.ResponseWriter, r *http.Request, list []Target) {
	items := make([]targetResponse, len(list))
	for i, t := range list {
		items[i] = toTarget(t)
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[targetResponse]{Items: items})
}

type targetRequest struct {
	Parameter      string  `json:"parameter"`
	Min            *string `json:"min"`
	Ideal          *string `json:"ideal"`
	Max            *string `json:"max"`
	FreshnessHours *int32  `json:"freshness_hours"`
}

type replaceTargetsRequest struct {
	Targets *[]targetRequest `json:"targets"` // required; [] clears every target
}

func (h *Handler) replaceTargets(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req replaceTargetsRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Targets == nil {
		v := apperr.NewValidator()
		v.Add("targets", "required", nil)
		h.fail(w, r, v.Err())
		return
	}
	in := make([]TargetInput, len(*req.Targets))
	for i, t := range *req.Targets {
		in[i] = TargetInput(t)
	}
	list, err := h.svc.ReplaceTargets(r.Context(), tanks.ActorFrom(r), id, in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeTargets(w, r, list)
}
