package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"reefcare/internal/platform/apperr"
)

type createTank struct {
	Name   string `json:"name"`
	Volume int    `json:"volume"`
}

func jsonRequest(body, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantKind    apperr.Kind
		wantCode    string
		wantField   string
	}{
		{name: "ok", body: `{"name":"Nano","volume":60}`, contentType: "application/json"},
		{name: "ok with charset", body: `{"name":"Nano"}`, contentType: "application/json; charset=utf-8"},
		{name: "wrong content type", body: `{}`, contentType: "text/plain", wantKind: apperr.KindUnsupportedMedia, wantCode: "unsupported_media_type"},
		{name: "missing content type", body: `{}`, wantKind: apperr.KindUnsupportedMedia, wantCode: "unsupported_media_type"},
		{name: "empty body", body: ``, contentType: "application/json", wantKind: apperr.KindBadRequest, wantCode: "malformed_json"},
		{name: "syntax error", body: `{"name":`, contentType: "application/json", wantKind: apperr.KindBadRequest, wantCode: "malformed_json"},
		{name: "unknown field", body: `{"name":"x","owner_id":"y"}`, contentType: "application/json", wantKind: apperr.KindBadRequest, wantCode: "unknown_field", wantField: "owner_id"},
		{name: "wrong type", body: `{"volume":"sixty"}`, contentType: "application/json", wantKind: apperr.KindBadRequest, wantCode: "malformed_json", wantField: "volume"},
		{name: "trailing data", body: `{"name":"a"}{"name":"b"}`, contentType: "application/json", wantKind: apperr.KindBadRequest, wantCode: "malformed_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst createTank
			err := DecodeJSON(jsonRequest(tt.body, tt.contentType), &dst)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			e, ok := apperr.As(err)
			if !ok || e.Kind != tt.wantKind || e.Code != tt.wantCode {
				t.Fatalf("got %v, want kind %d code %s", err, tt.wantKind, tt.wantCode)
			}
			if tt.wantField != "" && (len(e.Fields) != 1 || e.Fields[0].Field != tt.wantField) {
				t.Errorf("fields = %+v, want %s", e.Fields, tt.wantField)
			}
		})
	}
}

func TestDecodeJSONBodyTooLarge(t *testing.T) {
	r := jsonRequest(`{"name":"`+strings.Repeat("x", 100)+`"}`, "application/json")
	rec := httptest.NewRecorder()
	r.Body = http.MaxBytesReader(rec, r.Body, 32)
	err := DecodeJSON(r, &createTank{})
	if !apperr.IsKind(err, apperr.KindTooLarge) {
		t.Fatalf("want too large, got %v", err)
	}
}

func TestWriteAppError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", func() error {
			v := apperr.NewValidator()
			v.Add("readings[0].value", "out_of_range", map[string]string{"min": "0", "max": "30"})
			return v.Err()
		}(), 422, "validation_failed"},
		{"not found", apperr.NotFound(), 404, "not_found"},
		{"conflict", apperr.Conflict("version_conflict", "Stale."), 409, "version_conflict"},
		{"rate limited", apperr.RateLimited(1500 * time.Millisecond), 429, "rate_limited"},
		{"unknown error hides details", errors.New("pq: password=hunter2 failed"), 500, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteAppError(rec, httptest.NewRequest(http.MethodGet, "/", nil), discardLogger(), tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body map[string]ErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"].Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body["error"].Code, tt.wantCode)
			}
			if strings.Contains(rec.Body.String(), "hunter2") {
				t.Error("internal error details leaked")
			}
			if tt.wantStatus == 429 && rec.Header().Get("Retry-After") != "2" {
				t.Errorf("Retry-After = %q, want rounded-up seconds", rec.Header().Get("Retry-After"))
			}
			if tt.wantCode == "validation_failed" {
				d := body["error"].Details
				if len(d) != 1 || d[0].Field != "readings[0].value" || d[0].Params["max"] != "30" {
					t.Errorf("details = %+v", d)
				}
			}
		})
	}
}

type timelineKey struct {
	OccurredAt time.Time `json:"t"`
	ID         string    `json:"id"`
}

func TestPagination(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=2", nil)
	p, err := ParsePageParams[timelineKey](r)
	if err != nil || p.Limit != 2 || p.Cursor != nil {
		t.Fatalf("parse: %+v %v", p, err)
	}

	// Simulate a LIMIT limit+1 query returning 3 rows.
	base := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	rows := []timelineKey{{base, "c"}, {base.Add(-time.Hour), "b"}, {base.Add(-2 * time.Hour), "a"}}
	page, err := NewPage(rows, p.Limit, func(k timelineKey) any { return k })
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("page = %+v", page)
	}

	r = httptest.NewRequest(http.MethodGet, "/?cursor="+*page.NextCursor, nil)
	p, err = ParsePageParams[timelineKey](r)
	if err != nil || p.Cursor == nil || p.Cursor.ID != "b" || !p.Cursor.OccurredAt.Equal(rows[1].OccurredAt) {
		t.Fatalf("cursor round-trip: %+v %v", p.Cursor, err)
	}

	last, _ := NewPage(rows[:1], 2, func(k timelineKey) any { return k })
	if last.NextCursor != nil {
		t.Error("last page must not have a next cursor")
	}
	empty, _ := NewPage[timelineKey](nil, 2, func(k timelineKey) any { return k })
	if b, _ := json.Marshal(empty); !strings.Contains(string(b), `"items":[]`) {
		t.Errorf("empty page must serialize items as []: %s", b)
	}

	for _, q := range []string{"limit=0", "limit=201", "limit=abc", "cursor=!!!", "cursor=bm90LWpzb24", "cursor=bnVsbA"} {
		r := httptest.NewRequest(http.MethodGet, "/?"+q, nil)
		if _, err := ParsePageParams[timelineKey](r); !apperr.IsKind(err, apperr.KindBadRequest) {
			t.Errorf("%s: want bad request, got %v", q, err)
		}
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	var got netip.Addr
	h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIPFrom(r.Context())
	}))

	tests := []struct {
		name, remote, xff, want string
	}{
		{"direct client, header ignored", "203.0.113.7:5555", "1.2.3.4", "203.0.113.7"},
		{"trusted proxy", "10.0.0.2:443", "198.51.100.9", "198.51.100.9"},
		{"spoofed left-most hop ignored", "10.0.0.2:443", "1.1.1.1, 198.51.100.9", "198.51.100.9"},
		{"chain of trusted proxies", "10.0.0.2:443", "198.51.100.9, 10.0.0.5", "198.51.100.9"},
		{"garbage header falls back to proxy", "10.0.0.2:443", "not-an-ip", "10.0.0.2"},
		{"hop with port", "10.0.0.2:443", "198.51.100.9:51000", "198.51.100.9"},
		{"bracketed ipv6 hop with port", "10.0.0.2:443", "[2001:db8::7]:51000", "2001:db8::7"},
		{"bracketed ipv6 hop", "10.0.0.2:443", "[2001:db8::7]", "2001:db8::7"},
		{"ipv6 client", "[2001:db8::1]:1234", "", "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			h.ServeHTTP(httptest.NewRecorder(), r)
			if got.String() != tt.want {
				t.Errorf("client ip = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRateLimitKey(t *testing.T) {
	a := RateLimitKey(netip.MustParseAddr("2001:db8:1:2:aaaa::1"))
	b := RateLimitKey(netip.MustParseAddr("2001:db8:1:2:bbbb::9"))
	if a != b || a != "2001:db8:1:2::/64" {
		t.Errorf("addresses in one /64 must share a key: %q %q", a, b)
	}
	if k := RateLimitKey(netip.MustParseAddr("203.0.113.7")); k != "203.0.113.7" {
		t.Errorf("ipv4 key = %q", k)
	}
	if k := RateLimitKey(netip.Addr{}); k != "unknown" {
		t.Errorf("invalid key = %q", k)
	}
	if k := RateLimitNetKey(netip.MustParseAddr("2001:db8:1:ffff::1")); k != "2001:db8:1::/48" {
		t.Errorf("ipv6 net key = %q", k)
	}
	if k := RateLimitNetKey(netip.MustParseAddr("203.0.113.7")); k != "" {
		t.Errorf("ipv4 must have no net tier, got %q", k)
	}
}

func TestDecodeJSONTrailingDataOverLimitIs413(t *testing.T) {
	r := jsonRequest(`{"name":"a"}`+strings.Repeat(" ", 20)+`{"name":"`+strings.Repeat("x", 100)+`"}`, "application/json")
	r.Body = http.MaxBytesReader(httptest.NewRecorder(), r.Body, 40)
	if err := DecodeJSON(r, &createTank{}); !apperr.IsKind(err, apperr.KindTooLarge) {
		t.Fatalf("want too large, got %v", err)
	}
}

func TestRetryAfterIsAtLeastOneSecond(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAppError(rec, httptest.NewRequest(http.MethodGet, "/", nil), discardLogger(), apperr.RateLimited(time.Millisecond))
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}
}

func TestWrongTypeUsesJSONVocabulary(t *testing.T) {
	err := DecodeJSON(jsonRequest(`{"volume":"sixty"}`, "application/json"), &createTank{})
	e, ok := apperr.As(err)
	if !ok || len(e.Fields) != 1 || e.Fields[0].Params["expected"] != "number" {
		t.Fatalf("got %+v", err)
	}
}

func TestTypedNilAppErrorIsInternal(t *testing.T) {
	var typedNil *apperr.Error
	var err error = typedNil
	rec := httptest.NewRecorder()
	WriteAppError(rec, httptest.NewRequest(http.MethodGet, "/", nil), discardLogger(), err)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTruncatedIP(t *testing.T) {
	if p := TruncatedIP(netip.MustParseAddr("203.0.113.77")); p == nil || p.String() != "203.0.113.0/24" {
		t.Errorf("ipv4: %v", p)
	}
	if p := TruncatedIP(netip.MustParseAddr("2001:db8:abcd:12::1")); p == nil || p.String() != "2001:db8:abcd::/48" {
		t.Errorf("ipv6: %v", p)
	}
	if TruncatedIP(netip.Addr{}) != nil {
		t.Error("invalid address should give nil")
	}
}
