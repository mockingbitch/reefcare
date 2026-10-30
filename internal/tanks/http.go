package tanks

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/audit"
	"reefcare/internal/identity"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/patch"
)

// Handler exposes tanks over HTTP (docs/api-guidelines.md §5 Tanks).
type Handler struct {
	svc  *Service
	auth func(http.Handler) http.Handler
	log  *slog.Logger
}

// NewHandler returns the HTTP handler; requireAuth is identity.Handler.RequireAuth.
func NewHandler(svc *Service, requireAuth func(http.Handler) http.Handler, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: requireAuth, log: log}
}

// Mount registers the tank routes. Each sub mounts a tank-owned module's routes under
// /tanks/{tankID} (authentication applied); the module still authorizes every request
// through Authorizer.
func (h *Handler) Mount(r chi.Router, subs ...func(chi.Router)) {
	r.Route("/tanks", func(r chi.Router) {
		r.Use(h.auth)
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Route("/{tankID}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Patch("/", h.update)
			r.Post("/archive", h.archive)
			r.Get("/shares", h.listShares)
			r.Post("/shares", h.grant)
			r.Delete("/shares/{userID}", h.revoke)
			for _, sub := range subs {
				sub(r)
			}
		})
	})
}

// ActorFrom builds the Actor of an authenticated request, for tank-owned modules.
func ActorFrom(r *http.Request) Actor { return actor(r) }

func actor(r *http.Request) Actor {
	p, _ := identity.PrincipalFrom(r.Context())
	ip := httpserver.ClientIPFrom(r.Context())
	return Actor{
		UserID: p.UserID, EmailVerified: p.EmailVerified,
		Origin: audit.Origin{RequestID: httpserver.RequestIDFrom(r.Context()), IPPrefix: httpserver.TruncatedIP(ip), UserAgent: r.UserAgent()},
	}
}

// PathID parses a UUID path parameter; malformed IDs are 404 like unknown ones.
func PathID(r *http.Request, name string) (uuid.UUID, error) { return pathID(r, name) }

// pathID parses a UUID path parameter. A malformed ID is reported as 404, like any
// tank the caller cannot see.
func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.NotFound()
	}
	return id, nil
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type dimensionsJSON struct {
	Length decimal.Decimal `json:"length"`
	Width  decimal.Decimal `json:"width"`
	Height decimal.Decimal `json:"height"`
}

type tankResponse struct {
	ID             uuid.UUID        `json:"id"`
	OwnerID        uuid.UUID        `json:"owner_id"`
	Access         string           `json:"access"`
	Name           string           `json:"name"`
	Description    *string          `json:"description"`
	TankType       string           `json:"tank_type"`
	Status         string           `json:"status"`
	DimensionsCm   *dimensionsJSON  `json:"dimensions_cm"`
	DisplayVolumeL decimal.Decimal  `json:"display_volume_l"`
	SumpVolumeL    *decimal.Decimal `json:"sump_volume_l"`
	SystemVolumeL  decimal.Decimal  `json:"system_volume_l"`
	SetupDate      *string          `json:"setup_date"`
	WaterSource    *string          `json:"water_source"`
	SaltMix        *string          `json:"salt_mix"`
	ArchivedAt     *time.Time       `json:"archived_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Version        int32            `json:"version"`
}

func toResponse(t Tank) tankResponse {
	out := tankResponse{
		ID: t.ID, OwnerID: t.OwnerID, Access: t.Access, Name: t.Name, Description: t.Description,
		TankType: t.TankType, Status: t.Status, DisplayVolumeL: t.DisplayVolumeL, SumpVolumeL: t.SumpVolumeL,
		SystemVolumeL: t.SystemVolumeL, WaterSource: t.WaterSource, SaltMix: t.SaltMix,
		CreatedAt: t.CreatedAt.UTC(), UpdatedAt: t.UpdatedAt.UTC(), Version: t.Version,
	}
	if d := t.Dimensions; d != nil {
		out.DimensionsCm = &dimensionsJSON{Length: d.Length, Width: d.Width, Height: d.Height}
	}
	if t.SetupDate != nil {
		s := t.SetupDate.Format(time.DateOnly)
		out.SetupDate = &s
	}
	if t.ArchivedAt != nil {
		at := t.ArchivedAt.UTC()
		out.ArchivedAt = &at
	}
	return out
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpserver.ParsePageParams[ListCursor](r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := ListParams{Limit: page.FetchLimit(), Cursor: page.Cursor}
	if inc := r.URL.Query().Get("include"); inc != "" {
		for _, part := range strings.Split(inc, ",") {
			switch part {
			case "shared":
				p.IncludeShared = true
			case "archived":
				p.IncludeArchived = true
			default:
				h.fail(w, r, apperr.BadRequest("invalid_query", "Unknown include value.",
					apperr.FieldError{Field: "include", Code: "unsupported", Params: map[string]string{"allowed": "shared,archived"}}))
				return
			}
		}
	}
	tanks, err := h.svc.List(r.Context(), actor(r), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]tankResponse, len(tanks))
	for i, t := range tanks {
		items[i] = toResponse(t)
	}
	out, err := httpserver.NewPage(items, page.Limit, func(t tankResponse) any { return ListCursor{CreatedAt: t.CreatedAt, ID: t.ID} })
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

type createRequest struct {
	Name           string           `json:"name"`
	Description    *string          `json:"description"`
	TankType       string           `json:"tank_type"`
	Status         *string          `json:"status"`
	DimensionsCm   *DimensionsInput `json:"dimensions_cm"`
	DisplayVolumeL string           `json:"display_volume_l"`
	SumpVolumeL    *string          `json:"sump_volume_l"`
	SystemVolumeL  string           `json:"system_volume_l"`
	SetupDate      *string          `json:"setup_date"`
	WaterSource    *string          `json:"water_source"`
	SaltMix        *string          `json:"salt_mix"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.Create(r.Context(), actor(r), CreateInput{
		Name: req.Name, Description: req.Description, TankType: req.TankType, Status: req.Status,
		Dimensions: req.DimensionsCm, DisplayVolumeL: req.DisplayVolumeL, SumpVolumeL: req.SumpVolumeL,
		SystemVolumeL: req.SystemVolumeL, SetupDate: req.SetupDate, WaterSource: req.WaterSource, SaltMix: req.SaltMix,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/tanks/"+t.ID.String())
	httpserver.WriteJSON(w, r, http.StatusCreated, toResponse(t))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err == nil {
		var t Tank
		if t, err = h.svc.Get(r.Context(), actor(r), id); err == nil {
			httpserver.WriteJSON(w, r, http.StatusOK, toResponse(t))
			return
		}
	}
	h.fail(w, r, err)
}

type patchRequest struct {
	Version        *int32                       `json:"version"`
	Name           patch.Field[string]          `json:"name"`
	Description    patch.Field[string]          `json:"description"`
	TankType       patch.Field[string]          `json:"tank_type"`
	Status         patch.Field[string]          `json:"status"`
	DimensionsCm   patch.Field[DimensionsInput] `json:"dimensions_cm"`
	DisplayVolumeL patch.Field[string]          `json:"display_volume_l"`
	SumpVolumeL    patch.Field[string]          `json:"sump_volume_l"`
	SystemVolumeL  patch.Field[string]          `json:"system_volume_l"`
	SetupDate      patch.Field[string]          `json:"setup_date"`
	WaterSource    patch.Field[string]          `json:"water_source"`
	SaltMix        patch.Field[string]          `json:"salt_mix"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req patchRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.Update(r.Context(), actor(r), id, Patch{
		Version: req.Version, Name: req.Name, Description: req.Description, TankType: req.TankType,
		Status: req.Status, Dimensions: req.DimensionsCm, DisplayVolumeL: req.DisplayVolumeL,
		SumpVolumeL: req.SumpVolumeL, SystemVolumeL: req.SystemVolumeL, SetupDate: req.SetupDate,
		WaterSource: req.WaterSource, SaltMix: req.SaltMix,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toResponse(t))
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err == nil {
		var t Tank
		if t, err = h.svc.Archive(r.Context(), actor(r), id); err == nil {
			httpserver.WriteJSON(w, r, http.StatusOK, toResponse(t))
			return
		}
	}
	h.fail(w, r, err)
}

type shareResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Username    *string   `json:"username"`
	Permission  string    `json:"permission"`
	CreatedAt   time.Time `json:"created_at"`
}

func toShareResponse(s Share) shareResponse {
	return shareResponse{UserID: s.UserID, DisplayName: s.DisplayName, Username: s.Username, Permission: s.Permission, CreatedAt: s.CreatedAt.UTC()}
}

func (h *Handler) listShares(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	shares, err := h.svc.ListShares(r.Context(), actor(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// At most MaxSharesPerTank rows, so the list is never paginated.
	items := make([]shareResponse, len(shares))
	for i, s := range shares {
		items[i] = toShareResponse(s)
	}
	httpserver.WriteJSON(w, r, http.StatusOK, httpserver.Page[shareResponse]{Items: items})
}

type grantRequest struct {
	Email    *string `json:"email"`
	Username *string `json:"username"`
}

func (h *Handler) grant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req grantRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.Grant(r.Context(), actor(r), id, ShareInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusCreated, toShareResponse(s))
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "tankID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	grantee, err := pathID(r, "userID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Revoke(r.Context(), actor(r), id, grantee); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
