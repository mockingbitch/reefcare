package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func do(t *testing.T, h http.Handler, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	rec := do(t, NewRouter(discardLogger(), Options{}), http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("content type = %q", got)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header")
	}
}

func TestReadyz(t *testing.T) {
	ok := func(context.Context) error { return nil }
	bad := func(context.Context) error { return errors.New("connection refused to 10.0.0.5") }

	tests := []struct {
		name       string
		checks     map[string]ReadinessCheck
		wantStatus int
		wantChecks map[string]string
	}{
		{"all ok", map[string]ReadinessCheck{"database": ok, "migrations": ok}, http.StatusOK,
			map[string]string{"database": "ok", "migrations": "ok"}},
		{"one failing", map[string]ReadinessCheck{"database": ok, "migrations": bad}, http.StatusServiceUnavailable,
			map[string]string{"database": "ok", "migrations": "fail"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, NewRouter(discardLogger(), Options{Ready: tt.checks}), http.MethodGet, "/readyz", nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Error("readiness response leaks internal error details")
			}
			var body struct {
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.wantChecks {
				if body.Checks[k] != v {
					t.Errorf("check %s = %q, want %q", k, body.Checks[k], v)
				}
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	// httptest requests come from 192.0.2.1.
	viaProxy := NewRouter(discardLogger(), Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}})
	direct := NewRouter(discardLogger(), Options{})

	rec := do(t, viaProxy, http.MethodGet, "/healthz", map[string]string{RequestIDHeader: "proxy-supplied-123"})
	if got := rec.Header().Get(RequestIDHeader); got != "proxy-supplied-123" {
		t.Errorf("id from trusted proxy not propagated, got %q", got)
	}

	rec = do(t, direct, http.MethodGet, "/healthz", map[string]string{RequestIDHeader: "client-supplied-123"})
	if got := rec.Header().Get(RequestIDHeader); got == "client-supplied-123" || got == "" {
		t.Errorf("id from untrusted client must be replaced, got %q", got)
	}

	rec = do(t, viaProxy, http.MethodGet, "/healthz", map[string]string{RequestIDHeader: "bad id\n<script>"})
	got := rec.Header().Get(RequestIDHeader)
	if got == "" || strings.Contains(got, "<") {
		t.Errorf("malformed incoming id should be replaced, got %q", got)
	}
}

func TestMethodNotAllowedSetsAllow(t *testing.T) {
	h := NewRouter(discardLogger(), Options{Mount: func(api chi.Router) {
		api.Get("/tanks", func(http.ResponseWriter, *http.Request) {})
		api.Post("/tanks", func(http.ResponseWriter, *http.Request) {})
	}})
	rec := do(t, h, http.MethodDelete, "/api/v1/tanks", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") || strings.Contains(allow, "DELETE") {
		t.Errorf("Allow = %q", allow)
	}
}

func TestPanicAfterResponseStartedAbortsConnection(t *testing.T) {
	h := NewRouter(discardLogger(), Options{Mount: func(api chi.Router) {
		api.Get("/stream", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"partial":`))
			panic("boom")
		})
	}})
	defer func() {
		if rv := recover(); rv != http.ErrAbortHandler { //nolint:errorlint // sentinel identity check
			t.Fatalf("want http.ErrAbortHandler, got %v", rv)
		}
	}()
	do(t, h, http.MethodGet, "/api/v1/stream", nil)
	t.Fatal("expected the handler to abort")
}

func TestNotFoundEnvelope(t *testing.T) {
	rec := do(t, NewRouter(discardLogger(), Options{}), http.MethodGet, "/api/v1/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	e := body["error"]
	if e.Code != "not_found" || e.RequestID == "" || e.RequestID != rec.Header().Get(RequestIDHeader) {
		t.Errorf("unexpected envelope: %+v", e)
	}
}

func TestRecoverReturnsEnvelopeAndLogsRoute(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := NewRouter(log, Options{Mount: func(api chi.Router) {
		api.Get("/tanks/{tankID}", func(http.ResponseWriter, *http.Request) { panic("boom") })
	}})

	rec := do(t, h, http.MethodGet, "/api/v1/tanks/0192f3a4-secret-id?token=abc", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"internal_error"`) || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	out := logs.String()
	if !strings.Contains(out, `"route":"/api/v1/tanks/{tankID}"`) {
		t.Errorf("access log should record the route pattern: %s", out)
	}
	if strings.Contains(out, "token=abc") {
		t.Errorf("access log leaked the query string: %s", out)
	}
}
