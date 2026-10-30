//go:build integration

package identity_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

func TestUpdateProfile(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")

	me := b.Expect(b.Do("GET", "/me", nil), http.StatusOK, "")
	if me.Body["version"] != float64(1) || me.Body["unit_prefs"] == nil || me.Body["notification_prefs"] == nil {
		t.Fatalf("unexpected /me: %s", me.Raw)
	}

	r := b.Expect(b.Do("PATCH", "/me", map[string]any{
		"version": 1, "display_name": "  Lan Nguyễn ", "username": "Lan_Reef", "bio": "SPS keeper\nsince 2019",
		"experience_level": "intermediate", "locale": "en", "timezone": "Europe/Berlin",
		"unit_prefs": map[string]any{"volume": "gal_us", "alkalinity": "meq/L"}, "notification_prefs": map[string]any{"email_reminders": true},
	}), http.StatusOK, "")
	if r.Body["display_name"] != "Lan Nguyễn" || r.Body["username"] != "lan_reef" || r.Body["locale"] != "en" ||
		r.Body["timezone"] != "Europe/Berlin" || r.Body["version"] != float64(2) {
		t.Fatalf("unexpected profile: %s", r.Raw)
	}
	if u := r.Body["unit_prefs"].(map[string]any); u["volume"] != "gal_us" || u["alkalinity"] != "meq/L" {
		t.Fatalf("unit_prefs: %v", u)
	}

	// Merge semantics: absent members are kept, null clears, preference keys merge.
	r = b.Expect(b.Do("PATCH", "/me", map[string]any{
		"version": 2, "bio": nil, "unit_prefs": map[string]any{"volume": nil, "temperature": "F"},
	}), http.StatusOK, "")
	if r.Body["bio"] != nil || r.Body["username"] != "lan_reef" {
		t.Fatalf("merge patch: %s", r.Raw)
	}
	if u := r.Body["unit_prefs"].(map[string]any); u["volume"] != nil || u["temperature"] != "F" || u["alkalinity"] != "meq/L" {
		t.Fatalf("unit_prefs merge: %v", u)
	}

	// A stale version is rejected, never silently overwritten.
	b.Expect(b.Do("PATCH", "/me", map[string]any{"version": 2, "display_name": "Stale"}), http.StatusConflict, "version_conflict")

	r = b.Expect(b.Do("PATCH", "/me", map[string]any{
		"display_name": nil, "username": "no", "experience_level": "guru", "locale": "fr", "timezone": "Mars/Base",
		"bio": "a\u202Eb", "unit_prefs": map[string]any{"volume": "barrel", "colour": "blue"},
		"notification_prefs": map[string]any{"sms": true},
	}), http.StatusUnprocessableEntity, "validation_failed")
	want := map[string]string{
		"version": "required", "display_name": "required", "username": "invalid_format", "experience_level": "unsupported",
		"locale": "unsupported", "timezone": "invalid_timezone", "bio": "invalid_characters",
		"unit_prefs.volume": "unsupported_unit", "unit_prefs.colour": "unknown_field", "notification_prefs.sms": "unknown_field",
	}
	got := r.FieldCodes()
	for f, c := range want {
		if got[f] != c {
			t.Errorf("field %s: got %q want %q (all %v)", f, got[f], c, got)
		}
	}

	// Fields outside the allowlist cannot be patched, and wrong JSON types name the field.
	b.Expect(b.Do("PATCH", "/me", map[string]any{"version": 3, "email": "x@example.com"}), http.StatusBadRequest, "unknown_field")
	b.Expect(b.Do("PATCH", "/me", map[string]any{"version": 3, "status": "active"}), http.StatusBadRequest, "unknown_field")
	r = b.Expect(b.Do("PATCH", "/me", map[string]any{"version": 3, "display_name": 42}), http.StatusBadRequest, "malformed_json")
	if r.FieldCodes()["display_name"] != "wrong_type" {
		t.Fatalf("want field-level wrong_type: %s", r.Raw)
	}

	// Usernames are unique, case-insensitively.
	other, _ := e.SignUp("minh@example.com", "Minh")
	other.Expect(other.Do("PATCH", "/me", map[string]any{"version": 1, "username": "LAN_REEF"}), http.StatusConflict, "username_taken")
}

func TestDeleteAccount(t *testing.T) {
	e := apptest.New(t)
	laptop, _ := e.SignUp("lan@example.com", "Lan")
	phone := e.Browser()
	phone.Expect(phone.Login("lan@example.com", apptest.Password), http.StatusOK, "")

	r := laptop.Expect(laptop.Do("DELETE", "/me", map[string]string{"password": "not my password"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["password"] != "incorrect" {
		t.Fatalf("want password incorrect: %s", r.Raw)
	}
	laptop.Expect(laptop.Do("GET", "/me", nil), http.StatusOK, "")

	r = laptop.Expect(laptop.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	if r.Body["status"] != "deactivated" || r.Body["erase_after"] != "2026-11-28T03:00:00Z" {
		t.Fatalf("unexpected body: %s", r.Raw)
	}
	laptop.Expect(laptop.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	phone.Expect(phone.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	e.Browser().Expect(e.Browser().Login("lan@example.com", apptest.Password), http.StatusUnauthorized, "invalid_credentials")

	// No reset link is issued for a deactivated account.
	before := len(e.Outbox.All())
	e.Browser().Expect(e.Browser().Do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"}), http.StatusAccepted, "")
	time.Sleep(50 * time.Millisecond)
	for _, m := range e.Outbox.All()[before:] {
		if m.Kind == "reset_password" {
			t.Fatal("reset email sent to a deactivated account")
		}
	}
	if m := e.Outbox.WaitFor(t, "account_deactivated", "lan@example.com", 0); strings.Contains(m.Body, "token") {
		t.Fatalf("notification must not carry tokens: %q", m.Body)
	}

	var status string
	var audited int
	ctx := context.Background()
	if err := e.Pool.QueryRow(ctx, `SELECT status FROM users`).Scan(&status); err != nil || status != "deactivated" {
		t.Fatalf("status %q, err %v", status, err)
	}
	if err := e.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'account.deactivated'`).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("audit rows %d, err %v", audited, err)
	}
}

func TestLogoutAll(t *testing.T) {
	e := apptest.New(t)
	laptop, _ := e.SignUp("lan@example.com", "Lan")
	phone := e.Browser()
	phone.Expect(phone.Login("lan@example.com", apptest.Password), http.StatusOK, "")

	laptop.Expect(laptop.Do("POST", "/auth/logout-all", nil), http.StatusNoContent, "")
	laptop.Expect(laptop.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	phone.Expect(phone.Do("GET", "/me", nil), http.StatusUnauthorized, "unauthenticated")
	// Signing in again works: logout-all does not lock the account.
	laptop.Expect(laptop.Login("lan@example.com", apptest.Password), http.StatusOK, "")
}

// A verification link is bound to the address it was sent to (open item S3).
func TestVerificationTokenBoundToAddress(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	token := e.Outbox.LastToken(t, "verify_email", "lan@example.com")
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET email = 'other@example.com'`); err != nil {
		t.Fatal(err)
	}
	b := e.Browser()
	b.Expect(b.Do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusUnprocessableEntity, "invalid_token")
}

func TestGeneralWriteLimit(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	for i := range 120 {
		r := b.Do("PATCH", "/me", map[string]any{})
		if r.Status == http.StatusTooManyRequests {
			t.Fatalf("limited after %d writes", i)
		}
	}
	r := b.Expect(b.Do("PATCH", "/me", map[string]any{}), http.StatusTooManyRequests, "rate_limited")
	if r.Header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	b.Expect(b.Do("GET", "/me", nil), http.StatusOK, "") // reads are not limited
	e.Clock.Advance(time.Minute)

	// Forged writes (no valid CSRF token) are rejected before they use up the budget.
	csrf := b.CSRF
	b.CSRF = "forged"
	for range 130 {
		b.Expect(b.Do("PATCH", "/me", map[string]any{}), http.StatusForbidden, "csrf_failed")
	}
	b.CSRF = csrf
	b.Expect(b.Do("PATCH", "/me", map[string]any{}), http.StatusUnprocessableEntity, "validation_failed")
}
