package livestock

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/platform/patch"
	"reefcare/internal/tanks"
)

// Handler exposes species and livestock over HTTP (docs/api-guidelines.md §5).
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
	r.Get("/livestock", h.listEntries)
	r.Post("/livestock", h.addEntry)
}

// Mount registers routes addressed by their own id, and the species catalog.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/species", h.searchSpecies)
		r.Post("/species", h.createSpecies)
		r.Get("/livestock/{entryID}", h.getEntry)
		r.Patch("/livestock/{entryID}", h.updateEntry)
		r.Get("/livestock/{entryID}/events", h.listEvents)
		r.Post("/livestock/{entryID}/events", h.recordEvent)
		r.Post("/livestock/{entryID}/reconcile", h.reconcile)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

// --- responses ---

type speciesResponse struct {
	ID             uuid.UUID `json:"id"`
	Curated        bool      `json:"curated"`
	Category       string    `json:"category"`
	CoralType      *string   `json:"coral_type"`
	CommonNameVi   *string   `json:"common_name_vi"`
	CommonNameEn   *string   `json:"common_name_en"`
	ScientificName *string   `json:"scientific_name"`
	CreatedAt      time.Time `json:"created_at"`
}

func toSpecies(s Species) speciesResponse {
	return speciesResponse{ID: s.ID, Curated: s.Curated, Category: s.Category, CoralType: s.CoralType, CommonNameVi: s.CommonNameVi,
		CommonNameEn: s.CommonNameEn, ScientificName: s.ScientificName, CreatedAt: s.CreatedAt.UTC()}
}

type speciesRef struct {
	ID             uuid.UUID `json:"id"`
	CommonNameVi   *string   `json:"common_name_vi"`
	CommonNameEn   *string   `json:"common_name_en"`
	ScientificName *string   `json:"scientific_name"`
}

type entryResponse struct {
	ID               uuid.UUID   `json:"id"`
	TankID           uuid.UUID   `json:"tank_id"`
	Species          *speciesRef `json:"species"`
	Category         string      `json:"category"`
	CoralType        *string     `json:"coral_type"`
	DisplayName      string      `json:"display_name"`
	Form             *string     `json:"form"`
	QuarantineStatus *string     `json:"quarantine_status"`
	LocationNote     *string     `json:"location_note"`
	SourceNote       *string     `json:"source_note"`
	Notes            *string     `json:"notes"`
	CurrentQuantity  int32       `json:"current_quantity"`
	Present          bool        `json:"present"`
	AddedAt          time.Time   `json:"added_at"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Version          int32       `json:"version"`
}

func toEntry(e Entry) entryResponse {
	out := entryResponse{
		ID: e.ID, TankID: e.TankID, Category: e.Category, CoralType: e.CoralType, DisplayName: e.DisplayName, Form: e.Form,
		QuarantineStatus: e.QuarantineStatus, LocationNote: e.LocationNote,
		SourceNote: e.SourceNote, Notes: e.Notes, CurrentQuantity: e.CurrentQuantity, Present: e.CurrentQuantity > 0,
		AddedAt: e.AddedAt.UTC(), CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(), Version: e.Version,
	}
	if e.SpeciesID != nil && e.Species != nil {
		out.Species = &speciesRef{ID: *e.SpeciesID, CommonNameVi: e.Species.CommonNameVi, CommonNameEn: e.Species.CommonNameEn,
			ScientificName: e.Species.ScientificName}
	}
	return out
}

type eventResponse struct {
	ID            uuid.UUID `json:"id"`
	EntryID       uuid.UUID `json:"entry_id"`
	Type          string    `json:"type"`
	QuantityDelta int32     `json:"quantity_delta"`
	OccurredAt    time.Time `json:"occurred_at"`
	Note          *string   `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

func toEvent(e Event) eventResponse {
	return eventResponse{ID: e.ID, EntryID: e.EntryID, Type: e.Type, QuantityDelta: e.QuantityDelta,
		OccurredAt: e.OccurredAt.UTC(), Note: e.Note, CreatedAt: e.CreatedAt.UTC()}
}

func list[T, R any](items []T, f func(T) R) httpserver.Page[R] {
	out := make([]R, len(items))
	for i, it := range items {
		out[i] = f(it)
	}
	return httpserver.Page[R]{Items: out}
}

func writeCreated[R any](w http.ResponseWriter, r *http.Request, body R, created bool) {
	if !created {
		w.Header().Set(idempotency.ReplayedHeader, "true")
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, body)
}

// --- species ---

func (h *Handler) searchSpecies(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	var category *string
	if c := qs.Get("category"); c != "" {
		category = &c
	}
	var limit int32
	if l := qs.Get("limit"); l != "" {
		n, err := strconv.ParseInt(l, 10, 32)
		if err != nil || n < 1 || n > 200 {
			h.fail(w, r, apperr.BadRequest("invalid_query", "Invalid limit.",
				apperr.FieldError{Field: "limit", Code: "out_of_range", Params: map[string]string{"min": "1", "max": "200"}}))
			return
		}
		limit = int32(n)
	}
	q := qs.Get("q")
	if len([]rune(q)) > 100 {
		h.fail(w, r, apperr.BadRequest("invalid_query", "Search is too long.", apperr.FieldError{Field: "q", Code: "length"}))
		return
	}
	items, err := h.svc.SearchSpecies(r.Context(), tanks.ActorFrom(r), q, category, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toSpecies))
}

type speciesRequest struct {
	Category       string  `json:"category"`
	CoralType      *string `json:"coral_type"`
	CommonNameVi   *string `json:"common_name_vi"`
	CommonNameEn   *string `json:"common_name_en"`
	ScientificName *string `json:"scientific_name"`
}

func (h *Handler) createSpecies(w http.ResponseWriter, r *http.Request) {
	var req speciesRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	sp, err := h.svc.CreateSpecies(r.Context(), tanks.ActorFrom(r), SpeciesInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toSpecies(sp))
}

// --- entries ---

type entryRequest struct {
	SpeciesID        *uuid.UUID `json:"species_id"`
	Category         *string    `json:"category"`
	CoralType        *string    `json:"coral_type"`
	DisplayName      string     `json:"display_name"`
	Quantity         *int32     `json:"quantity"`
	AddedAt          *time.Time `json:"added_at"`
	Form             *string    `json:"form"`
	QuarantineStatus *string    `json:"quarantine_status"`
	LocationNote     *string    `json:"location_note"`
	SourceNote       *string    `json:"source_note"`
	Notes            *string    `json:"notes"`
}

func (h *Handler) addEntry(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req entryRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	hash, err := idempotency.Hash("POST /tanks/"+id.String()+"/livestock", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, created, err := h.svc.AddEntry(r.Context(), tanks.ActorFrom(r), id, EntryInput(req), idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeCreated(w, r, toEntry(e), created)
}

func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	include := r.URL.Query().Get("include")
	if include != "" && include != "gone" {
		h.fail(w, r, apperr.BadRequest("invalid_query", "Unknown include value.",
			apperr.FieldError{Field: "include", Code: "unsupported", Params: map[string]string{"allowed": "gone"}}))
		return
	}
	items, err := h.svc.ListEntries(r.Context(), tanks.ActorFrom(r), id, include == "gone")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, list(items, toEntry))
}

func (h *Handler) getEntry(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "entryID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.GetEntry(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEntry(e))
}

type entryPatchRequest struct {
	Version          *int32                 `json:"version"`
	SpeciesID        patch.Field[uuid.UUID] `json:"species_id"`
	Category         patch.Field[string]    `json:"category"`
	CoralType        patch.Field[string]    `json:"coral_type"`
	DisplayName      patch.Field[string]    `json:"display_name"`
	Form             patch.Field[string]    `json:"form"`
	QuarantineStatus patch.Field[string]    `json:"quarantine_status"`
	LocationNote     patch.Field[string]    `json:"location_note"`
	SourceNote       patch.Field[string]    `json:"source_note"`
	Notes            patch.Field[string]    `json:"notes"`
}

func (h *Handler) updateEntry(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "entryID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req entryPatchRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEntry(r.Context(), tanks.ActorFrom(r), id, EntryPatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEntry(e))
}

// --- events ---

type eventRequest struct {
	Type       string     `json:"type"`
	Quantity   *int32     `json:"quantity"`
	OccurredAt *time.Time `json:"occurred_at"`
	Note       *string    `json:"note"`
}

func (h *Handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "entryID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req eventRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	hash, err := idempotency.Hash("POST /livestock/"+id.String()+"/events", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, created, err := h.svc.RecordEvent(r.Context(), tanks.ActorFrom(r), id, EventInput(req), idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeCreated(w, r, toEvent(e), created)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "entryID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[EventCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	events, err := h.svc.ListEvents(r.Context(), tanks.ActorFrom(r), id, page.FetchLimit(), page.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]eventResponse, len(events))
	for i, e := range events {
		items[i] = toEvent(e)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(e eventResponse) any { return EventCursor{At: e.OccurredAt, ID: e.ID} })
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

type reconcileRequest struct {
	Count      *int32     `json:"count"`
	OccurredAt *time.Time `json:"occurred_at"`
	Note       *string    `json:"note"`
}

func (h *Handler) reconcile(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "entryID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req reconcileRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Count == nil {
		v := apperr.NewValidator()
		v.Add("count", "required", nil)
		h.fail(w, r, v.Err())
		return
	}
	hash, err := idempotency.Hash("POST /livestock/"+id.String()+"/reconcile", req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, _, err := h.svc.Reconcile(r.Context(), tanks.ActorFrom(r), id, *req.Count, req.OccurredAt, req.Note, idempotency.Key(r), hash)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toEntry(e))
}
