package equipment

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/platform/patch"
	"reefcare/internal/tanks"
)

// Handler exposes equipment and maintenance over HTTP (docs/api-guidelines.md §5).
type Handler struct {
	svc  *Service
	auth func(http.Handler) http.Handler
	log  *slog.Logger
}

// NewHandler returns the handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(svc *Service, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: requireAuth, log: log}
}

// MountTank registers routes under /tanks/{tankID}.
func (h *Handler) MountTank(r chi.Router) {
	r.Get("/equipment", h.listEquipment)
	r.Post("/equipment", h.addEquipment)
	r.Get("/maintenance-plans", h.listPlans)
	r.Post("/maintenance-plans", h.createPlan)
	r.Get("/maintenance-events", h.listEvents)
	r.Post("/maintenance-events", h.recordUnplanned)
}

// Mount registers routes addressed by their own id.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/equipment/{equipmentID}", h.getEquipment)
		r.Patch("/equipment/{equipmentID}", h.updateEquipment)
		r.Post("/equipment/{equipmentID}/remove", h.removeEquipment)
		r.Get("/equipment/{equipmentID}/configurations", h.listConfigs)
		r.Post("/equipment/{equipmentID}/configurations", h.addConfig)
		r.Patch("/maintenance-plans/{planID}", h.updatePlan)
		r.Post("/maintenance-plans/{planID}/events", h.perform)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type configResponse struct {
	ID          uuid.UUID       `json:"id"`
	VersionNo   int32           `json:"version_no"`
	Settings    json.RawMessage `json:"settings"`
	EffectiveAt time.Time       `json:"effective_at"`
	Note        *string         `json:"note"`
	CreatedAt   time.Time       `json:"created_at"`
}

func toConfig(c Configuration) configResponse {
	return configResponse{ID: c.ID, VersionNo: c.VersionNo, Settings: c.Settings, EffectiveAt: c.EffectiveAt.UTC(), Note: c.Note, CreatedAt: c.CreatedAt.UTC()}
}

type equipmentResponse struct {
	ID            uuid.UUID       `json:"id"`
	TankID        uuid.UUID       `json:"tank_id"`
	Category      string          `json:"category"`
	Name          string          `json:"name"`
	Manufacturer  *string         `json:"manufacturer"`
	Model         *string         `json:"model"`
	SerialNumber  *string         `json:"serial_number"`
	PurchasedOn   *string         `json:"purchased_on"`
	WarrantyUntil *string         `json:"warranty_until"`
	InstalledAt   time.Time       `json:"installed_at"`
	RemovedAt     *time.Time      `json:"removed_at"`
	RemovalReason *string         `json:"removal_reason"`
	Notes         *string         `json:"notes"`
	Configuration *configResponse `json:"configuration"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Version       int32           `json:"version"`
}

func dateString(d interface{ String() string }, ok bool) *string {
	if !ok {
		return nil
	}
	s := d.String()
	return &s
}

func toEquipment(e Equipment) equipmentResponse {
	out := equipmentResponse{
		ID: e.ID, TankID: e.TankID, Category: e.Category, Name: e.Name, Manufacturer: e.Manufacturer, Model: e.Model,
		SerialNumber: e.SerialNumber, InstalledAt: e.InstalledAt.UTC(), RemovalReason: e.RemovalReason, Notes: e.Notes,
		CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(), Version: e.Version,
	}
	if e.PurchasedOn != nil {
		out.PurchasedOn = dateString(*e.PurchasedOn, true)
	}
	if e.WarrantyUntil != nil {
		out.WarrantyUntil = dateString(*e.WarrantyUntil, true)
	}
	if e.RemovedAt != nil {
		t := e.RemovedAt.UTC()
		out.RemovedAt = &t
	}
	if e.Current != nil {
		c := toConfig(*e.Current)
		out.Configuration = &c
	}
	return out
}

type planResponse struct {
	ID              uuid.UUID  `json:"id"`
	TankID          uuid.UUID  `json:"tank_id"`
	EquipmentID     *uuid.UUID `json:"equipment_id"`
	EquipmentName   *string    `json:"equipment_name"`
	Title           string     `json:"title"`
	IntervalDays    int32      `json:"interval_days"`
	Timezone        string     `json:"timezone"`
	StartsOn        string     `json:"starts_on"`
	LastPerformedAt *time.Time `json:"last_performed_at"`
	NextDueOn       string     `json:"next_due_on"`
	Active          bool       `json:"active"`
	Overdue         bool       `json:"overdue"`
	DueToday        bool       `json:"due_today"`
	Notes           *string    `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Version         int32      `json:"version"`
}

func toPlan(p Plan) planResponse {
	out := planResponse{
		ID: p.ID, TankID: p.TankID, EquipmentID: p.EquipmentID, EquipmentName: p.EquipmentName, Title: p.Title,
		IntervalDays: p.IntervalDays, Timezone: p.Timezone, StartsOn: p.StartsOn.String(), NextDueOn: p.NextDueOn.String(),
		Active: p.Active, Overdue: p.Overdue, DueToday: p.DueToday, Notes: p.Notes, CreatedAt: p.CreatedAt.UTC(),
		UpdatedAt: p.UpdatedAt.UTC(), Version: p.Version,
	}
	if p.LastPerformedAt != nil {
		t := p.LastPerformedAt.UTC()
		out.LastPerformedAt = &t
	}
	return out
}

type eventResponse struct {
	ID          uuid.UUID  `json:"id"`
	TankID      uuid.UUID  `json:"tank_id"`
	PlanID      *uuid.UUID `json:"plan_id"`
	EquipmentID *uuid.UUID `json:"equipment_id"`
	Title       string     `json:"title"`
	PerformedAt time.Time  `json:"performed_at"`
	Notes       *string    `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toEvent(e Event) eventResponse {
	return eventResponse{ID: e.ID, TankID: e.TankID, PlanID: e.PlanID, EquipmentID: e.EquipmentID, Title: e.Title,
		PerformedAt: e.PerformedAt.UTC(), Notes: e.Notes, CreatedAt: e.CreatedAt.UTC()}
}

func list[T, R any](items []T, f func(T) R) httpserver.Page[R] {
	out := make([]R, len(items))
	for i, it := range items {
		out[i] = f(it)
	}
	return httpserver.Page[R]{Items: out}
}

type equipmentRequest struct {
	Category      string     `json:"category"`
	Name          string     `json:"name"`
	Manufacturer  *string    `json:"manufacturer"`
	Model         *string    `json:"model"`
	SerialNumber  *string    `json:"serial_number"`
	PurchasedOn   *string    `json:"purchased_on"`
	WarrantyUntil *string    `json:"warranty_until"`
	InstalledAt   *time.Time `json:"installed_at"`
	Notes         *string    `json:"notes"`
}

func (h *Handler) addEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req equipmentRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.AddEquipment(r.Context(), tanks.ActorFrom(r), id, DeviceInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toEquipment(e))
}

func (h *Handler) listEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	include := r.URL.Query().Get("include")
	if include != "" && include != "removed" {
		h.fail(w, r, apperr.BadRequest("invalid_query", "Unknown include value.",
			apperr.FieldError{Field: "include", Code: "unsupported", Params: map[string]string{"allowed": "removed"}}))
		return
	}
	items, err := h.svc.ListEquipment(r.Context(), tanks.ActorFrom(r), id, include == "removed")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toEquipment))
}

func (h *Handler) getEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "equipmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.GetEquipment(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEquipment(e))
}

type equipmentPatchRequest struct {
	Version       *int32              `json:"version"`
	Category      patch.Field[string] `json:"category"`
	Name          patch.Field[string] `json:"name"`
	Manufacturer  patch.Field[string] `json:"manufacturer"`
	Model         patch.Field[string] `json:"model"`
	SerialNumber  patch.Field[string] `json:"serial_number"`
	PurchasedOn   patch.Field[string] `json:"purchased_on"`
	WarrantyUntil patch.Field[string] `json:"warranty_until"`
	Notes         patch.Field[string] `json:"notes"`
}

func (h *Handler) updateEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "equipmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req equipmentPatchRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEquipment(r.Context(), tanks.ActorFrom(r), id, DevicePatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEquipment(e))
}

type removeRequest struct {
	RemovedAt *time.Time `json:"removed_at"`
	Reason    *string    `json:"reason"`
}

func (h *Handler) removeEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "equipmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req removeRequest
	if r.ContentLength != 0 {
		if err := httpserver.DecodeJSON(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	e, err := h.svc.RemoveEquipment(r.Context(), tanks.ActorFrom(r), id, req.RemovedAt, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEquipment(e))
}

type configRequest struct {
	Settings    json.RawMessage `json:"settings"`
	EffectiveAt *time.Time      `json:"effective_at"`
	Note        *string         `json:"note"`
}

func (h *Handler) addConfig(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "equipmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req configRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.AddConfiguration(r.Context(), tanks.ActorFrom(r), id, req.Settings, req.EffectiveAt, req.Note)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toConfig(c))
}

func (h *Handler) listConfigs(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "equipmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.ListConfigurations(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toConfig))
}

type planRequest struct {
	Title        string     `json:"title"`
	IntervalDays int32      `json:"interval_days"`
	Timezone     string     `json:"timezone"`
	StartsOn     string     `json:"starts_on"`
	EquipmentID  *uuid.UUID `json:"equipment_id"`
	Notes        *string    `json:"notes"`
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
	p, err := h.svc.CreatePlan(r.Context(), tanks.ActorFrom(r), id, PlanInput(req))
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
	items, err := h.svc.ListPlans(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toPlan))
}

type planPatchRequest struct {
	Version      *int32                 `json:"version"`
	Title        patch.Field[string]    `json:"title"`
	IntervalDays patch.Field[int32]     `json:"interval_days"`
	Timezone     patch.Field[string]    `json:"timezone"`
	StartsOn     patch.Field[string]    `json:"starts_on"`
	EquipmentID  patch.Field[uuid.UUID] `json:"equipment_id"`
	Active       patch.Field[bool]      `json:"active"`
	Notes        patch.Field[string]    `json:"notes"`
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
	p, err := h.svc.UpdatePlan(r.Context(), tanks.ActorFrom(r), id, PlanPatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toPlan(p))
}

type performRequest struct {
	PerformedAt *time.Time `json:"performed_at"`
	Notes       *string    `json:"notes"`
}

func (h *Handler) perform(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "planID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req performRequest
	if r.ContentLength != 0 {
		if err := httpserver.DecodeJSON(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	hash, err := idempotency.Hash("POST /maintenance-plans/"+id.String()+"/events", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, created, err := h.svc.Perform(r.Context(), tanks.ActorFrom(r), id, EventInput{PerformedAt: req.PerformedAt, Notes: req.Notes},
		idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeEvent(w, r, e, created)
}

type unplannedRequest struct {
	Title       string     `json:"title"`
	EquipmentID *uuid.UUID `json:"equipment_id"`
	PerformedAt *time.Time `json:"performed_at"`
	Notes       *string    `json:"notes"`
}

func (h *Handler) recordUnplanned(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req unplannedRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	hash, err := idempotency.Hash("POST /tanks/"+id.String()+"/maintenance-events", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, created, err := h.svc.RecordUnplanned(r.Context(), tanks.ActorFrom(r), id, EventInput(req), idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeEvent(w, r, e, created)
}

// writeEvent answers 201 with the event; a replayed retry is marked with Idempotent-Replayed.
func writeEvent(w http.ResponseWriter, r *http.Request, e Event, created bool) {
	if !created {
		w.Header().Set(idempotency.ReplayedHeader, "true")
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toEvent(e))
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[EventCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := EventFilter{Limit: page.FetchLimit(), Cursor: page.Cursor}
	for _, p := range []struct {
		name string
		dst  **uuid.UUID
	}{{"plan_id", &f.PlanID}, {"equipment_id", &f.EquipmentID}} {
		if v := r.URL.Query().Get(p.name); v != "" {
			u, err := uuid.Parse(v)
			if err != nil {
				h.fail(w, r, apperr.BadRequest("invalid_query", "Invalid id filter.", apperr.FieldError{Field: p.name, Code: "invalid"}))
				return
			}
			*p.dst = &u
		}
	}
	events, err := h.svc.ListEvents(r.Context(), tanks.ActorFrom(r), id, f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]eventResponse, len(events))
	for i, e := range events {
		items[i] = toEvent(e)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(e eventResponse) any { return EventCursor{At: e.PerformedAt, ID: e.ID} })
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}
