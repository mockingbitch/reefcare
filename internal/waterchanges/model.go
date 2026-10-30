// Package waterchanges schedules and records water changes (docs/user-stories.md
// US-WC-1..2, docs/domain-model.md "Water changes").
//
// Business rules:
//   - Recurring schedules use the recurrence subset in the schedule's time zone; their
//     occurrences are materialized ahead (Horizon) by a job and on create/edit.
//   - A scheduled occurrence whose time passed by more than OverdueGrace is overdue. It
//     stays overdue until the keeper completes or skips it; it is never completed
//     automatically. Editing or pausing a schedule replaces only future occurrences.
//   - Estimates (fraction of the system replaced) use the tank's estimated system volume and
//     always state their assumptions.
package waterchanges

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/recurrence"
	"reefcare/internal/platform/textcheck"
)

const (
	// OverdueGrace is how long after its time a scheduled change still counts as due
	// rather than overdue (a Sunday-morning change done Sunday evening is on time). Frequent
	// schedules get less: half the spacing between occurrences (see grace).
	OverdueGrace = 24 * time.Hour
	// Horizon is how far ahead occurrences exist.
	Horizon = 35 * 24 * time.Hour
	// maxPerPass bounds occurrences created per schedule and materialization pass.
	maxPerPass = 200
	// maxFutureSkew is how far in the future a "completed at" time may be (clock skew).
	maxFutureSkew = 5 * time.Minute
	// maxPlanAhead bounds one-time planned changes.
	maxPlanAhead = 2 * 365 * 24 * time.Hour
	// largeChange is the fraction above which the UI recommends extra care (community rule
	// of thumb, not a hard limit; large emergency changes are legitimate).
	largeChange = "0.5"
)

// earliest is the earliest accepted date for schedules and recorded changes.
var earliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// grace is the overdue grace of a rule: OverdueGrace, or half the smallest spacing between
// occurrences when that is shorter, so a change is overdue before the next one is due.
func grace(r recurrence.Rule) time.Duration { return min(OverdueGrace, r.MinGap()/2) }

// Schedule statuses.
const (
	StatusActive = "active"
	StatusPaused = "paused"
	StatusEnded  = "ended"
)

// Event statuses. "overdue" is derived (Event.Overdue), never stored.
const (
	EventScheduled = "scheduled"
	EventCompleted = "completed"
	EventSkipped   = "skipped"
)

// Schedule is a recurring water-change plan.
type Schedule struct {
	ID             uuid.UUID
	TankID         uuid.UUID
	RRule          string
	Timezone       string
	StartsOn       recurrence.Date
	EndsOn         *recurrence.Date
	PlannedVolumeL decimal.Decimal
	SaltMix        *string
	Notes          *string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int32
	// Next lists the next few occurrences (none unless active).
	Next     []time.Time
	Estimate *Estimate

	materializedUntil *time.Time
}

func (s Schedule) series() (recurrence.Series, error) {
	rule, err := recurrence.Parse(s.RRule)
	if err != nil {
		return recurrence.Series{}, err
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return recurrence.Series{}, err
	}
	return recurrence.Series{Rule: rule, Location: loc, StartsOn: s.StartsOn, EndsOn: s.EndsOn}, nil
}

// Event is one planned or performed water change.
type Event struct {
	ID             uuid.UUID
	TankID         uuid.UUID
	ScheduleID     *uuid.UUID
	ScheduledFor   *time.Time
	OverdueAfter   *time.Time
	PlannedVolumeL *decimal.Decimal
	Status         string
	Overdue        bool
	CompletedAt    *time.Time
	ActualVolumeL  *decimal.Decimal
	SystemVolumeL  *decimal.Decimal // captured at completion
	SaltMix        *string
	Notes          *string
	SkipReason     *string
	ResolvedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int32
	Estimate       *Estimate
}

// overdue is the derived state: still scheduled after its overdue_after time.
func overdue(status string, overdueAfter *time.Time, now time.Time) bool {
	return status == EventScheduled && overdueAfter != nil && overdueAfter.Before(now)
}

// Estimate relates a change's volume to the tank's estimated total system volume. It is
// the volume exchanged relative to the system — not a measured share of water replaced —
// and assumes complete mixing and the volume as entered; clients must show the
// assumptions and flags with it.
type Estimate struct {
	FractionOfSystem decimal.Decimal // volume / system volume, 3 decimals, never clamped
	SystemVolumeL    decimal.Decimal
	Basis            string   // "actual" (completed) or "planned"
	Assumptions      []string // stable codes the client localizes
	Flags            []string // stable warning codes, possibly empty
}

// Estimate bases, assumption and flag codes.
const (
	BasisActual  = "actual"
	BasisPlanned = "planned"

	AssumeTotalSystem      = "fraction_of_total_system" // display + sump + plumbing, as entered
	AssumeCompleteMixing   = "complete_mixing"
	AssumeSystemVolumeUser = "system_volume_as_entered"

	// FlagExceedsSystem: more than the system volume; check the volume or the unit.
	FlagExceedsSystem = "exceeds_system_volume"
	// FlagLargeChange: new water should closely match salinity, temperature, alkalinity.
	FlagLargeChange = "large_change"
)

// estimate returns nil when there is no volume to compare.
func estimate(volume *decimal.Decimal, systemVolume decimal.Decimal, basis string) *Estimate {
	if volume == nil || systemVolume.Sign() <= 0 {
		return nil
	}
	f, err := volume.DivRound(systemVolume, 3)
	if err != nil {
		return nil
	}
	e := &Estimate{
		FractionOfSystem: f, SystemVolumeL: systemVolume, Basis: basis,
		Assumptions: []string{AssumeTotalSystem, AssumeCompleteMixing, AssumeSystemVolumeUser}, Flags: []string{},
	}
	if volume.Cmp(systemVolume) > 0 {
		e.Flags = append(e.Flags, FlagExceedsSystem)
	} else if f.Cmp(decimal.MustParse(largeChange)) > 0 {
		e.Flags = append(e.Flags, FlagLargeChange)
	}
	return e
}

// --- input validation (pure) ---

type checker struct{ v *apperr.Validator }

func (c checker) volume(field, s string) decimal.Decimal {
	d, err := decimal.Parse(strings.TrimSpace(s))
	switch {
	case err != nil:
		c.v.Add(field, "invalid_decimal", nil)
	case d.Sign() <= 0:
		c.v.Add(field, "must_be_positive", nil)
	case !d.FitsNumeric(9, 2):
		c.v.Add(field, "out_of_range", map[string]string{"max_integer_digits": "7", "max_decimals": "2"})
	}
	return d
}

func (c checker) optVolume(field string, s *string) *decimal.Decimal {
	if s == nil {
		return nil
	}
	d := c.volume(field, *s)
	return &d
}

func (c checker) text(field string, s *string, maxLen int, multiLine bool) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	ok := textcheck.SingleLine(t)
	if multiLine {
		ok = textcheck.MultiLine(t)
	}
	if utf8.RuneCountInString(t) > maxLen {
		c.v.Add(field, "length", map[string]string{"max": strconv.Itoa(maxLen)})
	} else if !ok {
		c.v.Add(field, "invalid_characters", nil)
	}
	return &t
}

func (c checker) rrule(s string) string {
	r, err := recurrence.Parse(s)
	if err != nil {
		code := recurrence.CodeSyntax
		var e *recurrence.Error
		if errors.As(err, &e) {
			code = e.Code
		}
		c.v.Add("rrule", code, nil)
		return s
	}
	return r.String()
}

func (c checker) timezone(s string) string {
	if s == "" || s == "Local" || len(s) > 64 {
		c.v.Add("timezone", "invalid_timezone", nil)
		return s
	}
	if _, err := time.LoadLocation(s); err != nil {
		c.v.Add("timezone", "invalid_timezone", nil)
	}
	return s
}

// date parses YYYY-MM-DD between 2000-01-01 and ten years after now.
func (c checker) date(field, s string, now time.Time) recurrence.Date {
	d, err := recurrence.ParseDate(strings.TrimSpace(s))
	if err != nil {
		c.v.Add(field, "invalid_date", map[string]string{"format": "YYYY-MM-DD"})
		return d
	}
	if t := dateToTime(d); t.Before(earliest) || t.After(now.AddDate(10, 0, 0)) {
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01"})
	}
	return d
}

// instant checks an event time is after 2000-01-01 and at most ahead of now.
func (c checker) instant(field string, t time.Time, now time.Time, ahead time.Duration) {
	switch {
	case t.Before(earliest):
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01T00:00:00Z"})
	case t.After(now.Add(ahead)):
		c.v.Add(field, "in_future", nil)
	}
}

func (c checker) optDate(field string, s *string, now time.Time) *recurrence.Date {
	if s == nil {
		return nil
	}
	d := c.date(field, *s, now)
	return &d
}

func dateToTime(d recurrence.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func timeToDate(t time.Time) recurrence.Date { return recurrence.DateOf(t.UTC()) }
