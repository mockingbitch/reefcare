//go:build integration

package livestock_test

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

// The test clock starts at 2026-10-29 03:00 UTC.

func setup(t *testing.T) (*apptest.Env, *apptest.Browser, string) {
	t.Helper()
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	tank := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	return e, b, tank
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

func addFish(b *apptest.Browser, tank string, body map[string]any) apptest.Response {
	base := map[string]any{"display_name": "Clownfish pair", "category": "fish", "quantity": 2, "added_at": "2026-10-01T00:00:00Z"}
	for k, v := range body {
		base[k] = v
	}
	return b.Do("POST", "/tanks/"+tank+"/livestock", base)
}

func TestSpecies(t *testing.T) {
	e, b, tank := setup(t)
	sp := b.Expect(b.Do("POST", "/species", map[string]any{
		"category": "fish", "common_name_vi": "Cá hề", "common_name_en": "Ocellaris clownfish", "scientific_name": "Amphiprion ocellaris",
	}), http.StatusCreated, "").Body
	if sp["curated"] != false {
		t.Fatalf("species: %v", sp)
	}
	b.Expect(b.Do("POST", "/species", map[string]any{"category": "coral", "coral_type": "sps", "scientific_name": "Acropora millepora"}), http.StatusCreated, "")

	for q, n := range map[string]int{"": 2, "clown": 1, "CÁ HỀ": 1, "acropora": 1, "%": 0, "zzz": 0} {
		if got := len(items(b.Expect(b.Do("GET", "/species?q="+url.QueryEscape(q), nil), http.StatusOK, ""))); got != n {
			t.Errorf("q=%q: %d results, want %d", q, got, n)
		}
	}
	if got := len(items(b.Do("GET", "/species?category=coral", nil))); got != 1 {
		t.Errorf("coral: %d", got)
	}
	b.Expect(b.Do("GET", "/species?category=plant", nil), http.StatusUnprocessableEntity, "validation_failed")

	r := b.Expect(b.Do("POST", "/species", map[string]any{"category": "fish", "coral_type": "sps"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["coral_type"] != "coral_only" || c["common_name_vi"] != "name_required" {
		t.Fatalf("%v", c)
	}

	// Private species are invisible and unusable for others.
	other, _ := e.SignUp("minh@example.com", "Minh")
	if n := len(items(other.Do("GET", "/species", nil))); n != 0 {
		t.Fatalf("another user sees %d private species", n)
	}
	otherTank := other.Expect(other.Do("POST", "/tanks", map[string]any{"name": "B", "tank_type": "custom", "display_volume_l": "10", "system_volume_l": "10"}), http.StatusCreated, "").Body["id"].(string)
	r = other.Expect(addFish(other, otherTank, map[string]any{"species_id": sp["id"]}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["species_id"] != "species_not_found" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// The species sets the category; a different one is rejected.
	ent := b.Expect(addFish(b, tank, map[string]any{"species_id": sp["id"], "category": nil}), http.StatusCreated, "").Body
	if ent["category"] != "fish" || ent["species"].(map[string]any)["scientific_name"] != "Amphiprion ocellaris" {
		t.Fatalf("entry: %v", ent)
	}
	r = b.Expect(addFish(b, tank, map[string]any{"species_id": sp["id"], "category": "coral"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["category"] != "species_mismatch" {
		t.Fatalf("%v", r.FieldCodes())
	}
}

func TestQuantityFollowsEvents(t *testing.T) {
	e, b, tank := setup(t)
	ent := b.Expect(addFish(b, tank, nil), http.StatusCreated, "").Body
	id := ent["id"].(string)
	if ent["current_quantity"] != float64(2) || ent["present"] != true {
		t.Fatalf("created: %v", ent)
	}
	events := "/livestock/" + id + "/events"

	b.Expect(b.Do("POST", events, map[string]any{"type": "added", "quantity": 1, "occurred_at": "2026-10-10T00:00:00Z"}), http.StatusCreated, "")
	b.Expect(b.Do("POST", events, map[string]any{"type": "died", "quantity": 1, "note": "Found at the overflow"}), http.StatusCreated, "")
	got := b.Expect(b.Do("GET", "/livestock/"+id, nil), http.StatusOK, "").Body
	if got["current_quantity"] != float64(2) {
		t.Fatalf("quantity: %v", got["current_quantity"])
	}

	// Never below zero, also for a late, backdated event.
	b.Expect(b.Do("POST", events, map[string]any{"type": "sold", "quantity": 3}), http.StatusConflict, "quantity_underflow")
	// On the 5th only 2 were there (the third came on the 10th).
	b.Expect(b.Do("POST", events, map[string]any{"type": "died", "quantity": 3, "occurred_at": "2026-10-05T00:00:00Z"}), http.StatusConflict, "quantity_underflow")
	r := b.Expect(b.Do("POST", events, map[string]any{"type": "died", "quantity": 1, "occurred_at": "2026-09-01T00:00:00Z"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["occurred_at"] != "before_added" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Observations: note required, no quantity, no change.
	r = b.Expect(b.Do("POST", events, map[string]any{"type": "health_observation", "quantity": 1}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["quantity"] != "not_allowed" || c["note"] != "required" {
		t.Fatalf("%v", c)
	}
	b.Expect(b.Do("POST", events, map[string]any{"type": "health_observation", "note": "White spots on fins"}), http.StatusCreated, "")
	b.Expect(b.Do("POST", events, map[string]any{"type": "reconciliation", "quantity": 1}), http.StatusUnprocessableEntity, "validation_failed")
	b.Expect(b.Do("POST", events, map[string]any{"type": "died"}), http.StatusUnprocessableEntity, "validation_failed")

	// Reconcile: the difference is recorded, history kept; the same count again is a no-op.
	rec := b.Expect(b.Do("POST", "/livestock/"+id+"/reconcile", map[string]any{"count": 1, "note": "Counted at night"}), http.StatusOK, "").Body
	if rec["current_quantity"] != float64(1) {
		t.Fatalf("reconciled: %v", rec)
	}
	b.Expect(b.Do("POST", "/livestock/"+id+"/reconcile", map[string]any{"count": 1}), http.StatusOK, "")
	if n := count(t, e, `SELECT count(*) FROM livestock_events WHERE event_type = 'reconciliation' AND quantity_delta = -1`); n != 1 {
		t.Fatalf("reconciliation events = %d", n)
	}
	// Reconciling in the past is against the balance then, and must keep later balances.
	b.Expect(b.Do("POST", "/livestock/"+id+"/reconcile", map[string]any{"count": 0, "occurred_at": "2026-10-05T00:00:00Z"}), http.StatusConflict, "quantity_underflow")

	// Down to zero: gone, hidden by default, kept in history.
	b.Expect(b.Do("POST", events, map[string]any{"type": "transferred_out", "quantity": 1, "note": "To the frag tank"}), http.StatusCreated, "")
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/livestock", nil))); n != 0 {
		t.Fatalf("present entries = %d", n)
	}
	all := items(b.Do("GET", "/tanks/"+tank+"/livestock?include=gone", nil))
	if len(all) != 1 || all[0]["present"] != false || all[0]["current_quantity"] != float64(0) {
		t.Fatalf("gone: %v", all)
	}

	// The history is newest first and paginated; the timeline has every event.
	page := b.Expect(b.Do("GET", events+"?limit=3", nil), http.StatusOK, "")
	cursor, _ := page.Body["next_cursor"].(string)
	if len(items(page)) != 3 || cursor == "" || items(page)[0]["type"] != "transferred_out" {
		t.Fatalf("page 1: %v", page.Body)
	}
	rest := items(b.Expect(b.Do("GET", events+"?limit=10&cursor="+cursor, nil), http.StatusOK, ""))
	if len(rest) != 3 || rest[2]["type"] != "added" || rest[2]["quantity_delta"] != float64(2) {
		t.Fatalf("page 2: %v", rest)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE category = 'livestock'`); n != 6 {
		t.Fatalf("timeline = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM livestock_entries WHERE current_quantity <> (SELECT sum(quantity_delta) FROM livestock_events WHERE entry_id = livestock_entries.id)`); n != 0 {
		t.Fatal("cached quantity differs from the event sum")
	}
}

func TestIdempotencyAndConcurrency(t *testing.T) {
	e, b, tank := setup(t)
	first := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Tang", "category": "fish", "quantity": 1}, "Idempotency-Key", "entry-00001"), http.StatusCreated, "")
	again := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Tang", "category": "fish", "quantity": 1}, "Idempotency-Key", "entry-00001"), http.StatusCreated, "")
	if first.Body["id"] != again.Body["id"] || again.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("entry retry duplicated")
	}
	b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Other", "category": "fish", "quantity": 1}, "Idempotency-Key", "entry-00001"), http.StatusUnprocessableEntity, "idempotency_key_reused")
	if n := count(t, e, `SELECT count(*) FROM livestock_entries`); n != 1 {
		t.Fatalf("entries = %d", n)
	}

	id := b.Expect(addFish(b, tank, map[string]any{"quantity": 5}), http.StatusCreated, "").Body["id"].(string)
	events := "/livestock/" + id + "/events"
	d1 := b.Expect(b.Do("POST", events, map[string]any{"type": "died", "quantity": 1}, "Idempotency-Key", "death-00001"), http.StatusCreated, "")
	d2 := b.Expect(b.Do("POST", events, map[string]any{"type": "died", "quantity": 1}, "Idempotency-Key", "death-00001"), http.StatusCreated, "")
	if d1.Body["id"] != d2.Body["id"] || d2.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("event retry duplicated")
	}

	// Concurrent deaths: the balance never goes below zero (4 left, 8 tries of 1).
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = b.Do("POST", events, map[string]any{"type": "died", "quantity": 1}).Status
		}()
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 4 || conflict != 4 {
		t.Fatalf("created %d, conflicts %d (%v)", ok, conflict, codes)
	}
	if q := b.Do("GET", "/livestock/"+id, nil).Body["current_quantity"]; q != float64(0) {
		t.Fatalf("quantity = %v", q)
	}
}

func TestAuthorizationArchiveErasure(t *testing.T) {
	e, owner, tank := setup(t)
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	sp := owner.Expect(owner.Do("POST", "/species", map[string]any{"category": "fish", "common_name_en": "Clownfish"}), http.StatusCreated, "").Body["id"].(string)
	id := owner.Expect(addFish(owner, tank, map[string]any{"species_id": sp}), http.StatusCreated, "").Body["id"].(string)

	for _, p := range []string{"/tanks/" + tank + "/livestock", "/livestock/" + id, "/livestock/" + id + "/events"} {
		viewer.Expect(viewer.Do("GET", p, nil), http.StatusOK, "")
		stranger.Expect(stranger.Do("GET", p, nil), http.StatusNotFound, "not_found")
	}
	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", "/tanks/" + tank + "/livestock", map[string]any{"display_name": "x", "category": "fish", "quantity": 1}},
		{"PATCH", "/livestock/" + id, map[string]any{"version": 1, "display_name": "x"}},
		{"POST", "/livestock/" + id + "/events", map[string]any{"type": "died", "quantity": 1}},
		{"POST", "/livestock/" + id + "/reconcile", map[string]any{"count": 1}},
	}
	for _, c := range writes {
		viewer.Expect(viewer.Do(c.method, c.path, c.body), http.StatusForbidden, "forbidden")
		stranger.Expect(stranger.Do(c.method, c.path, c.body), http.StatusNotFound, "not_found")
	}
	stranger.Expect(stranger.Do("GET", "/livestock/not-a-uuid", nil), http.StatusNotFound, "not_found")

	// Patch: description only, with optimistic locking.
	upd := owner.Expect(owner.Do("PATCH", "/livestock/"+id, map[string]any{"version": 1, "display_name": "Nemo & Marlin", "location_note": "Left rock"}), http.StatusOK, "").Body
	if upd["display_name"] != "Nemo & Marlin" || upd["version"] != float64(2) || upd["current_quantity"] != float64(2) {
		t.Fatalf("patched: %v", upd)
	}
	owner.Expect(owner.Do("PATCH", "/livestock/"+id, map[string]any{"version": 1, "notes": "x"}), http.StatusConflict, "version_conflict")
	owner.Expect(owner.Do("PATCH", "/livestock/"+id, map[string]any{"version": 2, "current_quantity": 5}), http.StatusBadRequest, "")
	r := owner.Expect(owner.Do("PATCH", "/livestock/"+id, map[string]any{"version": 2, "category": "coral"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["category"] != "species_mismatch" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Archive: history readable, writes refused.
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	owner.Expect(owner.Do("POST", "/livestock/"+id+"/events", map[string]any{"type": "died", "quantity": 1}), http.StatusConflict, "tank_archived")
	owner.Expect(owner.Do("GET", "/livestock/"+id+"/events", nil), http.StatusOK, "")

	// Erasure removes livestock and own species.
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"livestock_events", "livestock_entries", "species", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

// Regression tests for the 2.7 reviews.
func TestReviewRules(t *testing.T) {
	e, b, tank := setup(t)

	// Unseen losses, finds and in-tank propagation.
	cuc := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Nassarius", "category": "invertebrate", "quantity": 10, "added_at": "2026-10-01T00:00:00Z"}), http.StatusCreated, "").Body["id"].(string)
	ev := "/livestock/" + cuc + "/events"
	for _, c := range []struct {
		typ   string
		delta float64
	}{{"missing", -3}, {"found", 1}, {"propagated", 4}} {
		got := b.Expect(b.Do("POST", ev, map[string]any{"type": c.typ, "quantity": int(abs(c.delta))}), http.StatusCreated, "").Body
		if got["quantity_delta"] != c.delta {
			t.Fatalf("%s: %v", c.typ, got)
		}
	}
	if q := b.Do("GET", "/livestock/"+cuc, nil).Body["current_quantity"]; q != float64(12) {
		t.Fatalf("quantity = %v", q)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'livestock.missing'`); n != 1 {
		t.Fatalf("missing on the timeline = %d", n)
	}
	b.Expect(b.Do("POST", ev, map[string]any{"type": "added", "quantity": 99999}), http.StatusUnprocessableEntity, "validation_failed")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "x", "category": "fish"}), http.StatusUnprocessableEntity, "validation_failed")

	// Times are compared as stored (microseconds): +400ns is the same instant, so the
	// later-recorded death goes after the find and the balance never dips below zero.
	tie := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Goby", "category": "fish", "quantity": 1, "added_at": "2026-10-01T00:00:00Z"}), http.StatusCreated, "").Body["id"].(string)
	b.Expect(b.Do("POST", "/livestock/"+tie+"/events", map[string]any{"type": "found", "quantity": 1, "occurred_at": "2026-10-20T00:00:00.0000004Z"}), http.StatusCreated, "")
	b.Expect(b.Do("POST", "/livestock/"+tie+"/events", map[string]any{"type": "died", "quantity": 2, "occurred_at": "2026-10-20T00:00:00Z"}), http.StatusCreated, "")
	if q := b.Do("GET", "/livestock/"+tie, nil).Body["current_quantity"]; q != float64(0) {
		t.Fatalf("quantity after ties = %v", q)
	}

	// History is append-only in the database too.
	if _, err := e.Pool.Exec(context.Background(), `UPDATE livestock_events SET quantity_delta = 50 WHERE event_type = 'propagated'`); err == nil {
		t.Fatal("livestock_events accepted an update")
	}

	// Coral type: on the entry, defaulted from the species, corals only.
	sps := b.Expect(b.Do("POST", "/species", map[string]any{"category": "coral", "coral_type": "sps", "scientific_name": "Montipora digitata"}), http.StatusCreated, "").Body["id"].(string)
	monti := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Green Monti", "species_id": sps, "quantity": 1, "form": "frag", "quarantine_status": "in_quarantine"}), http.StatusCreated, "").Body
	if monti["coral_type"] != "sps" || monti["category"] != "coral" || monti["quarantine_status"] != "in_quarantine" {
		t.Fatalf("frag: %v", monti)
	}
	r := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "x", "species_id": sps, "coral_type": "lps", "quantity": 1}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["coral_type"] != "species_mismatch" {
		t.Fatalf("%v", r.FieldCodes())
	}
	r = b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "x", "category": "fish", "coral_type": "lps", "quantity": 1}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["coral_type"] != "coral_only" {
		t.Fatalf("%v", r.FieldCodes())
	}
	b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Chaeto", "category": "algae", "quantity": 1, "form": "colony"}), http.StatusCreated, "")
	upd := b.Expect(b.Do("PATCH", "/livestock/"+monti["id"].(string), map[string]any{"version": 1, "quarantine_status": "completed"}), http.StatusOK, "").Body
	if upd["quarantine_status"] != "completed" || upd["coral_type"] != "sps" {
		t.Fatalf("patched: %v", upd)
	}

	// PATCH can't point at another keeper's private species.
	other, _ := e.SignUp("minh@example.com", "Minh")
	foreign := other.Expect(other.Do("POST", "/species", map[string]any{"category": "coral", "common_name_en": "Secret"}), http.StatusCreated, "").Body["id"].(string)
	r = b.Expect(b.Do("PATCH", "/livestock/"+monti["id"].(string), map[string]any{"version": 2, "species_id": foreign}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["species_id"] != "species_not_found" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Overflow is its own error.
	big := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Pods", "category": "invertebrate", "quantity": 10000}), http.StatusCreated, "").Body["id"].(string)
	b.Expect(b.Do("POST", "/livestock/"+big+"/reconcile", map[string]any{"count": 100000}), http.StatusOK, "")
	b.Expect(b.Do("POST", "/livestock/"+big+"/events", map[string]any{"type": "propagated", "quantity": 1}), http.StatusConflict, "quantity_overflow")

	// Retries are answered even after the tank was archived.
	body := map[string]any{"display_name": "Goby", "category": "fish", "quantity": 1}
	first := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", body, "Idempotency-Key", "entry-archive"), http.StatusCreated, "").Body["id"]
	died := b.Expect(b.Do("POST", "/livestock/"+cuc+"/events", map[string]any{"type": "died", "quantity": 1}, "Idempotency-Key", "event-archive"), http.StatusCreated, "").Body["id"]
	rec := map[string]any{"count": 5}
	b.Expect(b.Do("POST", "/livestock/"+big+"/reconcile", rec, "Idempotency-Key", "recon-archive"), http.StatusOK, "")
	b.Expect(b.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	again := b.Expect(b.Do("POST", "/tanks/"+tank+"/livestock", body, "Idempotency-Key", "entry-archive"), http.StatusCreated, "")
	if again.Body["id"] != first || again.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("entry replay after archive: %v", again.Body)
	}
	if got := b.Expect(b.Do("POST", "/livestock/"+cuc+"/events", map[string]any{"type": "died", "quantity": 1}, "Idempotency-Key", "event-archive"), http.StatusCreated, "").Body["id"]; got != died {
		t.Fatal("event replay after archive")
	}
	if q := b.Expect(b.Do("POST", "/livestock/"+big+"/reconcile", rec, "Idempotency-Key", "recon-archive"), http.StatusOK, "").Body["current_quantity"]; q != float64(5) {
		t.Fatalf("reconcile replay: %v", q)
	}
	b.Expect(b.Do("POST", "/livestock/"+big+"/reconcile", map[string]any{"count": 6}, "Idempotency-Key", "recon-archive"), http.StatusUnprocessableEntity, "idempotency_key_reused")
	b.Expect(b.Do("POST", "/livestock/"+cuc+"/events", map[string]any{"type": "died", "quantity": 1}, "Idempotency-Key", "event-new-01"), http.StatusConflict, "tank_archived")
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
