package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/mail"
)

// Job kinds and timing.
const (
	jobScan  = "notifications.scan"
	jobEmail = "notifications.email"
	jobPurge = "notifications.purge"

	// ScanInterval is how often sources are scanned.
	ScanInterval = 15 * time.Minute
	// Retention: read notifications are deleted after readRetention, all (read or not)
	// after maxRetention.
	readRetention = 90 * 24 * time.Hour
	maxRetention  = 180 * 24 * time.Hour

	scanBatch = 500
)

// Service stores notifications, runs the scan and delivers email.
type Service struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	mailer  mail.Sender
	baseURL string
	log     *slog.Logger
	sources []Source
}

// NewService wires the service; baseURL is the web app's public URL (links in email).
func NewService(pool *pgxpool.Pool, clk clock.Clock, mailer mail.Sender, baseURL string, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, mailer: mailer, baseURL: baseURL, log: log}
}

// AddSource registers a reminder source. Call during wiring.
func (s *Service) AddSource(src Source) { s.sources = append(s.sources, src) }

// RegisterJobs registers the scan, email and purge jobs.
func (s *Service) RegisterJobs(r *jobs.Runner) {
	r.Handle(jobScan, s.runScan)
	r.Every(jobScan, ScanInterval)
	r.Handle(jobEmail, s.runEmail)
	r.Handle(jobPurge, s.runPurge)
	r.Every(jobPurge, 24*time.Hour)
}

// --- scan ---

func (s *Service) runScan(ctx context.Context, _ jobs.Job) error {
	now := s.clock.Now()
	q := dbq.New(s.pool)
	var buf []Reminder
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		err := s.store(ctx, now, buf)
		buf = buf[:0]
		return err
	}
	// A failing source is logged and the others still run; the job fails (and retries)
	// afterwards. Repeats are harmless: dedupe keys make every insert idempotent.
	var errs []error
	for i, src := range s.sources {
		err := src(ctx, q, now, func(r Reminder) error {
			buf = append(buf, r)
			if len(buf) >= scanBatch {
				return flush()
			}
			return nil
		})
		if err == nil {
			err = flush()
		}
		if err != nil {
			buf = buf[:0]
			s.log.Error("reminder source failed", "source", i, "err", err)
			errs = append(errs, fmt.Errorf("reminder source %d: %w", i, err))
		}
	}
	if err := s.queueEmails(ctx, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

type recipient struct {
	active, verified bool
	prefs            map[string]bool
}

// store creates the reminders the recipients want, and queues one email job per recipient
// with something new to email.
func (s *Service) store(ctx context.Context, now time.Time, rs []Reminder) error {
	ids := make([]uuid.UUID, 0, len(rs))
	seen := map[uuid.UUID]bool{}
	for _, r := range rs {
		if !seen[r.UserID] {
			seen[r.UserID] = true
			ids = append(ids, r.UserID)
		}
	}
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		rows, err := q.GetNotificationRecipients(ctx, ids)
		if err != nil {
			return err
		}
		who := make(map[uuid.UUID]recipient, len(rows))
		for _, r := range rows {
			var prefs map[string]bool
			if len(r.NotificationPrefs) > 0 {
				if err := json.Unmarshal(r.NotificationPrefs, &prefs); err != nil {
					return fmt.Errorf("decode notification prefs: %w", err)
				}
			}
			who[r.ID] = recipient{active: r.Status == "active", verified: r.EmailVerified, prefs: prefs}
		}
		for _, r := range rs {
			u, ok := who[r.UserID]
			if !ok || !u.active || !wants(u.prefs, r.Category) {
				continue
			}
			var email *string
			if wantsEmail(u.prefs) && u.verified {
				pending := "pending"
				email = &pending
			}
			payload, err := json.Marshal(r.Payload)
			if err != nil {
				return fmt.Errorf("encode notification payload: %w", err)
			}
			tank := r.TankID
			_, err = q.InsertNotification(ctx, dbq.InsertNotificationParams{
				UserID: r.UserID, TankID: &tank, Category: r.Category, Kind: r.Kind, DedupeKey: r.DedupeKey,
				Payload: payload, OccurredAt: r.OccurredAt, EmailStatus: email, Now: now,
			})
			if err != nil && !database.IsNoRows(err) { // no row: already reminded
				return err
			}
		}
		return nil
	})
}

// queueEmails queues one email job per recipient with pending reminders, once per scan
// bucket. Running it after every scan also picks up reminders left pending by a rate
// limit or a job that gave up.
func (s *Service) queueEmails(ctx context.Context, now time.Time) error {
	q := dbq.New(s.pool)
	users, err := q.ListUsersWithPendingEmail(ctx)
	if err != nil {
		return err
	}
	bucket := now.Truncate(ScanInterval).UTC().Format(time.RFC3339)
	for _, id := range users {
		if _, err := jobs.Enqueue(ctx, q, now, jobs.Spec{
			Kind: jobEmail, Payload: emailJob{UserID: id}, DedupeKey: id.String() + ":" + bucket, MaxAttempts: 5,
		}); err != nil {
			return err
		}
	}
	return nil
}

// --- email ---

type emailJob struct {
	UserID uuid.UUID `json:"user_id"`
}

// runEmail sends the recipient's pending reminders as one email of counts, within the
// per-recipient limits. The rows are claimed in the sending transaction: a failed send rolls
// back and the job retries; a crash after the send but before the commit can repeat one
// email (at least once delivery).
func (s *Service) runEmail(ctx context.Context, j jobs.Job) error {
	var p emailJob
	if err := j.Decode(&p); err != nil {
		return err
	}
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		u, err := q.GetUserForNotificationEmail(ctx, p.UserID)
		if database.IsNoRows(err) {
			return nil // erased
		}
		if err != nil {
			return err
		}
		var prefs map[string]bool
		if len(u.NotificationPrefs) > 0 {
			if err := json.Unmarshal(u.NotificationPrefs, &prefs); err != nil {
				return fmt.Errorf("decode notification prefs: %w", err)
			}
		}
		if u.Status != "active" || !u.EmailVerified || !wantsEmail(prefs) {
			return q.SkipPendingNotificationEmails(ctx, p.UserID)
		}
		recent, err := q.RecentNotificationEmails(ctx, dbq.RecentNotificationEmailsParams{UserID: p.UserID, Since: now.Add(-24 * time.Hour)})
		if err != nil {
			return err
		}
		if !mayEmail(recent.Sent, recent.LastSent, now) || quietHours(now, u.Timezone) {
			return nil // stays pending; a later scan queues it again
		}
		cats, err := q.ClaimPendingNotificationEmails(ctx, dbq.ClaimPendingNotificationEmailsParams{UserID: p.UserID, Now: now})
		if err != nil || len(cats) == 0 {
			return err // nothing left: another job sent them
		}
		counts := map[string]int{}
		for _, c := range cats {
			counts[c]++
		}
		subject, body := emailBody(u.Locale, s.baseURL+"/notifications", counts)
		if err := s.mailer.Send(ctx, mail.Message{To: u.Email, Subject: subject, Body: body, Kind: "reminders"}); err != nil {
			// Stored in jobs.last_error: the reason only, never the address or text.
			return fmt.Errorf("send reminders: %s", mail.SafeReason(err))
		}
		return nil
	})
}

// --- purge ---

func (s *Service) runPurge(ctx context.Context, _ jobs.Job) error {
	now := s.clock.Now()
	q := dbq.New(s.pool)
	for _, purge := range []func() (int64, error){
		func() (int64, error) { return q.PurgeReadNotifications(ctx, now.Add(-readRetention)) },
		func() (int64, error) { return q.PurgeOldNotifications(ctx, now.Add(-maxRetention)) },
	} {
		for {
			n, err := purge()
			if err != nil {
				return err
			}
			if n < 5000 {
				break
			}
		}
	}
	return nil
}

// --- reading ---

// Cursor is the keyset of the notification list.
type Cursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// List returns the user's notifications, newest first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int32, cur *Cursor) ([]Notification, error) {
	p := dbq.ListNotificationsParams{UserID: userID, CursorAt: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), CursorID: uuid.Max, RowLimit: limit}
	if cur != nil {
		p.CursorAt, p.CursorID = cur.At, cur.ID
	}
	q := dbq.New(s.pool)
	var rows []dbq.ListNotificationsRow
	if unreadOnly {
		unread, err := q.ListUnreadNotifications(ctx, dbq.ListUnreadNotificationsParams(p))
		if err != nil {
			return nil, err
		}
		for _, r := range unread {
			rows = append(rows, dbq.ListNotificationsRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListNotifications(ctx, p); err != nil {
			return nil, err
		}
	}
	out := make([]Notification, len(rows))
	for i, r := range rows {
		out[i] = Notification{ID: r.ID, TankID: r.TankID, Category: r.Category, Kind: r.Kind, Payload: r.Payload,
			OccurredAt: r.OccurredAt, ReadAt: r.ReadAt, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// UnreadCount returns the number of unread notifications, capped at 100.
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return dbq.New(s.pool).CountUnreadNotifications(ctx, userID)
}

// MarkRead marks one of the user's notifications read (idempotent). Other users'
// notifications are not found.
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	n, err := dbq.New(s.pool).MarkNotificationRead(ctx, dbq.MarkNotificationReadParams{ID: id, UserID: userID, Now: s.clock.Now()})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound()
	}
	return nil
}

// MarkAllRead marks every unread notification of the user read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := dbq.New(s.pool).MarkAllNotificationsRead(ctx, dbq.MarkAllNotificationsReadParams{UserID: userID, Now: s.clock.Now()})
	return err
}

// EraseUserData deletes the account's notifications (identity.Eraser); it must run before
// tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.DeleteUserNotifications(ctx, userID); err != nil {
		return err
	}
	return q.DeleteTankOwnerNotifications(ctx, userID)
}
