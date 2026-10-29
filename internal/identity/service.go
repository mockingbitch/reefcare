// Package identity implements accounts, sessions, email verification and password
// management (docs/user-stories.md US-ID-1..4, docs/security.md §2).
package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	reefmail "reefcare/internal/platform/mail"
	"reefcare/internal/platform/ratelimit"
)

// Config holds identity settings. Durations follow docs/security.md §2.
type Config struct {
	PublicBaseURL      string
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	SessionTouchEvery  time.Duration
	VerifyTokenTTL     time.Duration
	ResetTokenTTL      time.Duration
}

// DefaultConfig returns the documented defaults.
func DefaultConfig(publicBaseURL string) Config {
	return Config{
		PublicBaseURL:      strings.TrimRight(publicBaseURL, "/"),
		SessionIdleTTL:     14 * 24 * time.Hour,
		SessionAbsoluteTTL: 60 * 24 * time.Hour,
		SessionTouchEvery:  5 * time.Minute,
		VerifyTokenTTL:     24 * time.Hour,
		ResetTokenTTL:      time.Hour,
	}
}

// Limits are the rate limiters guarding identity endpoints (docs/api-guidelines.md §6).
// *PerIP limiters key on Client.RateKey (IPv4 address or IPv6 /64); *PerNet limiters key
// on Client.RateNetKey (IPv6 /48 only) with a larger budget.
type Limits struct {
	LoginPerEmail, LoginPerIP, LoginPerNet          *ratelimit.Limiter
	RegisterPerEmail, RegisterPerIP, RegisterPerNet *ratelimit.Limiter
	ResetPerEmail, ResetPerIP, ResetPerNet          *ratelimit.Limiter
	ResetConfirmPerIP, VerifyPerIP, ResendPerUser   *ratelimit.Limiter
	PasswordChangePerUser                           *ratelimit.Limiter
}

// DefaultLimits returns the documented limits.
func DefaultLimits(clk clock.Clock) Limits {
	return Limits{
		LoginPerEmail:         ratelimit.New(10, 15*time.Minute, clk),
		LoginPerIP:            ratelimit.New(30, 15*time.Minute, clk),
		LoginPerNet:           ratelimit.New(100, 15*time.Minute, clk),
		RegisterPerEmail:      ratelimit.New(5, time.Hour, clk),
		RegisterPerIP:         ratelimit.New(5, time.Hour, clk),
		RegisterPerNet:        ratelimit.New(20, time.Hour, clk),
		ResetPerEmail:         ratelimit.New(5, time.Hour, clk),
		ResetPerIP:            ratelimit.New(5, time.Hour, clk),
		ResetPerNet:           ratelimit.New(20, time.Hour, clk),
		ResetConfirmPerIP:     ratelimit.New(10, 15*time.Minute, clk),
		VerifyPerIP:           ratelimit.New(20, 15*time.Minute, clk),
		ResendPerUser:         ratelimit.New(3, time.Hour, clk),
		PasswordChangePerUser: ratelimit.New(10, 15*time.Minute, clk),
	}
}

// Client identifies the caller of a request. RateKey and RateNetKey come from
// httpserver.RateLimitKey / RateLimitNetKey, Origin.IPPrefix from httpserver.TruncatedIP.
type Client struct {
	RateKey    string
	RateNetKey string // "" when there is no coarser tier (IPv4)
	Origin     audit.Origin
}

// Principal is an authenticated session.
type Principal struct {
	UserID        uuid.UUID
	SessionID     uuid.UUID
	EmailVerified bool
	csrfSecret    []byte
}

// User is the account view returned to its owner.
type User struct {
	ID              uuid.UUID
	Email           string
	EmailVerified   bool
	DisplayName     string
	Username        *string
	Bio             *string
	ExperienceLevel *string
	Locale          string
	Timezone        string
	Roles           []string
	CreatedAt       time.Time
	Version         int32
}

// Session carries new session credentials for the client (Token, CSRFToken are secrets).
type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	User      User
}

// Service implements identity workflows.
type Service struct {
	pool   *pgxpool.Pool
	clock  clock.Clock
	hasher *Hasher
	mailer reefmail.Sender
	limits Limits
	cfg    Config
	log    *slog.Logger
	bg     *background
}

// NewService wires the identity service. Call Close on shutdown.
func NewService(pool *pgxpool.Pool, clk clock.Clock, hasher *Hasher, mailer reefmail.Sender, limits Limits, cfg Config, log *slog.Logger) *Service {
	return &Service{
		pool: pool, clock: clk, hasher: hasher, mailer: mailer, limits: limits, cfg: cfg, log: log,
		bg: &background{log: log, slots: make(chan struct{}, 64)},
	}
}

// Close waits for background work (deferred audits, token issuing) until ctx expires.
func (s *Service) Close(ctx context.Context) error { return s.bg.wait(ctx) }

var (
	errInvalidCredentials = apperr.Unauthenticated("invalid_credentials", "Email or password is incorrect.")
	errInvalidToken       = apperr.Invalid("invalid_token", "This link is invalid or has expired.")
	errSuspended          = apperr.Forbidden("account_suspended", "This account is suspended.")
	errUnauthenticated    = apperr.Unauthenticated("unauthenticated", "Sign in to continue.")

	errEmailTaken   = errors.New("email already registered")
	errTokenPending = errors.New("a token of this purpose was issued concurrently")
)

const (
	purposeVerify = "verify_email"
	purposeReset  = "reset_password"
)

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validEmail(email string) bool {
	if n := len(email); n < 3 || n > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && addr.Name == ""
}

func allow(l *ratelimit.Limiter, key string) error {
	if key == "" {
		return nil
	}
	if ok, retry := l.Allow(key); !ok {
		return apperr.RateLimited(retry)
	}
	return nil
}

// allowClient applies the per-IP limiter and, for IPv6, the per-network one.
func allowClient(perIP, perNet *ratelimit.Limiter, c Client) error {
	if err := allow(perIP, c.RateKey); err != nil {
		return err
	}
	return allow(perNet, c.RateNetKey)
}

func (s *Service) send(ctx context.Context, m reefmail.Message) {
	if err := s.mailer.Send(ctx, m); err != nil {
		s.log.ErrorContext(ctx, "queue email", "kind", m.Kind, "err", err)
	}
}

// RegisterInput is the sign-up form.
type RegisterInput struct {
	Email, Password, DisplayName, Locale, Timezone string
}

// Register creates an unverified account and emails a verification link. To prevent
// account enumeration it behaves identically when the email is already registered,
// emailing the existing owner instead (US-ID-1 AC2).
func (s *Service) Register(ctx context.Context, c Client, in RegisterInput) error {
	email := normalizeEmail(in.Email)
	name := strings.TrimSpace(in.DisplayName)
	locale := in.Locale
	if locale == "" {
		locale = "vi"
	}
	tz := in.Timezone
	if tz == "" {
		tz = "Asia/Ho_Chi_Minh"
	}

	v := apperr.NewValidator()
	v.Check(validEmail(email), "email", "invalid_email", nil)
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		v.Add("display_name", "length", map[string]string{"min": "1", "max": "60"})
	} else if hasControlChars(name) {
		v.Add("display_name", "invalid_characters", nil)
	}
	v.Check(locale == "vi" || locale == "en", "locale", "unsupported", nil)
	v.Check(validTimezone(tz), "timezone", "invalid_timezone", nil)
	if p := passwordProblem(in.Password, email); p != "" {
		v.Add("password", p, map[string]string{"min": fmt.Sprint(minPasswordLen), "max": fmt.Sprint(maxPasswordLen)})
	}
	if err := v.Err(); err != nil {
		return err
	}
	if err := allowClient(s.limits.RegisterPerIP, s.limits.RegisterPerNet, c); err != nil {
		return err
	}
	if err := allow(s.limits.RegisterPerEmail, email); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	var msg reefmail.Message
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.CreateUser(ctx, dbq.CreateUserParams{
			Email: email, PasswordHash: hash, DisplayName: name, Locale: locale, Timezone: tz, Now: now,
		})
		if database.IsUniqueViolation(err, "users_email_key") {
			return errEmailTaken
		}
		if err != nil {
			return err
		}
		token, err := s.issueToken(ctx, q, row.ID, purposeVerify, now, s.cfg.VerifyTokenTTL)
		if err != nil {
			return err
		}
		msg = s.email(purposeVerify, locale, email, "/verify-email", token)
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &row.ID, Action: "auth.register", TargetType: "user", TargetID: &row.ID})
	})
	switch {
	case errors.Is(err, errEmailTaken):
		return s.notifyAlreadyRegistered(ctx, c, email)
	case err != nil:
		return err
	}
	s.send(ctx, msg)
	return nil
}

func (s *Service) notifyAlreadyRegistered(ctx context.Context, c Client, email string) error {
	q := dbq.New(s.pool)
	creds, err := q.GetUserCredentialsByEmail(ctx, email)
	if err != nil {
		return err
	}
	u, err := q.GetUserByID(ctx, creds.ID)
	if err != nil {
		return err
	}
	if err := audit.Record(ctx, q, s.clock.Now(), c.Origin, audit.Entry{
		ActorID: &u.ID, Action: "auth.register_duplicate", TargetType: "user", TargetID: &u.ID,
	}); err != nil {
		return err
	}
	if u.Status == "active" {
		s.send(ctx, s.email("already_registered", u.Locale, u.Email, "/login", ""))
	}
	return nil
}

// issueToken revokes live tokens of the purpose and creates a new one, returning the
// client-facing token. It must run inside a transaction.
func (s *Service) issueToken(ctx context.Context, q *dbq.Queries, userID uuid.UUID, purpose string, now time.Time, ttl time.Duration) (string, error) {
	_, token, err := newSecret()
	if err != nil {
		return "", err
	}
	if err := q.RevokeLiveAuthTokens(ctx, dbq.RevokeLiveAuthTokensParams{UserID: userID, Purpose: purpose, Now: now}); err != nil {
		return "", err
	}
	err = q.CreateAuthToken(ctx, dbq.CreateAuthTokenParams{
		UserID: userID, Purpose: purpose, TokenHash: hashToken(token), Now: now, ExpiresAt: now.Add(ttl),
	})
	if database.IsUniqueViolation(err, "auth_tokens_one_live") {
		return "", errTokenPending
	}
	return token, err
}

// VerifyEmail consumes a verification token.
func (s *Service) VerifyEmail(ctx context.Context, c Client, token string) error {
	if err := allow(s.limits.VerifyPerIP, c.RateKey); err != nil {
		return err
	}
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.ConsumeAuthToken(ctx, dbq.ConsumeAuthTokenParams{TokenHash: hashToken(token), Purpose: purposeVerify, Now: now})
		if database.IsNoRows(err) {
			return errInvalidToken
		}
		if err != nil {
			return err
		}
		if row.Status != "active" {
			return errInvalidToken
		}
		if _, err := q.MarkEmailVerified(ctx, dbq.MarkEmailVerifiedParams{ID: row.UserID, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &row.UserID, Action: "auth.email_verified", TargetType: "user", TargetID: &row.UserID})
	})
}

// ResendVerification emails a fresh verification link to an unverified account.
func (s *Service) ResendVerification(ctx context.Context, c Client, p Principal) error {
	if p.EmailVerified {
		return apperr.Conflict("already_verified", "This email address is already verified.")
	}
	if err := allow(s.limits.ResendPerUser, p.UserID.String()); err != nil {
		return err
	}
	now := s.clock.Now()
	var msg reefmail.Message
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		u, err := q.GetUserByID(ctx, p.UserID)
		if err != nil {
			return err
		}
		token, err := s.issueToken(ctx, q, u.ID, purposeVerify, now, s.cfg.VerifyTokenTTL)
		if err != nil {
			return err
		}
		msg = s.email(purposeVerify, u.Locale, u.Email, "/verify-email", token)
		return nil
	})
	if errors.Is(err, errTokenPending) {
		return nil // a concurrent request already sent one
	}
	if err != nil {
		return err
	}
	s.send(ctx, msg)
	return nil
}

// LoginInput is the sign-in form.
type LoginInput struct{ Email, Password string }

// Login verifies credentials and creates a session. Responses and timing do not reveal
// whether the email exists (US-ID-2 AC2): unknown accounts get a dummy hash
// verification, and the extra audit write for a real account happens off the request path.
func (s *Service) Login(ctx context.Context, c Client, in LoginInput) (Session, error) {
	email := normalizeEmail(in.Email)
	if err := allowClient(s.limits.LoginPerIP, s.limits.LoginPerNet, c); err != nil {
		return Session{}, err
	}
	// Reject malformed input before it can become a per-email limiter key or a hash input.
	if !validEmail(email) || utf8.RuneCountInString(in.Password) > maxPasswordLen {
		if err := s.hasher.VerifyDummy(ctx, ""); err != nil {
			return Session{}, err
		}
		return Session{}, errInvalidCredentials
	}
	if err := allow(s.limits.LoginPerEmail, email); err != nil {
		return Session{}, err
	}

	q := dbq.New(s.pool)
	creds, err := q.GetUserCredentialsByEmail(ctx, email)
	if database.IsNoRows(err) {
		if err := s.hasher.VerifyDummy(ctx, in.Password); err != nil {
			return Session{}, err
		}
		return Session{}, errInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	match, rehash, err := s.hasher.Verify(ctx, in.Password, creds.PasswordHash)
	if apperr.IsKind(err, apperr.KindUnavailable) {
		return Session{}, err
	}
	if err != nil {
		s.log.ErrorContext(ctx, "stored password hash is malformed", "user_id", creds.ID, "err", err)
		return Session{}, errInvalidCredentials
	}
	now := s.clock.Now()
	if !match {
		userID, origin := creds.ID, c.Origin
		s.bg.run("audit login failure", func(ctx context.Context) error {
			return audit.Record(ctx, dbq.New(s.pool), now, origin, audit.Entry{ActorID: &userID, Action: "auth.login_failed", TargetType: "user", TargetID: &userID})
		})
		return Session{}, errInvalidCredentials
	}
	switch creds.Status {
	case "suspended":
		return Session{}, errSuspended
	case "deactivated":
		return Session{}, errInvalidCredentials
	}
	if rehash {
		s.rehash(ctx, q, creds.ID, creds.PasswordHash, in.Password, now)
	}

	csrfSecret, _, err := newSecret()
	if err != nil {
		return Session{}, err
	}
	_, token, err := newSecret()
	if err != nil {
		return Session{}, err
	}
	expires := now.Add(s.cfg.SessionAbsoluteTTL)
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		ua := truncate(c.Origin.UserAgent, 512)
		id, err := q.CreateSession(ctx, dbq.CreateSessionParams{
			UserID: creds.ID, AuthEpoch: creds.AuthEpoch, TokenHash: hashToken(token), CsrfSecret: csrfSecret,
			Now: now, IdleExpiresAt: now.Add(s.cfg.SessionIdleTTL), ExpiresAt: expires,
			UserAgent: &ua, IpPrefix: c.Origin.IPPrefix,
		})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &creds.ID, Action: "auth.login", TargetType: "session", TargetID: &id})
	})
	if err != nil {
		return Session{}, err
	}
	u, err := s.user(ctx, q, creds.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, CSRFToken: csrfToken(csrfSecret), ExpiresAt: expires, User: u}, nil
}

func (s *Service) rehash(ctx context.Context, q *dbq.Queries, id uuid.UUID, oldHash, password string, now time.Time) {
	newHash, err := s.hasher.Hash(ctx, password)
	if err == nil {
		_, err = q.RehashUserPassword(ctx, dbq.RehashUserPasswordParams{ID: id, OldHash: oldHash, NewHash: newHash, Now: now})
	}
	if err != nil {
		s.log.WarnContext(ctx, "password rehash failed", "user_id", id, "err", err)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// Authenticate resolves a session token to a principal, sliding the idle timeout.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	now := s.clock.Now()
	q := dbq.New(s.pool)
	row, err := q.GetActiveSessionByTokenHash(ctx, dbq.GetActiveSessionByTokenHashParams{TokenHash: hashToken(token), Now: now})
	if database.IsNoRows(err) {
		return Principal{}, errUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	switch row.UserStatus {
	case "suspended":
		return Principal{}, errSuspended
	case "deactivated":
		return Principal{}, errUnauthenticated
	}
	if now.Sub(row.LastSeenAt) >= s.cfg.SessionTouchEvery {
		if _, err := q.TouchSession(ctx, dbq.TouchSessionParams{ID: row.ID, Now: now, IdleExpiresAt: now.Add(s.cfg.SessionIdleTTL)}); err != nil {
			s.log.WarnContext(ctx, "touch session failed", "err", err)
		}
	}
	return Principal{UserID: row.UserID, SessionID: row.ID, EmailVerified: row.EmailVerifiedAt != nil, csrfSecret: row.CsrfSecret}, nil
}

// ValidCSRF reports whether token matches the principal's session CSRF secret.
func (p Principal) ValidCSRF(token string) bool { return validCSRF(token, p.csrfSecret) }

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, c Client, p Principal) error {
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := q.RevokeSession(ctx, dbq.RevokeSessionParams{ID: p.SessionID, UserID: p.UserID, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &p.UserID, Action: "auth.logout", TargetType: "session", TargetID: &p.SessionID})
	})
}

// RequestPasswordReset emails a reset link if the account exists and is active. It
// always succeeds from the caller's point of view (US-ID-3 AC1); the token is issued in
// the background so existing and unknown emails take the same time to answer.
func (s *Service) RequestPasswordReset(ctx context.Context, c Client, rawEmail string) error {
	email := normalizeEmail(rawEmail)
	v := apperr.NewValidator()
	v.Check(validEmail(email), "email", "invalid_email", nil)
	if err := v.Err(); err != nil {
		return err
	}
	if err := allowClient(s.limits.ResetPerIP, s.limits.ResetPerNet, c); err != nil {
		return err
	}
	if err := allow(s.limits.ResetPerEmail, email); err != nil {
		return err
	}

	creds, err := dbq.New(s.pool).GetUserCredentialsByEmail(ctx, email)
	if database.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if creds.Status != "active" {
		return nil
	}
	origin := c.Origin
	s.bg.run("issue password reset", func(ctx context.Context) error {
		now := s.clock.Now()
		var msg reefmail.Message
		err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
			q := dbq.New(tx)
			u, err := q.GetUserByID(ctx, creds.ID)
			if err != nil {
				return err
			}
			token, err := s.issueToken(ctx, q, u.ID, purposeReset, now, s.cfg.ResetTokenTTL)
			if err != nil {
				return err
			}
			msg = s.email(purposeReset, u.Locale, u.Email, "/reset-password", token)
			return audit.Record(ctx, q, now, origin, audit.Entry{ActorID: &u.ID, Action: "auth.password_reset_requested", TargetType: "user", TargetID: &u.ID})
		})
		if errors.Is(err, errTokenPending) {
			return nil
		}
		if err != nil {
			return err
		}
		s.send(ctx, msg)
		return nil
	})
	return nil
}

// ConfirmPasswordReset sets a new password using a reset token and signs out every
// session of the account (US-ID-3 AC2). Completing a reset proves control of the
// mailbox, so the email also becomes verified.
func (s *Service) ConfirmPasswordReset(ctx context.Context, c Client, token, newPassword string) error {
	// Hashing is expensive; limit attempts before doing any work.
	if err := allow(s.limits.ResetConfirmPerIP, c.RateKey); err != nil {
		return err
	}
	if p := passwordProblem(newPassword, ""); p != "" {
		return passwordError("new_password", p)
	}
	hash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.ConsumeAuthToken(ctx, dbq.ConsumeAuthTokenParams{TokenHash: hashToken(token), Purpose: purposeReset, Now: now})
		if database.IsNoRows(err) {
			return errInvalidToken
		}
		if err != nil {
			return err
		}
		if row.Status != "active" {
			return errInvalidToken
		}
		creds, err := q.GetUserCredentialsByID(ctx, row.UserID)
		if err != nil {
			return err
		}
		if p := passwordProblem(newPassword, creds.Email); p != "" {
			return passwordError("new_password", p) // rolls back, so the token stays usable
		}
		if _, err := q.ChangeUserPassword(ctx, dbq.ChangeUserPasswordParams{ID: row.UserID, PasswordHash: hash, Now: now}); err != nil {
			return err
		}
		if _, err := q.MarkEmailVerified(ctx, dbq.MarkEmailVerifiedParams{ID: row.UserID, Now: now}); err != nil {
			return err
		}
		if _, err := q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: row.UserID, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &row.UserID, Action: "auth.password_reset", TargetType: "user", TargetID: &row.UserID})
	})
}

// ChangePassword replaces the password after verifying the current one. Every other
// session is revoked; the calling session stays signed in but gets a new token and
// CSRF secret (returned for new cookies), so a copied token stops working. The account
// owner is notified by email.
func (s *Service) ChangePassword(ctx context.Context, c Client, p Principal, current, newPassword string) (Session, error) {
	if err := allow(s.limits.PasswordChangePerUser, p.UserID.String()); err != nil {
		return Session{}, err
	}
	q := dbq.New(s.pool)
	creds, err := q.GetUserCredentialsByID(ctx, p.UserID)
	if err != nil {
		return Session{}, err
	}
	match, _, err := s.hasher.Verify(ctx, current, creds.PasswordHash)
	if apperr.IsKind(err, apperr.KindUnavailable) {
		return Session{}, err
	}
	if err != nil || !match {
		v := apperr.NewValidator()
		v.Add("current_password", "incorrect", nil)
		return Session{}, v.Err()
	}
	if p := passwordProblem(newPassword, creds.Email); p != "" {
		return Session{}, passwordError("new_password", p)
	}
	hash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return Session{}, err
	}
	csrfSecret, _, err := newSecret()
	if err != nil {
		return Session{}, err
	}
	_, token, err := newSecret()
	if err != nil {
		return Session{}, err
	}
	now := s.clock.Now()
	var expires time.Time
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		epoch, err := q.ChangeUserPassword(ctx, dbq.ChangeUserPasswordParams{ID: p.UserID, PasswordHash: hash, Now: now})
		if err != nil {
			return err
		}
		expires, err = q.RotateSession(ctx, dbq.RotateSessionParams{
			ID: p.SessionID, UserID: p.UserID, TokenHash: hashToken(token), CsrfSecret: csrfSecret, AuthEpoch: epoch,
		})
		if database.IsNoRows(err) {
			return errUnauthenticated // session revoked concurrently
		}
		if err != nil {
			return err
		}
		if _, err := q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: p.UserID, KeepID: &p.SessionID, Now: now}); err != nil {
			return err
		}
		if err := q.RevokeLiveAuthTokens(ctx, dbq.RevokeLiveAuthTokensParams{UserID: p.UserID, Purpose: purposeReset, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{ActorID: &p.UserID, Action: "auth.password_changed", TargetType: "user", TargetID: &p.UserID})
	})
	if err != nil {
		return Session{}, err
	}
	u, err := s.user(ctx, q, p.UserID)
	if err != nil {
		return Session{}, err
	}
	s.send(ctx, s.email("password_changed", u.Locale, u.Email, "/forgot-password", ""))
	return Session{Token: token, CSRFToken: csrfToken(csrfSecret), ExpiresAt: expires, User: u}, nil
}

func passwordError(field, code string) error {
	v := apperr.NewValidator()
	v.Add(field, code, map[string]string{"min": fmt.Sprint(minPasswordLen), "max": fmt.Sprint(maxPasswordLen)})
	return v.Err()
}

// Me returns the principal's account.
func (s *Service) Me(ctx context.Context, p Principal) (User, error) {
	return s.user(ctx, dbq.New(s.pool), p.UserID)
}

func (s *Service) user(ctx context.Context, q *dbq.Queries, id uuid.UUID) (User, error) {
	u, err := q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	roles, err := q.ListUserRoles(ctx, id)
	if err != nil {
		return User{}, err
	}
	return User{
		ID: u.ID, Email: u.Email, EmailVerified: u.EmailVerifiedAt != nil, DisplayName: u.DisplayName,
		Username: u.Username, Bio: u.Bio, ExperienceLevel: u.ExperienceLevel, Locale: u.Locale,
		Timezone: u.Timezone, Roles: roles, CreatedAt: u.CreatedAt, Version: u.Version,
	}, nil
}

func validTimezone(tz string) bool {
	if tz == "" || tz == "Local" || len(tz) > 64 {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// background runs bounded, best-effort work after a response is sent. When all slots
// are busy the work is dropped and logged rather than queued without limit; durable
// processing moves to the job queue (milestone 2.3).
type background struct {
	log   *slog.Logger
	slots chan struct{}
	wg    sync.WaitGroup
}

func (b *background) run(name string, fn func(ctx context.Context) error) {
	select {
	case b.slots <- struct{}{}:
	default:
		b.log.Error("background work dropped: too many in flight", "task", name)
		return
	}
	b.wg.Add(1)
	go func() {
		defer func() { <-b.slots; b.wg.Done() }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			b.log.Error("background work failed", "task", name, "err", err)
		}
	}()
}

func (b *background) wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { b.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("background work not finished: %w", ctx.Err())
	}
}
