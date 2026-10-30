package scheduling

import (
	"testing"
	"time"

	"reefcare/internal/platform/apperr"
)

func ptr(s string) *string { return &s }

func TestPreviewSchedule(t *testing.T) {
	// Thursday 2026-10-29 10:00 in Ho Chi Minh City (UTC+7).
	now := time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)

	p, err := PreviewSchedule(PreviewInput{
		RRule: "FREQ=WEEKLY;BYDAY=SU;BYHOUR=9", Timezone: "Asia/Ho_Chi_Minh", StartsOn: "2026-10-01",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.RRule != "FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0" {
		t.Errorf("canonical rrule = %q", p.RRule)
	}
	want := []string{"2026-11-01T02:00:00Z", "2026-11-08T02:00:00Z", "2026-11-15T02:00:00Z"}
	if len(p.NextOccurrences) != len(want) {
		t.Fatalf("occurrences = %v", p.NextOccurrences)
	}
	for i, w := range want {
		if got := p.NextOccurrences[i].UTC().Format(time.RFC3339); got != w {
			t.Errorf("occurrence %d = %s, want %s", i, got, w)
		}
	}
}

func TestPreviewRespectsStartAndEnd(t *testing.T) {
	now := time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)

	p, err := PreviewSchedule(PreviewInput{
		RRule: "FREQ=DAILY;BYHOUR=8", Timezone: "UTC", StartsOn: "2026-12-30", EndsOn: ptr("2026-12-31"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.NextOccurrences) != 2 || p.NextOccurrences[0].Format("2006-01-02") != "2026-12-30" {
		t.Fatalf("want the two days of the window, got %v", p.NextOccurrences)
	}

	ended, err := PreviewSchedule(PreviewInput{
		RRule: "FREQ=DAILY", Timezone: "UTC", StartsOn: "2026-01-01", EndsOn: ptr("2026-01-31"),
	}, now)
	if err != nil || len(ended.NextOccurrences) != 0 {
		t.Fatalf("a schedule that already ended has no upcoming dates: %v %v", ended.NextOccurrences, err)
	}
}

func TestPreviewValidation(t *testing.T) {
	now := time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		in    PreviewInput
		field string
		code  string
	}{
		{"bad rrule", PreviewInput{RRule: "nonsense", Timezone: "UTC", StartsOn: "2026-10-01"}, "rrule", "invalid_rrule"},
		{"unsupported part", PreviewInput{RRule: "FREQ=YEARLY", Timezone: "UTC", StartsOn: "2026-10-01"}, "rrule", ""},
		{"bad timezone", PreviewInput{RRule: "FREQ=DAILY", Timezone: "Mars/Olympus", StartsOn: "2026-10-01"}, "timezone", "invalid_timezone"},
		{"local timezone", PreviewInput{RRule: "FREQ=DAILY", Timezone: "Local", StartsOn: "2026-10-01"}, "timezone", "invalid_timezone"},
		{"bad start", PreviewInput{RRule: "FREQ=DAILY", Timezone: "UTC", StartsOn: "01/10/2026"}, "starts_on", "invalid_date"},
		{"start too far", PreviewInput{RRule: "FREQ=DAILY", Timezone: "UTC", StartsOn: "2040-01-01"}, "starts_on", "out_of_range"},
		{"end before start", PreviewInput{RRule: "FREQ=DAILY", Timezone: "UTC", StartsOn: "2026-10-10", EndsOn: ptr("2026-10-01")}, "ends_on", "before_start"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PreviewSchedule(tt.in, now)
			e, ok := apperr.As(err)
			if !ok || e.Kind != apperr.KindValidation {
				t.Fatalf("want validation error, got %v", err)
			}
			for _, f := range e.Fields {
				if f.Field == tt.field && (tt.code == "" || f.Code == tt.code) {
					return
				}
			}
			t.Fatalf("want %s/%s in %+v", tt.field, tt.code, e.Fields)
		})
	}
}
