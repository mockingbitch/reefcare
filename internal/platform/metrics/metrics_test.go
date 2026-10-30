package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMiddlewareUsesRoutePattern(t *testing.T) {
	m := New(nil)
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/api/v1/tanks/{tankID}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	for _, path := range []string{"/api/v1/tanks/abc", "/api/v1/tanks/def", "/secret-token-in-path"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("BREW", "/api/v1/tanks/abc", nil))

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	out := string(body)
	for _, want := range []string{
		`reefcare_http_requests_total{method="GET",route="/api/v1/tanks/{tankID}",status="404"} 2`,
		`reefcare_http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`reefcare_http_requests_total{method="OTHER",route="unmatched",status="405"} 1`,
		`go_goroutines`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, leak := range []string{"abc", "secret-token"} {
		if strings.Contains(out, leak) {
			t.Errorf("raw path %q leaked into metrics", leak)
		}
	}
}
