//go:build integration

package db_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/database/dbtest"
)

var t0 = time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)

func hash32(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return h[:]
}

func createUser(t *testing.T, q *dbq.Queries, email string) uuid.UUID {
	t.Helper()
	row, err := q.CreateUser(context.Background(), dbq.CreateUserParams{
		Email: email, PasswordHash: "$argon2id$v1", DisplayName: "Test", Locale: "vi", Timezone: "Asia/Ho_Chi_Minh", Now: t0,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return row.ID
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestEmailUniqueCaseInsensitive(t *testing.T) {
	pool := dbtest.New(t)
	q := dbq.New(pool)
	createUser(t, q, "Lan@Example.com")

	_, err := q.CreateUser(context.Background(), dbq.CreateUserParams{
		Email: "lan@example.COM", PasswordHash: "x", DisplayName: "Other", Locale: "vi", Timezone: "UTC", Now: t0,
	})
	if pgCode(err) != "23505" {
		t.Fatalf("want unique violation, got %v", err)
	}

	creds, err := q.GetUserCredentialsByEmail(context.Background(), "LAN@example.com")
	if err != nil {
		t.Fatalf("case-insensitive lookup: %v", err)
	}
	if creds.Status != "active" || creds.EmailVerifiedAt != nil || creds.AuthEpoch != 1 {
		t.Errorf("unexpected new-user state: %+v", creds)
	}
}

func TestConstraints(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	id := createUser(t, dbq.New(pool), "c@example.com")

	tests := []struct {
		name string
		sql  string
	}{
		{"unknown locale", `UPDATE users SET locale = 'fr' WHERE id = $1`},
		{"deactivated without timestamp", `UPDATE users SET status = 'deactivated' WHERE id = $1`},
		{"timestamp without deactivated status", `UPDATE users SET deactivated_at = now() WHERE id = $1`},
		{"rank stored as role", `INSERT INTO user_roles (user_id, role) VALUES ($1, 'reef_master')`},
		{"malformed audit action", `INSERT INTO audit_logs (actor_id, action) VALUES ($1, 'Login')`},
		{"full ipv4 address in audit", `INSERT INTO audit_logs (actor_id, action, ip_prefix) VALUES ($1, 'auth.login', '203.0.113.57/32')`},
		{"host bits in ipv4 prefix", `INSERT INTO audit_logs (actor_id, action, ip_prefix) VALUES ($1, 'auth.login', '203.0.113.57/24')`},
		{"ipv6 longer than /48", `INSERT INTO audit_logs (actor_id, action, ip_prefix) VALUES ($1, 'auth.login', '2001:db8:abcd:12::/64')`},
		{"token expiring before creation", `INSERT INTO auth_tokens (user_id, purpose, token_hash, created_at, expires_at)
			VALUES ($1, 'verify_email', decode(repeat('ab', 32), 'hex'), now(), now() - interval '1 second')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.sql, id)
			if pgCode(err) != "23514" {
				t.Fatalf("want check violation, got %v", err)
			}
		})
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	sentinel := errors.New("abort")

	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		createUser(t, dbq.New(tx), "rollback@example.com")
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
	_, err = dbq.New(pool).GetUserCredentialsByEmail(ctx, "rollback@example.com")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("user should not exist after rollback, got %v", err)
	}
}

func issueToken(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, purpose string, now time.Time) ([]byte, error) {
	t.Helper()
	tokenHash := hash32(t)
	err := database.InTx(context.Background(), pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.RevokeLiveAuthTokens(context.Background(), dbq.RevokeLiveAuthTokensParams{UserID: userID, Purpose: purpose, Now: now}); err != nil {
			return err
		}
		return q.CreateAuthToken(context.Background(), dbq.CreateAuthTokenParams{
			UserID: userID, Purpose: purpose, TokenHash: tokenHash, Now: now, ExpiresAt: now.Add(time.Hour),
		})
	})
	return tokenHash, err
}

func TestConsumeAuthTokenIsSingleUseUnderConcurrency(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "token@example.com")
	tokenHash, err := issueToken(t, pool, userID, "reset_password", t0)
	if err != nil {
		t.Fatal(err)
	}

	// Wrong purpose must not consume it.
	if _, err := q.ConsumeAuthToken(ctx, dbq.ConsumeAuthTokenParams{TokenHash: tokenHash, Purpose: "verify_email", Now: t0}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("wrong purpose: want no rows, got %v", err)
	}

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := q.ConsumeAuthToken(ctx, dbq.ConsumeAuthTokenParams{TokenHash: tokenHash, Purpose: "reset_password", Now: t0})
			switch {
			case err == nil && got.UserID == userID && got.Status == "active":
				wins.Add(1)
			case errors.Is(err, pgx.ErrNoRows):
			default:
				t.Errorf("unexpected result: %+v %v", got, err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("token consumed %d times, want exactly 1", wins.Load())
	}
}

func TestExpiredAndSupersededTokensAreRejected(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "expired@example.com")

	first, err := issueToken(t, pool, userID, "verify_email", t0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := issueToken(t, pool, userID, "verify_email", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	consume := func(h []byte, at time.Time) error {
		_, err := q.ConsumeAuthToken(ctx, dbq.ConsumeAuthTokenParams{TokenHash: h, Purpose: "verify_email", Now: at})
		return err
	}
	if err := consume(first, t0.Add(2*time.Minute)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("superseded token must be rejected, got %v", err)
	}
	if err := consume(second, t0.Add(2*time.Hour)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired token must be rejected, got %v", err)
	}

	var revoked, used int
	if err := pool.QueryRow(ctx, `SELECT count(revoked_at), count(used_at) FROM auth_tokens WHERE user_id = $1`, userID).Scan(&revoked, &used); err != nil {
		t.Fatal(err)
	}
	if revoked != 1 || used != 0 {
		t.Errorf("superseded token should be revoked, not used: revoked=%d used=%d", revoked, used)
	}
}

func TestConcurrentTokenIssueLeavesOneLiveToken(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	userID := createUser(t, dbq.New(pool), "resend@example.com")

	var ok, dup atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := issueToken(t, pool, userID, "reset_password", t0)
			switch {
			case err == nil:
				ok.Add(1)
			case pgCode(err) == "23505":
				dup.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_tokens
		WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, userID).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 1 || ok.Load() < 1 {
		t.Fatalf("live tokens = %d (ok=%d dup=%d), want exactly 1", live, ok.Load(), dup.Load())
	}
}

func newSession(t *testing.T, q *dbq.Queries, userID uuid.UUID, epoch int32, now time.Time) (uuid.UUID, []byte) {
	t.Helper()
	tokenHash := hash32(t)
	ip := netip.MustParsePrefix("203.0.113.0/24")
	id, err := q.CreateSession(context.Background(), dbq.CreateSessionParams{
		UserID: userID, AuthEpoch: epoch, TokenHash: tokenHash, CsrfSecret: hash32(t), Now: now,
		IdleExpiresAt: now.Add(14 * 24 * time.Hour), ExpiresAt: now.Add(60 * 24 * time.Hour),
		IpPrefix: &ip,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return id, tokenHash
}

func TestSessionLifecycle(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "session@example.com")
	otherUser := createUser(t, q, "other@example.com")

	keepID, keepHash := newSession(t, q, userID, 1, t0)
	_, otherHash := newSession(t, q, userID, 1, t0)

	lookup := func(hash []byte, at time.Time) error {
		_, err := q.GetActiveSessionByTokenHash(ctx, dbq.GetActiveSessionByTokenHashParams{TokenHash: hash, Now: at})
		return err
	}

	if err := lookup(keepHash, t0); err != nil {
		t.Fatalf("active session lookup: %v", err)
	}
	if err := lookup(keepHash, t0.Add(15*24*time.Hour)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("idle-expired session should not resolve, got %v", err)
	}

	// Touching an idle-expired session must not revive it.
	late := t0.Add(15 * 24 * time.Hour)
	n, err := q.TouchSession(ctx, dbq.TouchSessionParams{ID: keepID, Now: late, IdleExpiresAt: late.Add(14 * 24 * time.Hour)})
	if err != nil || n != 0 {
		t.Fatalf("touch of expired session: n=%d err=%v", n, err)
	}

	// Touch must never extend past the absolute expiry.
	if _, err := q.TouchSession(ctx, dbq.TouchSessionParams{ID: keepID, Now: t0, IdleExpiresAt: t0.Add(365 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := lookup(keepHash, t0.Add(61*24*time.Hour)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("session must not outlive absolute expiry, got %v", err)
	}

	// Revoking by id is scoped to the owner.
	if n, err := q.RevokeSession(ctx, dbq.RevokeSessionParams{ID: keepID, UserID: otherUser, Now: t0}); err != nil || n != 0 {
		t.Fatalf("foreign revoke must be a no-op: n=%d err=%v", n, err)
	}

	n, err = q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: userID, KeepID: &keepID, Now: t0})
	if err != nil || n != 1 {
		t.Fatalf("revoke others: n=%d err=%v", n, err)
	}
	if err := lookup(otherHash, t0); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("revoked session should not resolve, got %v", err)
	}
	if err := lookup(keepHash, t0); err != nil {
		t.Fatalf("kept session should still resolve: %v", err)
	}

	// Suspended users' sessions still resolve here; the service layer rejects them by status.
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	row, err := q.GetActiveSessionByTokenHash(ctx, dbq.GetActiveSessionByTokenHashParams{TokenHash: keepHash, Now: t0})
	if err != nil || row.UserStatus != "suspended" {
		t.Fatalf("want suspended status surfaced, got %+v %v", row, err)
	}
}

// A login that read the old credentials before a password change must not yield a
// usable session once the change has committed (review finding H1).
func TestPasswordChangeInvalidatesRacingLogin(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "race@example.com")

	// Login step 1: read credentials (epoch 1) and verify the old password.
	creds, err := q.GetUserCredentialsByEmail(ctx, "race@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, existingHash := newSession(t, q, userID, creds.AuthEpoch, t0)

	// Meanwhile the owner resets the password.
	if n, err := q.ChangeUserPassword(ctx, dbq.ChangeUserPasswordParams{ID: userID, PasswordHash: "$argon2id$new", Now: t0}); err != nil || n != 2 {
		t.Fatalf("change password: epoch=%d err=%v", n, err)
	}

	// Login step 2 completes afterwards with the stale epoch.
	_, racingHash := newSession(t, q, userID, creds.AuthEpoch, t0)

	for name, h := range map[string][]byte{"existing": existingHash, "racing": racingHash} {
		_, err := q.GetActiveSessionByTokenHash(ctx, dbq.GetActiveSessionByTokenHashParams{TokenHash: h, Now: t0})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("%s session must not resolve after password change, got %v", name, err)
		}
	}

	fresh, err := q.GetUserCredentialsByEmail(ctx, "race@example.com")
	if err != nil || fresh.AuthEpoch != 2 {
		t.Fatalf("epoch after change: %+v %v", fresh, err)
	}
	_, freshHash := newSession(t, q, userID, fresh.AuthEpoch, t0)
	if _, err := q.GetActiveSessionByTokenHash(ctx, dbq.GetActiveSessionByTokenHashParams{TokenHash: freshHash, Now: t0}); err != nil {
		t.Fatalf("session with the current epoch should resolve: %v", err)
	}
}

// A rehash computed from the old password must not overwrite a newer password (H2).
func TestRehashCannotUndoPasswordChange(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "rehash@example.com")

	if _, err := q.ChangeUserPassword(ctx, dbq.ChangeUserPasswordParams{ID: userID, PasswordHash: "$argon2id$new", Now: t0}); err != nil {
		t.Fatal(err)
	}
	n, err := q.RehashUserPassword(ctx, dbq.RehashUserPasswordParams{ID: userID, OldHash: "$argon2id$v1", NewHash: "$argon2id$v2-of-old", Now: t0})
	if err != nil || n != 0 {
		t.Fatalf("stale rehash must not apply: n=%d err=%v", n, err)
	}
	creds, err := q.GetUserCredentialsByEmail(ctx, "rehash@example.com")
	if err != nil || creds.PasswordHash != "$argon2id$new" {
		t.Fatalf("password hash = %q, want the changed one (err %v)", creds.PasswordHash, err)
	}

	n, err = q.RehashUserPassword(ctx, dbq.RehashUserPasswordParams{ID: userID, OldHash: "$argon2id$new", NewHash: "$argon2id$new-v2", Now: t0})
	if err != nil || n != 1 {
		t.Fatalf("current rehash should apply: n=%d err=%v", n, err)
	}
	if creds, _ := q.GetUserCredentialsByEmail(ctx, "rehash@example.com"); creds.AuthEpoch != 2 {
		t.Errorf("rehash must not bump auth_epoch, got %d", creds.AuthEpoch)
	}
}

func TestAuditLog(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	q := dbq.New(pool)
	userID := createUser(t, q, "audit@example.com")
	reqID := "req-12345678"
	ip := netip.MustParsePrefix("2001:db8:abcd::/48")

	if err := q.InsertAuditLog(ctx, dbq.InsertAuditLogParams{
		ActorID: &userID, Action: "auth.login", RequestID: &reqID, IpPrefix: &ip, Now: t0, // nil metadata
	}); err != nil {
		t.Fatalf("insert with nil metadata: %v", err)
	}
	var got netip.Prefix
	var meta string
	if err := pool.QueryRow(ctx, `SELECT ip_prefix, metadata::text FROM audit_logs WHERE actor_id = $1`, userID).Scan(&got, &meta); err != nil {
		t.Fatal(err)
	}
	if got != ip || meta != "{}" {
		t.Errorf("round-trip: ip=%v meta=%s", got, meta)
	}

	for name, stmt := range map[string]string{
		"update": `UPDATE audit_logs SET action = 'auth.logout' WHERE actor_id = $1`,
		"delete": `DELETE FROM audit_logs WHERE actor_id = $1`,
	} {
		if _, err := pool.Exec(ctx, stmt, userID); pgCode(err) != "42501" {
			t.Errorf("%s must be rejected as append-only, got %v", name, err)
		}
	}

	// Erasing the account detaches the actor but keeps the record.
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user with audit history: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id IS NULL AND action = 'auth.login'`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("audit row should survive with NULL actor: count=%d err=%v", remaining, err)
	}
}
