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

func count(t *testing.T, e *apptest.Env, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// waitJobs waits until no due job is pending or running, of any kind: jobs enqueue other
// jobs (erase_due → erase_account), and the background runner may be mid-way through
// one. kind only labels the failure message.
func waitJobs(t *testing.T, e *apptest.Env, kind string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for count(t, e, `SELECT count(*) FROM jobs WHERE status IN ('pending', 'running') AND run_at <= $1`, e.Clock.Now()) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%s jobs still queued", kind)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// J1: an SMTP outage delays the verification email instead of losing it, and the link
// that finally arrives is the only live one.
func TestEmailSurvivesSMTPOutage(t *testing.T) {
	e := apptest.New(t)
	e.Outbox.FailNext(2)
	e.Register("lan@example.com", "Lan")
	waitJobs(t, e, "identity.email")
	if n := len(e.Outbox.All()); n != 0 {
		t.Fatalf("no email can arrive while SMTP is down, got %d", n)
	}
	if got := count(t, e, `SELECT count(*) FROM jobs WHERE kind = 'identity.email' AND status = 'pending' AND attempts = 1 AND last_error LIKE 'send verify_email:%'`); got != 1 {
		t.Fatalf("failed email job must be scheduled for retry, got %d", got)
	}
	e.Clock.Advance(time.Minute) // first backoff
	waitJobs(t, e, "identity.email")
	e.Clock.Advance(2 * time.Minute) // second backoff
	token := e.Outbox.LastToken(t, "verify_email", "lan@example.com")

	b := e.Browser()
	b.Expect(b.Do("POST", "/auth/verify-email", map[string]string{"token": token}), http.StatusOK, "")
	if live := count(t, e, `SELECT count(*) FROM auth_tokens WHERE used_at IS NULL AND revoked_at IS NULL`); live != 0 {
		t.Fatalf("tokens of failed sends must be revoked, %d still live", live)
	}
}

// The queue never holds secrets or addresses: email jobs carry a user id only, and the
// token is created when the job runs.
func TestEmailJobsHoldNoSecrets(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	token := e.Outbox.LastToken(t, "verify_email", "lan@example.com")
	var payload string
	if err := e.Pool.QueryRow(context.Background(), `SELECT payload::text FROM jobs WHERE kind = 'identity.email'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, token) || strings.Contains(payload, "lan@example.com") || !strings.Contains(payload, `"user_id"`) {
		t.Fatalf("payload must hold only ids: %s", payload)
	}
}

// J2: failed logins are audited through a job, and unknown emails take the same path
// (one job each) so timing reveals nothing.
func TestLoginFailureAuditedThroughJob(t *testing.T) {
	e := apptest.New(t)
	e.Register("lan@example.com", "Lan")
	b := e.Browser()
	b.Expect(b.Login("lan@example.com", "wrong password here"), http.StatusUnauthorized, "invalid_credentials")
	b.Expect(b.Login("ghost@example.com", "wrong password here"), http.StatusUnauthorized, "invalid_credentials")
	if n := count(t, e, `SELECT count(*) FROM jobs WHERE kind = 'identity.login_failed'`); n != 2 {
		t.Fatalf("login_failed jobs = %d, want one per failed attempt", n)
	}
	waitJobs(t, e, "identity.login_failed")
	if n := count(t, e, `SELECT count(*) FROM audit_logs WHERE action = 'auth.login_failed'`); n != 1 {
		t.Fatalf("audits = %d, want 1 (the unknown email has no account to audit)", n)
	}

	// Password reset requests: one job for an existing and for an unknown email; only the
	// account owner gets mail.
	anon := e.Browser()
	anon.Expect(anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "lan@example.com"}), http.StatusAccepted, "")
	anon.Expect(anon.Do("POST", "/auth/password-reset/request", map[string]string{"email": "nobody@example.com"}), http.StatusAccepted, "")
	if n := count(t, e, `SELECT count(*) FROM jobs WHERE payload->>'template' = 'reset_password'`); n != 2 {
		t.Fatalf("reset jobs = %d, want 2", n)
	}
	e.Outbox.LastToken(t, "reset_password", "lan@example.com")
	waitJobs(t, e, "identity.email")
	for _, m := range e.Outbox.All() {
		if m.To == "nobody@example.com" {
			t.Fatal("no email may go to an address without an account")
		}
	}
}

// J3: expired and long-revoked sessions and old tokens are purged hourly.
func TestPurgeExpiredSessionsAndTokens(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	b.Expect(b.Do("POST", "/auth/logout", nil), http.StatusNoContent, "")
	live := e.Browser()
	live.Expect(live.Login("lan@example.com", apptest.Password), http.StatusOK, "")
	waitJobs(t, e, "identity.email")
	if n := count(t, e, `SELECT count(*) FROM user_sessions`); n != 2 {
		t.Fatalf("sessions = %d", n)
	}

	// The revoked session's 1-day grace period, and the used verification token's 24 h
	// lifetime plus its 1-day grace period, pass.
	e.Clock.Advance(49 * time.Hour)
	live.Expect(live.Do("GET", "/me", nil), http.StatusOK, "")
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM user_sessions`); n != 1 {
		t.Fatalf("revoked session not purged: %d left", n)
	}
	if n := count(t, e, `SELECT count(*) FROM auth_tokens`); n != 0 {
		t.Fatalf("expired verification token not purged: %d left", n)
	}

	e.Clock.Advance(61 * 24 * time.Hour) // past the absolute session lifetime
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM user_sessions`); n != 0 {
		t.Fatalf("expired session not purged: %d left", n)
	}
}

// J4: 30 days after deletion the account's private data is gone, the users row is
// anonymized (audit rows keep pointing at it), other users' data is untouched and the
// email address can be registered again.
func TestAccountErasedAfterThirtyDays(t *testing.T) {
	e := apptest.New(t)
	lan, lanID := e.SignUp("lan@example.com", "Lan")
	minh, minhID := e.SignUp("minh@example.com", "Minh")
	lanTank := lan.Expect(lan.Do("POST", "/tanks", map[string]any{"name": "Lan's reef", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "70"}), http.StatusCreated, "").Body["id"].(string)
	minhTank := minh.Expect(minh.Do("POST", "/tanks", map[string]any{"name": "Minh's reef", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "70"}), http.StatusCreated, "").Body["id"].(string)
	lan.Expect(lan.Do("POST", "/tanks/"+lanTank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	minh.Expect(minh.Do("POST", "/tanks/"+minhTank+"/shares", map[string]string{"email": "lan@example.com"}), http.StatusCreated, "")

	// The erasure placeholder can never be registered by someone else (which would make
	// the anonymizing UPDATE fail on the unique email and block the erasure).
	for _, squat := range []string{"erased:" + lanID, "deleted-" + lanID + "@erased.invalid"} {
		r := e.Browser().Do("POST", "/auth/register", map[string]string{"email": squat, "password": apptest.Password, "display_name": "Squatter"})
		if r.Status != http.StatusUnprocessableEntity || r.FieldCodes()["email"] != "invalid_email" {
			t.Fatalf("placeholder %q must be rejected: %s", squat, r.Raw)
		}
	}
	lan.Expect(lan.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Outbox.WaitFor(t, "account_deactivated", "lan@example.com", 0)

	e.Clock.Advance(29 * 24 * time.Hour)
	e.RunJobs()
	waitJobs(t, e, "identity.erase_due")
	if n := count(t, e, `SELECT count(*) FROM tanks WHERE owner_id = $1`, lanID); n != 1 {
		t.Fatal("nothing may be erased before 30 days")
	}

	e.Clock.Advance(2 * 24 * time.Hour)
	e.RunJobs() // erase_due enqueues the erasure (this pass or the background runner runs it)
	waitJobs(t, e, "identity.erase_account")

	checks := []struct {
		name string
		sql  string
		want int
	}{
		{"lan's tanks", `SELECT count(*) FROM tanks WHERE owner_id = $1`, 0},
		{"lan's tank events", `SELECT count(*) FROM tank_events WHERE tank_id NOT IN (SELECT id FROM tanks)`, 0},
		{"shares granted to lan", `SELECT count(*) FROM tank_shares WHERE user_id = $1`, 0},
		{"lan's sessions", `SELECT count(*) FROM user_sessions WHERE user_id = $1`, 0},
		{"lan's tokens", `SELECT count(*) FROM auth_tokens WHERE user_id = $1`, 0},
		{"anonymized row", `SELECT count(*) FROM users WHERE id = $1 AND email = 'erased:' || id::text AND display_name = 'Deleted user' AND username IS NULL AND erased_at IS NOT NULL`, 1},
		{"audit rows kept", `SELECT count(*) FROM audit_logs WHERE actor_id = $1`, -1},
		{"erasure audited", `SELECT count(*) FROM audit_logs WHERE action = 'account.erased' AND target_id = $1`, 1},
	}
	for _, c := range checks {
		var args []any
		if strings.Contains(c.sql, "$1") {
			args = append(args, lanID)
		}
		got := count(t, e, c.sql, args...)
		if (c.want == -1 && got == 0) || (c.want >= 0 && got != c.want) {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	// Minh's own tank and data are untouched (Minh signs in again: 31 idle days ended the
	// old session).
	minh.Expect(minh.Login("minh@example.com", apptest.Password), http.StatusOK, "")
	r := minh.Expect(minh.Do("GET", "/tanks/"+minhTank, nil), http.StatusOK, "")
	if r.Body["name"] != "Minh's reef" {
		t.Fatalf("other users' data changed: %s", r.Raw)
	}
	minh.Expect(minh.Do("GET", "/tanks/"+lanTank, nil), http.StatusNotFound, "not_found")
	if n := count(t, e, `SELECT count(*) FROM tank_shares WHERE tank_id = $1`, minhTank); n != 0 {
		t.Fatalf("minh's share to the erased account must be gone, %d left", n)
	}
	if n := count(t, e, `SELECT count(*) FROM users WHERE id = $1 AND email = 'minh@example.com'`, minhID); n != 1 {
		t.Fatal("other accounts must not be touched")
	}

	// Erasure is idempotent, and the address is free again.
	e.Jobs.RunDue(context.Background()) //nolint:errcheck // a second pass must be a no-op
	sent := len(e.Outbox.All())
	e.Register("lan@example.com", "Lan again")
	e.Outbox.TokenAfter(t, "verify_email", "lan@example.com", sent)
	if n := count(t, e, `SELECT count(*) FROM users WHERE email = 'lan@example.com'`); n != 1 {
		t.Fatalf("re-registration: %d accounts with the address", n)
	}
}
