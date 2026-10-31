//go:build integration

package intelligence_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/apptest"
	"reefcare/internal/db/dbq"
	"reefcare/internal/identity"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/waterparameters"
)

// The test clock starts at 2026-10-29 03:00 UTC.

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

func reading(param, value, unit string) map[string]any {
	return map[string]any{"parameter": param, "value": value, "unit": unit}
}

func findings(body map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	raw, _ := body["findings"].([]any)
	for _, f := range raw {
		m := f.(map[string]any)
		key := m["rule_id"].(string)
		if p, ok := m["parameter"].(string); ok {
			key += ":" + p
		}
		out[key] = m
	}
	return out
}

// publishKnowledge publishes an article with a safety warning, written by one editor and
// reviewed by another.
func publishKnowledge(t *testing.T, e *apptest.Env, slug string) {
	t.Helper()
	editor := func(email, name string) *apptest.Browser {
		b, _ := e.SignUp(email, name)
		if _, err := identity.SetRole(context.Background(), e.Pool, e.Clock, email, "administrator", true); err != nil {
			t.Fatal(err)
		}
		return b
	}
	a, b := editor("alice@example.com", "Alice"), editor("bob@example.com", "Bob")
	art := a.Expect(a.Do("POST", "/admin/knowledge/articles", map[string]any{"slug": slug, "category": "water_chemistry"}), http.StatusCreated, "").Body["id"].(string)
	v := a.Expect(a.Do("POST", "/admin/knowledge/articles/"+art+"/versions", map[string]any{
		"locale": "vi", "title": "Amoniac trong bể đã ổn định", "summary": "Tóm tắt.", "body_md": "Nội dung.", "claim_class": "expert",
		"parameters": []string{"ammonia"},
		"structured": map[string]any{"safety_warnings": []map[string]any{{"key": "retest", "text": "Đo lại bằng bộ test khác trước khi xử lý."}}},
	}), http.StatusCreated, "").Body["id"].(string)
	a.Expect(a.Do("POST", "/admin/knowledge/versions/"+v+"/submit", nil), http.StatusOK, "")
	b.Expect(b.Do("POST", "/admin/knowledge/versions/"+v+"/publish", nil), http.StatusOK, "")
}

func TestAssessments(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	stranger, _ := e.SignUp("eve@example.com", "Eve")
	tank := owner.Expect(owner.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "status": "active", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Clownfish", "category": "fish", "quantity": 2}), http.StatusCreated, "")
	owner.Expect(owner.Do("PUT", "/tanks/"+tank+"/parameter-targets", map[string]any{"targets": []any{
		map[string]any{"parameter": "alkalinity", "min": "7.8", "max": "8.8"},
	}}), http.StatusOK, "")
	publishKnowledge(t, e, "ammonia-nitrite-established-tank")

	// New water tests are assessed in the background: a burst gives one assessment, run at the
	// end of the 10-minute bucket on the latest state.
	test := owner.Expect(owner.Do("POST", "/tanks/"+tank+"/water-tests", map[string]any{"readings": []any{
		reading("ammonia", "0.6", "mg/L NH3"), reading("alkalinity", "8.2", "dKH"),
	}}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/water-tests", map[string]any{"readings": []any{
		reading("temperature", "31", "C"),
	}}), http.StatusCreated, "")
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM assessments`); n != 0 {
		t.Fatalf("assessed before the bucket ended: %d", n)
	}
	e.Clock.Advance(10 * time.Minute)
	e.RunJobs()
	list := items(owner.Expect(owner.Do("GET", "/tanks/"+tank+"/assessments", nil), http.StatusOK, ""))
	if len(list) != 1 || list[0]["trigger"] != "water_test" || list[0]["water_test_id"] != test || list[0]["max_severity"] != "urgent" ||
		list[0]["finding_count"].(float64) < 2 || list[0]["findings"] != nil {
		t.Fatalf("after test: %v", list)
	}
	// A second job for the same test (a retry) stores nothing new.
	var userID string
	if err := e.Pool.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'lan@example.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.Enqueue(context.Background(), dbq.New(e.Pool), e.Clock.Now(), jobs.Spec{
		Kind: waterparameters.JobAssessAfterTest, DedupeKey: "retry",
		Payload: waterparameters.AssessAfterTest{TankID: uuid.MustParse(tank), WaterTestID: uuid.MustParse(test), UserID: uuid.MustParse(userID)},
	}); err != nil {
		t.Fatal(err)
	}
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM assessments`); n != 1 {
		t.Fatalf("assessments after retry = %d", n)
	}

	// A manual assessment: urgent findings, evidence, cited knowledge with its warning.
	a := owner.Expect(owner.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusCreated, "").Body
	fs := findings(a)
	nh, wb := fs["NH-AMMONIA-NITRITE:ammonia"], fs["WB-WELFARE-BOUNDS:temperature"]
	if nh == nil || nh["severity"] != "urgent" || wb == nil || wb["severity"] != "urgent" {
		t.Fatalf("findings: %v", a["findings"])
	}
	if a["provisional"] != true || a["rule_version"] != "1.0.0" || a["knowledge_version"] == "none" || a["trigger"] != "manual" {
		t.Fatalf("meta: %v", a)
	}
	refs, _ := nh["knowledge_references"].([]any)
	if len(refs) != 1 || refs[0].(map[string]any)["slug"] != "ammonia-nitrite-established-tank" {
		t.Fatalf("knowledge refs: %v", nh["knowledge_references"])
	}
	copied := false
	for _, w := range nh["safety_warnings"].([]any) {
		if w.(map[string]any)["text"].(map[string]any)["key"] == "knowledge.safety_warning" {
			copied = true
		}
	}
	if !copied {
		t.Fatalf("knowledge safety warning not carried: %v", nh["safety_warnings"])
	}
	id := a["id"].(string)
	if n := count(t, e, `SELECT count(*) FROM assessment_evidence WHERE assessment_id = $1 AND kind = 'reading'`, id); n < 2 {
		t.Fatalf("evidence rows = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM assessment_knowledge_refs WHERE assessment_id = $1`, id); n != 1 {
		t.Fatalf("knowledge refs rows = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM tank_events WHERE event_type = 'assessment.created'`); n != 2 {
		t.Fatalf("timeline = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM assessments WHERE id = $1 AND input_snapshot ? 'readings'`, id); n != 1 {
		t.Fatal("snapshot not stored")
	}

	// Readers see them; only the owner runs one; strangers see nothing.
	got := viewer.Expect(viewer.Do("GET", "/assessments/"+id, nil), http.StatusOK, "").Body
	if len(findings(got)) != len(fs) {
		t.Fatalf("viewer view: %v", got)
	}
	viewer.Expect(viewer.Do("GET", "/tanks/"+tank+"/assessments", nil), http.StatusOK, "")
	viewer.Expect(viewer.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusForbidden, "forbidden")
	stranger.Expect(stranger.Do("GET", "/assessments/"+id, nil), http.StatusNotFound, "not_found")
	stranger.Expect(stranger.Do("GET", "/tanks/"+tank+"/assessments", nil), http.StatusNotFound, "not_found")
	stranger.Expect(stranger.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusNotFound, "not_found")
	stranger.Expect(stranger.Do("GET", "/assessments/not-a-uuid", nil), http.StatusNotFound, "not_found")

	// Pagination.
	page := owner.Expect(owner.Do("GET", "/tanks/"+tank+"/assessments?limit=1", nil), http.StatusOK, "")
	cursor, _ := page.Body["next_cursor"].(string)
	if len(items(page)) != 1 || cursor == "" || items(page)[0]["id"] != id {
		t.Fatalf("page 1: %v", page.Raw)
	}
	if rest := items(owner.Do("GET", "/tanks/"+tank+"/assessments?limit=1&cursor="+cursor, nil)); len(rest) != 1 || rest[0]["trigger"] != "water_test" {
		t.Fatalf("page 2: %v", rest)
	}

	// Assessments are immutable: only a one-way retraction is possible.
	ctx := context.Background()
	for _, sql := range []string{
		`UPDATE assessments SET confidence_label = 'high' WHERE id = $1`,
		`UPDATE assessments SET requested_by = (SELECT id FROM users WHERE email = 'eve@example.com') WHERE id = $1`,
		`UPDATE assessment_evidence SET ref_id = gen_random_uuid() WHERE assessment_id = $1`,
		`UPDATE assessment_knowledge_refs SET version_id = version_id WHERE assessment_id = $1`,
	} {
		if _, err := e.Pool.Exec(ctx, sql, id); err == nil {
			t.Fatalf("allowed: %s", sql)
		}
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE assessments SET status = 'retracted', retracted_at = now(), retract_reason = 'test' WHERE id = $1`, id); err != nil {
		t.Fatalf("retraction refused: %v", err)
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE assessments SET status = 'generated', retracted_at = NULL, retract_reason = NULL WHERE id = $1`, id); err == nil {
		t.Fatal("a retraction was undone")
	}

	// Archived tanks are not assessed, by hand or after a test.
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/archive", nil), http.StatusOK, "")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusConflict, "tank_archived")
	owner.Expect(owner.Do("GET", "/assessments/"+id, nil), http.StatusOK, "")

	// Erasure removes them.
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	for _, table := range []string{"assessment_evidence", "assessment_knowledge_refs", "assessments", "tanks"} {
		if n := count(t, e, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d rows left after erasure", table, n)
		}
	}
}

func TestKnowledgeLocaleAndMissingArticles(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	v := owner.Expect(owner.Do("GET", "/me", nil), http.StatusOK, "").Body["version"]
	owner.Expect(owner.Do("PATCH", "/me", map[string]any{"version": v, "locale": "en"}), http.StatusOK, "")
	tank := owner.Expect(owner.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "status": "active", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/livestock", map[string]any{"display_name": "Goby", "category": "fish", "quantity": 1}), http.StatusCreated, "")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/water-tests", map[string]any{"readings": []any{reading("ammonia", "0.5", "mg/L NH3")}}), http.StatusCreated, "")

	// No article: no reference, never an invented one.
	a := owner.Expect(owner.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusCreated, "").Body
	nh := findings(a)["NH-AMMONIA-NITRITE:ammonia"]
	if refs, _ := nh["knowledge_references"].([]any); len(refs) != 0 || a["knowledge_version"] != "none" {
		t.Fatalf("refs without articles: %v %v", nh["knowledge_references"], a["knowledge_version"])
	}

	// Only a Vietnamese version exists: an English reader gets it, labelled vi.
	publishKnowledge(t, e, "ammonia-nitrite-established-tank")
	a = owner.Expect(owner.Do("POST", "/tanks/"+tank+"/assessments", nil), http.StatusCreated, "").Body
	refs, _ := findings(a)["NH-AMMONIA-NITRITE:ammonia"]["knowledge_references"].([]any)
	if a["locale"] != "en" || len(refs) != 1 || refs[0].(map[string]any)["locale"] != "vi" {
		t.Fatalf("locale fallback: %v %v", a["locale"], refs)
	}
}
