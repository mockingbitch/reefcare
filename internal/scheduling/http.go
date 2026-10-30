package scheduling

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/httpserver"
)

// Handler serves GET /recurrence/preview.
type Handler struct {
	clock clock.Clock
	auth  func(http.Handler) http.Handler
	log   *slog.Logger
}

// NewHandler returns the handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(clk clock.Clock, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{clock: clk, auth: requireAuth, log: log}
}

// Mount registers the preview route. It requires sign-in only to keep the endpoint off
// the public surface; it reads no stored data.
func (h *Handler) Mount(r chi.Router) {
	r.With(h.auth).Get("/recurrence/preview", h.preview)
}

type previewResponse struct {
	RRule           string      `json:"rrule"`
	NextOccurrences []time.Time `json:"next_occurrences"`
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := PreviewInput{RRule: q.Get("rrule"), Timezone: q.Get("timezone"), StartsOn: q.Get("starts_on")}
	if q.Has("ends_on") && q.Get("ends_on") != "" {
		ends := q.Get("ends_on")
		in.EndsOn = &ends
	}
	p, err := PreviewSchedule(in, h.clock.Now())
	if err != nil {
		httpserver.WriteAppError(w, r, h.log, err)
		return
	}
	out := previewResponse{RRule: p.RRule, NextOccurrences: make([]time.Time, len(p.NextOccurrences))}
	for i, t := range p.NextOccurrences {
		out.NextOccurrences[i] = t.UTC()
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}
