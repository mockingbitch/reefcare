//go:build integration

package equipment_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

// The test clock starts at 2026-10-29 03:00 UTC = 10:00 in Ho Chi Minh City and
// 2026-10-28 20:00 in Los Angeles.

func setup(t *testing.T) (*apptest.Env, *apptest.Browser, string) {
	t.Helper()
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	tank := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "sps_dominant", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	return e, b, tank
}

func addDevice(b *apptest.Browser, tank string, body map[string]any) apptest.Response {
	base := map[string]any{"category": "return_pump", "name": "Return pump", "manufacturer": "Red Sea", "model": "ReefRun 5500"}
	for k, v := range body {
		base[k] = v
	}
	return b.Do("POST", "/tanks/"+tank+"/equipment", base)
}

func addPlan(b *apptest.Browser, tank string, body map[string]any) apptest.Response {
	base := map[string]any{"title": "Clean skimmer cup", "interval_days": 7, "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29"}
	for k, v := range body {
		base[k] = v
	}
	return b.Do("POST", "/tanks/"+tank+"/maintenance-plans", base)
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

func TestEquipmentLifecycle(t *testing.T) {
	e, b, tank := setup(t)

	// Two devices of the same model are allowed.
	pump := b.Expect(addDevice(b, tank, map[string]any{"purchased_on": "2026-01-10", "warranty_until": "2028-01-10"}), http.StatusCreated, "").Body
	b.Expect(addDevice(b, tank, map[string]any{"name": "Spare pump"}), http.StatusCreated, "")
	id := pump["id"].(string)
	if pump["purchased_on"] != "2026-01-10" || pump["configuration"] != nil || pump["removed_at"] != nil {
		t.Fatalf("created: %v", pump)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'equipment.installed'`); n != 2 {
		t.Fatalf("installed events = %d", n)
	}

	// Validation.
	r := b.Expect(addDevice(b, tank, map[string]any{"category": "toaster", "name": " "}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["category"] != "unsupported" || c["name"] != "length" {
		t.Fatalf("%v", c)
	}
	r = b.Expect(addDevice(b, tank, map[string]any{"purchased_on": "2026-05-01", "warranty_until": "2026-01-01"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["warranty_until"] != "before_purchase" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(addDevice(b, tank, map[string]any{"installed_at": "2027-01-01T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["installed_at"] != "in_future" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Patch with optimistic locking.
	upd := b.Expect(b.Do("PATCH", "/equipment/"+id, map[string]any{"version": 1, "name": "Main return", "serial_number": "RR-123"}), http.StatusOK, "").Body
	if upd["name"] != "Main return" || upd["serial_number"] != "RR-123" || upd["version"] != float64(2) {
		t.Fatalf("patched: %v", upd)
	}
	b.Expect(b.Do("PATCH", "/equipment/"+id, map[string]any{"version": 1, "name": "Stale"}), http.StatusConflict, "version_conflict")
	b.Expect(b.Do("PATCH", "/equipment/"+id, map[string]any{"name": "No version"}), http.StatusUnprocessableEntity, "validation_failed")

	// Configuration versions are immutable; the device shows the latest.
	path := "/equipment/" + id + "/configurations"
	b.Expect(b.Do("POST", path, map[string]any{"settings": map[string]any{"speed_pct": 70, "feed_mode": true}}), http.StatusCreated, "")
	e.Clock.Advance(time.Hour)
	c2 := b.Expect(b.Do("POST", path, map[string]any{"settings": map[string]any{"speed_pct": 60}, "note": "Less flow"}), http.StatusCreated, "").Body
	if c2["version_no"] != float64(2) {
		t.Fatalf("config: %v", c2)
	}
	versions := items(b.Expect(b.Do("GET", path, nil), http.StatusOK, ""))
	if len(versions) != 2 || versions[0]["version_no"] != float64(2) || versions[1]["settings"].(map[string]any)["speed_pct"] != float64(70) {
		t.Fatalf("versions: %v", versions)
	}
	got := b.Expect(b.Do("GET", "/equipment/"+id, nil), http.StatusOK, "").Body
	if cfg, _ := got["configuration"].(map[string]any); cfg == nil || cfg["version_no"] != float64(2) {
		t.Fatalf("current configuration: %v", got["configuration"])
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'equipment.configuration_changed'`); n != 2 {
		t.Fatalf("configuration events = %d", n)
	}
	for _, bad := range []any{[]int{1}, map[string]any{}, map[string]any{"nested": map[string]any{"a": 1}}, "x"} {
		b.Expect(b.Do("POST", path, map[string]any{"settings": bad}), http.StatusUnprocessableEntity, "validation_failed")
	}

	// Removal keeps the device in history, stops its plans and is idempotent.
	plan := b.Expect(addPlan(b, tank, map[string]any{"equipment_id": id, "title": "Clean pump impeller", "interval_days": 90}), http.StatusCreated, "").Body
	if plan["equipment_name"] != "Main return" {
		t.Fatalf("plan: %v", plan)
	}
	rm := b.Expect(b.Do("POST", "/equipment/"+id+"/remove", map[string]any{"reason": "Replaced"}), http.StatusOK, "").Body
	if rm["removed_at"] == nil || rm["removal_reason"] != "Replaced" {
		t.Fatalf("removed: %v", rm)
	}
	b.Expect(b.Do("POST", "/equipment/"+id+"/remove", nil), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'equipment.removed'`); n != 1 {
		t.Fatalf("removed events = %d", n)
	}
	b.Expect(b.Do("PATCH", "/equipment/"+id, map[string]any{"version": 3, "name": "x"}), http.StatusConflict, "equipment_removed")
	b.Expect(b.Do("POST", path, map[string]any{"settings": map[string]any{"a": 1}}), http.StatusConflict, "equipment_removed")
	r = b.Expect(addPlan(b, tank, map[string]any{"equipment_id": id}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["equipment_id"] != "equipment_removed" {
		t.Fatalf("%v", r.FieldCodes())
	}
	plans := items(b.Expect(b.Do("GET", "/tanks/"+tank+"/maintenance-plans", nil), http.StatusOK, ""))
	if len(plans) != 1 || plans[0]["active"] != false || plans[0]["overdue"] != false {
		t.Fatalf("plans after removal: %v", plans)
	}

	if n := len(items(b.Do("GET", "/tanks/"+tank+"/equipment", nil))); n != 1 {
		t.Fatalf("installed devices = %d", n)
	}
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/equipment?include=removed", nil))); n != 2 {
		t.Fatalf("all devices = %d", n)
	}
}

func TestMaintenanceDue(t *testing.T) {
	e, b, tank := setup(t)

	// Due state is derived in the plan's zone: on 2026-10-29 03:00 UTC it is still the 28th in LA.
	due := b.Expect(addPlan(b, tank, nil), http.StatusCreated, "").Body
	late := b.Expect(addPlan(b, tank, map[string]any{"title": "Replace carbon", "starts_on": "2026-10-28", "interval_days": 30}), http.StatusCreated, "").Body
	la := b.Expect(addPlan(b, tank, map[string]any{"title": "Check heater", "starts_on": "2026-10-28", "timezone": "America/Los_Angeles"}), http.StatusCreated, "").Body
	if due["due_today"] != true || due["overdue"] != false || due["next_due_on"] != "2026-10-29" {
		t.Fatalf("due: %v", due)
	}
	if late["overdue"] != true {
		t.Fatalf("late: %v", late)
	}
	if la["due_today"] != true || la["overdue"] != false {
		t.Fatalf("LA: %v", la)
	}
	plans := items(b.Do("GET", "/tanks/"+tank+"/maintenance-plans", nil))
	if len(plans) != 3 || plans[0]["title"] != "Replace carbon" {
		t.Fatalf("soonest due first: %v", plans)
	}

	// Performing moves the due date; recording an older performance late never moves it back.
	id := due["id"].(string)
	ev := b.Expect(b.Do("POST", "/maintenance-plans/"+id+"/events", map[string]any{"notes": "Cup was full"}), http.StatusCreated, "").Body
	if ev["plan_id"] != id || ev["title"] != "Clean skimmer cup" {
		t.Fatalf("event: %v", ev)
	}
	b.Expect(b.Do("POST", "/maintenance-plans/"+id+"/events", map[string]any{"performed_at": "2026-10-01T02:00:00Z"}), http.StatusCreated, "")
	b.Expect(b.Do("POST", "/maintenance-plans/"+id+"/events", map[string]any{"performed_at": "2027-01-01T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	var p map[string]any
	for _, it := range items(b.Do("GET", "/tanks/"+tank+"/maintenance-plans", nil)) {
		if it["id"] == id {
			p = it
		}
	}
	if p["next_due_on"] != "2026-11-05" || p["due_today"] != false || p["last_performed_at"] != "2026-10-29T03:00:00Z" {
		t.Fatalf("after perform: %v", p)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'maintenance.performed'`); n != 2 {
		t.Fatalf("performed events = %d", n)
	}

	// A week later it is due again, then overdue; it is never completed automatically.
	e.Clock.Advance(8 * 24 * time.Hour)
	b.Login("lan@example.com", apptest.Password)
	for _, it := range items(b.Do("GET", "/tanks/"+tank+"/maintenance-plans", nil)) {
		if it["id"] == id && it["overdue"] != true {
			t.Fatalf("overdue: %v", it)
		}
	}

	// Editing the interval recomputes the due date from the last performance.
	upd := b.Expect(b.Do("PATCH", "/maintenance-plans/"+id, map[string]any{"version": p["version"], "interval_days": 14}), http.StatusOK, "").Body
	if upd["next_due_on"] != "2026-11-12" || upd["overdue"] != false {
		t.Fatalf("patched: %v", upd)
	}
	b.Expect(b.Do("PATCH", "/maintenance-plans/"+id, map[string]any{"version": p["version"], "title": "Stale"}), http.StatusConflict, "version_conflict")
	r := b.Expect(b.Do("PATCH", "/maintenance-plans/"+id, map[string]any{"version": upd["version"], "interval_days": 0, "timezone": "Mars/Base"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["interval_days"] != "out_of_range" || c["timezone"] != "invalid_timezone" {
		t.Fatalf("%v", c)
	}
	off := b.Expect(b.Do("PATCH", "/maintenance-plans/"+id, map[string]any{"version": upd["version"], "active": false}), http.StatusOK, "").Body
	if off["active"] != false || off["due_today"] != false || off["overdue"] != false {
		t.Fatalf("inactive: %v", off)
	}

	// Unplanned maintenance, filtered history and pagination.
	dev := b.Expect(addDevice(b, tank, nil), http.StatusCreated, "").Body["id"].(string)
	b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "Descaled pump", "equipment_id": dev}), http.StatusCreated, "")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": " "}), http.StatusUnprocessableEntity, "validation_failed")
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/maintenance-events?equipment_id="+dev, nil))); n != 1 {
		t.Fatalf("by equipment = %d", n)
	}
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/maintenance-events?plan_id="+id, nil))); n != 2 {
		t.Fatalf("by plan = %d", n)
	}
	page := b.Expect(b.Do("GET", "/tanks/"+tank+"/maintenance-events?limit=2", nil), http.StatusOK, "")
	first := items(page)
	cursor, _ := page.Body["next_cursor"].(string)
	if len(first) != 2 || first[0]["title"] != "Descaled pump" || cursor == "" {
		t.Fatalf("page 1: %v", page.Body)
	}
	rest := b.Expect(b.Do("GET", "/tanks/"+tank+"/maintenance-events?limit=2&cursor="+cursor, nil), http.StatusOK, "")
	if len(items(rest)) != 1 || rest.Body["next_cursor"] != nil {
		t.Fatalf("page 2: %v", rest.Body)
	}
	if items(rest)[0]["performed_at"] != "2026-10-01T02:00:00Z" {
		t.Fatalf("oldest last: %v", items(rest))
	}
}

func TestPlanEquipmentMustBeOnTheTank(t *testing.T) {
	e, b, tank := setup(t)
	other, _ := e.SignUp("minh@example.com", "Minh")
	otherTank := other.Expect(other.Do("POST", "/tanks", map[string]any{"name": "B", "tank_type": "custom", "display_volume_l": "10", "system_volume_l": "10"}), http.StatusCreated, "").Body["id"].(string)
	foreign := other.Expect(addDevice(other, otherTank, nil), http.StatusCreated, "").Body["id"].(string)

	r := b.Expect(addPlan(b, tank, map[string]any{"equipment_id": foreign}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["equipment_id"] != "equipment_not_found" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x", "equipment_id": foreign}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["equipment_id"] != "equipment_not_found" {
		t.Fatalf("%v", r.FieldCodes())
	}
}

func TestAuthorizationArchiveErasure(t *testing.T) {
	e, owner, tank := setup(t)
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	dev := owner.Expect(addDevice(owner, tank, nil), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/equipment/"+dev+"/configurations", map[string]any{"settings": map[string]any{"speed_pct": 50}}), http.StatusCreated, "")
	plan := owner.Expect(addPlan(owner, tank, map[string]any{"equipment_id": dev}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/maintenance-plans/"+plan+"/events", nil), http.StatusCreated, "")

	for _, p := range []string{"/tanks/" + tank + "/equipment", "/equipment/" + dev, "/equipment/" + dev + "/configurations", "/tanks/" + tank + "/maintenance-plans", "/tanks/" + tank + "/maintenance-events"} {
		viewer.Expect(viewer.Do("GET", p, nil), http.StatusOK, "")
	}
	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", "/tanks/" + tank + "/equipment", map[string]any{"category": "heater", "name": "H"}},
		{"PATCH", "/equipment/" + dev, map[string]any{"version": 1, "name": "x"}},
		{"POST", "/equipment/" + dev + "/remove", nil},
		{"POST", "/equipment/" + dev + "/configurations", map[string]any{"settings": map[string]any{"a": 1}}},
		{"POST", "/tanks/" + tank + "/maintenance-plans", map[string]any{"title": "x", "interval_days": 1, "timezone": "UTC", "starts_on": "2026-10-29"}},
		{"PATCH", "/maintenance-plans/" + plan, map[string]any{"version": 1, "title": "x"}},
		{"POST", "/maintenance-plans/" + plan + "/events", nil},
		{"POST", "/tanks/" + tank + "/maintenance-events", map[string]any{"title": "x"}},
	}
	for _, c := range writes {
		viewer.Expect(viewer.Do(c.method, c.path, c.body), http.StatusForbidden, "forbidden")
		stranger.Expect(stranger.Do(c.method, c.path, c.body), http.StatusNotFound, "not_found")
	}
	for _, p := range []string{"/tanks/" + tank + "/equipment", "/equipment/" + dev, "/equipment/" + dev + "/configurations", "/tanks/" + tank + "/maintenance-plans", "/tanks/" + tank + "/maintenance-events", "/equipment/not-a-uuid"} {
		stranger.Expect(stranger.Do("GET", p, nil), http.StatusNotFound, "not_found")
	}
	stranger.Expect(stranger.Do("PATCH", "/maintenance-plans/not-a-uuid", map[string]any{"version": 1}), http.StatusNotFound, "not_found")

	// Archive: plans stop being due and writes are refused; history stays readable.
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM maintenance_plans WHERE active`); n != 0 {
		t.Fatalf("%d active plans on an archived tank", n)
	}
	owner.Expect(addDevice(owner, tank, nil), http.StatusConflict, "tank_archived")
	owner.Expect(owner.Do("POST", "/maintenance-plans/"+plan+"/events", nil), http.StatusConflict, "tank_archived")
	owner.Expect(owner.Do("GET", "/tanks/"+tank+"/maintenance-events", nil), http.StatusOK, "")

	// Erasure removes every equipment and maintenance row.
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"maintenance_events", "maintenance_plans", "equipment_configurations", "tank_equipment", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

// Regression tests for the 2.6 review.
func TestReviewRules(t *testing.T) {
	e, b, tank := setup(t)
	dev := b.Expect(addDevice(b, tank, map[string]any{"installed_at": "2026-10-01T00:00:00Z"}), http.StatusCreated, "").Body["id"].(string)

	// installed_at is fixed at creation.
	b.Expect(b.Do("PATCH", "/equipment/"+dev, map[string]any{"version": 1, "installed_at": "2026-09-01T00:00:00Z"}), http.StatusBadRequest, "")

	// Configuration versions follow time, so the latest is always current; settings keep numbers exactly.
	path := "/equipment/" + dev + "/configurations"
	cfg := b.Expect(b.Do("POST", path, map[string]any{"settings": json.RawMessage(`{"big": 12345678901234567890, "dec": 1.10}`), "effective_at": "2026-10-20T00:00:00Z"}), http.StatusCreated, "")
	if !strings.Contains(cfg.Raw, `12345678901234567890`) || !strings.Contains(cfg.Raw, `1.10`) {
		t.Fatalf("numbers changed: %s", cfg.Raw)
	}
	r := b.Expect(b.Do("POST", path, map[string]any{"settings": map[string]any{"a": 1}, "effective_at": "2026-10-10T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["effective_at"] != "before_current" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", path, map[string]any{"settings": map[string]any{"a": 1}, "effective_at": "2026-09-10T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes()["effective_at"]; c != "before_installed" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Idempotent perform: a retry returns the first event and changes nothing.
	plan := b.Expect(addPlan(b, tank, map[string]any{"equipment_id": dev}), http.StatusCreated, "").Body["id"].(string)
	body := map[string]any{"notes": "done"}
	first := b.Expect(b.Do("POST", "/maintenance-plans/"+plan+"/events", body, "Idempotency-Key", "perform-0001"), http.StatusCreated, "")
	again := b.Expect(b.Do("POST", "/maintenance-plans/"+plan+"/events", body, "Idempotency-Key", "perform-0001"), http.StatusCreated, "")
	if first.Body["id"] != again.Body["id"] || again.Header.Get("Idempotent-Replayed") != "true" || first.Header.Get("Idempotent-Replayed") != "" {
		t.Fatalf("replay: %v / %v", first.Body, again.Body)
	}
	if n := count(t, e, `SELECT count(*) FROM maintenance_events`) + count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'maintenance.performed'`); n != 2 {
		t.Fatalf("a retry was recorded twice (%d rows)", n)
	}
	b.Expect(b.Do("POST", "/maintenance-plans/"+plan+"/events", map[string]any{"notes": "other"}, "Idempotency-Key", "perform-0001"), http.StatusUnprocessableEntity, "idempotency_key_reused")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x"}, "Idempotency-Key", "perform-0001"), http.StatusUnprocessableEntity, "idempotency_key_reused")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x"}, "Idempotency-Key", "short"), http.StatusUnprocessableEntity, "validation_failed")
	u1 := b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "Water top-off"}, "Idempotency-Key", "unplanned-01"), http.StatusCreated, "")
	u2 := b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "Water top-off"}, "Idempotency-Key", "unplanned-01"), http.StatusCreated, "")
	if u1.Body["id"] != u2.Body["id"] {
		t.Fatal("unplanned retry duplicated")
	}

	// Unplanned work on a device stays within its lifetime.
	r = b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x", "equipment_id": dev, "performed_at": "2026-09-01T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["performed_at"] != "before_installed" {
		t.Fatalf("%v", r.FieldCodes())
	}
	b.Expect(b.Do("POST", "/equipment/"+dev+"/remove", map[string]any{"removed_at": "2026-10-25T00:00:00Z"}), http.StatusOK, "")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x", "equipment_id": dev, "performed_at": "2026-10-24T00:00:00Z"}), http.StatusCreated, "")
	r = b.Expect(b.Do("POST", "/tanks/"+tank+"/maintenance-events", map[string]any{"title": "x", "equipment_id": dev}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["performed_at"] != "after_removed" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// A plan on a removed device can't be turned back on, but can be moved to another device.
	var version any
	for _, p := range items(b.Do("GET", "/tanks/"+tank+"/maintenance-plans", nil)) {
		if p["id"] == plan {
			version = p["version"]
		}
	}
	r = b.Expect(b.Do("PATCH", "/maintenance-plans/"+plan, map[string]any{"version": version, "active": true}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["equipment_id"] != "equipment_removed" {
		t.Fatalf("%v", r.FieldCodes())
	}
	other := b.Expect(addDevice(b, tank, nil), http.StatusCreated, "").Body["id"].(string)
	moved := b.Expect(b.Do("PATCH", "/maintenance-plans/"+plan, map[string]any{"version": version, "active": true, "equipment_id": other}), http.StatusOK, "").Body
	if moved["active"] != true || moved["equipment_id"] != other {
		t.Fatalf("moved: %v", moved)
	}

	// A foreign plan filter just finds nothing.
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	st := stranger.Expect(stranger.Do("POST", "/tanks", map[string]any{"name": "E", "tank_type": "custom", "display_volume_l": "10", "system_volume_l": "10"}), http.StatusCreated, "").Body["id"].(string)
	if n := len(items(stranger.Expect(stranger.Do("GET", "/tanks/"+st+"/maintenance-events?plan_id="+plan, nil), http.StatusOK, ""))); n != 0 {
		t.Fatalf("foreign plan filter leaked %d events", n)
	}
}
