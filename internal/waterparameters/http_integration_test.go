//go:build integration

package waterparameters_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"reefcare/internal/apptest"
	"reefcare/internal/waterparameters"
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

func readings(body map[string]any) []map[string]any {
	raw, _ := body["readings"].([]any)
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

func reading(param, value, unit string) map[string]any {
	return map[string]any{"parameter": param, "value": value, "unit": unit}
}

func record(b *apptest.Browser, tank, at string, rs ...map[string]any) apptest.Response {
	body := map[string]any{"readings": rs}
	if at != "" {
		body["measured_at"] = at
	}
	return b.Do("POST", "/tanks/"+tank+"/water-tests", body)
}

func TestCatalogMatchesDatabase(t *testing.T) {
	e, b, _ := setup(t)
	rows, err := e.Pool.Query(context.Background(), `SELECT code, canonical_unit FROM parameter_types ORDER BY display_order`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var db []string
	for rows.Next() {
		var code, unit string
		if err := rows.Scan(&code, &unit); err != nil {
			t.Fatal(err)
		}
		db = append(db, code+"|"+unit)
	}
	var cat []string
	for _, p := range waterparameters.Catalog {
		cat = append(cat, p.Code+"|"+p.CanonicalUnit)
	}
	if !slices.Equal(db, cat) {
		t.Fatalf("parameter_types %v != catalog %v", db, cat)
	}

	list := items(b.Expect(b.Do("GET", "/parameter-types", nil), http.StatusOK, ""))
	if len(list) != len(waterparameters.Catalog) || list[0]["code"] != "alkalinity" || list[0]["hard_max"] != "30" {
		t.Fatalf("catalog: %v", list)
	}
	e.Browser().Expect(e.Browser().Do("GET", "/parameter-types", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestRecordConvertAndValidate(t *testing.T) {
	e, b, tank := setup(t)
	r := b.Expect(record(b, tank, "2026-10-28T08:00:00Z",
		reading("alkalinity", "3", "meq/L"),
		reading("salinity", "1.0264", "SG"),
		reading("phosphate", "10", "ppb PO4-P"),
		map[string]any{"parameter": "calcium", "value": "430", "unit": "ppm", "method": "titration", "test_kit": "Salifert"},
	), http.StatusCreated, "").Body
	rs := readings(r)
	if len(rs) != 4 {
		t.Fatalf("readings: %v", r)
	}
	want := map[string][3]any{ // canonical value, unit, estimate
		"alkalinity": {"8.41", "dKH", false},
		"salinity":   {"35", "ppt", true},
		"phosphate":  {"0.0307", "mg/L PO4", false},
		"calcium":    {"430", "mg/L", false},
	}
	for _, rd := range rs {
		w := want[rd["parameter"].(string)]
		if rd["canonical_value"] != w[0] || rd["canonical_unit"] != w[1] || rd["estimate"] != w[2] || rd["status"] != "valid" {
			t.Errorf("%s: %v", rd["parameter"], rd)
		}
	}
	if rs[0]["input_value"] != "3" || rs[0]["input_unit"] != "meq/L" {
		t.Errorf("original value lost: %v", rs[0])
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'measurement.recorded' AND tank_id = $1`, tank); n != 1 {
		t.Fatalf("timeline events = %d", n)
	}

	// Soft warnings are accepted and reported, never applied.
	w := readings(b.Expect(record(b, tank, "2026-10-28T09:00:00Z", reading("salinity", "1.025", "ppt")), http.StatusCreated, "").Body)
	if ws, _ := w[0]["warnings"].([]any); len(ws) != 1 || ws[0] != "looks_like_specific_gravity" || w[0]["canonical_value"] != "1" {
		t.Fatalf("salinity warning: %v", w[0])
	}

	// Validation.
	cases := []struct {
		body  map[string]any
		field string
		code  string
	}{
		{map[string]any{"readings": []any{}}, "readings", "required"},
		{map[string]any{"readings": []any{reading("potassium", "400", "mg/L")}}, "readings[0].parameter", "unsupported"},
		{map[string]any{"readings": []any{reading("ph", "8.1", "pH"), reading("ph", "8.2", "pH")}}, "readings[1].parameter", "duplicate"},
		{map[string]any{"readings": []any{reading("alkalinity", "8", "mg/L")}}, "readings[0].unit", "unsupported_unit"},
		{map[string]any{"readings": []any{reading("alkalinity", "8,2", "dKH")}}, "readings[0].value", "invalid_decimal"},
		{map[string]any{"readings": []any{reading("ph", "15", "pH")}}, "readings[0].value", "out_of_range"},
		{map[string]any{"readings": []any{reading("salinity", "1.2", "SG")}}, "readings[0].value", "out_of_range"},
		{map[string]any{"measured_at": "2026-10-29T04:00:00Z", "readings": []any{reading("ph", "8.1", "pH")}}, "measured_at", "in_future"},
	}
	for _, c := range cases {
		got := b.Expect(b.Do("POST", "/tanks/"+tank+"/water-tests", c.body), http.StatusUnprocessableEntity, "validation_failed").FieldCodes()
		if got[c.field] != c.code {
			t.Errorf("%v: %v, want %s=%s", c.body, got, c.field, c.code)
		}
	}
	b.Expect(b.Do("POST", "/tanks/"+tank+"/water-tests", map[string]any{"readings": []any{reading("ph", "8.1", "pH")}, "extra": 1}), http.StatusBadRequest, "")
}

func TestListCorrectAndLatest(t *testing.T) {
	e, b, tank := setup(t)
	first := b.Expect(record(b, tank, "2026-10-20T08:00:00Z", reading("alkalinity", "8.0", "dKH"), reading("nitrate", "5", "mg/L NO3")), http.StatusCreated, "").Body
	b.Expect(record(b, tank, "2026-10-27T08:00:00Z", reading("alkalinity", "7.5", "dKH")), http.StatusCreated, "")
	b.Expect(record(b, tank, "2026-10-28T08:00:00Z", reading("ph", "8.1", "pH")), http.StatusCreated, "")

	// Keyset pagination, newest first, and the parameter filter.
	p1 := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests?limit=2", nil), http.StatusOK, "")
	l1 := items(p1)
	next, _ := p1.Body["next_cursor"].(string)
	if len(l1) != 2 || l1[0]["measured_at"] != "2026-10-28T08:00:00Z" || next == "" {
		t.Fatalf("page 1: %v", p1.Body)
	}
	p2 := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests?limit=2&cursor="+url.QueryEscape(next), nil), http.StatusOK, "")
	if l2 := items(p2); len(l2) != 1 || l2[0]["id"] != first["id"] || p2.Body["next_cursor"] != nil {
		t.Fatalf("page 2: %v", p2.Body)
	}
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/water-tests?parameter=alkalinity", nil))); n != 2 {
		t.Fatalf("alkalinity tests = %d", n)
	}
	if n := len(items(b.Do("GET", "/tanks/"+tank+"/water-tests?from=2026-10-27T00:00:00Z&to=2026-10-28T00:00:00Z", nil))); n != 1 {
		t.Fatalf("range tests = %d", n)
	}
	b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests?from=yesterday", nil), http.StatusBadRequest, "invalid_query")

	// Correction: the old reading stays, superseded, and only the newest can be corrected.
	old := readings(first)[0]
	oldID := old["id"].(string)
	b.Expect(b.Do("POST", "/readings/"+oldID+"/corrections", map[string]any{"value": "8.4", "unit": "dKH"}), http.StatusUnprocessableEntity, "validation_failed")
	fix := b.Expect(b.Do("POST", "/readings/"+oldID+"/corrections", map[string]any{"value": "8.4", "unit": "dKH", "reason": "Misread the syringe"}), http.StatusCreated, "").Body
	if fix["supersedes_id"] != oldID || fix["canonical_value"] != "8.4" || fix["measured_at"] != "2026-10-20T08:00:00Z" {
		t.Fatalf("correction: %v", fix)
	}
	b.Expect(b.Do("POST", "/readings/"+oldID+"/corrections", map[string]any{"value": "8.5", "unit": "dKH", "reason": "again"}), http.StatusConflict, "already_superseded")
	test := b.Expect(b.Do("GET", "/water-tests/"+first["id"].(string), nil), http.StatusOK, "").Body
	statuses := map[string]string{}
	for _, rd := range readings(test) {
		statuses[rd["id"].(string)] = rd["status"].(string)
	}
	if len(statuses) != 3 || statuses[oldID] != "superseded" || statuses[fix["id"].(string)] != "valid" {
		t.Fatalf("history: %v", statuses)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'measurement.corrected'`); n != 1 {
		t.Fatalf("correction events = %d", n)
	}
	// History is append-only in the database too.
	if _, err := e.Pool.Exec(context.Background(), `UPDATE water_test_readings SET canonical_value = 1 WHERE id = $1`, oldID); err == nil {
		t.Fatal("reading edited in place")
	}

	// Latest: one row per catalog parameter, newest valid reading, freshness.
	latest := map[string]map[string]any{}
	for _, l := range items(b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests/latest", nil), http.StatusOK, "")) {
		latest[l["parameter"].(string)] = l
	}
	if len(latest) != len(waterparameters.Catalog) {
		t.Fatalf("latest: %v", latest)
	}
	alk := latest["alkalinity"]["reading"].(map[string]any)
	if alk["canonical_value"] != "7.5" || latest["alkalinity"]["stale"] != false {
		t.Fatalf("latest alkalinity: %v", latest["alkalinity"])
	}
	if latest["nitrate"]["stale"] != false || latest["calcium"]["reading"] != nil || latest["calcium"]["stale"] != true {
		t.Fatalf("latest nitrate/calcium: %v %v", latest["nitrate"], latest["calcium"])
	}
	e.Clock.Advance(15 * 24 * time.Hour) // nitrate's 14-day window has passed
	for _, l := range items(b.Do("GET", "/tanks/"+tank+"/water-tests/latest", nil)) {
		if l["parameter"] == "nitrate" && l["stale"] != true {
			t.Fatalf("nitrate should be stale: %v", l)
		}
	}
}

func TestTargetsAndTrends(t *testing.T) {
	e, b, tank := setup(t)
	targets := "/tanks/" + tank + "/parameter-targets"
	b.Expect(record(b, tank, "2026-10-01T08:00:00Z", reading("alkalinity", "7.0", "dKH")), http.StatusCreated, "")

	// Versioned targets: the first reading keeps the target valid when it was measured.
	b.Expect(b.Do("PUT", targets, map[string]any{"targets": []any{
		map[string]any{"parameter": "alkalinity", "min": "7.5", "ideal": "8", "max": "9"},
	}}), http.StatusOK, "")
	e.Clock.Advance(time.Hour)
	b.Expect(b.Do("PUT", targets, map[string]any{"targets": []any{
		map[string]any{"parameter": "alkalinity", "min": "6.5", "max": "9"},
		map[string]any{"parameter": "nitrate", "max": "10", "freshness_hours": 72},
	}}), http.StatusOK, "")
	cur := items(b.Expect(b.Do("GET", targets, nil), http.StatusOK, ""))
	if len(cur) != 2 || cur[0]["min"] != "6.5" || cur[0]["ideal"] != nil {
		t.Fatalf("current targets: %v", cur)
	}
	if n := count(t, e, `SELECT count(*) FROM parameter_targets WHERE tank_id = $1`, tank); n != 3 {
		t.Fatalf("target versions = %d", n)
	}
	e.Clock.Advance(time.Hour)
	in := readings(b.Expect(record(b, tank, "", reading("alkalinity", "7.0", "dKH"), reading("nitrate", "12", "ppm")), http.StatusCreated, "").Body)
	for _, rd := range in {
		want := map[string]string{"alkalinity": "within", "nitrate": "above"}[rd["parameter"].(string)]
		if rd["target_status"] != want {
			t.Errorf("%s target_status = %v, want %s", rd["parameter"], rd["target_status"], want)
		}
	}
	for _, l := range items(b.Do("GET", "/tanks/"+tank+"/water-tests/latest", nil)) {
		if l["parameter"] == "nitrate" && l["freshness_hours"] != float64(72) {
			t.Fatalf("target freshness: %v", l)
		}
	}
	// A test backdated before the targets keeps no status in history, but /latest compares
	// the newest value with the current target.
	back := readings(b.Expect(record(b, tank, "2026-10-28T00:00:00Z", reading("magnesium", "1300", "mg/L")), http.StatusCreated, "").Body)
	if back[0]["target_status"] != nil {
		t.Fatalf("backdated status: %v", back[0])
	}
	b.Expect(b.Do("PUT", targets, map[string]any{"targets": []any{
		map[string]any{"parameter": "alkalinity", "min": "6.5", "max": "9"},
		map[string]any{"parameter": "nitrate", "max": "10", "freshness_hours": 72},
		map[string]any{"parameter": "magnesium", "min": "1250", "max": "1450"},
	}}), http.StatusOK, "")
	for _, l := range items(b.Do("GET", "/tanks/"+tank+"/water-tests/latest", nil)) {
		if l["parameter"] == "magnesium" && l["reading"].(map[string]any)["target_status"] != "within" {
			t.Fatalf("latest magnesium: %v", l)
		}
	}

	bad := []struct {
		target map[string]any
		field  string
		code   string
	}{
		{map[string]any{"parameter": "alkalinity", "min": "9", "max": "8"}, "targets[0].max", "below_min"},
		{map[string]any{"parameter": "alkalinity", "min": "7", "ideal": "10", "max": "9"}, "targets[0].ideal", "outside_range"},
		{map[string]any{"parameter": "alkalinity"}, "targets[0]", "required"},
		{map[string]any{"parameter": "ph", "max": "20"}, "targets[0].max", "out_of_range"},
		{map[string]any{"parameter": "ph", "freshness_hours": 0}, "targets[0].freshness_hours", "out_of_range"},
	}
	for _, c := range bad {
		got := b.Expect(b.Do("PUT", targets, map[string]any{"targets": []any{c.target}}), http.StatusUnprocessableEntity, "validation_failed").FieldCodes()
		if got[c.field] != c.code {
			t.Errorf("%v: %v", c.target, got)
		}
	}

	// An annotation from another module.
	b.Expect(b.Do("POST", "/tanks/"+tank+"/water-changes", map[string]any{"completed_at": "2026-10-15T00:00:00Z", "actual_volume_l": "10"}), http.StatusCreated, "")

	tr := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests/trends?parameters=alkalinity,nitrate&from=2026-09-01T00:00:00Z", nil), http.StatusOK, "").Body
	series := tr["series"].([]any)
	alk := series[0].(map[string]any)
	if alk["parameter"] != "alkalinity" || alk["unit"] != "dKH" || len(alk["points"].([]any)) != 2 || len(alk["targets"].([]any)) != 2 {
		t.Fatalf("alkalinity series: %v", alk)
	}
	if ann := tr["annotations"].([]any); len(ann) != 1 || ann[0].(map[string]any)["category"] != "water_change" {
		t.Fatalf("annotations: %v", tr["annotations"])
	}
	for _, q := range []string{"", "parameters=potassium", "parameters=ph&from=2026-10-29T00:00:00Z&to=2026-10-01T00:00:00Z",
		"parameters=ph&from=2020-01-01T00:00:00Z&to=2026-01-01T00:00:00Z"} {
		b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests/trends?"+q, nil), http.StatusBadRequest, "invalid_query")
	}
}

func TestIdempotency(t *testing.T) {
	e, b, tank := setup(t)
	path := "/tanks/" + tank + "/water-tests"
	body := map[string]any{"readings": []any{reading("ph", "8.1", "pH")}}
	first := b.Expect(b.Do("POST", path, body, "Idempotency-Key", "test-00001"), http.StatusCreated, "")
	again := b.Expect(b.Do("POST", path, body, "Idempotency-Key", "test-00001"), http.StatusCreated, "")
	if again.Body["id"] != first.Body["id"] || again.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %v %v", again.Body, again.Header)
	}
	b.Expect(b.Do("POST", path, map[string]any{"readings": []any{reading("ph", "8.2", "pH")}}, "Idempotency-Key", "test-00001"), http.StatusUnprocessableEntity, "idempotency_key_reused")

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			b.Expect(b.Do("POST", path, body, "Idempotency-Key", "test-race-1"), http.StatusCreated, "")
		})
	}
	wg.Wait()
	if n := count(t, e, `SELECT count(*) FROM water_tests WHERE idempotency_key = 'test-race-1'`); n != 1 {
		t.Fatalf("concurrent retries created %d tests", n)
	}
}

func TestAuthorizationArchiveErasure(t *testing.T) {
	e, owner, tank := setup(t)
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	test := owner.Expect(record(owner, tank, "2026-10-28T08:00:00Z", reading("ph", "8.1", "pH")), http.StatusCreated, "").Body
	testID := test["id"].(string)
	rdID := readings(test)[0]["id"].(string)
	owner.Expect(owner.Do("PUT", "/tanks/"+tank+"/parameter-targets", map[string]any{"targets": []any{map[string]any{"parameter": "ph", "min": "7.9"}}}), http.StatusOK, "")

	for _, p := range []string{"/tanks/" + tank + "/water-tests", "/water-tests/" + testID, "/tanks/" + tank + "/water-tests/latest",
		"/tanks/" + tank + "/water-tests/trends?parameters=ph", "/tanks/" + tank + "/parameter-targets"} {
		viewer.Expect(viewer.Do("GET", p, nil), http.StatusOK, "")
		stranger.Expect(stranger.Do("GET", p, nil), http.StatusNotFound, "not_found")
	}
	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", "/tanks/" + tank + "/water-tests", map[string]any{"readings": []any{reading("ph", "8.1", "pH")}}},
		{"POST", "/readings/" + rdID + "/corrections", map[string]any{"value": "8", "unit": "pH", "reason": "x"}},
		{"PUT", "/tanks/" + tank + "/parameter-targets", map[string]any{"targets": []any{}}},
	}
	for _, c := range writes {
		viewer.Expect(viewer.Do(c.method, c.path, c.body), http.StatusForbidden, "forbidden")
		stranger.Expect(stranger.Do(c.method, c.path, c.body), http.StatusNotFound, "not_found")
	}
	stranger.Expect(stranger.Do("GET", "/water-tests/not-a-uuid", nil), http.StatusNotFound, "not_found")

	// A correction, so erasure must also delete a reading together with the one it supersedes.
	fix := owner.Expect(owner.Do("POST", "/readings/"+rdID+"/corrections", map[string]any{"value": "8", "unit": "pH", "reason": "x"}), http.StatusCreated, "").Body
	writes[1].path = "/readings/" + fix["id"].(string) + "/corrections"

	// Archive: history readable, writes refused.
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	for _, c := range writes {
		owner.Expect(owner.Do(c.method, c.path, c.body), http.StatusConflict, "tank_archived")
	}
	owner.Expect(owner.Do("GET", "/water-tests/"+testID, nil), http.StatusOK, "")

	// Erasure removes tests, readings (incl. corrections) and targets.
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"water_test_readings", "water_tests", "parameter_targets", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

// Regression tests for the 2.1/2.2 review.
func TestConcurrencyAndImmutability(t *testing.T) {
	e, b, tank := setup(t)
	targets := "/tanks/" + tank + "/parameter-targets"

	// Concurrent replacements queue on the tank row instead of deadlocking.
	var wg sync.WaitGroup
	statuses := make([]int, 6)
	for i := range statuses {
		wg.Go(func() {
			statuses[i] = b.Do("PUT", targets, map[string]any{"targets": []any{
				map[string]any{"parameter": "alkalinity", "min": "7", "max": fmt.Sprintf("%d", 9+i)},
			}}).Status
		})
	}
	wg.Wait()
	for i, st := range statuses {
		if st != http.StatusOK {
			t.Fatalf("PUT %d: status %d", i, st)
		}
	}
	if n := count(t, e, `SELECT count(*) FROM parameter_targets WHERE tank_id = $1 AND valid_to IS NULL`, tank); n != 1 {
		t.Fatalf("current targets = %d", n)
	}

	// A missing list is an error; an empty list clears.
	r := b.Expect(b.Do("PUT", targets, map[string]any{}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["targets"] != "required" {
		t.Fatalf("%v", r.FieldCodes())
	}
	if n := len(items(b.Expect(b.Do("PUT", targets, map[string]any{"targets": []any{}}), http.StatusOK, ""))); n != 0 {
		t.Fatalf("targets after clearing = %d", n)
	}

	// Concurrent corrections of one reading: exactly one wins.
	test := b.Expect(record(b, tank, "2026-10-28T08:00:00Z", reading("calcium", "400", "mg/L")), http.StatusCreated, "").Body
	rdID := readings(test)[0]["id"].(string)
	codes := make([]int, 4)
	for i := range codes {
		wg.Go(func() {
			codes[i] = b.Do("POST", "/readings/"+rdID+"/corrections", map[string]any{"value": "420", "unit": "mg/L", "reason": "Retest"}).Status
		})
	}
	wg.Wait()
	slices.Sort(codes)
	if !slices.Equal(codes, []int{http.StatusCreated, http.StatusConflict, http.StatusConflict, http.StatusConflict}) {
		t.Fatalf("concurrent corrections: %v", codes)
	}

	// History cannot be rewritten in the database either.
	ctx := context.Background()
	for _, stmt := range []string{
		`UPDATE water_test_readings SET input_value = 1 WHERE id = '` + rdID + `'`,
		`UPDATE water_test_readings SET status = 'valid' WHERE id = '` + rdID + `'`,
		`UPDATE water_tests SET measured_at = measured_at - interval '1 day' WHERE tank_id = '` + tank + `'`,
		`UPDATE parameter_targets SET max_value = 99 WHERE tank_id = '` + tank + `'`,
		`UPDATE parameter_targets SET valid_to = valid_to + interval '1 hour' WHERE tank_id = '` + tank + `' AND valid_to IS NOT NULL`,
	} {
		if _, err := e.Pool.Exec(ctx, stmt); err == nil || !strings.Contains(err.Error(), "42501") {
			t.Errorf("%s: %v", stmt, err)
		}
	}
}

func TestTrendTruncationKeepsNewest(t *testing.T) {
	e, b, tank := setup(t)
	b.Expect(record(b, tank, "2026-10-28T08:00:00Z", reading("ph", "8.1", "pH")), http.StatusCreated, "")
	// 3001 temperature readings straight into the table (the API would need 3001 requests).
	_, err := e.Pool.Exec(context.Background(), `
		WITH t AS (
			INSERT INTO water_tests (tank_id, measured_at, source)
			SELECT $1, timestamptz '2026-10-01' + n * interval '5 minutes', 'sensor' FROM generate_series(1, 3001) n
			RETURNING id, tank_id, measured_at)
		INSERT INTO water_test_readings (water_test_id, tank_id, parameter, measured_at, input_value, input_unit, canonical_value)
		SELECT id, tank_id, 'temperature', measured_at, 25, 'C', 25 FROM t`, tank)
	if err != nil {
		t.Fatal(err)
	}
	tr := b.Expect(b.Do("GET", "/tanks/"+tank+"/water-tests/trends?parameters=ph,temperature&from=2026-09-30T00:00:00Z", nil), http.StatusOK, "").Body
	series := tr["series"].([]any)
	ph, temp := series[0].(map[string]any), series[1].(map[string]any)
	pts := temp["points"].([]any)
	if tr["truncated"] != true || temp["truncated"] != true || ph["truncated"] != false || len(ph["points"].([]any)) != 1 || len(pts) != 3000 {
		t.Fatalf("truncation: %v %v %d", tr["truncated"], ph, len(pts))
	}
	first, last := pts[0].(map[string]any)["t"].(string), pts[len(pts)-1].(map[string]any)["t"].(string)
	if first != "2026-10-01T00:10:00Z" || last != "2026-10-11T10:05:00Z" {
		t.Fatalf("kept %s..%s, want the newest 3000 in time order", first, last)
	}
}
