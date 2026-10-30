package dosing

import (
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
	"reefcare/internal/platform/patch"
	"reefcare/internal/tanks"
)

// Handler exposes dosing over HTTP (docs/api-guidelines.md §5 Dosing).
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
	r.Get("/dosing-plans", h.listPlans)
	r.Post("/dosing-plans", h.createPlan)
	r.Get("/dosing-history", h.history)
	r.Post("/dosing-events", h.adHoc)
}

// Mount registers product routes and routes addressed by their own id.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/dosing-products", h.listProducts)
		r.Post("/dosing-products", h.createProduct)
		r.Patch("/dosing-plans/{planID}", h.updatePlan)
		r.Post("/dosing-plans/{planID}/pause", h.status(PlanPaused))
		r.Post("/dosing-plans/{planID}/resume", h.status(PlanActive))
		r.Post("/dosing-plans/{planID}/complete", h.status(PlanCompleted))
		r.Post("/dosing-events/{eventID}/resolve", h.resolve)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type productResponse struct {
	ID          uuid.UUID `json:"id"`
	Curated     bool      `json:"curated"`
	Name        string    `json:"name"`
	Brand       *string   `json:"brand"`
	Form        string    `json:"form"`
	DefaultUnit string    `json:"default_unit"`
	Notes       *string   `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

func toProduct(p Product) productResponse {
	return productResponse{ID: p.ID, Curated: p.OwnerID == nil, Name: p.Name, Brand: p.Brand, Form: p.Form,
		DefaultUnit: p.DefaultUnit, Notes: p.Notes, CreatedAt: p.CreatedAt.UTC()}
}

type productRefJSON struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Brand *string   `json:"brand,omitempty"`
	Form  string    `json:"form,omitempty"`
}

type planResponse struct {
	ID              uuid.UUID       `json:"id"`
	TankID          uuid.UUID       `json:"tank_id"`
	Product         productRefJSON  `json:"product"`
	Amount          decimal.Decimal `json:"amount"`
	AmountUnit      string          `json:"amount_unit"`
	RRule           string          `json:"rrule"`
	Timezone        string          `json:"timezone"`
	StartsOn        string          `json:"starts_on"`
	EndsOn          *string         `json:"ends_on"`
	DeliveryMethod  string          `json:"delivery_method"`
	Status          string          `json:"status"`
	Notes           *string         `json:"notes"`
	NextOccurrences []time.Time     `json:"next_occurrences"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Version         int32           `json:"version"`
}

func toPlan(p Plan) planResponse {
	out := planResponse{
		ID: p.ID, TankID: p.TankID,
		Product: productRefJSON{ID: p.Product.ID, Name: p.Product.Name, Brand: p.Product.Brand, Form: p.Product.Form},
		Amount:  p.Amount, AmountUnit: p.Unit, RRule: p.RRule, Timezone: p.Timezone, StartsOn: p.StartsOn.String(),
		DeliveryMethod: p.DeliveryMethod, Status: p.Status, Notes: p.Notes, NextOccurrences: make([]time.Time, len(p.Next)),
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC(), Version: p.Version,
	}
	for i, t := range p.Next {
		out.NextOccurrences[i] = t.UTC()
	}
	if p.EndsOn != nil {
		e := p.EndsOn.String()
		out.EndsOn = &e
	}
	return out
}

type eventResponse struct {
	ID            uuid.UUID        `json:"id"`
	TankID        uuid.UUID        `json:"tank_id"`
	PlanID        *uuid.UUID       `json:"plan_id"`
	Product       productRefJSON   `json:"product"`
	ScheduledFor  *time.Time       `json:"scheduled_for"`
	PlannedAmount *decimal.Decimal `json:"planned_amount"`
	ActualAmount  *decimal.Decimal `json:"actual_amount"`
	ActualSource  *string          `json:"actual_source"`
	Deviation     *string          `json:"deviation"`
	AmountUnit    string           `json:"amount_unit"`
	Status        string           `json:"status"`
	ExecutedAt    *time.Time       `json:"executed_at"`
	Reason        *string          `json:"reason"`
	ReasonCode    *string          `json:"reason_code"`
	Notes         *string          `json:"notes"`
	ReportedBy    *string          `json:"reported_by"`
	MissedAt      *time.Time       `json:"missed_at"`
	ResolvedAt    *time.Time       `json:"resolved_at"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	Version       int32            `json:"version"`
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func toEvent(e Event) eventResponse {
	var dev *string
	if d := deviation(e.PlannedAmount, e.ActualAmount); d != "" {
		dev = &d
	}
	return eventResponse{
		ID: e.ID, TankID: e.TankID, PlanID: e.PlanID, Product: productRefJSON{ID: e.Product.ID, Name: e.Product.Name},
		ScheduledFor: utc(e.ScheduledFor), PlannedAmount: e.PlannedAmount, ActualAmount: e.ActualAmount,
		ActualSource: e.ActualSource, Deviation: dev, AmountUnit: e.Unit, ReasonCode: e.ReasonCode,
		Status: e.Status, ExecutedAt: utc(e.ExecutedAt), Reason: e.Reason, Notes: e.Notes, ReportedBy: e.ReportedBy,
		MissedAt: utc(e.MissedAt), ResolvedAt: utc(e.ResolvedAt), CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(),
		Version: e.Version,
	}
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListProducts(r.Context(), tanks.ActorFrom(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]productResponse, len(list))
	for i, p := range list {
		items[i] = toProduct(p)
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[productResponse]{Items: items})
}

type productRequest struct {
	Name        string  `json:"name"`
	Brand       *string `json:"brand"`
	Form        string  `json:"form"`
	DefaultUnit string  `json:"default_unit"`
	Notes       *string `json:"notes"`
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), tanks.ActorFrom(r), ProductInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toProduct(p))
}

type planRequest struct {
	ProductID      uuid.UUID `json:"product_id"`
	Amount         string    `json:"amount"`
	AmountUnit     string    `json:"amount_unit"`
	RRule          string    `json:"rrule"`
	Timezone       string    `json:"timezone"`
	StartsOn       string    `json:"starts_on"`
	EndsOn         *string   `json:"ends_on"`
	DeliveryMethod string    `json:"delivery_method"`
	Notes          *string   `json:"notes"`
}

func (h *Handler) createPlan(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req planRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreatePlan(r.Context(), tanks.ActorFrom(r), id, PlanInput{
		ProductID: req.ProductID, Amount: req.Amount, Unit: req.AmountUnit, RRule: req.RRule, Timezone: req.Timezone,
		StartsOn: req.StartsOn, EndsOn: req.EndsOn, DeliveryMethod: req.DeliveryMethod, Notes: req.Notes,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toPlan(p))
}

func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	list, err := h.svc.ListPlans(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]planResponse, len(list))
	for i, p := range list {
		items[i] = toPlan(p)
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[planResponse]{Items: items})
}

type planPatchRequest struct {
	Version        *int32              `json:"version"`
	Amount         patch.Field[string] `json:"amount"`
	AmountUnit     patch.Field[string] `json:"amount_unit"`
	RRule          patch.Field[string] `json:"rrule"`
	Timezone       patch.Field[string] `json:"timezone"`
	StartsOn       patch.Field[string] `json:"starts_on"`
	EndsOn         patch.Field[string] `json:"ends_on"`
	DeliveryMethod patch.Field[string] `json:"delivery_method"`
	Notes          patch.Field[string] `json:"notes"`
}

func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "planID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req planPatchRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.UpdatePlan(r.Context(), tanks.ActorFrom(r), id, PlanPatch{
		Version: req.Version, Amount: req.Amount, Unit: req.AmountUnit, RRule: req.RRule, Timezone: req.Timezone,
		StartsOn: req.StartsOn, EndsOn: req.EndsOn, DeliveryMethod: req.DeliveryMethod, Notes: req.Notes,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toPlan(p))
}

func (h *Handler) status(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := tanks.PathID(r, "planID")
		if err != nil {
			h.fail(w, r, err)
			return
		}
		p, err := h.svc.SetStatus(r.Context(), tanks.ActorFrom(r), id, status)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpserver.WriteJSON(w, r, http.StatusOK, toPlan(p))
	}
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[HistoryCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := HistoryFilter{Limit: page.FetchLimit(), Cursor: page.Cursor}
	q := r.URL.Query()
	if st := q.Get("status"); st != "" {
		allowed := map[string]bool{}
		for _, a := range AllStatuses {
			allowed[a] = true
		}
		for _, s := range strings.Split(st, ",") {
			if !allowed[s] {
				h.fail(w, r, apperr.BadRequest("invalid_query", "Unknown status filter.",
					apperr.FieldError{Field: "status", Code: "unsupported", Params: map[string]string{"allowed": strings.Join(AllStatuses, ",")}}))
				return
			}
			f.Statuses = append(f.Statuses, s)
		}
	}
	for _, p := range []struct {
		name string
		dst  **time.Time
	}{{"from", &f.From}, {"to", &f.To}} {
		if v := q.Get(p.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				h.fail(w, r, apperr.BadRequest("invalid_query", "Invalid time filter.",
					apperr.FieldError{Field: p.name, Code: "invalid_datetime", Params: map[string]string{"format": "RFC 3339"}}))
				return
			}
			*p.dst = &t
		}
	}
	events, err := h.svc.History(r.Context(), tanks.ActorFrom(r), id, f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]eventResponse, len(events))
	for i, e := range events {
		items[i] = toEvent(e)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(e eventResponse) any {
		at := e.ScheduledFor
		if e.ExecutedAt != nil {
			at = e.ExecutedAt
		}
		return HistoryCursor{At: *at, ID: e.ID}
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

type resolveRequest struct {
	Status             string     `json:"status"`
	ActualAmount       *string    `json:"actual_amount"`
	ExecutedAt         *time.Time `json:"executed_at"`
	Reason             *string    `json:"reason"`
	Notes              *string    `json:"notes"`
	ConfirmOverPlanned bool       `json:"confirm_over_planned"`
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "eventID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req resolveRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Resolve(r.Context(), tanks.ActorFrom(r), id, ResolveInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEvent(e))
}

type adHocRequest struct {
	ProductID  uuid.UUID  `json:"product_id"`
	Amount     string     `json:"amount"`
	AmountUnit string     `json:"amount_unit"`
	ExecutedAt *time.Time `json:"executed_at"`
	Notes      *string    `json:"notes"`
}

func (h *Handler) adHoc(w http.ResponseWriter, r *http.Request) {
	tankID, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req adHocRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	hash, err := idempotency.Hash("POST /tanks/"+tankID.String()+"/dosing-events", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, created, err := h.svc.RecordAdHoc(r.Context(), tanks.ActorFrom(r), tankID, AdHocInput{
		ProductID: req.ProductID, Amount: req.Amount, Unit: req.AmountUnit, ExecutedAt: req.ExecutedAt, Notes: req.Notes,
	}, idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		w.Header().Set(idempotency.ReplayedHeader, "true")
	}
	httpserver.WriteJSON(w, r, status, toEvent(e))
}
