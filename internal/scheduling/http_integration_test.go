//go:build integration

package scheduling_test

import (
	"net/http"
	"net/url"
	"testing"

	"reefcare/internal/apptest"
)

func previewPath(q url.Values) string { return "/recurrence/preview?" + q.Encode() }

// The preview must match what saving the same schedule produces.
func TestPreviewMatchesSavedSchedule(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")

	q := url.Values{
		"rrule": {"FREQ=WEEKLY;BYDAY=SU;BYHOUR=9"}, "timezone": {"Asia/Ho_Chi_Minh"}, "starts_on": {"2026-10-01"},
	}
	preview := b.Expect(b.Do("GET", previewPath(q), nil), http.StatusOK, "")

	tank := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	saved := b.Expect(b.Do("POST", "/tanks/"+tank+"/water-change-schedules", map[string]any{
		"rrule": q.Get("rrule"), "timezone": q.Get("timezone"), "starts_on": q.Get("starts_on"), "planned_volume_l": "10",
	}), http.StatusCreated, "")

	if preview.Body["rrule"] != saved.Body["rrule"] {
		t.Errorf("canonical rrule: preview %v, saved %v", preview.Body["rrule"], saved.Body["rrule"])
	}
	pn, _ := preview.Body["next_occurrences"].([]any)
	sn, _ := saved.Body["next_occurrences"].([]any)
	if len(pn) != 3 || len(pn) != len(sn) {
		t.Fatalf("occurrences: preview %v, saved %v", pn, sn)
	}
	for i := range pn {
		if pn[i] != sn[i] {
			t.Errorf("occurrence %d: preview %v, saved %v", i, pn[i], sn[i])
		}
	}
}

func TestPreviewRequiresSignInAndValidates(t *testing.T) {
	e := apptest.New(t)
	anon := e.Browser()
	q := url.Values{"rrule": {"FREQ=DAILY"}, "timezone": {"UTC"}, "starts_on": {"2026-10-01"}}
	anon.Expect(anon.Do("GET", previewPath(q), nil), http.StatusUnauthorized, "unauthenticated")

	b, _ := e.SignUp("lan@example.com", "Lan")
	bad := url.Values{"rrule": {"FREQ=HOURLY"}, "timezone": {"Mars/Olympus"}, "starts_on": {"2026-10-10"}, "ends_on": {"2026-10-01"}}
	r := b.Expect(b.Do("GET", previewPath(bad), nil), http.StatusUnprocessableEntity, "validation_failed")
	codes := r.FieldCodes()
	if codes["timezone"] != "invalid_timezone" || codes["ends_on"] != "before_start" || codes["rrule"] == "" {
		t.Fatalf("field codes: %v", codes)
	}
}
