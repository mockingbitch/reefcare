package waterchanges

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
	"reefcare/internal/platform/patch"
	"reefcare/internal/tanks"
)

// Handler exposes water changes over HTTP (docs/api-guidelines.md §5 Water changes).
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
	r.Get("/water-change-schedules", h.listSchedules)
	r.Post("/water-change-schedules", h.createSchedule)
	r.Get("/water-changes", h.list)
	r.Post("/water-changes", h.record)
}

// Mount registers routes addressed by their own id.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Patch("/water-change-schedules/{scheduleID}", h.updateSchedule)
		r.Post("/water-changes/{eventID}/complete", h.complete)
		r.Post("/water-changes/{eventID}/skip", h.skip)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type estimateJSON struct {
	FractionOfSystem decimal.Decimal `json:"fraction_of_system"`
	SystemVolumeL    decimal.Decimal `json:"system_volume_l"`
	Basis            string          `json:"basis"`
	Assumptions      []string        `json:"assumptions"`
	Flags            []string        `json:"flags"`
}

func toEstimate(e *Estimate) *estimateJSON {
	if e == nil {
		return nil
	}
	return &estimateJSON{FractionOfSystem: e.FractionOfSystem, SystemVolumeL: e.SystemVolumeL, Basis: e.Basis, Assumptions: e.Assumptions, Flags: e.Flags}
}

type scheduleResponse struct {
	ID              uuid.UUID       `json:"id"`
	TankID          uuid.UUID       `json:"tank_id"`
	RRule           string          `json:"rrule"`
	Timezone        string          `json:"timezone"`
	StartsOn        string          `json:"starts_on"`
	EndsOn          *string         `json:"ends_on"`
	PlannedVolumeL  decimal.Decimal `json:"planned_volume_l"`
	SaltMix         *string         `json:"salt_mix"`
	Notes           *string         `json:"notes"`
	Status          string          `json:"status"`
	NextOccurrences []time.Time     `json:"next_occurrences"`
	Estimate        *estimateJSON   `json:"estimate"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Version         int32           `json:"version"`
}

func toSchedule(s Schedule) scheduleResponse {
	out := scheduleResponse{
		ID: s.ID, TankID: s.TankID, RRule: s.RRule, Timezone: s.Timezone, StartsOn: s.StartsOn.String(),
		PlannedVolumeL: s.PlannedVolumeL, SaltMix: s.SaltMix, Notes: s.Notes, Status: s.Status,
		NextOccurrences: make([]time.Time, len(s.Next)), Estimate: toEstimate(s.Estimate),
		CreatedAt: s.CreatedAt.UTC(), UpdatedAt: s.UpdatedAt.UTC(), Version: s.Version,
	}
	for i, t := range s.Next {
		out.NextOccurrences[i] = t.UTC()
	}
	if s.EndsOn != nil {
		e := s.EndsOn.String()
		out.EndsOn = &e
	}
	return out
}

type eventResponse struct {
	ID             uuid.UUID        `json:"id"`
	TankID         uuid.UUID        `json:"tank_id"`
	ScheduleID     *uuid.UUID       `json:"schedule_id"`
	ScheduledFor   *time.Time       `json:"scheduled_for"`
	PlannedVolumeL *decimal.Decimal `json:"planned_volume_l"`
	Status         string           `json:"status"`
	Overdue        bool             `json:"overdue"`
	CompletedAt    *time.Time       `json:"completed_at"`
	ActualVolumeL  *decimal.Decimal `json:"actual_volume_l"`
	SaltMix        *string          `json:"salt_mix"`
	Notes          *string          `json:"notes"`
	SkipReason     *string          `json:"skip_reason"`
	ResolvedAt     *time.Time       `json:"resolved_at"`
	Estimate       *estimateJSON    `json:"estimate"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Version        int32            `json:"version"`
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func toEvent(e Event) eventResponse {
	return eventResponse{
		ID: e.ID, TankID: e.TankID, ScheduleID: e.ScheduleID, ScheduledFor: utcPtr(e.ScheduledFor),
		PlannedVolumeL: e.PlannedVolumeL, Status: e.Status, Overdue: e.Overdue, CompletedAt: utcPtr(e.CompletedAt),
		ActualVolumeL: e.ActualVolumeL, SaltMix: e.SaltMix, Notes: e.Notes, SkipReason: e.SkipReason,
		ResolvedAt: utcPtr(e.ResolvedAt), Estimate: toEstimate(e.Estimate),
		CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(), Version: e.Version,
	}
}

func tankID(r *http.Request) (uuid.UUID, error) { return tanks.PathID(r, "tankID") }

type scheduleRequest struct {
	RRule          string  `json:"rrule"`
	Timezone       string  `json:"timezone"`
	StartsOn       string  `json:"starts_on"`
	EndsOn         *string `json:"ends_on"`
	PlannedVolumeL string  `json:"planned_volume_l"`
	SaltMix        *string `json:"salt_mix"`
	Notes          *string `json:"notes"`
}

func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := tankID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req scheduleRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.CreateSchedule(r.Context(), tanks.ActorFrom(r), id, ScheduleInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toSchedule(s))
}

func (h *Handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	id, err := tankID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	list, err := h.svc.ListSchedules(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]scheduleResponse, len(list))
	for i, s := range list {
		items[i] = toSchedule(s)
	}
	// A tank has few schedules; the list is not paginated.
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[scheduleResponse]{Items: items})
}

type schedulePatchRequest struct {
	Version        *int32              `json:"version"`
	RRule          patch.Field[string] `json:"rrule"`
	Timezone       patch.Field[string] `json:"timezone"`
	StartsOn       patch.Field[string] `json:"starts_on"`
	EndsOn         patch.Field[string] `json:"ends_on"`
	PlannedVolumeL patch.Field[string] `json:"planned_volume_l"`
	SaltMix        patch.Field[string] `json:"salt_mix"`
	Notes          patch.Field[string] `json:"notes"`
	Status         patch.Field[string] `json:"status"`
}

func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "scheduleID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req schedulePatchRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.UpdateSchedule(r.Context(), tanks.ActorFrom(r), id, SchedulePatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toSchedule(s))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, err := tankID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[ListCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := ListFilter{Limit: page.FetchLimit(), Cursor: page.Cursor}
	q := r.URL.Query()
	if st := q.Get("status"); st != "" {
		for _, s := range strings.Split(st, ",") {
			switch s {
			case "overdue":
				f.Overdue = true
			case "scheduled":
				f.Upcoming = true
			case "completed":
				f.Completed = true
			case "skipped":
				f.Skipped = true
			default:
				h.fail(w, r, apperr.BadRequest("invalid_query", "Unknown status filter.",
					apperr.FieldError{Field: "status", Code: "unsupported", Params: map[string]string{"allowed": "overdue,scheduled,completed,skipped"}}))
				return
			}
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
	events, err := h.svc.List(r.Context(), tanks.ActorFrom(r), id, f)
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
		if e.CompletedAt != nil {
			at = e.CompletedAt
		}
		return ListCursor{At: *at, ID: e.ID}
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

type recordRequest struct {
	ScheduledFor   *time.Time `json:"scheduled_for"`
	PlannedVolumeL *string    `json:"planned_volume_l"`
	CompletedAt    *time.Time `json:"completed_at"`
	ActualVolumeL  *string    `json:"actual_volume_l"`
	SaltMix        *string    `json:"salt_mix"`
	Notes          *string    `json:"notes"`
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	id, err := tankID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req recordRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Record(r.Context(), tanks.ActorFrom(r), id, RecordInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toEvent(e))
}

type completeRequest struct {
	CompletedAt   *time.Time `json:"completed_at"`
	ActualVolumeL string     `json:"actual_volume_l"`
	SaltMix       *string    `json:"salt_mix"`
	Notes         *string    `json:"notes"`
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "eventID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req completeRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Complete(r.Context(), tanks.ActorFrom(r), id, CompleteInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEvent(e))
}

type skipRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) skip(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "eventID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req skipRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Skip(r.Context(), tanks.ActorFrom(r), id, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEvent(e))
}
