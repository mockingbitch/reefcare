package recurrence

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func mustRule(t *testing.T, s string) Rule {
	t.Helper()
	r, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return r
}

func TestParseCanonical(t *testing.T) {
	for in, want := range map[string]string{
		"FREQ=WEEKLY": "FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0",
		"RRULE:freq=weekly;byday=su,we,su;byhour=18":          "FREQ=WEEKLY;BYDAY=WE,SU;BYHOUR=18;BYMINUTE=0",
		"FREQ=DAILY;INTERVAL=3;BYHOUR=20,8;BYMINUTE=30":       "FREQ=DAILY;INTERVAL=3;BYHOUR=8,20;BYMINUTE=30",
		"FREQ=MONTHLY;BYMONTHDAY=15,1":                        "FREQ=MONTHLY;BYMONTHDAY=1,15;BYHOUR=9;BYMINUTE=0",
		"FREQ=WEEKLY;INTERVAL=1;BYDAY=MO;BYHOUR=9;BYMINUTE=0": "FREQ=WEEKLY;BYDAY=MO;BYHOUR=9;BYMINUTE=0",
	} {
		if got := mustRule(t, in).String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", in, got, want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"":                                CodeSyntax,
		"INTERVAL=2":                      CodeSyntax,
		"FREQ=HOURLY":                     CodeUnsupported,
		"FREQ=YEARLY":                     CodeUnsupported,
		"FREQ=WEEKLY;COUNT=3":             CodeUnsupported,
		"FREQ=WEEKLY;UNTIL=20270101":      CodeUnsupported,
		"FREQ=WEEKLY;BYDAY=1MO":           CodeSyntax,
		"FREQ=DAILY;BYDAY=MO":             CodeUnsupported,
		"FREQ=WEEKLY;BYMONTHDAY=3":        CodeUnsupported,
		"FREQ=MONTHLY;BYMONTHDAY=31":      CodeSyntax,
		"FREQ=DAILY;INTERVAL=0":           CodeSyntax,
		"FREQ=DAILY;BYHOUR=24":            CodeSyntax,
		"FREQ=DAILY;BYHOUR=1,2,3,4,5,6,7": CodeSyntax,
		"FREQ=DAILY;BYMINUTE=0,30":        CodeSyntax,
		"FREQ=DAILY;FREQ=WEEKLY":          CodeSyntax,
		"FREQ=DAILY;;":                    CodeSyntax,
	}
	for in, code := range cases {
		_, err := Parse(in)
		var e *Error
		if !errors.As(err, &e) || e.Code != code {
			t.Errorf("Parse(%q) = %v, want code %s", in, err, code)
		}
	}
}

func fmtAll(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format("2006-01-02 Mon 15:04 MST")
	}
	return out
}

func eq(t *testing.T, got []time.Time, want ...string) {
	t.Helper()
	g := fmtAll(got)
	if len(g) != len(want) {
		t.Fatalf("got %d occurrences %v, want %v", len(g), g, want)
	}
	for i := range g {
		if g[i] != want[i] {
			t.Fatalf("occurrence %d: got %s, want %s (all %v)", i, g[i], want[i], g)
		}
	}
}

func TestWeeklyDefaultsToStartWeekday(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	s := Series{Rule: mustRule(t, "FREQ=WEEKLY"), Location: hcm, StartsOn: Date{2026, 11, 1}} // a Sunday
	got := s.Between(time.Date(2026, 10, 1, 0, 0, 0, 0, hcm), time.Date(2026, 11, 30, 0, 0, 0, 0, hcm), 10)
	eq(t, got, "2026-11-01 Sun 09:00 +07", "2026-11-08 Sun 09:00 +07", "2026-11-15 Sun 09:00 +07",
		"2026-11-22 Sun 09:00 +07", "2026-11-29 Sun 09:00 +07")
}

func TestWeeklyByDayIntervalAndEnd(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	end := Date{2026, 11, 25}
	// Start on a Thursday: the Monday of that week is before the start and is skipped.
	s := Series{Rule: mustRule(t, "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,SA;BYHOUR=18"), Location: hcm, StartsOn: Date{2026, 10, 29}, EndsOn: &end}
	got := s.Between(time.Date(2026, 1, 1, 0, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), 100)
	eq(t, got, "2026-10-31 Sat 18:00 +07", "2026-11-09 Mon 18:00 +07", "2026-11-14 Sat 18:00 +07", "2026-11-23 Mon 18:00 +07")
}

func TestDailyMultipleHoursAndWindow(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	s := Series{Rule: mustRule(t, "FREQ=DAILY;BYHOUR=8,20"), Location: hcm, StartsOn: Date{2026, 1, 1}}
	// from is inclusive, to exclusive; a start far in the past is fine.
	got := s.Between(time.Date(2026, 10, 30, 8, 0, 0, 0, hcm), time.Date(2026, 10, 31, 20, 0, 0, 0, hcm), 10)
	eq(t, got, "2026-10-30 Fri 08:00 +07", "2026-10-30 Fri 20:00 +07", "2026-10-31 Sat 08:00 +07")
	if n := len(s.Between(time.Date(2026, 1, 1, 0, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), 3)); n != 3 {
		t.Fatalf("limit not honored: %d", n)
	}
}

func TestMonthly(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	s := Series{Rule: mustRule(t, "FREQ=MONTHLY;INTERVAL=2;BYMONTHDAY=1,15"), Location: hcm, StartsOn: Date{2026, 11, 10}}
	got := s.Between(time.Date(2026, 11, 1, 0, 0, 0, 0, hcm), time.Date(2027, 4, 1, 0, 0, 0, 0, hcm), 10)
	eq(t, got, "2026-11-15 Sun 09:00 +07", "2027-01-01 Fri 09:00 +07", "2027-01-15 Fri 09:00 +07", "2027-03-01 Mon 09:00 +07", "2027-03-15 Mon 09:00 +07")
	// A start on the 31st defaults to the 28th so every month has the day.
	s = Series{Rule: mustRule(t, "FREQ=MONTHLY"), Location: hcm, StartsOn: Date{2027, 1, 31}}
	got = s.Between(time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), time.Date(2027, 4, 1, 0, 0, 0, 0, hcm), 10)
	eq(t, got, "2027-02-28 Sun 09:00 +07", "2027-03-28 Sun 09:00 +07")
}

// Wall-clock times hold across DST: 09:00 stays 09:00 local, so the UTC offset changes.
func TestDSTKeepsLocalTime(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	s := Series{Rule: mustRule(t, "FREQ=WEEKLY;BYDAY=SU"), Location: berlin, StartsOn: Date{2026, 3, 22}}
	got := s.Between(time.Date(2026, 3, 1, 0, 0, 0, 0, berlin), time.Date(2026, 4, 6, 0, 0, 0, 0, berlin), 10)
	eq(t, got, "2026-03-22 Sun 09:00 CET", "2026-03-29 Sun 09:00 CEST", "2026-04-05 Sun 09:00 CEST")
	if got[1].Sub(got[0]) != 7*24*time.Hour-time.Hour {
		t.Fatalf("the DST week is an hour shorter: %v", got[1].Sub(got[0]))
	}
}

func TestDSTGapAndOverlap(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	// 02:30 does not exist on 2026-03-29 (clocks jump 02:00 → 03:00): it moves to 03:30.
	s := Series{Rule: mustRule(t, "FREQ=DAILY;BYHOUR=2;BYMINUTE=30"), Location: berlin, StartsOn: Date{2026, 3, 28}}
	got := s.Between(time.Date(2026, 3, 28, 0, 0, 0, 0, berlin), time.Date(2026, 3, 31, 0, 0, 0, 0, berlin), 10)
	eq(t, got, "2026-03-28 Sat 02:30 CET", "2026-03-29 Sun 03:30 CEST", "2026-03-30 Mon 02:30 CEST")
	// 02:30 happens twice on 2026-10-25: exactly one occurrence.
	s = Series{Rule: mustRule(t, "FREQ=DAILY;BYHOUR=2;BYMINUTE=30"), Location: berlin, StartsOn: Date{2026, 10, 25}}
	got = s.Between(time.Date(2026, 10, 25, 0, 0, 0, 0, berlin), time.Date(2026, 10, 26, 0, 0, 0, 0, berlin), 10)
	if len(got) != 1 {
		t.Fatalf("fall-back day must have one occurrence, got %v", fmtAll(got))
	}
}

func TestEmptyWindowsAndEnd(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	end := Date{2026, 11, 1}
	s := Series{Rule: mustRule(t, "FREQ=DAILY"), Location: hcm, StartsOn: Date{2026, 10, 30}, EndsOn: &end}
	if got := s.Between(time.Date(2026, 11, 2, 0, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), 10); len(got) != 0 {
		t.Fatalf("after the end: %v", fmtAll(got))
	}
	if got := s.Between(time.Date(2026, 11, 2, 0, 0, 0, 0, hcm), time.Date(2026, 11, 1, 0, 0, 0, 0, hcm), 10); len(got) != 0 {
		t.Fatal("empty window")
	}
	eq(t, s.Between(time.Date(2026, 1, 1, 0, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), 10),
		"2026-10-30 Fri 09:00 +07", "2026-10-31 Sat 09:00 +07", "2026-11-01 Sun 09:00 +07")
}

func TestMinGap(t *testing.T) {
	day := 24 * time.Hour
	for in, want := range map[string]time.Duration{
		"FREQ=WEEKLY":                      7 * day,
		"FREQ=WEEKLY;INTERVAL=2":           14 * day,
		"FREQ=WEEKLY;BYDAY=MO,TH":          3 * day,
		"FREQ=WEEKLY;BYDAY=MO,SU":          day,
		"FREQ=DAILY;INTERVAL=3":            3 * day,
		"FREQ=DAILY;BYHOUR=8,20":           12 * time.Hour,
		"FREQ=DAILY;BYHOUR=0,4,8,12,16,20": 4 * time.Hour,
		"FREQ=MONTHLY":                     28 * day,
		"FREQ=MONTHLY;BYMONTHDAY=1,15":     14 * day,
	} {
		if got := mustRule(t, in).MinGap(); got != want {
			t.Errorf("%s: MinGap = %v, want %v", in, got, want)
		}
	}
}

// West of UTC a gap time must also move forward, and two hours collapsing into the same
// instant yield one occurrence.
func TestDSTGapWestOfUTC(t *testing.T) {
	for _, zone := range []string{"America/New_York", "America/Sao_Paulo", "Europe/Berlin"} {
		loc := mustLoc(t, zone)
		// Find the spring-forward day of 2027 in this zone (if any).
		var gapDay *Date
		for d := time.Date(2027, 1, 1, 12, 0, 0, 0, loc); d.Year() == 2027; d = d.AddDate(0, 0, 1) {
			_, a := d.Add(-24 * time.Hour).Zone()
			_, b := d.Zone()
			if b > a {
				gd := DateOf(d)
				gapDay = &gd
				break
			}
		}
		if gapDay == nil {
			continue // São Paulo has no DST since 2019
		}
		for h := 0; h < 24; h++ {
			t0 := time.Date(gapDay.Year, gapDay.Month, gapDay.Day, h, 30, 0, 0, loc)
			if t0.Hour() == h {
				continue
			}
			s := Series{Rule: mustRule(t, fmt.Sprintf("FREQ=DAILY;BYHOUR=%d,%d;BYMINUTE=30", h-1, h)), Location: loc, StartsOn: *gapDay}
			got := s.Between(time.Date(gapDay.Year, gapDay.Month, gapDay.Day, 0, 0, 0, 0, loc), time.Date(gapDay.Year, gapDay.Month, gapDay.Day, 23, 0, 0, 0, loc), 10)
			if len(got) != 2 || !got[1].After(got[0]) || got[1].Hour() != h+1 {
				t.Errorf("%s: gap hour %d must move forward to %d:30, got %v", zone, h, h+1, fmtAll(got))
			}
		}
	}
}

func TestOldStartIsCheap(t *testing.T) {
	hcm := mustLoc(t, "Asia/Ho_Chi_Minh")
	for _, rule := range []string{"FREQ=DAILY", "FREQ=WEEKLY;INTERVAL=3;BYDAY=TU", "FREQ=MONTHLY;INTERVAL=5;BYMONTHDAY=3"} {
		s := Series{Rule: mustRule(t, rule), Location: hcm, StartsOn: Date{1900, 1, 1}}
		got := s.Between(time.Date(2026, 1, 1, 0, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm), 400)
		if len(got) == 0 {
			t.Errorf("%s from 1900: no occurrences in 2026", rule)
			continue
		}
		// Same phase as walking from the start: compare with a start one period later.
		ref := Series{Rule: s.Rule, Location: hcm, StartsOn: Date{1900, 1, 1}}
		if g := ref.Between(got[0].Add(-time.Second), got[0].Add(time.Second), 1); len(g) != 1 {
			t.Errorf("%s: first 2026 occurrence %v inconsistent", rule, got[0])
		}
	}
	// Phase check: every 3 weeks from Tue 2026-01-06 hits 2026-01-27, 2026-02-17…
	s := Series{Rule: mustRule(t, "FREQ=WEEKLY;INTERVAL=3;BYDAY=TU"), Location: hcm, StartsOn: Date{2026, 1, 6}}
	eq(t, s.Between(time.Date(2026, 2, 1, 0, 0, 0, 0, hcm), time.Date(2026, 3, 15, 0, 0, 0, 0, hcm), 10),
		"2026-02-17 Tue 09:00 +07", "2026-03-10 Tue 09:00 +07")
}
