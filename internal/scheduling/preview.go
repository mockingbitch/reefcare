// Package scheduling lets clients preview a recurrence rule before saving a schedule,
// using the same rules the server applies (docs/open-items.md W6: show the next dates
// before saving). It stores nothing and touches no tank data.
package scheduling

import (
	"errors"
	"strings"
	"time"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/recurrence"
)

// PreviewCount is how many upcoming occurrences a preview returns.
const PreviewCount = 3

var earliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// PreviewInput is a schedule's timing as a client would submit it.
type PreviewInput struct {
	RRule    string
	Timezone string
	StartsOn string
	EndsOn   *string
}

// Preview is the canonical rule and its next occurrences from now.
type Preview struct {
	RRule           string
	NextOccurrences []time.Time
}

// PreviewSchedule validates in with the same field codes as schedule creation
// (`invalid_rrule`, `unsupported_rrule_part`, `invalid_timezone`, `invalid_date`,
// `out_of_range`, `before_start`) and returns up to PreviewCount occurrences at or after now.
func PreviewSchedule(in PreviewInput, now time.Time) (Preview, error) {
	v := apperr.NewValidator()

	rule, err := recurrence.Parse(strings.TrimSpace(in.RRule))
	if err != nil {
		code := recurrence.CodeSyntax
		var e *recurrence.Error
		if errors.As(err, &e) {
			code = e.Code
		}
		v.Add("rrule", code, nil)
	}

	var loc *time.Location
	if tz := in.Timezone; tz == "" || tz == "Local" || len(tz) > 64 {
		v.Add("timezone", "invalid_timezone", nil)
	} else if loc, err = time.LoadLocation(tz); err != nil {
		v.Add("timezone", "invalid_timezone", nil)
	}

	starts, startsOK := date(v, "starts_on", in.StartsOn, now)
	var ends *recurrence.Date
	if in.EndsOn != nil {
		if d, ok := date(v, "ends_on", *in.EndsOn, now); ok {
			ends = &d
			if startsOK && d.Before(starts) {
				v.Add("ends_on", "before_start", nil)
			}
		}
	}
	if err := v.Err(); err != nil {
		return Preview{}, err
	}

	series := recurrence.Series{Rule: rule, Location: loc, StartsOn: starts, EndsOn: ends}
	return Preview{
		RRule: rule.String(),
		// Same horizon the schedule services use for next_occurrences.
		NextOccurrences: series.Between(now, now.Add(400*24*time.Hour), PreviewCount),
	}, nil
}

// date parses YYYY-MM-DD between 2000-01-01 and ten years after now.
func date(v *apperr.Validator, field, s string, now time.Time) (recurrence.Date, bool) {
	d, err := recurrence.ParseDate(strings.TrimSpace(s))
	if err != nil {
		v.Add(field, "invalid_date", map[string]string{"format": "YYYY-MM-DD"})
		return d, false
	}
	if t := time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC); t.Before(earliest) || t.After(now.AddDate(10, 0, 0)) {
		v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01"})
		return d, false
	}
	return d, true
}
