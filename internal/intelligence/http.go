package intelligence

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/platform/httpserver"
	"reefcare/internal/tanks"
)

// Handler exposes assessments (docs/api-guidelines.md §5 Intelligence).
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
	r.Get("/assessments", h.list)
	r.Post("/assessments", h.assess)
}

// Mount registers routes addressed by their own id.
func (h *Handler) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/assessments/{assessmentID}", h.get)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type assessmentResponse struct {
	ID               uuid.UUID  `json:"id"`
	TankID           uuid.UUID  `json:"tank_id"`
	Trigger          string     `json:"trigger"`
	WaterTestID      *uuid.UUID `json:"water_test_id"`
	Status           string     `json:"status"`
	FindingCount     int32      `json:"finding_count"`
	MaxSeverity      *string    `json:"max_severity"`
	Confidence       string     `json:"confidence_label"`
	RuleVersion      string     `json:"rule_version"`
	Provisional      bool       `json:"provisional"`
	KnowledgeVersion string     `json:"knowledge_version,omitempty"`
	SnapshotVersion  int32      `json:"input_snapshot_version,omitempty"`
	Locale           string     `json:"locale,omitempty"`
	RetractedAt      *time.Time `json:"retracted_at,omitempty"`
	RetractReason    *string    `json:"retract_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	Findings         []Finding  `json:"findings,omitempty"`
}

func toResponse(a Assessment) assessmentResponse {
	out := assessmentResponse{ID: a.ID, TankID: a.TankID, Trigger: a.Trigger, WaterTestID: a.SourceID, Status: a.Status,
		FindingCount: a.FindingCount, MaxSeverity: a.MaxSeverity, Confidence: a.Confidence, RuleVersion: a.RuleVersion,
		Provisional: a.Provisional, KnowledgeVersion: a.KnowledgeVersion, SnapshotVersion: a.SnapshotVersion, Locale: a.Locale,
		RetractedAt: a.RetractedAt, RetractReason: a.RetractReason, CreatedAt: a.CreatedAt.UTC()}
	if a.Result != nil {
		out.Findings = a.Result.Findings
		if out.Findings == nil {
			out.Findings = []Finding{}
		}
	}
	return out
}

func (h *Handler) assess(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.Assess(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toResponse(a))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpserver.ParsePageParams[Cursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	as, err := h.svc.List(r.Context(), tanks.ActorFrom(r), id, page.FetchLimit(), page.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]assessmentResponse, len(as))
	for i, a := range as {
		items[i] = toResponse(a)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(a assessmentResponse) any { return Cursor{At: a.CreatedAt, ID: a.ID} })
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := tanks.PathID(r, "assessmentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.Get(r.Context(), tanks.ActorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toResponse(a))
}
