//go:build integration

package waterchanges_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

// The test clock starts at 2026-10-29 03:00 UTC = Thursday 10:00 in Ho Chi Minh City.
const weeklySunday = "FREQ=WEEKLY;BYDAY=SU;BYHOUR=9;BYMINUTE=0"

func setup(t *testing.T) (*apptest.Env, *apptest.Browser, string) {
	t.Helper()
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	tank := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	return e, b, tank
}

func schedule(t *testing.T, b *apptest.Browser, tank string, body map[string]any) apptest.Response {
	t.Helper()
	base := map[string]any{"rrule": weeklySunday, "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-01", "planned_volume_l": "10"}
	for k, v := range body {
		base[k] = v
	}
	return b.Do("POST", "/tanks/"+tank+"/water-change-schedules", base)
}

func items(r apptest.Response) []map[string]any {
	raw, _ := r.Body["items"].([]any)
	out := make([]map[string]any, len(raw))
	for i, it := range raw {
		out[i] = it.(map[string]any)
	}
	return out
}

func count(t *testing.T, e *apptest.Env, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestScheduleMaterializesUpcomingOccurrences(t *testing.T) {
	e, b, tank := setup(t)
	r := b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	if r.Body["rrule"] != weeklySunday || r.Body["status"] != "active" {
		t.Fatalf("schedule: %s", r.Raw)
	}
	next := r.Body["next_occurrences"].([]any)
	if len(next) != 3 || next[0] != "2026-11-01T02:00:00Z" || next[1] != "2026-11-08T02:00:00Z" {
		t.Fatalf("next occurrences (09:00 +07): %v", next)
	}
	est := r.Body["estimate"].(map[string]any)
	if est["fraction_of_system"] != "0.139" || est["system_volume_l"] != "72" || est["basis"] != "planned" ||
		len(est["assumptions"].([]any)) != 3 || len(est["flags"].([]any)) != 0 {
		t.Fatalf("estimate: %v", est)
	}

	// Sundays within 35 days, none in the past even though starts_on is.
	list := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes", nil), http.StatusOK, "")
	got := items(list)
	if len(got) != 5 {
		t.Fatalf("occurrences = %d: %s", len(got), list.Raw)
	}
	if got[len(got)-1]["scheduled_for"] != "2026-11-01T02:00:00Z" || got[0]["scheduled_for"] != "2026-11-29T02:00:00Z" {
		t.Fatalf("order/newest first: %v … %v", got[0]["scheduled_for"], got[len(got)-1]["scheduled_for"])
	}
	for _, it := range got {
		if it["status"] != "scheduled" || it["overdue"] != false || it["planned_volume_l"] != "10" {
			t.Fatalf("occurrence: %v", it)
		}
	}

	// The hourly job keeps the horizon ahead; re-running never duplicates.
	e.Clock.Advance(14 * 24 * time.Hour)
	e.RunJobs()
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM water_change_events WHERE scheduled_for < $1`, e.Clock.Now().Add(35*24*time.Hour)); n != 7 {
		t.Fatalf("after 14 days the horizon must hold 7 Sundays, got %d", n)
	}
	if n := count(t, e, `SELECT count(*) - count(DISTINCT scheduled_for) FROM water_change_events`); n != 0 {
		t.Fatal("duplicate occurrences")
	}
}

func TestScheduleValidation(t *testing.T) {
	_, b, tank := setup(t)
	r := b.Expect(schedule(t, b, tank, map[string]any{
		"rrule": "FREQ=HOURLY", "timezone": "Mars/Base", "starts_on": "2026-13-01", "planned_volume_l": "0",
	}), http.StatusUnprocessableEntity, "validation_failed")
	want := map[string]string{"rrule": "unsupported_rrule_part", "timezone": "invalid_timezone", "starts_on": "invalid_date", "planned_volume_l": "must_be_positive"}
	for f, c := range want {
		if r.FieldCodes()[f] != c {
			t.Errorf("%s: got %q want %q (%v)", f, r.FieldCodes()[f], c, r.FieldCodes())
		}
	}
	r = b.Expect(schedule(t, b, tank, map[string]any{"rrule": "FREQ=WEEKLY;BYDAY=1MO", "ends_on": "2026-09-01"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["rrule"] != "invalid_rrule" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(schedule(t, b, tank, map[string]any{"ends_on": "2026-09-01"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["ends_on"] != "before_start" {
		t.Fatalf("%v", r.FieldCodes())
	}
	// Canonical storage.
	r = b.Expect(schedule(t, b, tank, map[string]any{"rrule": "freq=weekly;byday=su"}), http.StatusCreated, "")
	if r.Body["rrule"] != weeklySunday {
		t.Fatalf("canonical rrule: %v", r.Body["rrule"])
	}
}

// US-WC-2 AC1: an overdue change stays overdue until the keeper acts; nothing completes
// it automatically.
func TestOverdueUntilResolved(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	e.Clock.Advance(4 * 24 * time.Hour) // Nov 2 03:00 UTC: Sunday Nov 1 09:00 +07 is > 24 h ago
	overdue := items(b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?status=overdue", nil), http.StatusOK, ""))
	if len(overdue) != 1 || overdue[0]["overdue"] != true || overdue[0]["scheduled_for"] != "2026-11-01T02:00:00Z" {
		t.Fatalf("overdue: %v", overdue)
	}
	upcoming := items(b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?status=scheduled", nil), http.StatusOK, ""))
	for _, u := range upcoming {
		if u["overdue"] != false {
			t.Fatal("status=scheduled lists upcoming only")
		}
	}

	e.Clock.Advance(20 * 24 * time.Hour) // Nov 22 03:00 UTC: Nov 1, 8 and 15 are overdue; Nov 22 is not yet
	e.RunJobs()
	b.Expect(b.Login("lan@example.com", apptest.Password), http.StatusOK, "") // 14-day idle timeout passed
	still := items(b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?status=overdue", nil), http.StatusOK, ""))
	if len(still) != 3 || count(t, e, `SELECT count(*) FROM water_change_events WHERE status <> 'scheduled'`) != 0 {
		t.Fatalf("missed changes must stay overdue and unresolved: %d overdue", len(still))
	}
	b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?status=late", nil), http.StatusBadRequest, "invalid_query")
}

func TestCompleteAndSkip(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	e.Clock.Advance(4 * 24 * time.Hour)
	list := items(b.Do("GET", "/tanks/"+tank+"/water-changes", nil))
	first := list[len(list)-1]["id"].(string) // Nov 1, overdue
	second := list[len(list)-2]["id"].(string)

	r := b.Expect(b.Do("POST", "/water-changes/"+first+"/complete", map[string]any{"actual_volume_l": "12", "completed_at": "2026-11-02T01:00:00Z", "notes": "Late, but done"}), http.StatusOK, "")
	if r.Body["status"] != "completed" || r.Body["overdue"] != false || r.Body["actual_volume_l"] != "12" ||
		r.Body["estimate"].(map[string]any)["fraction_of_system"] != "0.167" || r.Body["resolved_at"] != "2026-11-02T03:00:00Z" {
		t.Fatalf("completed: %s", r.Raw)
	}
	// Idempotent repeat; a different completion or a skip afterwards conflicts.
	b.Expect(b.Do("POST", "/water-changes/"+first+"/complete", map[string]any{"actual_volume_l": "12", "completed_at": "2026-11-02T01:00:00Z", "notes": "Late, but done"}), http.StatusOK, "")
	b.Expect(b.Do("POST", "/water-changes/"+first+"/complete", map[string]any{"actual_volume_l": "15"}), http.StatusConflict, "already_resolved")
	b.Expect(b.Do("POST", "/water-changes/"+first+"/skip", map[string]any{"reason": "x"}), http.StatusConflict, "already_resolved")

	r = b.Expect(b.Do("POST", "/water-changes/"+second+"/complete", map[string]any{"actual_volume_l": "-1", "completed_at": "2027-01-01T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["actual_volume_l"] != "must_be_positive" || c["completed_at"] != "in_future" {
		t.Fatalf("%v", c)
	}
	b.Expect(b.Do("POST", "/water-changes/"+second+"/skip", map[string]any{"reason": "  "}), http.StatusUnprocessableEntity, "validation_failed")
	r = b.Expect(b.Do("POST", "/water-changes/"+second+"/skip", map[string]any{"reason": "Traveling"}), http.StatusOK, "")
	if r.Body["status"] != "skipped" || r.Body["skip_reason"] != "Traveling" {
		t.Fatalf("skipped: %s", r.Raw)
	}
	b.Expect(b.Do("POST", "/water-changes/"+second+"/skip", map[string]any{"reason": "Traveling"}), http.StatusOK, "")

	// US-WC-2 AC2: the timeline shows both.
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE tank_id = $1 AND category = 'water_change'`, tank); n != 2 {
		t.Fatalf("timeline water_change events = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'water_change.completed' AND occurred_at = '2026-11-02T01:00:00Z'`); n != 1 {
		t.Fatal("the completed event happens at completed_at")
	}
}

func TestEditPauseEnd(t *testing.T) {
	e, b, tank := setup(t)
	s := b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	id := s.Body["id"].(string)
	e.Clock.Advance(4 * 24 * time.Hour) // Nov 1 overdue
	list := items(b.Do("GET", "/tanks/"+tank+"/water-changes", nil))
	b.Expect(b.Do("POST", "/water-changes/"+list[len(list)-2]["id"].(string)+"/complete", map[string]any{"actual_volume_l": "10"}), http.StatusOK, "") // Nov 8 done early

	r := b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 1, "rrule": "FREQ=WEEKLY;BYDAY=SA;BYHOUR=18", "planned_volume_l": "8"}), http.StatusOK, "")
	if r.Body["version"] != float64(2) || r.Body["planned_volume_l"] != "8" {
		t.Fatalf("patched: %s", r.Raw)
	}
	rows := items(b.Do("GET", "/tanks/"+tank+"/water-changes?status=scheduled", nil))
	if len(rows) < 4 { // regression: the emptied upcoming window must be refilled at once
		t.Fatalf("after an edit the next 35 days need ~5 Saturdays, got %d", len(rows))
	}
	for _, it := range rows {
		at, _ := time.Parse(time.RFC3339, it["scheduled_for"].(string))
		if at.In(time.FixedZone("+07", 7*3600)).Weekday() != time.Saturday || it["planned_volume_l"] != "8" {
			t.Fatalf("upcoming occurrences must follow the edit: %v", it)
		}
	}
	if n := count(t, e, `SELECT count(*) FROM water_change_events WHERE status = 'scheduled' AND scheduled_for = '2026-11-01T02:00:00Z'`); n != 1 {
		t.Fatal("the overdue occurrence is history and must survive the edit")
	}
	if n := count(t, e, `SELECT count(*) FROM water_change_events WHERE status = 'completed'`); n != 1 {
		t.Fatal("resolved occurrences must survive the edit")
	}
	b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 1, "notes": "stale"}), http.StatusConflict, "version_conflict")

	// Pausing removes upcoming occurrences and the job adds none.
	b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 2, "status": "paused"}), http.StatusOK, "")
	e.Clock.Advance(24 * time.Hour)
	e.RunJobs()
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/water-changes?status=scheduled", nil))); n != 0 {
		t.Fatalf("paused schedule still has %d upcoming occurrences", n)
	}
	r = b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 3, "status": "active"}), http.StatusOK, "")
	if len(r.Body["next_occurrences"].([]any)) != 3 || len(items(b.Do("GET", "/tanks/"+tank+"/water-changes?status=scheduled", nil))) == 0 {
		t.Fatal("resuming must materialize again")
	}
	b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 4, "status": "ended"}), http.StatusOK, "")
	b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 5, "status": "active"}), http.StatusConflict, "schedule_ended")
	b.Expect(b.Do("PATCH", "/water-change-schedules/"+id, map[string]any{"version": 5, "status": "done"}), http.StatusConflict, "schedule_ended")
}

func TestRecordOneOff(t *testing.T) {
	e, b, tank := setup(t)
	path := "/tanks/" + tank + "/water-changes"
	r := b.Expect(b.Do("POST", path, map[string]any{"completed_at": "2026-10-28T10:00:00Z", "actual_volume_l": "20", "salt_mix": "Red Sea"}), http.StatusCreated, "")
	if r.Body["status"] != "completed" || r.Body["schedule_id"] != nil || r.Body["estimate"].(map[string]any)["fraction_of_system"] != "0.278" {
		t.Fatalf("recorded: %s", r.Raw)
	}
	r = b.Expect(b.Do("POST", path, map[string]any{"scheduled_for": "2026-11-05T02:00:00Z", "planned_volume_l": "15"}), http.StatusCreated, "")
	if r.Body["status"] != "scheduled" || r.Body["overdue"] != false {
		t.Fatalf("planned: %s", r.Raw)
	}
	for _, bad := range []map[string]any{
		{},
		{"completed_at": "2026-10-28T10:00:00Z", "scheduled_for": "2026-11-05T02:00:00Z", "actual_volume_l": "1"},
		{"completed_at": "2026-12-01T00:00:00Z", "actual_volume_l": "5"},
		{"completed_at": "2026-10-28T10:00:00Z"},
		{"scheduled_for": "2026-11-05T02:00:00Z"},
	} {
		b.Expect(b.Do("POST", path, bad), http.StatusUnprocessableEntity, "validation_failed")
	}
	r = b.Expect(b.Do("POST", path, map[string]any{"completed_at": "yesterday", "actual_volume_l": "5"}), http.StatusBadRequest, "malformed_json")
	if r.FieldCodes()["completed_at"] == "" {
		t.Fatalf("malformed time must name its field: %s", r.Raw)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'water_change.completed'`); n != 1 {
		t.Fatal("a recorded change is on the timeline")
	}
}

func TestAuthorization(t *testing.T) {
	e, owner, tank := setup(t)
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	sched := owner.Expect(schedule(t, owner, tank, nil), http.StatusCreated, "").Body["id"].(string)
	event := items(owner.Do("GET", "/tanks/"+tank+"/water-changes", nil))[0]["id"].(string)

	// Viewers read, never write.
	viewer.Expect(viewer.Do("GET", "/tanks/"+tank+"/water-change-schedules", nil), http.StatusOK, "")
	viewer.Expect(viewer.Do("GET", "/tanks/"+tank+"/water-changes", nil), http.StatusOK, "")
	viewer.Expect(schedule(t, viewer, tank, nil), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("POST", "/water-changes/"+event+"/complete", map[string]any{"actual_volume_l": "10"}), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("PATCH", "/water-change-schedules/"+sched, map[string]any{"version": 1, "status": "paused"}), http.StatusForbidden, "forbidden")

	// Strangers get 404 everywhere, like for unknown ids.
	missing := "0192f0c4-0000-7000-8000-000000000000"
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/tanks/" + tank + "/water-change-schedules", nil},
		{"POST", "/tanks/" + tank + "/water-change-schedules", map[string]any{"rrule": weeklySunday, "timezone": "UTC", "starts_on": "2026-11-01", "planned_volume_l": "1"}},
		{"GET", "/tanks/" + tank + "/water-changes", nil},
		{"POST", "/tanks/" + tank + "/water-changes", map[string]any{"completed_at": "2026-10-28T10:00:00Z", "actual_volume_l": "1"}},
		{"PATCH", "/water-change-schedules/" + sched, map[string]any{"version": 1, "status": "paused"}},
		{"PATCH", "/water-change-schedules/" + missing, map[string]any{"version": 1, "status": "paused"}},
		{"POST", "/water-changes/" + event + "/complete", map[string]any{"actual_volume_l": "1"}},
		{"POST", "/water-changes/" + event + "/skip", map[string]any{"reason": "x"}},
		{"POST", "/water-changes/" + missing + "/skip", map[string]any{"reason": "x"}},
		{"POST", "/water-changes/not-a-uuid/skip", map[string]any{"reason": "x"}},
	} {
		t.Run(fmt.Sprintf("%s %s", c.method, c.path), func(t *testing.T) {
			stranger.Expect(stranger.Do(c.method, c.path, c.body), http.StatusNotFound, "not_found")
		})
	}

	// Archived tanks are read-only: schedules end, upcoming occurrences go away, and changes
	// still open (here one overdue) are resolved as skipped with reason "tank_archived".
	e.Clock.Advance(4 * 24 * time.Hour)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM water_change_events WHERE status = 'scheduled'`); n != 0 {
		t.Fatalf("%d open changes left on the archived tank", n)
	}
	if n := count(t, e, `SELECT count(*) FROM water_change_events WHERE status = 'skipped' AND skip_reason = 'tank_archived'`); n != 1 {
		t.Fatalf("the overdue change must be resolved as skipped, got %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM water_change_schedules WHERE status = 'ended'`); n != 1 {
		t.Fatal("the schedule must end with the tank")
	}
	owner.Expect(schedule(t, owner, tank, nil), http.StatusConflict, "tank_archived")
	owner.Expect(owner.Do("POST", "/water-changes/"+event+"/complete", map[string]any{"actual_volume_l": "10"}), http.StatusNotFound, "not_found") // removed with the archive
	before := count(t, e, `SELECT count(*) FROM water_change_events`)
	e.Clock.Advance(30 * 24 * time.Hour)
	e.RunJobs()
	if after := count(t, e, `SELECT count(*) FROM water_change_events`); after != before {
		t.Fatalf("archived tank got new occurrences: %d → %d", before, after)
	}
	owner.Expect(owner.Login("lan@example.com", apptest.Password), http.StatusOK, "")
	owner.Expect(owner.Do("GET", "/tanks/"+tank+"/water-changes", nil), http.StatusOK, "")
}

func TestPaginationAndRange(t *testing.T) {
	_, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, map[string]any{"rrule": "FREQ=DAILY;BYHOUR=8,20"}), http.StatusCreated, "")
	seen := map[string]bool{}
	cursor := ""
	for range 20 {
		r := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?limit=25"+cursor, nil), http.StatusOK, "")
		for _, it := range items(r) {
			if seen[it["id"].(string)] {
				t.Fatal("duplicate across pages")
			}
			seen[it["id"].(string)] = true
		}
		next, _ := r.Body["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = "&cursor=" + next
	}
	if len(seen) != 70 { // twice a day for 35 days
		t.Fatalf("paginated %d occurrences, want 70", len(seen))
	}
	r := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?from=2026-11-01T00:00:00Z&to=2026-11-03T00:00:00Z", nil), http.StatusOK, "")
	if n := len(items(r)); n != 4 {
		t.Fatalf("range Nov 1–2 = %d, want 4", n)
	}
	b.Expect(b.Do("GET", "/tanks/"+tank+"/water-changes?from=tomorrow", nil), http.StatusBadRequest, "invalid_query")
}

func TestErasedWithTheAccount(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", map[string]any{"completed_at": "2026-10-28T10:00:00Z", "actual_volume_l": "20"}), http.StatusCreated, "")
	b.Expect(b.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"water_change_events", "water_change_schedules", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

// Correcting the tank volume later must not rewrite the estimate of past changes.
func TestEstimateHistoryIsStable(t *testing.T) {
	_, b, tank := setup(t)
	done := b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", map[string]any{"completed_at": "2026-10-28T10:00:00Z", "actual_volume_l": "18"}), http.StatusCreated, "")
	planned := b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", map[string]any{"scheduled_for": "2026-11-05T02:00:00Z", "planned_volume_l": "18"}), http.StatusCreated, "")
	if done.Body["estimate"].(map[string]any)["fraction_of_system"] != "0.25" {
		t.Fatalf("%s", done.Raw)
	}
	b.Expect(b.Do("PATCH", "/tanks/"+tank, map[string]any{"version": 1, "system_volume_l": "90"}), http.StatusOK, "")
	for _, it := range items(b.Do("GET", "/tanks/"+tank+"/water-changes", nil)) {
		est := it["estimate"].(map[string]any)
		switch it["id"] {
		case done.Body["id"]:
			if est["fraction_of_system"] != "0.25" || est["system_volume_l"] != "72" || est["basis"] != "actual" {
				t.Fatalf("completed history changed: %v", est)
			}
		case planned.Body["id"]:
			if est["fraction_of_system"] != "0.2" || est["system_volume_l"] != "90" || est["basis"] != "planned" {
				t.Fatalf("planned estimates follow the current volume: %v", est)
			}
		}
	}
	big := b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", map[string]any{"completed_at": "2026-10-28T11:00:00Z", "actual_volume_l": "100"}), http.StatusCreated, "")
	if f := big.Body["estimate"].(map[string]any)["flags"].([]any); len(f) != 1 || f[0] != "exceeds_system_volume" {
		t.Fatalf("flags: %v", f)
	}
}

func TestConcurrentIdenticalCompletionsAreIdempotent(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, nil), http.StatusCreated, "")
	event := items(b.Do("GET", "/tanks/"+tank+"/water-changes", nil))[0]["id"].(string)
	statuses := make(chan int, 6)
	for range 6 {
		go func() {
			statuses <- b.Do("POST", "/water-changes/"+event+"/complete", map[string]any{"actual_volume_l": "10", "completed_at": "2026-10-29T02:00:00Z"}).Status
		}()
	}
	for range 6 {
		if s := <-statuses; s != http.StatusOK {
			t.Fatalf("identical concurrent completion got %d", s)
		}
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'water_change.completed'`); n != 1 {
		t.Fatalf("timeline events = %d", n)
	}
	b.Expect(b.Do("POST", "/water-changes/"+event+"/complete", map[string]any{"actual_volume_l": "10", "completed_at": "2026-10-29T02:00:00Z", "notes": "different"}), http.StatusConflict, "already_resolved")
}

// Frequent schedules get a shorter grace: half the spacing.
func TestGraceFollowsSchedule(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(schedule(t, b, tank, map[string]any{"rrule": "FREQ=DAILY;BYHOUR=8,20"}), http.StatusCreated, "")
	first := items(b.Do("GET", "/tanks/"+tank+"/water-changes?limit=200", nil))
	last := first[len(first)-1] // 2026-10-29 20:00 +07 = 13:00 UTC
	if last["scheduled_for"] != "2026-10-29T13:00:00Z" {
		t.Fatalf("first occurrence %v", last["scheduled_for"])
	}
	e.Clock.Advance(16*time.Hour + time.Minute) // 19:01 UTC: 6 h after the first occurrence
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/water-changes?status=overdue", nil))); n != 1 {
		t.Fatalf("twice-daily change must be overdue after 6 h, got %d", n)
	}
}

func TestDateBounds(t *testing.T) {
	_, b, tank := setup(t)
	r := b.Expect(schedule(t, b, tank, map[string]any{"starts_on": "1999-12-31"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["starts_on"] != "out_of_range" {
		t.Fatalf("%v", r.FieldCodes())
	}
	for _, body := range []map[string]any{
		{"completed_at": "1999-01-01T00:00:00Z", "actual_volume_l": "5"},
		{"scheduled_for": "2030-01-01T00:00:00Z", "planned_volume_l": "5"},
	} {
		b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", body), http.StatusUnprocessableEntity, "validation_failed")
	}
}
