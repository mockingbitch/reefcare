//go:build integration

package dosing_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

// The test clock starts at 2026-10-29 03:00 UTC = Thursday 10:00 in Ho Chi Minh City.

func setup(t *testing.T) (*apptest.Env, *apptest.Browser, string, string) {
	t.Helper()
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	tank := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "sps_dominant", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	product := b.Expect(b.Do("POST", "/dosing-products", map[string]any{
		"name": "All-For-Reef", "brand": "Tropic Marin", "form": "liquid", "default_unit": "ml",
	}), http.StatusCreated, "").Body["id"].(string)
	return e, b, tank, product
}

func plan(b *apptest.Browser, tank, product string, body map[string]any) apptest.Response {
	base := map[string]any{
		"product_id": product, "amount": "5", "amount_unit": "ml", "rrule": "FREQ=DAILY;BYHOUR=8,20",
		"timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-01",
	}
	for k, v := range body {
		base[k] = v
	}
	return b.Do("POST", "/tanks/"+tank+"/dosing-plans", base)
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

func TestProducts(t *testing.T) {
	e, b, tank, product := setup(t)
	list := items(b.Expect(b.Do("GET", "/dosing-products", nil), http.StatusOK, ""))
	if len(list) != 1 || list[0]["curated"] != false || list[0]["name"] != "All-For-Reef" {
		t.Fatalf("products: %v", list)
	}
	r := b.Expect(b.Do("POST", "/dosing-products", map[string]any{"name": " ", "form": "gel", "default_unit": "cups"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["name"] != "length" || c["form"] != "unsupported" {
		t.Fatalf("%v", c)
	}
	r = b.Expect(b.Do("POST", "/dosing-products", map[string]any{"name": "Kalk", "form": "powder", "default_unit": "drops"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["default_unit"] != "incompatible_unit" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Private products are invisible and unusable for others.
	other, _ := e.SignUp("minh@example.com", "Minh")
	if n := len(items(other.Do("GET", "/dosing-products", nil))); n != 0 {
		t.Fatalf("another user sees %d private products", n)
	}
	otherTank := other.Expect(other.Do("POST", "/tanks", map[string]any{"name": "B", "tank_type": "custom", "display_volume_l": "10", "system_volume_l": "10"}), http.StatusCreated, "").Body["id"].(string)
	r = other.Expect(plan(other, otherTank, product, nil), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["product_id"] != "product_not_found" {
		t.Fatalf("%v", r.FieldCodes())
	}
	_ = tank
}

func TestPlanMaterializesAndMissed(t *testing.T) {
	e, b, tank, product := setup(t)
	r := b.Expect(plan(b, tank, product, nil), http.StatusCreated, "")
	if r.Body["product"].(map[string]any)["name"] != "All-For-Reef" || r.Body["amount"] != "5" || r.Body["delivery_method"] != "manual" {
		t.Fatalf("plan: %s", r.Raw)
	}
	if next := r.Body["next_occurrences"].([]any); next[0] != "2026-10-29T13:00:00Z" {
		t.Fatalf("next: %v", next) // 20:00 +07 today; 08:00 has passed
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled'`); n != 14 {
		t.Fatalf("7 days × 2 = 14 occurrences, got %d", n)
	}
	b.Expect(plan(b, tank, product, map[string]any{"amount_unit": "g"}), http.StatusUnprocessableEntity, "validation_failed")
	b.Expect(plan(b, tank, product, map[string]any{"amount": "1.23456"}), http.StatusUnprocessableEntity, "validation_failed")

	// Grace for twice a day is 6 h: at 19:01 UTC the 13:00 UTC dose is missed, never completed.
	e.Clock.Advance(16*time.Hour + time.Minute)
	e.RunJobs()
	missed := items(b.Expect(b.Do("GET", "/tanks/"+tank+"/dosing-history?status=missed", nil), http.StatusOK, ""))
	if len(missed) != 1 || missed[0]["scheduled_for"] != "2026-10-29T13:00:00Z" || missed[0]["missed_at"] == nil || missed[0]["actual_amount"] != nil {
		t.Fatalf("missed: %v", missed)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'dosing.missed'`); n != 1 {
		t.Fatalf("timeline dosing.missed = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status IN ('completed','partial')`); n != 0 {
		t.Fatal("nothing may be completed automatically")
	}

	// A missed dose can still be resolved; completed defaults to the planned amount.
	id := missed[0]["id"].(string)
	// A late dose needs an explicit time: was it given on time and not logged, or now?
	r = b.Expect(b.Do("POST", "/dosing-events/"+id+"/resolve", map[string]any{"status": "completed"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["executed_at"] != "required_for_late_dose" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", "/dosing-events/"+id+"/resolve", map[string]any{"status": "completed", "executed_at": "2026-10-29T15:00:00Z"}), http.StatusOK, "")
	if r.Body["status"] != "completed" || r.Body["actual_amount"] != "5" || r.Body["actual_source"] != "planned_default" || r.Body["missed_at"] == nil {
		t.Fatalf("resolved missed: %s", r.Raw)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'dosing.completed' AND summary->>'previous_status' = 'missed'`); n != 1 {
		t.Fatal("resolving a missed dose is recorded as a correction on the timeline")
	}
	// The 15-minute job keeps marking later ones; the horizon moves forward hourly.
	e.Clock.Advance(3 * 24 * time.Hour)
	b.Expect(b.Login("lan@example.com", apptest.Password), http.StatusOK, "")
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'missed'`); n != 6 {
		t.Fatalf("missed after 3 more days = %d, want 6", n)
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE scheduled_for > $1`, e.Clock.Now().Add(6*24*time.Hour)); n == 0 {
		t.Fatal("materialization must keep the 7-day horizon")
	}
}

func TestResolveRules(t *testing.T) {
	_, b, tank, product := setup(t)
	b.Expect(plan(b, tank, product, nil), http.StatusCreated, "")
	evs := items(b.Do("GET", "/tanks/"+tank+"/dosing-history?status=scheduled&limit=200", nil))
	id := evs[len(evs)-1]["id"].(string)
	path := "/dosing-events/" + id + "/resolve"

	for body, field := range map[string]map[string]any{
		"actual_amount": {"status": "partial"},
		"reason":        {"status": "skipped"},
		"status":        {"status": "done"},
	} {
		r := b.Expect(b.Do("POST", path, field), http.StatusUnprocessableEntity, "validation_failed")
		if r.FieldCodes()[body] == "" {
			t.Errorf("want field error on %s: %s", body, r.Raw)
		}
	}
	r := b.Expect(b.Do("POST", path, map[string]any{"status": "partial", "actual_amount": "5"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["actual_amount"] != "not_less_than_planned" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", path, map[string]any{"status": "completed", "actual_amount": "4"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["status"] != "use_partial" {
		t.Fatalf("less than planned is partial: %v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", path, map[string]any{"status": "completed", "actual_amount": "50"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["actual_amount"] != "confirm_over_planned" {
		t.Fatalf("10× planned needs confirmation: %v", r.FieldCodes())
	}
	b.Expect(b.Do("POST", path, map[string]any{"status": "completed", "executed_at": "2026-10-20T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	other := evs[len(evs)-2]["id"].(string)
	r = b.Expect(b.Do("POST", "/dosing-events/"+other+"/resolve", map[string]any{"status": "completed", "actual_amount": "6", "confirm_over_planned": true}), http.StatusOK, "")
	if r.Body["deviation"] != "over" || r.Body["actual_source"] != "entered" {
		t.Fatalf("over-dose: %s", r.Raw)
	}
	b.Expect(b.Do("POST", path, map[string]any{"status": "failed", "reason": "Pump tubing clogged"}), http.StatusOK, "")
	b.Expect(b.Do("POST", path, map[string]any{"status": "failed", "reason": "Pump tubing clogged"}), http.StatusOK, "") // idempotent
	b.Expect(b.Do("POST", path, map[string]any{"status": "completed"}), http.StatusConflict, "already_resolved")
	b.Expect(b.Do("GET", "/tanks/"+tank+"/dosing-history?status=late", nil), http.StatusBadRequest, "invalid_query")
}

func TestPlanLifecycle(t *testing.T) {
	e, b, tank, product := setup(t)
	p := b.Expect(plan(b, tank, product, nil), http.StatusCreated, "")
	id := p.Body["id"].(string)

	r := b.Expect(b.Do("PATCH", "/dosing-plans/"+id, map[string]any{"version": 1, "amount": "6.5"}), http.StatusOK, "")
	if r.Body["amount"] != "6.5" || r.Body["version"] != float64(2) {
		t.Fatalf("%s", r.Raw)
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled' AND planned_amount = 6.5`); n < 13 {
		t.Fatalf("upcoming occurrences must follow the edit, got %d", n)
	}
	b.Expect(b.Do("PATCH", "/dosing-plans/"+id, map[string]any{"version": 1, "amount": "7"}), http.StatusConflict, "version_conflict")

	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/pause", nil), http.StatusOK, "")
	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/pause", nil), http.StatusOK, "") // idempotent
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled'`); n != 0 {
		t.Fatalf("paused plan keeps %d upcoming doses", n)
	}
	r = b.Expect(b.Do("POST", "/dosing-plans/"+id+"/resume", nil), http.StatusOK, "")
	if r.Body["status"] != "active" || count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled'`) == 0 {
		t.Fatal("resume must materialize again")
	}
	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/complete", nil), http.StatusOK, "")
	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/complete", nil), http.StatusOK, "") // repeating is a no-op
	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/resume", nil), http.StatusConflict, "plan_completed")
	b.Expect(b.Do("PATCH", "/dosing-plans/"+id, map[string]any{"version": 5, "amount": "1"}), http.StatusConflict, "plan_completed")
}

func TestAdHocIdempotency(t *testing.T) {
	e, b, tank, product := setup(t)
	path := "/tanks/" + tank + "/dosing-events"
	body := map[string]any{"product_id": product, "amount": "2.5", "amount_unit": "ml", "executed_at": "2026-10-29T01:00:00Z"}
	first := b.Expect(b.Do("POST", path, body, "Idempotency-Key", "dose-retry-0001"), http.StatusCreated, "")
	again := b.Expect(b.Do("POST", path, body, "Idempotency-Key", "dose-retry-0001"), http.StatusCreated, "")
	if first.Body["id"] != again.Body["id"] || again.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("retry must replay the first dose: %v vs %v", first.Body["id"], again.Body["id"])
	}
	body["amount"] = "3"
	b.Expect(b.Do("POST", path, body, "Idempotency-Key", "dose-retry-0001"), http.StatusUnprocessableEntity, "idempotency_key_reused")
	b.Expect(b.Do("POST", path, body), http.StatusCreated, "")
	b.Expect(b.Do("POST", path, body), http.StatusCreated, "")
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE plan_id IS NULL`); n != 3 {
		t.Fatalf("ad-hoc doses = %d, want 3", n)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'dosing.completed'`); n != 3 {
		t.Fatalf("timeline = %d", n)
	}
	body["executed_at"] = "2027-01-01T00:00:00Z"
	b.Expect(b.Do("POST", path, body), http.StatusUnprocessableEntity, "validation_failed")
	b.Expect(b.Do("POST", path, body, "Idempotency-Key", "short"), http.StatusUnprocessableEntity, "validation_failed")
}

func TestAuthorizationArchiveErasure(t *testing.T) {
	e, owner, tank, product := setup(t)
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	p := owner.Expect(plan(owner, tank, product, nil), http.StatusCreated, "").Body["id"].(string)
	ev := items(owner.Do("GET", "/tanks/"+tank+"/dosing-history", nil))[0]["id"].(string)

	viewer.Expect(viewer.Do("GET", "/tanks/"+tank+"/dosing-plans", nil), http.StatusOK, "")
	viewer.Expect(viewer.Do("GET", "/tanks/"+tank+"/dosing-history", nil), http.StatusOK, "")
	viewer.Expect(viewer.Do("POST", "/dosing-plans/"+p+"/pause", nil), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("POST", "/dosing-events/"+ev+"/resolve", map[string]any{"status": "completed"}), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("POST", "/tanks/"+tank+"/dosing-events", map[string]any{"product_id": product, "amount": "1", "amount_unit": "ml"}), http.StatusForbidden, "forbidden")

	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/tanks/" + tank + "/dosing-plans", nil},
		{"GET", "/tanks/" + tank + "/dosing-history", nil},
		{"POST", "/tanks/" + tank + "/dosing-events", map[string]any{"product_id": product, "amount": "1", "amount_unit": "ml"}},
		{"PATCH", "/dosing-plans/" + p, map[string]any{"version": 1, "amount": "1"}},
		{"POST", "/dosing-plans/" + p + "/complete", nil},
		{"POST", "/dosing-events/" + ev + "/resolve", map[string]any{"status": "completed"}},
		{"POST", "/dosing-events/not-a-uuid/resolve", map[string]any{"status": "completed"}},
	} {
		stranger.Expect(stranger.Do(c.method, c.path, c.body), http.StatusNotFound, "not_found")
	}

	// Archive: plans complete, scheduled doses are resolved as skipped, missed stay missed.
	e.Clock.Advance(16*time.Hour + time.Minute) // 19:01 UTC: the 13:00 UTC dose is missed
	e.RunJobs()
	e.Clock.Advance(7 * time.Hour) // 02:01 UTC: the 01:00 UTC dose is late but inside its grace
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled'`); n != 0 {
		t.Fatalf("%d scheduled doses on an archived tank", n)
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'skipped' AND reason_code = 'tank_archived' AND reason IS NULL`); n == 0 {
		t.Fatal("archive skips carry the system reason code, not free text")
	}
	if count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'missed'`) == 0 || count(t, e, `SELECT count(*) FROM dosing_plans WHERE status = 'completed'`) != 1 {
		t.Fatal("missed doses stay missed and the plan completes")
	}

	// Erasure removes dosing data and the owner's private products.
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"dosing_events", "dosing_plans", "dosing_products", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

// Pump plans: ReefCare can't know whether the pump delivered, so late occurrences become
// "unconfirmed", never "missed", and don't flood the timeline.
func TestPumpPlansAreUnconfirmed(t *testing.T) {
	e, b, tank, product := setup(t)
	b.Expect(plan(b, tank, product, map[string]any{"delivery_method": "dosing_pump", "rrule": "FREQ=DAILY;BYHOUR=0,4,8,12,16,20"}), http.StatusCreated, "")
	e.Clock.Advance(24 * time.Hour)
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'unconfirmed'`); n < 5 {
		t.Fatalf("unconfirmed = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'missed'`) + count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'dosing.missed'`); n != 0 {
		t.Fatalf("pump doses must not be missed: %d", n)
	}
	// The keeper can still record an exception.
	id := items(b.Do("GET", "/tanks/"+tank+"/dosing-history?status=unconfirmed", nil))[0]["id"].(string)
	b.Expect(b.Do("POST", "/dosing-events/"+id+"/resolve", map[string]any{"status": "failed", "reason": "Pump ran dry", "actual_amount": "1"}), http.StatusOK, "")
}

// Moving a plan's time or pausing it removes the doses still inside their grace, so two
// doses are never open for the same slot (a double-dose risk) and a paused plan doesn't
// produce "missed" doses.
func TestRetimingLeavesNoDoubleOpenDose(t *testing.T) {
	e, b, tank, product := setup(t)
	id := b.Expect(plan(b, tank, product, map[string]any{"rrule": "FREQ=DAILY;BYHOUR=11"}), http.StatusCreated, "").Body["id"].(string) // 11:00 +07 = 04:00 UTC
	e.Clock.Advance(2 * time.Hour)                                                                                                      // 05:00 UTC: today's dose is 1 h late, inside its 12 h grace
	b.Expect(b.Do("PATCH", "/dosing-plans/"+id, map[string]any{"version": 1, "rrule": "FREQ=DAILY;BYHOUR=18"}), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status = 'scheduled' AND scheduled_for < '2026-10-30T00:00:00Z'`); n != 1 {
		t.Fatalf("today must have exactly one open dose after retiming, got %d", n)
	}
	b.Expect(b.Do("POST", "/dosing-plans/"+id+"/pause", nil), http.StatusOK, "")
	e.Clock.Advance(24 * time.Hour)
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM dosing_events WHERE status IN ('missed', 'scheduled')`); n != 0 {
		t.Fatalf("a paused plan left %d open or missed doses", n)
	}
}

func TestConcurrentKeyedAdHoc(t *testing.T) {
	e, b, tank, product := setup(t)
	body := map[string]any{"product_id": product, "amount": "2", "amount_unit": "ml", "executed_at": "2026-10-29T01:00:00Z"}
	ids := make(chan any, 5)
	for range 5 {
		go func() {
			ids <- b.Do("POST", "/tanks/"+tank+"/dosing-events", body, "Idempotency-Key", "same-key-123").Body["id"]
		}()
	}
	first := <-ids
	for range 4 {
		if id := <-ids; id != first {
			t.Fatalf("concurrent retries returned different doses: %v vs %v", id, first)
		}
	}
	if n := count(t, e, `SELECT count(*) FROM dosing_events`); n != 1 {
		t.Fatalf("doses = %d", n)
	}
}
