package identity

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"reefcare/internal/audit"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/httpserver"
	"reefcare/internal/platform/patch"
)

const (
	sessionCookie = "rc_session"
	csrfCookie    = "rc_csrf"
	csrfHeader    = "X-CSRF-Token"
)

// Handler exposes identity over HTTP (docs/api-guidelines.md §5 Auth & profile).
type Handler struct {
	svc          *Service
	log          *slog.Logger
	secureCookie bool
}

// NewHandler returns the HTTP handler. secureCookie must be true whenever the public
// URL is https (always in production).
func NewHandler(svc *Service, log *slog.Logger, secureCookie bool) *Handler {
	return &Handler{svc: svc, log: log, secureCookie: secureCookie}
}

type ctxKey int

const (
	principalKey ctxKey = iota
	authErrKey
)

// PrincipalFrom returns the authenticated principal, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Authenticate resolves the session cookie, if present, into a principal on the
// context. It never rejects a request; RequireAuth does.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(h.cookieName(sessionCookie))
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		p, err := h.svc.Authenticate(ctx, c.Value)
		if err != nil {
			if _, ok := apperr.As(err); !ok {
				httpserver.WriteAppError(w, r, h.log, err)
				return
			}
			ctx = context.WithValue(ctx, authErrKey, err)
		} else {
			ctx = context.WithValue(ctx, principalKey, p)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuth rejects requests without a valid session, and state-changing requests
// whose X-CSRF-Token does not match the session or that exceed the user's write budget.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok {
			err, _ := r.Context().Value(authErrKey).(error)
			if err == nil || apperr.IsKind(err, apperr.KindUnauthenticated) {
				h.clearCookies(w)
				err = errUnauthenticated
			}
			httpserver.WriteAppError(w, r, h.log, err)
			return
		}
		if !httpserver.IsSafeMethod(r.Method) {
			if !p.ValidCSRF(r.Header.Get(csrfHeader)) {
				httpserver.WriteError(w, r, http.StatusForbidden, "csrf_failed", "Missing or invalid CSRF token.")
				return
			}
			// The general write budget (docs/api-guidelines.md §6) is charged only after
			// the CSRF check, so forged requests cannot drain a victim's budget.
			// Endpoints with stricter limits keep them as well.
			if err := allow(h.svc.limits.WritesPerUser, p.UserID.String()); err != nil {
				h.fail(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Mount registers the identity routes. Authenticate must already be in the chain.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.register)
		r.Post("/verify-email", h.verifyEmail)
		r.Post("/login", h.login)
		r.Post("/password-reset/request", h.requestReset)
		r.Post("/password-reset/confirm", h.confirmReset)
		r.Group(func(r chi.Router) {
			r.Use(h.RequireAuth)
			r.Post("/logout", h.logout)
			r.Post("/logout-all", h.logoutAll)
			r.Post("/verify-email/resend", h.resendVerification)
		})
	})
	r.Group(func(r chi.Router) {
		r.Use(h.RequireAuth)
		r.Get("/me", h.me)
		r.Patch("/me", h.updateMe)
		r.Delete("/me", h.deleteMe)
		r.Post("/me/password", h.changePassword)
	})
}

func client(r *http.Request) Client {
	ip := httpserver.ClientIPFrom(r.Context())
	return Client{
		RateKey:    httpserver.RateLimitKey(ip),
		RateNetKey: httpserver.RateLimitNetKey(ip),
		Origin: audit.Origin{
			RequestID: httpserver.RequestIDFrom(r.Context()),
			IPPrefix:  httpserver.TruncatedIP(ip),
			UserAgent: r.UserAgent(),
		},
	}
}

type userResponse struct {
	ID              uuid.UUID         `json:"id"`
	Email           string            `json:"email"`
	EmailVerified   bool              `json:"email_verified"`
	DisplayName     string            `json:"display_name"`
	Username        *string           `json:"username"`
	Bio             *string           `json:"bio"`
	ExperienceLevel *string           `json:"experience_level"`
	Locale          string            `json:"locale"`
	Timezone        string            `json:"timezone"`
	UnitPrefs       map[string]string `json:"unit_prefs"`
	NotifyPrefs     map[string]bool   `json:"notification_prefs"`
	Roles           []string          `json:"roles"`
	CreatedAt       time.Time         `json:"created_at"`
	Version         int32             `json:"version"`
}

func toUserResponse(u User) userResponse {
	return userResponse{
		ID: u.ID, Email: u.Email, EmailVerified: u.EmailVerified, DisplayName: u.DisplayName,
		Username: u.Username, Bio: u.Bio, ExperienceLevel: u.ExperienceLevel, Locale: u.Locale,
		Timezone: u.Timezone, UnitPrefs: u.UnitPrefs, NotifyPrefs: u.NotifyPrefs, Roles: u.Roles,
		CreatedAt: u.CreatedAt.UTC(), Version: u.Version,
	}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteAppError(w, r, h.log, err)
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
	Timezone    string `json:"timezone"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Register(r.Context(), client(r), RegisterInput(req)); err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusAccepted, map[string]string{"status": "verification_sent"})
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), client(r), req.Token); err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *Handler) resendVerification(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.ResendVerification(r.Context(), client(r), p); err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusAccepted, map[string]string{"status": "verification_sent"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User      userResponse `json:"user"`
	CSRFToken string       `json:"csrf_token"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.Login(r.Context(), client(r), LoginInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// Signing in again replaces the previous session rather than leaving it orphaned.
	if prev, ok := PrincipalFrom(r.Context()); ok {
		if err := h.svc.Logout(r.Context(), client(r), prev); err != nil {
			h.log.WarnContext(r.Context(), "revoke previous session", "err", err)
		}
	}
	h.setCookies(w, s)
	httpserver.WriteJSON(w, r, http.StatusOK, loginResponse{User: toUserResponse(s.User), CSRFToken: s.CSRFToken})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.Logout(r.Context(), client(r), p); err != nil {
		h.fail(w, r, err)
		return
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

type resetRequest struct {
	Email string `json:"email"`
}

func (h *Handler) requestReset(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), client(r), req.Email); err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusAccepted, map[string]string{"status": "reset_requested"})
}

type resetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) confirmReset(w http.ResponseWriter, r *http.Request) {
	var req resetConfirmRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.ConfirmPasswordReset(r.Context(), client(r), req.Token, req.NewPassword); err != nil {
		h.fail(w, r, err)
		return
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	u, err := h.svc.Me(r.Context(), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toUserResponse(u))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	s, err := h.svc.ChangePassword(r.Context(), client(r), p, req.CurrentPassword, req.NewPassword)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookies(w, s) // the session was rotated
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]string{"csrf_token": s.CSRFToken})
}

// cookieName adds the __Host- prefix whenever cookies are Secure: browsers then only
// accept the cookie from this exact host over https with Path=/ and no Domain, so a
// sibling subdomain cannot plant or overwrite a session (cookie tossing).
func (h *Handler) cookieName(base string) string {
	if h.secureCookie {
		return "__Host-" + base
	}
	return base
}

// Cookie attributes (docs/security.md §2, §4): Secure follows the public URL scheme, so it
// is always set in production (config requires https there) and off for http://localhost.
func (h *Handler) setCookies(w http.ResponseWriter, s Session) {
	maxAge := int(s.ExpiresAt.Sub(h.svc.clock.Now()).Seconds())
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is config-driven, see above
		Name: h.cookieName(sessionCookie), Value: s.Token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode,
	})
	// Deliberately not HttpOnly: the frontend reads it to send X-CSRF-Token after a reload.
	// It is useless to an attacker without the HttpOnly session cookie.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // see comment above
		Name: h.cookieName(csrfCookie), Value: s.CSRFToken, Path: "/", MaxAge: maxAge,
		Secure: h.secureCookie, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookies(w http.ResponseWriter) {
	for _, base := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // expiring cookies; attributes mirror setCookies
			Name: h.cookieName(base), Value: "", Path: "/", MaxAge: -1,
			HttpOnly: base == sessionCookie, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode,
		})
	}
}

func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.LogoutAll(r.Context(), client(r), p); err != nil {
		h.fail(w, r, err)
		return
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

type updateMeRequest struct {
	Version           *int32                          `json:"version"`
	DisplayName       patch.Field[string]             `json:"display_name"`
	Username          patch.Field[string]             `json:"username"`
	Bio               patch.Field[string]             `json:"bio"`
	ExperienceLevel   patch.Field[string]             `json:"experience_level"`
	Locale            patch.Field[string]             `json:"locale"`
	Timezone          patch.Field[string]             `json:"timezone"`
	UnitPrefs         patch.Field[map[string]*string] `json:"unit_prefs"`
	NotificationPrefs patch.Field[map[string]*bool]   `json:"notification_prefs"`
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var req updateMeRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	u, err := h.svc.UpdateProfile(r.Context(), p, ProfilePatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, toUserResponse(u))
}

type deleteMeRequest struct {
	Password string `json:"password"`
}

type deleteMeResponse struct {
	Status     string    `json:"status"`
	EraseAfter time.Time `json:"erase_after"`
}

func (h *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	var req deleteMeRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	eraseAfter, err := h.svc.DeleteAccount(r.Context(), client(r), p, req.Password)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.clearCookies(w)
	httpserver.WriteJSON(w, r, http.StatusAccepted, deleteMeResponse{Status: "deactivated", EraseAfter: eraseAfter.UTC()})
}
