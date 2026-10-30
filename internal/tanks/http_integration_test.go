//go:build integration

package tanks_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

func nano() map[string]any {
	return map[string]any{
		"name": "Nano SPS 60", "tank_type": "sps_dominant", "status": "active",
		"display_volume_l": "60", "sump_volume_l": "25.50", "system_volume_l": "72",
		"dimensions_cm": map[string]string{"length": "60", "width": "40", "height": "35.5"},
		"setup_date":    "2025-06-01", "salt_mix": "Red Sea Coral Pro", "water_source": "RO/DI",
		"description": "Mostly Acropora.\nSkimmer + refugium.",
	}
}

func create(t *testing.T, b *apptest.Browser, body map[string]any) apptest.Response {
	t.Helper()
	return b.Expect(b.Do("POST", "/tanks", body), http.StatusCreated, "")
}

func TestCreateGetList(t *testing.T) {
	e := apptest.New(t)
	b, userID := e.SignUp("lan@example.com", "Lan")

	r := create(t, b, nano())
	id := r.Body["id"].(string)
	if r.Header.Get("Location") != "/api/v1/tanks/"+id {
		t.Errorf("Location = %q", r.Header.Get("Location"))
	}
	want := map[string]any{
		"owner_id": userID, "access": "owner", "name": "Nano SPS 60", "status": "active", "display_volume_l": "60",
		"sump_volume_l": "25.5", "system_volume_l": "72", "setup_date": "2025-06-01", "version": float64(1), "archived_at": nil,
	}
	for k, v := range want {
		if r.Body[k] != v {
			t.Errorf("%s = %#v, want %#v (%s)", k, r.Body[k], v, r.Raw)
		}
	}
	if d := r.Body["dimensions_cm"].(map[string]any); d["height"] != "35.5" || d["length"] != "60" {
		t.Errorf("dimensions: %v", d)
	}

	got := b.Expect(b.Do("GET", "/tanks/"+id, nil), http.StatusOK, "")
	if got.Raw != r.Raw {
		t.Errorf("GET differs from create response:\n%s\n%s", got.Raw, r.Raw)
	}

	// Minimal tank: defaults and nulls.
	m := create(t, b, map[string]any{"name": "QT", "tank_type": "fish_only", "display_volume_l": "40", "system_volume_l": "40"})
	if m.Body["status"] != "cycling" || m.Body["dimensions_cm"] != nil || m.Body["sump_volume_l"] != nil || m.Body["setup_date"] != nil {
		t.Errorf("minimal tank: %s", m.Raw)
	}

	list := b.Expect(b.Do("GET", "/tanks", nil), http.StatusOK, "")
	items := list.Body["items"].([]any)
	if len(items) != 2 || list.Body["next_cursor"] != nil {
		t.Fatalf("list: %s", list.Raw)
	}

	// tank.created is on the timeline, in the same transaction as the tank.
	var events int
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tank_events WHERE tank_id = $1 AND event_type = 'tank.created' AND category = 'milestone'`, id).Scan(&events); err != nil || events != 1 {
		t.Fatalf("tank.created events = %d, err %v", events, err)
	}
}

func TestCreateValidation(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")

	r := b.Expect(b.Do("POST", "/tanks", map[string]any{
		"name": " ", "tank_type": "pond", "status": "archived", "display_volume_l": "-5", "sump_volume_l": "1e3",
		"system_volume_l": "72.125", "dimensions_cm": map[string]string{"length": "60", "width": "", "height": "0"},
		"setup_date": "2026-13-01", "salt_mix": "x\u202Ey",
	}), http.StatusUnprocessableEntity, "validation_failed")
	want := map[string]string{
		"name": "length", "tank_type": "unsupported", "status": "unsupported", "display_volume_l": "must_be_positive",
		"sump_volume_l": "invalid_decimal", "system_volume_l": "out_of_range", "dimensions_cm.width": "invalid_decimal",
		"dimensions_cm.height": "must_be_positive", "setup_date": "invalid_date", "salt_mix": "invalid_characters",
	}
	got := r.FieldCodes()
	for f, c := range want {
		if got[f] != c {
			t.Errorf("field %s: got %q want %q (all %v)", f, got[f], c, got)
		}
	}

	future := nano()
	future["setup_date"] = "2026-11-05"
	if r := b.Do("POST", "/tanks", future); r.FieldCodes()["setup_date"] != "out_of_range" {
		t.Errorf("future setup date: %s", r.Raw)
	}
	huge := nano()
	huge["system_volume_l"] = "10000000"
	if r := b.Do("POST", "/tanks", huge); r.FieldCodes()["system_volume_l"] != "out_of_range" {
		t.Errorf("huge volume: %s", r.Raw)
	}

	// Decimals must be JSON strings, and server-owned fields cannot be set.
	numeric := nano()
	numeric["display_volume_l"] = 60
	if r := b.Expect(b.Do("POST", "/tanks", numeric), http.StatusBadRequest, "malformed_json"); r.FieldCodes()["display_volume_l"] != "wrong_type" {
		t.Errorf("numeric decimal: %s", r.Raw)
	}
	owned := nano()
	owned["owner_id"] = "00000000-0000-0000-0000-000000000001"
	b.Expect(b.Do("POST", "/tanks", owned), http.StatusBadRequest, "unknown_field")
	extra := nano()
	extra["dimensions_cm"] = map[string]string{"length": "1", "width": "1", "height": "1", "depth": "1"}
	b.Expect(b.Do("POST", "/tanks", extra), http.StatusBadRequest, "unknown_field")

	// Smaller system than display volume is allowed (rock displacement; US-TK-1 AC1).
	odd := nano()
	odd["system_volume_l"] = "50"
	create(t, b, odd)
}

func TestUpdateAndArchive(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	id := create(t, b, nano()).Body["id"].(string)
	path := "/tanks/" + id

	b.Expect(b.Do("PATCH", path, map[string]any{"name": "No version"}), http.StatusUnprocessableEntity, "validation_failed")
	r := b.Expect(b.Do("PATCH", path, map[string]any{
		"version": 1, "name": "Nano SPS", "sump_volume_l": nil, "dimensions_cm": nil, "status": "paused", "system_volume_l": "65.25",
	}), http.StatusOK, "")
	if r.Body["name"] != "Nano SPS" || r.Body["sump_volume_l"] != nil || r.Body["dimensions_cm"] != nil ||
		r.Body["status"] != "paused" || r.Body["system_volume_l"] != "65.25" || r.Body["version"] != float64(2) ||
		r.Body["salt_mix"] != "Red Sea Coral Pro" {
		t.Fatalf("patched: %s", r.Raw)
	}
	b.Expect(b.Do("PATCH", path, map[string]any{"version": 1, "name": "Stale"}), http.StatusConflict, "version_conflict")
	r = b.Expect(b.Do("PATCH", path, map[string]any{"version": 2, "name": nil, "status": "archived"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["name"] != "required" || c["status"] != "unsupported" {
		t.Fatalf("codes: %v", c)
	}

	r = b.Expect(b.Do("POST", path+"/archive", nil), http.StatusOK, "")
	if r.Body["status"] != "archived" || r.Body["archived_at"] != "2026-10-29T03:00:00Z" {
		t.Fatalf("archived: %s", r.Raw)
	}
	b.Expect(b.Do("POST", path+"/archive", nil), http.StatusOK, "") // idempotent
	b.Expect(b.Do("PATCH", path, map[string]any{"version": 3, "name": "x"}), http.StatusConflict, "tank_archived")
	b.Expect(b.Do("GET", path, nil), http.StatusOK, "") // history stays readable

	if items := b.Do("GET", "/tanks", nil).Body["items"].([]any); len(items) != 0 {
		t.Fatalf("archived tanks are hidden by default: %v", items)
	}
	if items := b.Do("GET", "/tanks?include=archived", nil).Body["items"].([]any); len(items) != 1 {
		t.Fatalf("include=archived: %v", items)
	}
	b.Expect(b.Do("GET", "/tanks?include=deleted", nil), http.StatusBadRequest, "invalid_query")

	var events int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM tank_events WHERE tank_id = $1 AND event_type = 'tank.archived'`, id).Scan(&events); err != nil || events != 1 {
		t.Fatalf("tank.archived events = %d, err %v", events, err)
	}
}

func TestConcurrentUpdatesNeverLoseWrites(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	id := create(t, b, nano()).Body["id"].(string)

	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for i := range 8 {
		wg.Go(func() {
			statuses <- b.Do("PATCH", "/tanks/"+id, map[string]any{"version": 1, "name": fmt.Sprintf("Writer %d", i)}).Status
		})
	}
	wg.Wait()
	close(statuses)
	ok, conflicts := 0, 0
	for s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflicts++
		}
	}
	if ok != 1 || conflicts != 7 {
		t.Fatalf("ok=%d conflicts=%d, want exactly one winner", ok, conflicts)
	}
}

func TestPagination(t *testing.T) {
	e := apptest.New(t)
	b, _ := e.SignUp("lan@example.com", "Lan")
	for i := range 5 {
		body := nano()
		body["name"] = fmt.Sprintf("Tank %d", i)
		create(t, b, body)
		e.Clock.Advance(time.Minute)
	}
	seen := map[string]bool{}
	var names []string
	cursor := ""
	for range 3 {
		r := b.Expect(b.Do("GET", "/tanks?limit=2"+cursor, nil), http.StatusOK, "")
		for _, it := range r.Body["items"].([]any) {
			m := it.(map[string]any)
			if seen[m["id"].(string)] {
				t.Fatalf("duplicate across pages: %v", m["id"])
			}
			seen[m["id"].(string)] = true
			names = append(names, m["name"].(string))
		}
		next, _ := r.Body["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = "&cursor=" + next
	}
	if fmt.Sprint(names) != "[Tank 4 Tank 3 Tank 2 Tank 1 Tank 0]" {
		t.Fatalf("order: %v", names)
	}
	b.Expect(b.Do("GET", "/tanks?cursor=garbage", nil), http.StatusBadRequest, "invalid_query")
	b.Expect(b.Do("GET", "/tanks?limit=201", nil), http.StatusBadRequest, "invalid_query")
}

func TestSharing(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	viewer, viewerID := e.SignUp("minh@example.com", "Minh")
	viewer.Expect(viewer.Do("PATCH", "/me", map[string]any{"version": 1, "username": "minh"}), http.StatusOK, "")
	id := create(t, owner, nano()).Body["id"].(string)
	path := "/tanks/" + id

	// Unverified owners cannot share (the lookup reveals whether an account exists).
	e.Register("unverified@example.com", "U")
	unverified := e.Browser()
	unverified.Expect(unverified.Login("unverified@example.com", apptest.Password), http.StatusOK, "")
	uid := create(t, unverified, nano()).Body["id"].(string)
	unverified.Expect(unverified.Do("POST", "/tanks/"+uid+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusForbidden, "email_not_verified")

	r := owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"email": "ghost@example.com"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["email"] != "grantee_not_found" {
		t.Fatalf("ghost: %s", r.Raw)
	}
	r = owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"email": "LAN@example.com"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["email"] != "cannot_share_with_self" {
		t.Fatalf("self: %s", r.Raw)
	}
	owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"email": "minh@example.com", "username": "minh"}), http.StatusUnprocessableEntity, "validation_failed")

	r = owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"username": "MINH"}), http.StatusCreated, "")
	if r.Body["user_id"] != viewerID || r.Body["permission"] != "viewer" || r.Body["display_name"] != "Minh" {
		t.Fatalf("share: %s", r.Raw)
	}
	if _, leaked := r.Body["email"]; leaked {
		t.Fatal("share responses must not expose the grantee's email")
	}
	owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusConflict, "duplicate")

	// The viewer can read but not change anything.
	g := viewer.Expect(viewer.Do("GET", path, nil), http.StatusOK, "")
	if g.Body["access"] != "viewer" {
		t.Fatalf("viewer access: %s", g.Raw)
	}
	if items := viewer.Do("GET", "/tanks", nil).Body["items"].([]any); len(items) != 0 {
		t.Fatal("shared tanks are only listed with include=shared")
	}
	if items := viewer.Do("GET", "/tanks?include=shared", nil).Body["items"].([]any); len(items) != 1 {
		t.Fatal("include=shared must list the shared tank")
	}
	viewer.Expect(viewer.Do("PATCH", path, map[string]any{"version": 1, "name": "Mine now"}), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("POST", path+"/archive", nil), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("GET", path+"/shares", nil), http.StatusForbidden, "forbidden")
	viewer.Expect(viewer.Do("POST", path+"/shares", map[string]string{"email": "lan@example.com"}), http.StatusForbidden, "forbidden")

	shares := owner.Expect(owner.Do("GET", path+"/shares", nil), http.StatusOK, "")
	if items := shares.Body["items"].([]any); len(items) != 1 {
		t.Fatalf("shares: %s", shares.Raw)
	}

	// Revocation takes effect immediately.
	owner.Expect(owner.Do("DELETE", path+"/shares/"+viewerID, nil), http.StatusNoContent, "")
	viewer.Expect(viewer.Do("GET", path, nil), http.StatusNotFound, "not_found")
	owner.Expect(owner.Do("DELETE", path+"/shares/"+viewerID, nil), http.StatusNotFound, "not_found")

	// A viewer may leave a tank on their own.
	owner.Expect(owner.Do("POST", path+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	viewer.Expect(viewer.Do("DELETE", path+"/shares/"+viewerID, nil), http.StatusNoContent, "")
	viewer.Expect(viewer.Do("GET", path, nil), http.StatusNotFound, "not_found")

	var grants, revokes int
	ctx := context.Background()
	if err := e.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action = 'tank.share.grant'), count(*) FILTER (WHERE action = 'tank.share.revoke') FROM audit_logs`).Scan(&grants, &revokes); err != nil || grants != 2 || revokes != 2 {
		t.Fatalf("audit grants=%d revokes=%d err=%v", grants, revokes, err)
	}
}

// An account squatting on someone else's (unverified) address must never receive the
// shares meant for that address; a deactivated grantee disappears from the share list.
func TestShareGranteeMustBeVerifiedAndActive(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	id := create(t, owner, nano()).Body["id"].(string)
	path := "/tanks/" + id + "/shares"

	e.Register("partner@shop.example", "Squatter")
	r := owner.Expect(owner.Do("POST", path, map[string]string{"email": "partner@shop.example"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["email"] != "grantee_not_found" {
		t.Fatalf("unverified grantee: %s", r.Raw)
	}

	viewer, _ := e.SignUp("minh@example.com", "Minh")
	owner.Expect(owner.Do("POST", path, map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	viewer.Expect(viewer.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	if items := owner.Expect(owner.Do("GET", path, nil), http.StatusOK, "").Body["items"].([]any); len(items) != 0 {
		t.Fatalf("deactivated grantee still listed: %v", items)
	}
}

func TestDeactivatedOwnerHidesSharedTanks(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	id := create(t, owner, nano()).Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+id+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	viewer.Expect(viewer.Do("GET", "/tanks/"+id, nil), http.StatusOK, "")

	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	viewer.Expect(viewer.Do("GET", "/tanks/"+id, nil), http.StatusNotFound, "not_found")
	if items := viewer.Do("GET", "/tanks?include=shared", nil).Body["items"].([]any); len(items) != 0 {
		t.Fatal("tanks of deactivated owners must disappear from viewers' lists")
	}
}

// Every tank endpoint answers a stranger exactly as it answers a tank that does not
// exist, so tank IDs cannot be probed (US-TK-2 AC1, docs/security.md §3).
func TestStrangersGet404Everywhere(t *testing.T) {
	e := apptest.New(t)
	owner, ownerID := e.SignUp("lan@example.com", "Lan")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	id := create(t, owner, nano()).Body["id"].(string)
	missing := "0192f0c4-0000-7000-8000-000000000000"

	endpoints := []struct {
		method, path string
		body         any
	}{
		{"GET", "/tanks/%s", nil},
		{"PATCH", "/tanks/%s", map[string]any{"version": 1, "name": "pwned"}},
		{"POST", "/tanks/%s/archive", nil},
		{"GET", "/tanks/%s/shares", nil},
		{"POST", "/tanks/%s/shares", map[string]string{"email": "eve@example.com"}},
		{"DELETE", "/tanks/%s/shares/" + ownerID, nil},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			real := stranger.Expect(stranger.Do(ep.method, fmt.Sprintf(ep.path, id), ep.body), http.StatusNotFound, "not_found")
			fake := stranger.Expect(stranger.Do(ep.method, fmt.Sprintf(ep.path, missing), ep.body), http.StatusNotFound, "not_found")
			stranger.Expect(stranger.Do(ep.method, fmt.Sprintf(ep.path, "not-a-uuid"), ep.body), http.StatusNotFound, "not_found")
			delete(real.Body["error"].(map[string]any), "request_id")
			delete(fake.Body["error"].(map[string]any), "request_id")
			if fmt.Sprint(real.Body) != fmt.Sprint(fake.Body) {
				t.Fatalf("responses differ: %v vs %v", real.Body, fake.Body)
			}
		})
	}
	if items := stranger.Do("GET", "/tanks?include=shared,archived", nil).Body["items"].([]any); len(items) != 0 {
		t.Fatal("strangers must not list other users' tanks")
	}
	// Nothing changed.
	r := owner.Expect(owner.Do("GET", "/tanks/"+id, nil), http.StatusOK, "")
	if r.Body["name"] != "Nano SPS 60" || r.Body["status"] != "active" {
		t.Fatalf("tank changed by stranger: %s", r.Raw)
	}

	anon := e.Browser()
	anon.Expect(anon.Do("GET", "/tanks", nil), http.StatusUnauthorized, "unauthenticated")
	anon.Expect(anon.Do("GET", "/tanks/"+id, nil), http.StatusUnauthorized, "unauthenticated")
}

// The per-owner tank limit holds under concurrent creates (the owner row is locked).
func TestTankLimitUnderConcurrency(t *testing.T) {
	e := apptest.New(t)
	b, userID := e.SignUp("lan@example.com", "Lan")
	if _, err := e.Pool.Exec(context.Background(), `
		INSERT INTO tanks (owner_id, name, tank_type, display_volume_l, system_volume_l)
		SELECT $1, 'Tank ' || g, 'custom', 10, 10 FROM generate_series(1, 99) g`, userID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 6)
	for range 6 {
		wg.Go(func() { statuses <- b.Do("POST", "/tanks", nano()).Status })
	}
	wg.Wait()
	close(statuses)
	created, limited := 0, 0
	for s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			limited++
		}
	}
	if created != 1 || limited != 5 {
		t.Fatalf("created=%d limited=%d, want exactly one create past 99", created, limited)
	}
}
