//go:build integration

package identity_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

const strongPassword = apptest.Password

func TestRegisterVerifyLoginLogout(t *testing.T) {
	e := apptest.New(t)
	e.Register("Lan@Example.com", "Lan")
	token := e.Outbox.LastToken(t, "verify_email", "lan@example.com")

	b := e.Browser()
	b.Expect(b.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	r := b.Login("lan@example.com", strongPassword)
	b.Expect(r, http.StatusOK, "")
	user := r.Body["user"].(map[string]any)
	if user["email"] != "lan@example.com" || user["email_verified"] != false || b.CSRF == "" {
		t.Fatalf("unexpected login body: %s", r.Raw)
	}
	if strings.Contains(r.Raw, "password") {
		t.Fatal("login response must not include password data")
	}
	var sessionCookie *http.Cookie
	for _, c := range b.HTTP.Jar.Cookies(apptest.MustURL(e.Srv.URL)) {
		if c.Name == "rc_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if sc := r.Header.Values("Set-Cookie"); !strings.Contains(strings.Join(sc, ";"), "HttpOnly") {
		t.Errorf("session cookie must be HttpOnly: %v", sc)
	}

	b.Expect(b.Do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusOK, "")
	b.Expect(b.Do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusUnprocessableEntity, "invalid_token")

	r = b.Do("GET", "/me", nil)
	b.Expect(r, http.StatusOK, "")
	if r.Body["email_verified"] != true || r.Body["timezone"] != "Asia/Ho_Chi_Minh" {
		t.Fatalf("unexpected /me: %s", r.Raw)
	}

	csrf := b.CSRF
	b.CSRF = ""
	b.Expect(b.Do("POST", "/auth/logout", nil), http.StatusForbidden, "csrf_failed")
	b.CSRF = "wrong-" + csrf
	b.Expect(b.Do("POST", "/auth/logout", nil), http.StatusForbidden, "csrf_failed")
	b.CSRF = csrf
	b.Expect(b.Do("POST", "/auth/logout", nil), http.StatusNoContent, "")
	b.Expect(b.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	// The revoked token is useless even if replayed.
	replay := e.Browser()
	replay.HTTP.Jar.SetCookies(apptest.MustURL(e.Srv.URL), []*http.Cookie{sessionCookie})
	replay.Expect(replay.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestDuplicateRegistrationIsIndistinguishable(t *testing.T) {
	e := apptest.New(t)
	b := e.Browser()
	body := map[string]string{"email": "minh@example.com", "password": strongPassword, "display_name": "Minh"}
	first := b.Do("POST", "/auth/register", body)
	body["email"] = "MINH@example.com"
	body["display_name"] = "Someone else"
	second := b.Do("POST", "/auth/register", body)

	if first.Status != second.Status || first.Raw != second.Raw {
		t.Fatalf("responses differ: %d %s vs %d %s", first.Status, first.Raw, second.Status, second.Raw)
	}
	// Emails come from jobs; wait for both, then check nothing else was sent.
	e.Outbox.WaitFor(t, "verify_email", "minh@example.com", 0)
	e.Outbox.WaitFor(t, "already_registered", "minh@example.com", 0)
	kinds := []string{}
	for _, m := range e.Outbox.All() {
		kinds = append(kinds, m.Kind)
	}
	slices.Sort(kinds)
	if strings.Join(kinds, ",") != "already_registered,verify_email" {
		t.Fatalf("emails sent: %v", kinds)
	}
	var users int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("users = %d, err %v", users, err)
	}
}

func TestRegisterValidation(t *testing.T) {
	e := apptest.New(t)
	b := e.Browser()

	r := b.Do("POST", "/auth/register", map[string]string{
		"email": "not-an-email", "password": "short", "display_name": "", "locale": "fr", "timezone": "Mars/Olympus",
	})
	b.Expect(r, http.StatusUnprocessableEntity, "validation_failed")
	got := map[string]string{}
	for _, d := range r.Body["error"].(map[string]any)["details"].([]any) {
		m := d.(map[string]any)
		got[m["field"].(string)] = m["code"].(string)
	}
	want := map[string]string{"email": "invalid_email", "password": "too_short", "display_name": "length", "locale": "unsupported", "timezone": "invalid_timezone"}
	for f, c := range want {
		if got[f] != c {
			t.Errorf("field %s: got %q want %q (all: %v)", f, got[f], c, got)
		}
	}

	b.Expect(b.Do("POST", "/auth/register", map[string]string{
		"email": "a@example.com", "password": "Password123", "display_name": "A",
	}), http.StatusUnprocessableEntity, "validation_failed")

	b.Expect(b.Do("POST", "/auth/register", map[string]any{"email": "a@example.com", "role": "administrator"}),
		http.StatusBadRequest, "unknown_field")
	if n := len(e.Outbox.All()); n != 0 {
		t.Fatalf("no email should be sent for invalid input, got %d", n)
	}
}

func TestLoginFailuresAreUniformAndRateLimited(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	b := e.Browser()

	unknown := b.Login("nobody@example.com", strongPassword)
	wrong := b.Login("lan@example.com", "wrong password here")
	b.Expect(unknown, http.StatusUnauthorized, "invalid_credentials")
	b.Expect(wrong, http.StatusUnauthorized, "invalid_credentials")
	if unknown.Body["error"].(map[string]any)["message"] != wrong.Body["error"].(map[string]any)["message"] {
		t.Fatal("unknown-email and wrong-password responses must be identical")
	}

	for range 9 {
		b.Login("lan@example.com", "wrong password here")
	}
	r := b.Login("lan@example.com", strongPassword)
	b.Expect(r, http.StatusTooManyRequests, "rate_limited")
	if r.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}

	// The limit refills over time (10 per 15 minutes).
	e.Clock.Advance(15 * time.Minute)
	b.Expect(b.Login("lan@example.com", strongPassword), http.StatusOK, "")
}

func TestPasswordResetRevokesAllSessions(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	laptop, phone := e.Browser(), e.Browser()
	laptop.Expect(laptop.Login("lan@example.com", strongPassword), http.StatusOK, "")
	phone.Expect(phone.Login("lan@example.com", strongPassword), http.StatusOK, "")

	anon := e.Browser()
	known := anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "LAN@example.com"})
	unknown := anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "ghost@example.com"})
	if known.Status != http.StatusAccepted || known.Raw != unknown.Raw {
		t.Fatalf("reset request responses differ: %s vs %s", known.Raw, unknown.Raw)
	}
	token := e.Outbox.LastToken(t, "reset_password", "lan@example.com")
	for _, m := range e.Outbox.All() {
		if m.To == "ghost@example.com" {
			t.Fatal("no email may be sent to unknown addresses")
		}
	}

	anon.Expect(anon.Do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "lan"}),
		http.StatusUnprocessableEntity, "validation_failed")
	newPassword := "a brand new reef password"
	anon.Expect(anon.Do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPassword}),
		http.StatusNoContent, "")
	anon.Expect(anon.Do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPassword}),
		http.StatusUnprocessableEntity, "invalid_token")

	laptop.Expect(laptop.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	phone.Expect(phone.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	anon.Expect(anon.Login("lan@example.com", strongPassword), http.StatusUnauthorized, "invalid_credentials")
	anon.Expect(anon.Login("lan@example.com", newPassword), http.StatusOK, "")

	// Reset links expire after an hour.
	sent := len(e.Outbox.All())
	anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"})
	late := e.Outbox.TokenAfter(t, "reset_password", "lan@example.com", sent)
	e.Clock.Advance(61 * time.Minute)
	anon.Expect(anon.Do("POST", "/auth/password-reset/confirm", map[string]string{"token": late, "new_password": "yet another reef password"}),
		http.StatusUnprocessableEntity, "invalid_token")

	var audited int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'auth.password_reset'`).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("password reset audit rows = %d, err %v", audited, err)
	}
}

func TestChangePasswordKeepsCurrentSessionOnly(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	laptop, phone := e.Browser(), e.Browser()
	laptop.Expect(laptop.Login("lan@example.com", strongPassword), http.StatusOK, "")
	phone.Expect(phone.Login("lan@example.com", strongPassword), http.StatusOK, "")

	r := laptop.Do("POST", "/me/password", map[string]string{"current_password": "not my password", "new_password": "a brand new reef password"})
	laptop.Expect(r, http.StatusUnprocessableEntity, "validation_failed")
	if !strings.Contains(r.Raw, `"current_password"`) {
		t.Fatalf("expected current_password field error: %s", r.Raw)
	}

	// A copy of the laptop's session (e.g. stolen) taken before the change.
	stolen := e.Browser()
	stolen.HTTP.Jar.SetCookies(apptest.MustURL(e.Srv.URL), laptop.HTTP.Jar.Cookies(apptest.MustURL(e.Srv.URL)))
	stolen.Expect(stolen.Do("GET", "/me", nil), http.StatusOK, "")

	oldCSRF := laptop.CSRF
	r = laptop.Do("POST", "/me/password", map[string]string{"current_password": strongPassword, "new_password": "a brand new reef password"})
	laptop.Expect(r, http.StatusOK, "")
	newCSRF, _ := r.Body["csrf_token"].(string)
	if newCSRF == "" || newCSRF == oldCSRF {
		t.Fatalf("session must be rotated with a new CSRF token: %s", r.Raw)
	}
	laptop.CSRF = newCSRF
	laptop.Expect(laptop.Do("GET", "/me", nil), http.StatusOK, "")
	laptop.Expect(laptop.Do("POST", "/auth/verify-email/resend", nil), http.StatusAccepted, "") // new CSRF works

	phone.Expect(phone.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	stolen.Expect(stolen.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")

	if m := e.Outbox.WaitFor(t, "password_changed", "lan@example.com", 0); strings.Contains(m.Body, "token") {
		t.Errorf("notification must not contain tokens: %q", m.Body)
	}
}

func TestResetMarksEmailVerified(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	anon := e.Browser()
	anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"})
	token := e.Outbox.LastToken(t, "reset_password", "lan@example.com")
	anon.Expect(anon.Do("POST", "/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "a brand new reef password"}),
		http.StatusNoContent, "")
	r := anon.Login("lan@example.com", "a brand new reef password")
	anon.Expect(r, http.StatusOK, "")
	if r.Body["user"].(map[string]any)["email_verified"] != true {
		t.Fatalf("completing a reset proves mailbox control and must verify the email: %s", r.Raw)
	}
}

func TestHostileInputs(t *testing.T) {
	e := apptest.New(t)
	b := e.Browser()

	// A huge "email" is rejected before it can become a rate-limiter key.
	r := b.Login(strings.Repeat("a", 500_000)+"@example.com", strongPassword)
	b.Expect(r, http.StatusUnauthorized, "invalid_credentials")

	// Display names with control or bidi characters never reach storage or email.
	for _, name := range []string{"Lan\r\n\r\nYour account is locked: https://evil.example", "a\u202Eb"} {
		r := b.Do("POST", "/auth/register", map[string]string{"email": "x@example.com", "password": strongPassword, "display_name": name})
		b.Expect(r, http.StatusUnprocessableEntity, "validation_failed")
		if !strings.Contains(r.Raw, "invalid_characters") {
			t.Errorf("want invalid_characters for %q: %s", name, r.Raw)
		}
	}

	// Emails never contain the user-chosen display name.
	e.Register("victim@example.com", "Click http://evil.example now")
	if m := e.Outbox.WaitFor(t, "verify_email", "victim@example.com", 0); strings.Contains(m.Body, "evil.example") {
		t.Fatalf("display name leaked into email: %q", m.Body)
	}
}

func TestSuspendedAccount(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	b := e.Browser()
	b.Expect(b.Login("lan@example.com", strongPassword), http.StatusOK, "")

	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET status = 'suspended'`); err != nil {
		t.Fatal(err)
	}
	b.Expect(b.Do("GET", "/me", nil), http.StatusForbidden, "account_suspended")
	e.Browser().Expect(e.Browser().Login("lan@example.com", strongPassword), http.StatusForbidden, "account_suspended")
	// A wrong password never reveals the suspension.
	e.Browser().Expect(e.Browser().Login("lan@example.com", "wrong password here"), http.StatusUnauthorized, "invalid_credentials")
}

func TestSessionIdleExpiry(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	b := e.Browser()
	b.Expect(b.Login("lan@example.com", strongPassword), http.StatusOK, "")

	e.Clock.Advance(13 * 24 * time.Hour)
	b.Expect(b.Do("GET", "/me", nil), http.StatusOK, "") // activity slides the idle timeout
	e.Clock.Advance(13 * 24 * time.Hour)
	b.Expect(b.Do("GET", "/me", nil), http.StatusOK, "")
	e.Clock.Advance(15 * 24 * time.Hour)
	b.Expect(b.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestCrossSiteRequestsRejected(t *testing.T) {
	e := apptest.New(t)
	b := e.Browser()
	body := map[string]string{"email": "lan@example.com", "password": strongPassword}
	b.Expect(b.Do("POST", "/auth/login", body, "Origin", "https://evil.example"), http.StatusForbidden, "csrf_failed")
	b.Expect(b.Do("POST", "/auth/login", body, "Sec-Fetch-Site", "cross-site"), http.StatusForbidden, "csrf_failed")
	b.Expect(b.Do("POST", "/auth/login", body, "Origin", "http://reefcare.test"), http.StatusUnauthorized, "invalid_credentials")
}
