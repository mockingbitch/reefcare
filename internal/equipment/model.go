// Package equipment tracks devices on a tank with versioned configurations, and
// interval-based maintenance (docs/user-stories.md US-EQ-1..3, milestone 2.6).
//
// Business rules:
//   - Several devices of the same model may be on one tank. A device is removed (with a
//     date and optional reason), never deleted; its history stays.
//   - Configuration changes create a new immutable version and an
//     equipment.configuration_changed timeline event; earlier versions stay visible.
//   - A maintenance plan is due interval_days after the local date it was last performed
//     (or on its start date). It is overdue once that date has passed in the plan's time
//     zone — derived, never stored, never completed automatically.
package equipment

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/recurrence"
	"reefcare/internal/platform/textcheck"
)

// Limits.
const (
	MaxEquipmentPerTank = 200
	MaxPlansPerTank     = 100
	maxSettingsKeys     = 50
	maxFutureSkew       = 5 * time.Minute
)

var (
	earliest   = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	categories = map[string]bool{
		"lighting": true, "return_pump": true, "flow_pump": true, "skimmer": true, "heater": true, "chiller": true,
		"dosing_pump": true, "ato": true, "reactor": true, "uv": true, "controller": true, "filter": true,
		"sensor": true, "other": true,
	}
)

// Equipment is a device on a tank.
type Equipment struct {
	ID            uuid.UUID
	TankID        uuid.UUID
	Category      string
	Name          string
	Manufacturer  *string
	Model         *string
	SerialNumber  *string
	PurchasedOn   *recurrence.Date
	WarrantyUntil *recurrence.Date
	InstalledAt   time.Time
	RemovedAt     *time.Time
	RemovalReason *string
	Notes         *string
	Current       *Configuration // latest version, if any
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int32
}

// Configuration is one immutable settings version.
type Configuration struct {
	ID          uuid.UUID
	VersionNo   int32
	Settings    json.RawMessage
	EffectiveAt time.Time
	Note        *string
	CreatedAt   time.Time
}

// Plan is a recurring maintenance task.
type Plan struct {
	ID              uuid.UUID
	TankID          uuid.UUID
	EquipmentID     *uuid.UUID
	EquipmentName   *string
	Title           string
	IntervalDays    int32
	Timezone        string
	StartsOn        recurrence.Date
	LastPerformedAt *time.Time
	NextDueOn       recurrence.Date
	Active          bool
	Overdue         bool
	DueToday        bool
	Notes           *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int32
}

// Event is maintenance that was done.
type Event struct {
	ID          uuid.UUID
	TankID      uuid.UUID
	PlanID      *uuid.UUID
	EquipmentID *uuid.UUID
	Title       string
	PerformedAt time.Time
	Notes       *string
	CreatedAt   time.Time
}

// localToday is today's date in tz.
func localToday(now time.Time, tz string) recurrence.Date {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return recurrence.DateOf(now.In(loc))
}

// nextDue is the local date of last + interval days, or starts on a later start date.
func nextDue(last *time.Time, starts recurrence.Date, interval int32, tz string) recurrence.Date {
	if last == nil {
		return starts
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	d := recurrence.DateOf(last.In(loc))
	due := recurrence.DateOf(time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC).AddDate(0, 0, int(interval)))
	if due.Before(starts) {
		return starts
	}
	return due
}

// decorate derives the plan's due state at now.
func decorate(p Plan, now time.Time) Plan {
	today := localToday(now, p.Timezone)
	p.Overdue = p.Active && p.NextDueOn.Before(today)
	p.DueToday = p.Active && p.NextDueOn == today
	return p
}

// --- validation ---

type checker struct{ v *apperr.Validator }

func (c checker) name(field, s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(t); n < 1 || n > maxLen {
		c.v.Add(field, "length", map[string]string{"min": "1", "max": strconv.Itoa(maxLen)})
	} else if !textcheck.SingleLine(t) {
		c.v.Add(field, "invalid_characters", nil)
	}
	return t
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

func (c checker) category(s string) string {
	c.v.Check(categories[s], "category", "unsupported", nil)
	return s
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
	if t := dateToTime(d); t.Before(earliest.AddDate(-30, 0, 0)) || t.After(now.AddDate(10, 0, 0)) {
		c.v.Add(field, "out_of_range", nil)
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

// instant checks an event time: after 2000-01-01 and not in the future.
func (c checker) instant(field string, t, now time.Time) {
	switch {
	case t.Before(earliest):
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01T00:00:00Z"})
	case t.After(now.Add(maxFutureSkew)):
		c.v.Add(field, "in_future", nil)
	}
}

func (c checker) interval(n int32) int32 {
	c.v.Check(n >= 1 && n <= 730, "interval_days", "out_of_range", map[string]string{"min": "1", "max": "730"})
	return n
}

// settings validates a flat JSON object of at most 50 keys whose values are strings (≤ 200
// characters), numbers or booleans. It is free-form metadata (light schedules, pump speeds)
// that the app never interprets.
func (c checker) settings(raw json.RawMessage) json.RawMessage {
	// UseNumber keeps numbers exactly as sent (no float64 round trip).
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil || m == nil || dec.More() {
		c.v.Add("settings", "must_be_object", nil)
		return nil
	}
	if len(m) == 0 || len(m) > maxSettingsKeys {
		c.v.Add("settings", "keys_count", map[string]string{"min": "1", "max": strconv.Itoa(maxSettingsKeys)})
		return nil
	}
	for k, val := range m {
		if n := utf8.RuneCountInString(k); n < 1 || n > 50 || !textcheck.SingleLine(k) {
			c.v.Add("settings."+k, "invalid_key", nil)
			continue
		}
		switch x := val.(type) {
		case string:
			if utf8.RuneCountInString(x) > 200 || !textcheck.SingleLine(x) {
				c.v.Add("settings."+k, "invalid_value", nil)
			}
		case json.Number, bool:
		default:
			c.v.Add("settings."+k, "invalid_value", map[string]string{"allowed": "string,number,boolean"})
		}
	}
	out, err := json.Marshal(m)
	if err != nil || len(out) > 8000 {
		c.v.Add("settings", "too_large", nil)
		return nil
	}
	return out
}

func dateToTime(d recurrence.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func timeToDate(t time.Time) recurrence.Date { return recurrence.DateOf(t.UTC()) }

func optDateToTime(d *recurrence.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := dateToTime(*d)
	return &t
}

func optTimeToDate(t *time.Time) *recurrence.Date {
	if t == nil {
		return nil
	}
	d := timeToDate(*t)
	return &d
}
