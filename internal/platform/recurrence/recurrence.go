// Package recurrence generates occurrences of recurring schedules (water changes, dosing,
// maintenance) from a small, well-defined subset of RFC 5545 RRULE syntax:
//
//	FREQ=DAILY|WEEKLY|MONTHLY   required
//	INTERVAL=1..365             default 1
//	BYDAY=MO,TU,…               WEEKLY only; default: the weekday of the start date
//	BYMONTHDAY=1..28,…          MONTHLY only; default: the start date's day, capped at 28 so
//	                            every month has it
//	BYHOUR=0..23,…              at most 6 values; default 9
//	BYMINUTE=0..59              one value; default 0
//
// COUNT and UNTIL are not supported: schedules carry an explicit end date instead.
// Occurrences are wall-clock times in the schedule's IANA time zone, so "09:00 every
// Sunday" stays 09:00 across DST changes. A local time that does not exist (spring-forward
// gap) moves forward by the gap in every zone (02:30 → 03:30); an ambiguous one (fall-back)
// takes the first instant. Two candidates landing on the same instant yield one occurrence.
//
// The library is in-house rather than a dependency (docs/architecture.md §2 decision note)
// because the subset is small and DST behaviour must be exactly as tested here.
package recurrence

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Freq is the base period.
type Freq string

// Frequencies.
const (
	Daily   Freq = "DAILY"
	Weekly  Freq = "WEEKLY"
	Monthly Freq = "MONTHLY"
)

// Rule is a parsed recurrence rule.
type Rule struct {
	Freq       Freq
	Interval   int
	ByDay      []time.Weekday // WEEKLY
	ByMonthDay []int          // MONTHLY
	ByHour     []int
	ByMinute   int
}

// Problem codes reported in *Error, usable as API field-error codes.
const (
	CodeSyntax      = "invalid_rrule"
	CodeUnsupported = "unsupported_rrule_part"
)

// Error explains why a rule was rejected.
type Error struct {
	Code   string // CodeSyntax or CodeUnsupported
	Detail string
}

func (e *Error) Error() string { return "rrule: " + e.Detail }

func syntaxErr(format string, a ...any) error {
	return &Error{Code: CodeSyntax, Detail: fmt.Sprintf(format, a...)}
}

var weekdays = map[string]time.Weekday{
	"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday,
	"FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday,
}

var weekdayCodes = map[time.Weekday]string{
	time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE", time.Thursday: "TH",
	time.Friday: "FR", time.Saturday: "SA", time.Sunday: "SU",
}

// Parse validates s. The optional "RRULE:" prefix is accepted.
func Parse(s string) (Rule, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "RRULE:")
	if s == "" || len(s) > 200 {
		return Rule{}, syntaxErr("empty or too long")
	}
	r := Rule{Interval: 1, ByMinute: 0}
	seen := map[string]bool{}
	hourSet := false
	for _, part := range strings.Split(s, ";") {
		key, val, ok := strings.Cut(part, "=")
		key = strings.ToUpper(strings.TrimSpace(key))
		val = strings.ToUpper(strings.TrimSpace(val))
		if !ok || key == "" || val == "" {
			return Rule{}, syntaxErr("malformed part %q", part)
		}
		if seen[key] {
			return Rule{}, syntaxErr("duplicate %s", key)
		}
		seen[key] = true
		switch key {
		case "FREQ":
			switch Freq(val) {
			case Daily, Weekly, Monthly:
				r.Freq = Freq(val)
			default:
				return Rule{}, &Error{Code: CodeUnsupported, Detail: "FREQ must be DAILY, WEEKLY or MONTHLY"}
			}
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 365 {
				return Rule{}, syntaxErr("INTERVAL must be 1..365")
			}
			r.Interval = n
		case "BYDAY":
			for _, d := range strings.Split(val, ",") {
				wd, ok := weekdays[d]
				if !ok {
					return Rule{}, syntaxErr("BYDAY value %q (use MO..SU, no ordinals)", d)
				}
				if !slices.Contains(r.ByDay, wd) {
					r.ByDay = append(r.ByDay, wd)
				}
			}
		case "BYMONTHDAY":
			days, err := ints(val, 1, 28, 28)
			if err != nil {
				return Rule{}, syntaxErr("BYMONTHDAY must be 1..28: %v", err)
			}
			r.ByMonthDay = days
		case "BYHOUR":
			hours, err := ints(val, 0, 23, 6)
			if err != nil {
				return Rule{}, syntaxErr("BYHOUR must be up to 6 values 0..23: %v", err)
			}
			r.ByHour, hourSet = hours, true
		case "BYMINUTE":
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 || n > 59 {
				return Rule{}, syntaxErr("BYMINUTE must be one value 0..59")
			}
			r.ByMinute = n
		default:
			return Rule{}, &Error{Code: CodeUnsupported, Detail: key + " is not supported"}
		}
	}
	if r.Freq == "" {
		return Rule{}, syntaxErr("FREQ is required")
	}
	if len(r.ByDay) > 0 && r.Freq != Weekly {
		return Rule{}, &Error{Code: CodeUnsupported, Detail: "BYDAY is only supported with FREQ=WEEKLY"}
	}
	if len(r.ByMonthDay) > 0 && r.Freq != Monthly {
		return Rule{}, &Error{Code: CodeUnsupported, Detail: "BYMONTHDAY is only supported with FREQ=MONTHLY"}
	}
	if !hourSet {
		r.ByHour = []int{9}
	}
	slices.SortFunc(r.ByDay, func(a, b time.Weekday) int { return int(mondayIndex(a)) - int(mondayIndex(b)) })
	return r, nil
}

func ints(val string, lo, hi, maxN int) ([]int, error) {
	var out []int
	for _, p := range strings.Split(val, ",") {
		n, err := strconv.Atoi(p)
		if err != nil || n < lo || n > hi {
			return nil, fmt.Errorf("value %q", p)
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	if len(out) > maxN {
		return nil, errors.New("too many values")
	}
	slices.Sort(out)
	return out, nil
}

// String returns the canonical form (stable for storage and comparison).
func (r Rule) String() string {
	parts := []string{"FREQ=" + string(r.Freq)}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		codes := make([]string, len(r.ByDay))
		for i, d := range r.ByDay {
			codes[i] = weekdayCodes[d]
		}
		parts = append(parts, "BYDAY="+strings.Join(codes, ","))
	}
	if len(r.ByMonthDay) > 0 {
		parts = append(parts, "BYMONTHDAY="+join(r.ByMonthDay))
	}
	parts = append(parts, "BYHOUR="+join(r.ByHour), "BYMINUTE="+strconv.Itoa(r.ByMinute))
	return strings.Join(parts, ";")
}

func join(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ",")
}

// Date is a calendar date without a time zone.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the date part of t in t's location.
func DateOf(t time.Time) Date { y, m, d := t.Date(); return Date{y, m, d} }

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, err
	}
	return DateOf(t), nil
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day) }

// Before reports whether d is earlier than o.
func (d Date) Before(o Date) bool { return d.compare(o) < 0 }

func (d Date) compare(o Date) int {
	switch {
	case d.Year != o.Year:
		return d.Year - o.Year
	case d.Month != o.Month:
		return int(d.Month) - int(o.Month)
	default:
		return d.Day - o.Day
	}
}

func (d Date) utc() time.Time { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC) }

func (d Date) addDays(n int) Date { return DateOf(d.utc().AddDate(0, 0, n)) }

// Series is a rule anchored at a start date (inclusive) in a time zone, optionally ending
// on an end date (inclusive).
type Series struct {
	Rule     Rule
	Location *time.Location
	StartsOn Date
	EndsOn   *Date
}

// maxPeriods bounds the periods walked per call, counted from the first relevant one.
const maxPeriods = 40_000

// Between returns the occurrences t with from <= t < to, in order, at most limit of them.
func (s Series) Between(from, to time.Time, limit int) []time.Time {
	var out []time.Time
	if limit <= 0 || !from.Before(to) {
		return out
	}
	var last time.Time
	first := s.firstPeriod(from)
	for period := first; period < first+maxPeriods; period++ {
		days, past := s.periodDays(period)
		if past {
			return out
		}
		for _, d := range days {
			if d.Before(s.StartsOn) {
				continue
			}
			if s.EndsOn != nil && s.EndsOn.Before(d) {
				return out
			}
			for _, h := range s.Rule.ByHour {
				t := wallClock(d, h, s.Rule.ByMinute, s.Location)
				if !t.Before(to) {
					return out
				}
				if !t.Before(from) && !t.Equal(last) {
					last = t
					out = append(out, t)
					if len(out) == limit {
						return out
					}
				}
			}
		}
	}
	return out
}

// wallClock is the instant of local time h:m on d in loc. A time inside a spring-forward
// gap is read with the offset in force just before the transition, which moves it forward
// by the gap whatever the zone (time.Date alone moves it backwards west of UTC).
func wallClock(d Date, h, m int, loc *time.Location) time.Time {
	t := time.Date(d.Year, d.Month, d.Day, h, m, 0, 0, loc)
	if t.Hour() == h && t.Minute() == m {
		return t
	}
	_, before := t.Add(-12 * time.Hour).Zone()
	return time.Date(d.Year, d.Month, d.Day, h, m, 0, 0, time.UTC).Add(-time.Duration(before) * time.Second).In(loc)
}

// firstPeriod skips the periods that end before from (minus one for safety), so a series
// that started long ago costs the same as a new one.
func (s Series) firstPeriod(from time.Time) int {
	target := DateOf(from.In(s.Location))
	if !s.StartsOn.Before(target) {
		return 0
	}
	var elapsed int
	switch s.Rule.Freq {
	case Daily:
		elapsed = int(target.utc().Sub(s.StartsOn.utc()).Hours() / 24)
	case Weekly:
		elapsed = int(target.utc().Sub(s.StartsOn.utc()).Hours()/24) / 7
	case Monthly:
		elapsed = (target.Year-s.StartsOn.Year)*12 + int(target.Month) - int(s.StartsOn.Month)
	}
	return max(0, elapsed/s.Rule.Interval-1)
}

// periodDays returns the candidate dates of period n in order; past reports that the
// period lies entirely after EndsOn (so iteration can stop).
func (s Series) periodDays(n int) ([]Date, bool) {
	step := n * s.Rule.Interval
	var days []Date
	switch s.Rule.Freq {
	case Daily:
		days = []Date{s.StartsOn.addDays(step)}
	case Weekly:
		// Weeks start on Monday; period 0 is the week containing the start date.
		monday := s.StartsOn.addDays(-int(mondayIndex(s.StartsOn.utc().Weekday())))
		week := monday.addDays(7 * step)
		byDay := s.Rule.ByDay
		if len(byDay) == 0 {
			byDay = []time.Weekday{s.StartsOn.utc().Weekday()}
		}
		for _, wd := range byDay {
			days = append(days, week.addDays(int(mondayIndex(wd))))
		}
	case Monthly:
		first := time.Date(s.StartsOn.Year, s.StartsOn.Month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, step, 0)
		byMonthDay := s.Rule.ByMonthDay
		if len(byMonthDay) == 0 {
			byMonthDay = []int{min(s.StartsOn.Day, 28)}
		}
		for _, md := range byMonthDay {
			days = append(days, DateOf(first.AddDate(0, 0, md-1)))
		}
	}
	past := s.EndsOn != nil && len(days) > 0 && s.EndsOn.Before(days[0])
	return days, past
}

// mondayIndex maps Monday..Sunday to 0..6.
func mondayIndex(d time.Weekday) time.Weekday { return (d + 6) % 7 }

// MinGap is a lower bound on the time between two consecutive occurrences, ignoring DST
// shifts: the smallest spacing of BYHOUR within a day, of BYDAY within a week, or of
// BYMONTHDAY within a month (months taken as 28 days), and the interval itself.
func (r Rule) MinGap() time.Duration {
	day := 24 * time.Hour
	gap := time.Duration(r.Interval) * day
	switch r.Freq {
	case Weekly:
		gap = time.Duration(r.Interval) * 7 * day
		if n := len(r.ByDay); n > 1 {
			idx := make([]int, n)
			for i, d := range r.ByDay {
				idx[i] = int(mondayIndex(d))
			}
			gap = min(gap, time.Duration(minSpacing(idx, 7))*day)
		}
	case Monthly:
		gap = time.Duration(r.Interval) * 28 * day
		if len(r.ByMonthDay) > 1 {
			gap = min(gap, time.Duration(minSpacing(r.ByMonthDay, 28))*day)
		}
	}
	if len(r.ByHour) > 1 {
		gap = min(gap, time.Duration(minSpacing(r.ByHour, 24))*time.Hour)
	}
	return gap
}

// minSpacing is the smallest distance between sorted values on a cycle of length period.
func minSpacing(sorted []int, period int) int {
	best := period
	for i := range sorted {
		next := sorted[(i+1)%len(sorted)]
		if i == len(sorted)-1 {
			next += period
		}
		best = min(best, next-sorted[i])
	}
	return best
}
