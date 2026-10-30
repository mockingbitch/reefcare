// Package dosing records what is dosed into a tank (docs/domain-model.md "Dosing",
// milestone 2.4): products, recurring plans and dose events.
//
// Business rules:
//   - Plans (planned amount, unit, schedule, delivery method) are separate from events
//     (planned vs actual amount, status, executed_at, reason).
//   - Occurrences are materialized Horizon ahead. One that is still scheduled after its
//     grace becomes "missed" — set only by the scheduler, never by assuming a dose
//     happened. The keeper may resolve a missed dose later; missed_at keeps that history.
//   - Skipped and failed doses need a reason; completed and partial doses an actual
//     amount (a completed dose defaults to the planned amount).
//   - No concentration is stored or inferred from a product (.claude/rules/reef-domain.md):
//     ReefCare records amounts, never what they do to the water.
package dosing

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
	// Horizon is how far ahead occurrences exist (docs/architecture.md §5).
	Horizon = 7 * 24 * time.Hour
	// MissedGrace is the longest a scheduled dose may be late before it is missed;
	// frequent plans get half their spacing instead.
	MissedGrace = 12 * time.Hour
	// maxPerPass bounds occurrences created per plan and pass (6 a day for 7 days is 42).
	maxPerPass = 200
	// MaxOwnProducts bounds private products per user.
	MaxOwnProducts = 200
	maxFutureSkew  = 5 * time.Minute
)

var earliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// grace is the missed grace of a rule.
func grace(r recurrence.Rule) time.Duration { return min(MissedGrace, r.MinGap()/2) }

// Units, product forms, delivery methods, statuses (CHECK constraints in 00007_dosing.sql).
var (
	units = map[string]bool{"ml": true, "l": true, "drops": true, "g": true, "mg": true, "tablets": true}
	// unitsByForm: a plan's unit must make sense for the product's form.
	unitsByForm = map[string][]string{
		"liquid": {"ml", "l", "drops"},
		"powder": {"g", "mg", "ml", "l"}, // ml/l: dissolved by the keeper ("prepared solution")
		"tablet": {"tablets"},
		"other":  {"ml", "l", "drops", "g", "mg", "tablets"},
	}
	deliveryMethods = map[string]bool{"manual": true, "dosing_pump": true}
)

// Plan statuses.
const (
	PlanActive    = "active"
	PlanPaused    = "paused"
	PlanCompleted = "completed"
)

// Event statuses.
const (
	Scheduled = "scheduled"
	Completed = "completed"
	Partial   = "partial"
	Skipped   = "skipped"
	Failed    = "failed"
	Missed    = "missed"
	// Unconfirmed is a pump-plan occurrence nobody confirmed: ReefCare cannot know whether
	// the pump delivered it, so it is neither missed nor done.
	Unconfirmed = "unconfirmed"
)

// AllStatuses lists every event status (history filter default).
var AllStatuses = []string{Scheduled, Completed, Partial, Skipped, Failed, Missed, Unconfirmed}

const (
	// MaxPlansPerTank bounds plans (each adds up to 42 occurrences a week).
	MaxPlansPerTank = 50
	// overConfirm: a completion above this multiple of the planned amount must be
	// confirmed explicitly. A heuristic for typos and overdoses, not chemistry.
	overConfirm = "1.5"
	// lateWindow bounds how long before its scheduled time a dose may be recorded.
	lateWindow = 24 * time.Hour
)

// Product is something that is dosed.
type Product struct {
	ID          uuid.UUID
	OwnerID     *uuid.UUID // nil: curated catalog entry
	Name        string
	Brand       *string
	Form        string
	DefaultUnit string
	Notes       *string
	CreatedAt   time.Time
}

// ProductRef is the product as embedded in plans and events.
type ProductRef struct {
	ID    uuid.UUID
	Name  string
	Brand *string
	Form  string
}

// Plan is a recurring dose.
type Plan struct {
	ID             uuid.UUID
	TankID         uuid.UUID
	Product        ProductRef
	Amount         decimal.Decimal
	Unit           string
	RRule          string
	Timezone       string
	StartsOn       recurrence.Date
	EndsOn         *recurrence.Date
	DeliveryMethod string
	Status         string
	Notes          *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int32
	Next           []time.Time

	materializedUntil *time.Time
}

func (p Plan) series() (recurrence.Series, error) {
	rule, err := recurrence.Parse(p.RRule)
	if err != nil {
		return recurrence.Series{}, err
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return recurrence.Series{}, err
	}
	return recurrence.Series{Rule: rule, Location: loc, StartsOn: p.StartsOn, EndsOn: p.EndsOn}, nil
}

// Event is one scheduled occurrence or one dose given.
type Event struct {
	ID            uuid.UUID
	TankID        uuid.UUID
	PlanID        *uuid.UUID
	Product       ProductRef
	ScheduledFor  *time.Time
	MissedAfter   *time.Time
	PlannedAmount *decimal.Decimal
	ActualAmount  *decimal.Decimal
	ActualSource  *string // entered | planned_default
	Unit          string
	Status        string
	ExecutedAt    *time.Time
	Reason        *string
	ReasonCode    *string // system reason: tank_archived, plan_paused, plan_changed
	Notes         *string
	ReportedBy    *string
	MissedAt      *time.Time
	ResolvedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int32
}

// --- validation ---

type checker struct{ v *apperr.Validator }

// amount parses a positive numeric(12,4) amount; more decimals are rejected, not rounded.
func (c checker) amount(field, s string) decimal.Decimal {
	d, err := decimal.Parse(strings.TrimSpace(s))
	switch {
	case err != nil:
		c.v.Add(field, "invalid_decimal", nil)
	case d.Sign() <= 0:
		c.v.Add(field, "must_be_positive", nil)
	case !d.FitsNumeric(12, 4):
		c.v.Add(field, "out_of_range", map[string]string{"max_integer_digits": "8", "max_decimals": "4"})
	}
	return d
}

func (c checker) unit(field, u string) string {
	c.v.Check(units[u], field, "unsupported_unit", map[string]string{"allowed": "ml,l,drops,g,mg,tablets"})
	return u
}

func (c checker) unitForForm(field, u, form string) {
	if !units[u] {
		return // reported by unit()
	}
	for _, ok := range unitsByForm[form] {
		if ok == u {
			return
		}
	}
	c.v.Add(field, "incompatible_unit", map[string]string{"form": form, "allowed": strings.Join(unitsByForm[form], ",")})
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

func (c checker) name(field, s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(t); n < 1 || n > maxLen {
		c.v.Add(field, "length", map[string]string{"min": "1", "max": strconv.Itoa(maxLen)})
	} else if !textcheck.SingleLine(t) {
		c.v.Add(field, "invalid_characters", nil)
	}
	return t
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

func (c checker) optDate(field string, s *string, now time.Time) *recurrence.Date {
	if s == nil {
		return nil
	}
	d := c.date(field, *s, now)
	return &d
}

// instant checks a dose time: after 2000-01-01 and not in the future (clock skew aside).
func (c checker) instant(field string, t, now time.Time) {
	switch {
	case t.Before(earliest):
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01T00:00:00Z"})
	case t.After(now.Add(maxFutureSkew)):
		c.v.Add(field, "in_future", nil)
	}
}

func dateToTime(d recurrence.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func timeToDate(t time.Time) recurrence.Date { return recurrence.DateOf(t.UTC()) }

func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameAmount(a *decimal.Decimal, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Cmp(*b) == 0
}

// deviation compares actual with planned: "over", "under" or "" (equal or not comparable).
func deviation(planned, actual *decimal.Decimal) string {
	if planned == nil || actual == nil {
		return ""
	}
	switch actual.Cmp(*planned) {
	case 1:
		return "over"
	case -1:
		return "under"
	}
	return ""
}

// idempotencyKeyOK accepts 8–128 printable ASCII characters.
func idempotencyKeyOK(k string) bool {
	if len(k) < 8 || len(k) > 128 {
		return false
	}
	for i := range len(k) {
		if k[i] < 0x21 || k[i] > 0x7e {
			return false
		}
	}
	return true
}
