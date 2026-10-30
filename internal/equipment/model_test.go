package equipment

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/recurrence"
)

func date(s string) recurrence.Date {
	d, err := recurrence.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestNextDue(t *testing.T) {
	at := func(s string) *time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err)
		}
		return &v
	}
	for _, c := range []struct {
		name     string
		last     *time.Time
		starts   string
		interval int32
		tz       string
		want     string
	}{
		{"never performed", nil, "2026-11-01", 7, "UTC", "2026-11-01"},
		{"local date of performance", at("2026-10-28T20:00:00Z"), "2026-10-01", 7, "Asia/Ho_Chi_Minh", "2026-11-05"}, // 03:00 on the 29th locally
		{"same instant west of UTC", at("2026-10-28T20:00:00Z"), "2026-10-01", 7, "America/Los_Angeles", "2026-11-04"},
		{"across a DST change", at("2026-10-31T12:00:00Z"), "2026-10-01", 7, "America/New_York", "2026-11-07"},
		{"across month and year", at("2026-12-28T00:00:00Z"), "2026-01-01", 30, "UTC", "2027-01-27"},
		{"never before the start date", at("2026-10-01T00:00:00Z"), "2026-12-01", 7, "UTC", "2026-12-01"},
		{"unknown zone falls back to UTC", at("2026-10-28T20:00:00Z"), "2026-10-01", 1, "Nowhere/City", "2026-10-29"},
	} {
		if got := nextDue(c.last, date(c.starts), c.interval, c.tz); got != date(c.want) {
			t.Errorf("%s: got %v, want %s", c.name, got, c.want)
		}
	}
}

func TestDecorate(t *testing.T) {
	now := time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC) // 10:00 in HCMC, 20:00 on the 28th in LA
	for _, c := range []struct {
		due, tz        string
		active         bool
		overdue, today bool
	}{
		{"2026-10-29", "Asia/Ho_Chi_Minh", true, false, true},
		{"2026-10-28", "Asia/Ho_Chi_Minh", true, true, false},
		{"2026-10-28", "America/Los_Angeles", true, false, true},
		{"2026-10-30", "Asia/Ho_Chi_Minh", true, false, false},
		{"2026-10-01", "Asia/Ho_Chi_Minh", false, false, false},
	} {
		p := decorate(Plan{NextDueOn: date(c.due), Timezone: c.tz, Active: c.active}, now)
		if p.Overdue != c.overdue || p.DueToday != c.today {
			t.Errorf("%+v: overdue=%v today=%v", c, p.Overdue, p.DueToday)
		}
	}
}

func TestSettings(t *testing.T) {
	many := map[string]int{}
	for i := range 51 {
		many[strings.Repeat("k", i+1)] = i
	}
	tooMany, _ := json.Marshal(many)
	for _, c := range []struct {
		raw   string
		field string
		code  string
	}{
		{`{"speed_pct": 70, "feed": true, "mode": "pulse"}`, "", ""},
		{`[]`, "settings", "must_be_object"},
		{`null`, "settings", "must_be_object"},
		{`"x"`, "settings", "must_be_object"},
		{`{}`, "settings", "keys_count"},
		{string(tooMany), "settings", "keys_count"},
		{`{"a": {"b": 1}}`, "settings.a", "invalid_value"},
		{`{"a": [1]}`, "settings.a", "invalid_value"},
		{`{"a": null}`, "settings.a", "invalid_value"},
		{`{"a": "` + strings.Repeat("x", 201) + `"}`, "settings.a", "invalid_value"},
		{`{"a": "line\nbreak"}`, "settings.a", "invalid_value"},
		{`{"` + strings.Repeat("k", 51) + `": 1}`, "settings." + strings.Repeat("k", 51), "invalid_key"},
	} {
		v := &apperr.Validator{}
		out := checker{v}.settings(json.RawMessage(c.raw))
		codes := map[string]string{}
		if ae, ok := apperr.As(v.Err()); ok {
			for _, f := range ae.Fields {
				codes[f.Field] = f.Code
			}
		}
		if c.code == "" {
			if len(codes) != 0 || out == nil {
				t.Errorf("%s: unexpected %v", c.raw, codes)
			}
			continue
		}
		if codes[c.field] != c.code {
			t.Errorf("%.40s: got %v, want %s=%s", c.raw, codes, c.field, c.code)
		}
	}
}
