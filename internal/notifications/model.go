// Package notifications turns due and missed work into in-app reminders with opt-in email
// (docs/user-stories.md US-NT-1, milestone 2.9).
//
// Business rules:
//   - Modules register reminder sources; a scan job (every 15 minutes) collects what is due
//     or missed and stores one notification per occurrence. dedupe_key names the
//     occurrence, so the same reminder is never created or delivered twice (AC1).
//   - Only the tank owner is reminded. Each category can be turned off with the
//     preference reminders_<category> (default on); email is opt-in (email_reminders,
//     default off) and needs a verified address (AC2).
//   - Emails are batched per recipient and sent by a durable job; a notification is
//     emailed at most once.
//   - Reminders never complete anything: they only point at work the keeper can do.
package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
)

// Categories (CHECK constraint on notifications.category).
const (
	CategoryDosing      = "dosing"
	CategoryWaterChange = "water_change"
	CategoryMaintenance = "maintenance"
)

// Kinds.
const (
	KindDosingMissed       = "dosing.missed"
	KindWaterChangeDueSoon = "water_change.upcoming"
	KindWaterChangeOverdue = "water_change.overdue"
	KindMaintenanceDue     = "maintenance.due"
	KindMaintenanceOverdue = "maintenance.overdue"
)

// Preference keys (validated by identity's notification_prefs).
const (
	prefEmail  = "email_reminders"
	prefPrefix = "reminders_"
)

// Reminder is one thing a source wants to remind a tank owner about.
type Reminder struct {
	UserID     uuid.UUID
	TankID     uuid.UUID
	Category   string
	Kind       string
	DedupeKey  string    // names the occurrence, e.g. "dosing.missed:<event id>"
	OccurredAt time.Time // when it is due / was missed
	// Payload holds render-ready parameters the recipient can already see (names, times,
	// amounts); never secrets or other users' data.
	Payload map[string]any
}

// Source emits the reminders due at now. It must page through its data (emit may be
// called many times) and look only at a bounded recent window: the scan runs every
// ScanInterval and dedupe makes repeats harmless.
type Source func(ctx context.Context, q *dbq.Queries, now time.Time, emit func(Reminder) error) error

// Notification is a stored reminder.
type Notification struct {
	ID         uuid.UUID
	TankID     *uuid.UUID
	Category   string
	Kind       string
	Payload    []byte
	OccurredAt time.Time
	ReadAt     *time.Time
	CreatedAt  time.Time
}

// wants reports whether prefs allow in-app reminders of category (default on).
func wants(prefs map[string]bool, category string) bool {
	on, set := prefs[prefPrefix+category]
	return !set || on
}

// wantsEmail reports whether the recipient opted in to reminder email (default off).
func wantsEmail(prefs map[string]bool) bool { return prefs[prefEmail] }

// --- email ---

// Reminder email limits per recipient.
const (
	maxEmailsPerDay = 4
	minEmailGap     = 3 * time.Hour
)

// Email copy per locale. Like identity's emails it contains no user-written text (tank
// names, titles) and no display name: whoever set up the account may not own the mailbox,
// so ReefCare must never relay their words. It lists counts per category and links to the
// app, where the details are.
type emailCopy struct {
	subject, intro, footer string
	categories             map[string]string
}

var emailCopies = map[string]emailCopy{
	"vi": {
		subject: "Nhắc việc ReefCare",
		intro:   "Xin chào,\n\nBạn có %d nhắc việc mới trên ReefCare:\n",
		footer:  "\nXem chi tiết tại: %s\n\nBạn nhận email này vì đã bật email nhắc việc. Tắt trong phần Cài đặt của ReefCare.\n",
		categories: map[string]string{
			CategoryDosing: "Dosing", CategoryWaterChange: "Thay nước", CategoryMaintenance: "Bảo trì",
		},
	},
	"en": {
		subject: "ReefCare reminders",
		intro:   "Hello,\n\nYou have %d new reminders in ReefCare:\n",
		footer:  "\nSee the details at: %s\n\nYou get this email because reminder email is on. Turn it off in ReefCare settings.\n",
		categories: map[string]string{
			CategoryDosing: "Dosing", CategoryWaterChange: "Water changes", CategoryMaintenance: "Maintenance",
		},
	},
}

var categoryOrder = []string{CategoryDosing, CategoryWaterChange, CategoryMaintenance}

// emailBody renders a reminder email from counts per category.
func emailBody(locale, link string, counts map[string]int) (subject, body string) {
	c, ok := emailCopies[locale]
	if !ok {
		c = emailCopies["vi"]
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	var b strings.Builder
	fmt.Fprintf(&b, c.intro, total)
	for _, cat := range categoryOrder {
		if n := counts[cat]; n > 0 {
			fmt.Fprintf(&b, "- %s: %d\n", c.categories[cat], n)
		}
	}
	fmt.Fprintf(&b, c.footer, link)
	return c.subject, b.String()
}

// quietHours reports whether it is night (22:00–07:00) in the recipient's zone; reminder
// email waits until morning (a "due today" reminder is created just after local midnight).
func quietHours(now time.Time, tz string) bool {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	h := now.In(loc).Hour()
	return h >= 22 || h < 7
}

// mayEmail applies the per-recipient limits: at most maxEmailsPerDay reminder emails in
// 24 hours, minEmailGap apart. Reminders that can't be sent now stay pending.
func mayEmail(sentLastDay int32, lastSent, now time.Time) bool {
	return sentLastDay < maxEmailsPerDay && !lastSent.After(now.Add(-minEmailGap))
}
