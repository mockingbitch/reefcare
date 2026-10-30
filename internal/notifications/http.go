package notifications

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/identity"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/httpserver"
)

// Handler exposes the caller's notifications (docs/api-guidelines.md §5 Notifications).
type Handler struct {
	svc  *Service
	auth func(http.Handler) http.Handler
	log  *slog.Logger
}

// NewHandler returns the handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(svc *Service, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: requireAuth, log: log}
}

// Mount registers the routes.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/notifications", h.list)
		r.Get("/notifications/unread-count", h.unreadCount)
		r.Post("/notifications/read-all", h.readAll)
		r.Post("/notifications/{notificationID}/read", h.read)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

func userID(r *http.Request) uuid.UUID {
	p, _ := identity.PrincipalFrom(r.Context())
	return p.UserID
}

type notificationResponse struct {
	ID         uuid.UUID       `json:"id"`
	TankID     *uuid.UUID      `json:"tank_id"`
	Category   string          `json:"category"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	ReadAt     *time.Time      `json:"read_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpserver.ParsePageParams[Cursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	unread := false
	switch v := r.URL.Query().Get("unread"); v {
	case "", "false":
	case "true":
		unread = true
	default:
		h.fail(w, r, apperr.BadRequest("invalid_query", "unread must be true or false.", apperr.FieldError{Field: "unread", Code: "invalid"}))
		return
	}
	ns, err := h.svc.List(r.Context(), userID(r), unread, page.FetchLimit(), page.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]notificationResponse, len(ns))
	for i, n := range ns {
		items[i] = notificationResponse{ID: n.ID, TankID: n.TankID, Category: n.Category, Kind: n.Kind, Payload: n.Payload,
			OccurredAt: n.OccurredAt.UTC(), ReadAt: n.ReadAt, CreatedAt: n.CreatedAt.UTC()}
		if n.ReadAt != nil {
			t := n.ReadAt.UTC()
			items[i].ReadAt = &t
		}
	}
	out, err := httpserver.NewPage(items, page.Limit, func(n notificationResponse) any { return Cursor{At: n.CreatedAt, ID: n.ID} })
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.UnreadCount(r.Context(), userID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]any{"count": n, "capped": n >= 100})
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "notificationID"))
	if err != nil {
		h.fail(w, r, apperr.NotFound())
		return
	}
	if err := h.svc.MarkRead(r.Context(), userID(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkAllRead(r.Context(), userID(r)); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
