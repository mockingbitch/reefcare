package httpserver

import (
	"net/http"
	"slices"
)

// IsSafeMethod reports whether m never changes server state (GET, HEAD, OPTIONS).
func IsSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// OriginCheck rejects state-changing requests initiated by other sites
// (docs/security.md §4). A present Origin header must be one of allowed; without one,
// browsers' Sec-Fetch-Site is consulted. Non-browser clients send neither and pass;
// authenticated requests are additionally protected by the session CSRF token.
func OriginCheck(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsSafeMethod(r.Method) {
				origin := r.Header.Get("Origin")
				crossSite := r.Header.Get("Sec-Fetch-Site") == "cross-site"
				if (origin != "" && !slices.Contains(allowed, origin)) || (origin == "" && crossSite) {
					WriteError(w, r, http.StatusForbidden, "csrf_failed", "Cross-site request rejected.")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
