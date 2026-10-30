package identity

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/jobs"
	reefmail "reefcare/internal/platform/mail"
)

// Job kinds owned by identity.
const (
	jobEmail        = "identity.email"
	jobLoginFailed  = "identity.login_failed"
	jobPurgeAuth    = "identity.purge_auth"
	jobEraseDue     = "identity.erase_due"
	jobEraseAccount = "identity.erase_account"
)

// Email templates besides the token emails (purposeVerify, purposeReset).
const (
	templateAlreadyRegistered  = "already_registered"
	templatePasswordChanged    = "password_changed"
	templateAccountDeactivated = "account_deactivated"
)

// jobOrigin is the part of audit.Origin worth keeping for an audit written later.
type jobOrigin struct {
	RequestID string        `json:"request_id,omitempty"`
	IPPrefix  *netip.Prefix `json:"ip_prefix,omitempty"`
}

func originOf(o audit.Origin) jobOrigin {
	return jobOrigin{RequestID: o.RequestID, IPPrefix: o.IPPrefix}
}

func (o jobOrigin) audit() audit.Origin {
	return audit.Origin{RequestID: o.RequestID, IPPrefix: o.IPPrefix}
}

// emailJob carries only ids: the address is read and any token created when the job
// runs, so no secret or address is stored in the queue.
type emailJob struct {
	UserID   *uuid.UUID `json:"user_id"` // nil: no account (enumeration-safe no-op)
	Template string     `json:"template"`
	Origin   jobOrigin  `json:"origin,omitzero"`
}

func enqueueEmail(ctx context.Context, q *dbq.Queries, now time.Time, userID *uuid.UUID, template string, o audit.Origin) error {
	p := emailJob{UserID: userID, Template: template}
	if userID != nil { // no-op rows for unknown emails keep no request data
		p.Origin = originOf(o)
	}
	// Unknown emails still pay for one insert (uniform timing), but the row is born
	// completed so it never competes with real emails for workers.
	_, err := jobs.Enqueue(ctx, q, now, jobs.Spec{Kind: jobEmail, Payload: p, MaxAttempts: 8, Completed: userID == nil})
	return err
}

type loginFailedJob struct {
	UserID *uuid.UUID `json:"user_id"` // nil: unknown email
	At     time.Time  `json:"at"`
	Origin jobOrigin  `json:"origin,omitzero"`
}

func enqueueLoginFailed(ctx context.Context, q *dbq.Queries, now time.Time, userID *uuid.UUID, o audit.Origin) error {
	p := loginFailedJob{UserID: userID, At: now}
	if userID != nil {
		p.Origin = originOf(o)
	}
	_, err := jobs.Enqueue(ctx, q, now, jobs.Spec{Kind: jobLoginFailed, Payload: p, MaxAttempts: 5, Completed: userID == nil})
	return err
}

// Eraser deletes one module's data of an account being erased, using q bound to the
// erasure transaction. Every module that stores user-owned data registers one
// (docs/security.md §9); identity itself anonymizes the users row last.
type Eraser func(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error

// OnErase registers an Eraser. Call during wiring, before jobs run.
func (s *Service) OnErase(e Eraser) { s.erasers = append(s.erasers, e) }

// RegisterJobs adds identity's job handlers and periodic tasks to r.
func (s *Service) RegisterJobs(r *jobs.Runner) {
	r.Handle(jobEmail, s.runEmail)
	r.Handle(jobLoginFailed, s.runLoginFailed)
	r.Handle(jobPurgeAuth, s.runPurgeAuth)
	r.Every(jobPurgeAuth, time.Hour)
	r.Handle(jobEraseDue, s.runEraseDue)
	r.Every(jobEraseDue, time.Hour)
	r.Handle(jobEraseAccount, s.runEraseAccount)
}

func (s *Service) runEmail(ctx context.Context, j jobs.Job) error {
	var p emailJob
	if err := j.Decode(&p); err != nil {
		return err
	}
	if p.UserID == nil {
		return nil
	}
	u, err := dbq.New(s.pool).GetUserForMail(ctx, *p.UserID)
	if database.IsNoRows(err) {
		return nil // erased or never existed
	}
	if err != nil {
		return err
	}
	active := u.Status == "active"

	var msg reefmail.Message
	switch p.Template {
	case purposeVerify, purposeReset:
		if !active || (p.Template == purposeVerify && u.EmailVerifiedAt != nil) {
			return nil
		}
		// A retry issues a fresh token and revokes the unsent one; at most one link of
		// each purpose is ever live.
		path, ttl, action := "/verify-email", s.cfg.VerifyTokenTTL, ""
		if p.Template == purposeReset {
			path, ttl, action = "/reset-password", s.cfg.ResetTokenTTL, "auth.password_reset_requested"
		}
		now := s.clock.Now()
		err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
			q := dbq.New(tx)
			token, err := s.issueToken(ctx, q, u.ID, u.Email, p.Template, now, ttl)
			if err != nil {
				return err
			}
			msg = s.email(p.Template, u.Locale, u.Email, path, token)
			if action == "" || j.Attempt > 1 { // audit the request once, not per retry
				return nil
			}
			return audit.Record(ctx, q, now, p.Origin.audit(), audit.Entry{ActorID: &u.ID, Action: action, TargetType: "user", TargetID: &u.ID})
		})
		if errors.Is(err, errTokenPending) {
			return nil // a concurrent job of the same purpose is sending one
		}
		if err != nil {
			return err
		}
	case templateAlreadyRegistered:
		if !active {
			return nil
		}
		msg = s.email(p.Template, u.Locale, u.Email, "/login", "")
	case templatePasswordChanged:
		if !active {
			return nil
		}
		msg = s.email(p.Template, u.Locale, u.Email, "/forgot-password", "")
	case templateAccountDeactivated:
		if u.Status != "deactivated" {
			return nil // reactivated or changed since
		}
		msg = s.email(p.Template, u.Locale, u.Email, "/", "")
	default:
		return jobs.Permanent(fmt.Errorf("unknown email template %q", p.Template))
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		// Stored in jobs.last_error: the reason only, never the reply text (it can echo
		// the recipient address).
		return fmt.Errorf("send %s: %s", p.Template, reefmail.SafeReason(err))
	}
	return nil
}

func (s *Service) runLoginFailed(ctx context.Context, j jobs.Job) error {
	var p loginFailedJob
	if err := j.Decode(&p); err != nil {
		return err
	}
	if p.UserID == nil {
		return nil
	}
	err := audit.Record(ctx, dbq.New(s.pool), p.At, p.Origin.audit(), audit.Entry{
		ActorID: p.UserID, Action: "auth.login_failed", TargetType: "user", TargetID: p.UserID,
	})
	if database.IsForeignKeyViolation(err) {
		return nil // the account no longer exists
	}
	return err
}

// runPurgeAuth deletes sessions that can never resolve again (expired, or revoked more
// than a day ago) and email-link tokens a day after they expired (open item J3).
func (s *Service) runPurgeAuth(ctx context.Context, _ jobs.Job) error {
	const batch = 1000
	q := dbq.New(s.pool)
	now := s.clock.Now()
	for {
		n, err := q.PurgeSessions(ctx, dbq.PurgeSessionsParams{Now: now, RevokedBefore: now.Add(-24 * time.Hour), Batch: batch})
		if err != nil {
			return fmt.Errorf("purge sessions: %w", err)
		}
		if n < batch {
			break
		}
	}
	for {
		n, err := q.PurgeAuthTokens(ctx, dbq.PurgeAuthTokensParams{ExpiredBefore: now.Add(-24 * time.Hour), Batch: batch})
		if err != nil {
			return fmt.Errorf("purge auth tokens: %w", err)
		}
		if n < batch {
			return nil
		}
	}
}

// runEraseDue enqueues one erasure job per account deactivated more than
// AccountEraseDelay ago. The user id is the dedupe key, so overlapping runs are harmless.
func (s *Service) runEraseDue(ctx context.Context, _ jobs.Job) error {
	const batch = 500
	q := dbq.New(s.pool)
	now := s.clock.Now()
	var afterAt *time.Time
	var afterID *uuid.UUID
	for {
		due, err := q.ListUsersDueForErasure(ctx, dbq.ListUsersDueForErasureParams{
			DeactivatedBefore: now.Add(-AccountEraseDelay), AfterAt: afterAt, AfterID: afterID, Batch: batch,
		})
		if err != nil {
			return fmt.Errorf("list accounts due for erasure: %w", err)
		}
		for _, u := range due {
			// Keyed by the deactivation too, so a later deactivation of the same account is
			// not blocked by an earlier job's dedupe row.
			key := "user:" + u.ID.String()
			if u.DeactivatedAt != nil {
				key += ":" + u.DeactivatedAt.UTC().Format(time.RFC3339Nano)
			}
			created, err := jobs.Enqueue(ctx, q, now, jobs.Spec{Kind: jobEraseAccount, DedupeKey: key, Payload: eraseJob{UserID: u.ID}, MaxAttempts: 5})
			if err != nil {
				return err
			}
			// Erasure must eventually happen: a buried attempt gets new attempts every
			// run (and keeps logging errors) instead of blocking until cleanup.
			if !created {
				if revived, err := jobs.Revive(ctx, q, now, jobEraseAccount, key); err != nil {
					return err
				} else if revived {
					s.log.Error("account erasure failed before; retrying", "user_id", u.ID)
				}
			}
		}
		if len(due) < batch {
			return nil
		}
		last := due[len(due)-1]
		afterAt, afterID = last.DeactivatedAt, &last.ID
	}
}

type eraseJob struct {
	UserID uuid.UUID `json:"user_id"`
}

// runEraseAccount erases a deactivated account in one transaction (docs/security.md §9):
// other modules delete the account's private data through their Erasers, then
// identity deletes credentials and anonymizes the users row, which stays because audit
// rows (and later the reputation ledger) reference it. Re-running is a no-op.
func (s *Service) runEraseAccount(ctx context.Context, j jobs.Job) error {
	var p eraseJob
	if err := j.Decode(&p); err != nil {
		return err
	}
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := q.LockUserForErasure(ctx, dbq.LockUserForErasureParams{
			ID: p.UserID, DeactivatedBefore: now.Add(-AccountEraseDelay),
		}); database.IsNoRows(err) {
			return nil // already erased, reactivated, or deactivated again more recently
		} else if err != nil {
			return err
		}
		for _, erase := range s.erasers {
			if err := erase(ctx, q, p.UserID); err != nil {
				return fmt.Errorf("erase module data: %w", err)
			}
		}
		if err := q.DeleteUserCredentials(ctx, p.UserID); err != nil {
			return err
		}
		if err := q.DeleteUserJobs(ctx, dbq.DeleteUserJobsParams{UserID: p.UserID, CurrentJobID: j.ID}); err != nil {
			return err
		}
		if _, err := q.AnonymizeUser(ctx, dbq.AnonymizeUserParams{ID: p.UserID, Now: now}); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, audit.Origin{}, audit.Entry{Action: "account.erased", TargetType: "user", TargetID: &p.UserID})
	})
}
