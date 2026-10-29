//go:build integration

package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/app"
	"reefcare/internal/config"
	"reefcare/internal/identity"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database/dbtest"
	"reefcare/internal/platform/mail"
)

const strongPassword = "correct horse battery"

type outbox struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (o *outbox) Send(_ context.Context, m mail.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.msgs = append(o.msgs, m)
	return nil
}

func (o *outbox) all() []mail.Message {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]mail.Message(nil), o.msgs...)
}

// Tokens travel in the URL fragment so they never reach server logs or Referer headers.
var tokenRe = regexp.MustCompile(`#token=([A-Za-z0-9_-]+)`)

// waitFor returns the most recent message of kind sent to `to`, polling briefly because
// some emails are produced by background work after the response.
func (o *outbox) waitFor(t *testing.T, kind, to string, after int) mail.Message {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		msgs := o.all()
		for i := len(msgs) - 1; i >= after; i-- {
			if msgs[i].Kind == kind && msgs[i].To == to {
				return msgs[i]
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s email to %s; outbox: %+v", kind, to, msgs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// lastToken returns the token from the most recent message of kind sent to `to`.
func (o *outbox) lastToken(t *testing.T, kind, to string) string {
	t.Helper()
	return o.tokenAfter(t, kind, to, 0)
}

func (o *outbox) tokenAfter(t *testing.T, kind, to string, after int) string {
	t.Helper()
	m := tokenRe.FindStringSubmatch(o.waitFor(t, kind, to, after).Body)
	if m == nil {
		t.Fatalf("no fragment token in %s email", kind)
	}
	return m[1]
}

type env struct {
	t      *testing.T
	srv    *httptest.Server
	pool   *pgxpool.Pool
	clock  *clock.Fixed
	outbox *outbox
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	e := &env{t: t, pool: pool, clock: clock.NewFixed(time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)), outbox: &outbox{}}
	h, closeModules := app.NewHTTPHandler(app.Deps{
		Config: config.Config{PublicBaseURL: "http://reefcare.test", AllowedOrigins: []string{"http://reefcare.test"}},
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Pool:   pool, Clock: e.clock, Mailer: e.outbox,
		Argon2: identity.Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1},
	})
	e.srv = httptest.NewServer(h)
	t.Cleanup(func() {
		e.srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := closeModules(ctx); err != nil {
			t.Error(err)
		}
	})
	return e
}

// browser is a client with its own cookie jar, like one signed-in device.
type browser struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) browser() *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{e: e, http: &http.Client{Jar: jar}}
}

type response struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r response) errCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func (b *browser) do(method, path string, body any, headers ...string) response {
	b.e.t.Helper()
	var rd io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, b.e.srv.URL+"/api/v1"+path, rd)
	if err != nil {
		b.e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.csrf != "" {
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := b.http.Do(req)
	if err != nil {
		b.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := response{status: resp.StatusCode, header: resp.Header, raw: string(raw)}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (b *browser) expect(r response, status int, code string) {
	b.e.t.Helper()
	if r.status != status || (code != "" && r.errCode() != code) {
		b.e.t.Fatalf("got %d %s, want %d %s", r.status, r.raw, status, code)
	}
}

func (e *env) register(email, name string) {
	e.t.Helper()
	b := e.browser()
	b.expect(b.do("POST", "/auth/register", map[string]string{
		"email": email, "password": strongPassword, "display_name": name, "locale": "vi", "timezone": "Asia/Ho_Chi_Minh",
	}), http.StatusAccepted, "")
}

func (b *browser) login(email, password string) response {
	b.e.t.Helper()
	r := b.do("POST", "/auth/login", map[string]string{"email": email, "password": password})
	if r.status == http.StatusOK {
		b.csrf, _ = r.body["csrf_token"].(string)
	}
	return r
}

func TestRegisterVerifyLoginLogout(t *testing.T) {
	e := newEnv(t)
	e.register("Lan@Example.com", "Lan")
	token := e.outbox.lastToken(t, "verify_email", "lan@example.com")

	b := e.browser()
	b.expect(b.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	r := b.login("lan@example.com", strongPassword)
	b.expect(r, http.StatusOK, "")
	user := r.body["user"].(map[string]any)
	if user["email"] != "lan@example.com" || user["email_verified"] != false || b.csrf == "" {
		t.Fatalf("unexpected login body: %s", r.raw)
	}
	if strings.Contains(r.raw, "password") {
		t.Fatal("login response must not include password data")
	}
	var sessionCookie *http.Cookie
	for _, c := range b.http.Jar.Cookies(mustURL(e.srv.URL)) {
		if c.Name == "rc_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if sc := r.header.Values("Set-Cookie"); !strings.Contains(strings.Join(sc, ";"), "HttpOnly") {
		t.Errorf("session cookie must be HttpOnly: %v", sc)
	}

	b.expect(b.do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusOK, "")
	b.expect(b.do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusUnprocessableEntity, "invalid_token")

	r = b.do("GET", "/me", nil)
	b.expect(r, http.StatusOK, "")
	if r.body["email_verified"] != true || r.body["timezone"] != "Asia/Ho_Chi_Minh" {
		t.Fatalf("unexpected /me: %s", r.raw)
	}

	csrf := b.csrf
	b.csrf = ""
	b.expect(b.do("POST", "/auth/logout", nil), http.StatusForbidden, "csrf_failed")
	b.csrf = "wrong-" + csrf
	b.expect(b.do("POST", "/auth/logout", nil), http.StatusForbidden, "csrf_failed")
	b.csrf = csrf
	b.expect(b.do("POST", "/auth/logout", nil), http.StatusNoContent, "")
	b.expect(b.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	// The revoked token is useless even if replayed.
	replay := e.browser()
	replay.http.Jar.SetCookies(mustURL(e.srv.URL), []*http.Cookie{sessionCookie})
	replay.expect(replay.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestDuplicateRegistrationIsIndistinguishable(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	body := map[string]string{"email": "minh@example.com", "password": strongPassword, "display_name": "Minh"}
	first := b.do("POST", "/auth/register", body)
	body["email"] = "MINH@example.com"
	body["display_name"] = "Someone else"
	second := b.do("POST", "/auth/register", body)

	if first.status != second.status || first.raw != second.raw {
		t.Fatalf("responses differ: %d %s vs %d %s", first.status, first.raw, second.status, second.raw)
	}
	kinds := []string{}
	for _, m := range e.outbox.all() {
		kinds = append(kinds, m.Kind)
	}
	if strings.Join(kinds, ",") != "verify_email,already_registered" {
		t.Fatalf("emails sent: %v", kinds)
	}
	var users int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("users = %d, err %v", users, err)
	}
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	r := b.do("POST", "/auth/register", map[string]string{
		"email": "not-an-email", "password": "short", "display_name": "", "locale": "fr", "timezone": "Mars/Olympus",
	})
	b.expect(r, http.StatusUnprocessableEntity, "validation_failed")
	got := map[string]string{}
	for _, d := range r.body["error"].(map[string]any)["details"].([]any) {
		m := d.(map[string]any)
		got[m["field"].(string)] = m["code"].(string)
	}
	want := map[string]string{"email": "invalid_email", "password": "too_short", "display_name": "length", "locale": "unsupported", "timezone": "invalid_timezone"}
	for f, c := range want {
		if got[f] != c {
			t.Errorf("field %s: got %q want %q (all: %v)", f, got[f], c, got)
		}
	}

	b.expect(b.do("POST", "/auth/register", map[string]string{
		"email": "a@example.com", "password": "Password123", "display_name": "A",
	}), http.StatusUnprocessableEntity, "validation_failed")

	b.expect(b.do("POST", "/auth/register", map[string]any{"email": "a@example.com", "role": "administrator"}),
		http.StatusBadRequest, "unknown_field")
	if n := len(e.outbox.all()); n != 0 {
		t.Fatalf("no email should be sent for invalid input, got %d", n)
	}
}

func TestLoginFailuresAreUniformAndRateLimited(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	b := e.browser()

	unknown := b.login("nobody@example.com", strongPassword)
	wrong := b.login("lan@example.com", "wrong password here")
	b.expect(unknown, http.StatusUnauthorized, "invalid_credentials")
	b.expect(wrong, http.StatusUnauthorized, "invalid_credentials")
	if unknown.body["error"].(map[string]any)["message"] != wrong.body["error"].(map[string]any)["message"] {
		t.Fatal("unknown-email and wrong-password responses must be identical")
	}

	for range 9 {
		b.login("lan@example.com", "wrong password here")
	}
	r := b.login("lan@example.com", strongPassword)
	b.expect(r, http.StatusTooManyRequests, "rate_limited")
	if r.header.Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}

	// The limit refills over time (10 per 15 minutes).
	e.clock.Advance(15 * time.Minute)
	b.expect(b.login("lan@example.com", strongPassword), http.StatusOK, "")
}

func TestPasswordResetRevokesAllSessions(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	laptop, phone := e.browser(), e.browser()
	laptop.expect(laptop.login("lan@example.com", strongPassword), http.StatusOK, "")
	phone.expect(phone.login("lan@example.com", strongPassword), http.StatusOK, "")

	anon := e.browser()
	known := anon.do("POST", "/auth/password-reset/request", map[string]string{"email": "LAN@example.com"})
	unknown := anon.do("POST", "/auth/password-reset/request", map[string]string{"email": "ghost@example.com"})
	if known.status != http.StatusAccepted || known.raw != unknown.raw {
		t.Fatalf("reset request responses differ: %s vs %s", known.raw, unknown.raw)
	}
	token := e.outbox.lastToken(t, "reset_password", "lan@example.com")
	for _, m := range e.outbox.all() {
		if m.To == "ghost@example.com" {
			t.Fatal("no email may be sent to unknown addresses")
		}
	}

	anon.expect(anon.do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "lan"}),
		http.StatusUnprocessableEntity, "validation_failed")
	newPassword := "a brand new reef password"
	anon.expect(anon.do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPassword}),
		http.StatusNoContent, "")
	anon.expect(anon.do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPassword}),
		http.StatusUnprocessableEntity, "invalid_token")

	laptop.expect(laptop.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	phone.expect(phone.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	anon.expect(anon.login("lan@example.com", strongPassword), http.StatusUnauthorized, "invalid_credentials")
	anon.expect(anon.login("lan@example.com", newPassword), http.StatusOK, "")

	// Reset links expire after an hour.
	sent := len(e.outbox.all())
	anon.do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"})
	late := e.outbox.tokenAfter(t, "reset_password", "lan@example.com", sent)
	e.clock.Advance(61 * time.Minute)
	anon.expect(anon.do("POST", "/auth/password-reset/confirm", map[string]string{"token": late, "new_password": "yet another reef password"}),
		http.StatusUnprocessableEntity, "invalid_token")

	var audited int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'auth.password_reset'`).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("password reset audit rows = %d, err %v", audited, err)
	}
}

func TestChangePasswordKeepsCurrentSessionOnly(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	laptop, phone := e.browser(), e.browser()
	laptop.expect(laptop.login("lan@example.com", strongPassword), http.StatusOK, "")
	phone.expect(phone.login("lan@example.com", strongPassword), http.StatusOK, "")

	r := laptop.do("POST", "/me/password", map[string]string{"current_password": "not my password", "new_password": "a brand new reef password"})
	laptop.expect(r, http.StatusUnprocessableEntity, "validation_failed")
	if !strings.Contains(r.raw, `"current_password"`) {
		t.Fatalf("expected current_password field error: %s", r.raw)
	}

	// A copy of the laptop's session (e.g. stolen) taken before the change.
	stolen := e.browser()
	stolen.http.Jar.SetCookies(mustURL(e.srv.URL), laptop.http.Jar.Cookies(mustURL(e.srv.URL)))
	stolen.expect(stolen.do("GET", "/me", nil), http.StatusOK, "")

	oldCSRF := laptop.csrf
	r = laptop.do("POST", "/me/password", map[string]string{"current_password": strongPassword, "new_password": "a brand new reef password"})
	laptop.expect(r, http.StatusOK, "")
	newCSRF, _ := r.body["csrf_token"].(string)
	if newCSRF == "" || newCSRF == oldCSRF {
		t.Fatalf("session must be rotated with a new CSRF token: %s", r.raw)
	}
	laptop.csrf = newCSRF
	laptop.expect(laptop.do("GET", "/me", nil), http.StatusOK, "")
	laptop.expect(laptop.do("POST", "/auth/verify-email/resend", nil), http.StatusAccepted, "") // new CSRF works

	phone.expect(phone.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	stolen.expect(stolen.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	if m := e.outbox.waitFor(t, "password_changed", "lan@example.com", 0); strings.Contains(m.Body, "token") {
		t.Errorf("notification must not contain tokens: %q", m.Body)
	}
}

func TestResetMarksEmailVerified(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	anon := e.browser()
	anon.do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"})
	token := e.outbox.lastToken(t, "reset_password", "lan@example.com")
	anon.expect(anon.do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "a brand new reef password"}),
		http.StatusNoContent, "")
	r := anon.login("lan@example.com", "a brand new reef password")
	anon.expect(r, http.StatusOK, "")
	if r.body["user"].(map[string]any)["email_verified"] != true {
		t.Fatalf("completing a reset proves mailbox control and must verify the email: %s", r.raw)
	}
}

func TestHostileInputs(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	// A huge "email" is rejected before it can become a rate-limiter key.
	r := b.login(strings.Repeat("a", 500_000)+"@example.com", strongPassword)
	b.expect(r, http.StatusUnauthorized, "invalid_credentials")

	// Display names with control or bidi characters never reach storage or email.
	for _, name := range []string{"Lan\r\n\r\nYour account is locked: https://evil.example", "a\u202Eb"} {
		r := b.do("POST", "/auth/register", map[string]string{"email": "x@example.com", "password": strongPassword, "display_name": name})
		b.expect(r, http.StatusUnprocessableEntity, "validation_failed")
		if !strings.Contains(r.raw, "invalid_characters") {
			t.Errorf("want invalid_characters for %q: %s", name, r.raw)
		}
	}

	// Emails never contain the user-chosen display name.
	e.register("victim@example.com", "Click http://evil.example now")
	if m := e.outbox.waitFor(t, "verify_email", "victim@example.com", 0); strings.Contains(m.Body, "evil.example") {
		t.Fatalf("display name leaked into email: %q", m.Body)
	}
}

func TestSuspendedAccount(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	b := e.browser()
	b.expect(b.login("lan@example.com", strongPassword), http.StatusOK, "")

	if _, err := e.pool.Exec(context.Background(), `UPDATE users SET status = 'suspended'`); err != nil {
		t.Fatal(err)
	}
	b.expect(b.do("GET", "/me", nil), http.StatusForbidden, "account_suspended")
	e.browser().expect(e.browser().login("lan@example.com", strongPassword), http.StatusForbidden, "account_suspended")
	// A wrong password never reveals the suspension.
	e.browser().expect(e.browser().login("lan@example.com", "wrong password here"), http.StatusUnauthorized, "invalid_credentials")
}

func TestSessionIdleExpiry(t *testing.T) {
	e := newEnv(t)
	e.register("lan@example.com", "Lan")
	b := e.browser()
	b.expect(b.login("lan@example.com", strongPassword), http.StatusOK, "")

	e.clock.Advance(13 * 24 * time.Hour)
	b.expect(b.do("GET", "/me", nil), http.StatusOK, "") // activity slides the idle timeout
	e.clock.Advance(13 * 24 * time.Hour)
	b.expect(b.do("GET", "/me", nil), http.StatusOK, "")
	e.clock.Advance(15 * 24 * time.Hour)
	b.expect(b.do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestCrossSiteRequestsRejected(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	body := map[string]string{"email": "lan@example.com", "password": strongPassword}
	b.expect(b.do("POST", "/auth/login", body, "Origin", "https://evil.example"), http.StatusForbidden, "csrf_failed")
	b.expect(b.do("POST", "/auth/login", body, "Sec-Fetch-Site", "cross-site"), http.StatusForbidden, "csrf_failed")
	b.expect(b.do("POST", "/auth/login", body, "Origin", "http://reefcare.test"), http.StatusUnauthorized, "invalid_credentials")
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
