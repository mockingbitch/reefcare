// Package httpserver provides the HTTP router, shared middleware, error envelope,
// and health endpoints.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxJSONBody = 1 << 20 // 1 MiB, see docs/security.md §4

// ReadinessCheck reports whether a dependency is ready to serve traffic.
type ReadinessCheck func(ctx context.Context) error

// Options configures the root router.
type Options struct {
	// Ready lists the checks behind /readyz.
	Ready map[string]ReadinessCheck
	// TrustedProxies are peers whose X-Forwarded-For header is honored.
	TrustedProxies []netip.Prefix
	// AllowedOrigins may send state-changing requests to /api/v1 (see OriginCheck).
	AllowedOrigins []string
	// Mount registers feature-module routes on the /api/v1 sub-router.
	Mount func(api chi.Router)
	// Observe, when set, wraps every request (e.g. metrics.Middleware).
	Observe func(http.Handler) http.Handler
}

// NewRouter builds the root router with shared middleware and health endpoints.
func NewRouter(log *slog.Logger, opts Options) http.Handler {
	r := chi.NewRouter()
	// Order matters: ClientIP decides whether RequestID may trust an incoming ID, and
	// LimitBody must see the server's own ResponseWriter so oversized requests close
	// the connection after the reply.
	r.Use(ClientIP(opts.TrustedProxies), RequestID, LimitBody(maxJSONBody), AccessLog(log))
	if opts.Observe != nil {
		r.Use(opts.Observe)
	}
	r.Use(Recover(log), SecurityHeaders)

	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(log, opts.Ready))

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(OriginCheck(opts.AllowedOrigins))
		if opts.Mount != nil {
			opts.Mount(api)
		}
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		// chi only sets Allow in its default handler; RFC 9110 §15.5.6 requires it.
		if allowed := allowedMethods(r, req.URL.Path); allowed != "" {
			w.Header().Set("Allow", allowed)
		}
		WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	})
	return r
}

func allowedMethods(routes chi.Routes, path string) string {
	var allowed []string
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		if routes.Match(chi.NewRouteContext(), m, path) {
			allowed = append(allowed, m)
		}
	}
	return strings.Join(allowed, ", ")
}

// NewServer wraps handler in an http.Server with conservative timeouts.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func handleReadyz(log *slog.Logger, checks map[string]ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		status := http.StatusOK
		for name, check := range checks {
			if err := check(ctx); err != nil {
				// Details stay in logs; the public response only names the failing check.
				log.WarnContext(ctx, "readiness check failed", "check", name, "err", err)
				results[name] = "fail"
				status = http.StatusServiceUnavailable
				continue
			}
			results[name] = "ok"
		}
		overall := "ok"
		if status != http.StatusOK {
			overall = "unavailable"
		}
		WriteJSON(w, r, status, map[string]any{"status": overall, "checks": results})
	}
}
