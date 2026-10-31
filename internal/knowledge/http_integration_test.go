//go:build integration

package knowledge_test

import (
	"context"
	"net/http"
	"testing"

	"reefcare/internal/apptest"
	"reefcare/internal/identity"
)

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

func editor(t *testing.T, e *apptest.Env, email, name string) *apptest.Browser {
	t.Helper()
	b, _ := e.SignUp(email, name)
	if _, err := identity.SetRole(context.Background(), e.Pool, e.Clock, email, "administrator", true); err != nil {
		t.Fatal(err)
	}
	return b
}

func draft(title string) map[string]any {
	return map[string]any{
		"locale": "vi", "title": title, "summary": "Tóm tắt.", "body_md": "Nội dung **bài**.", "claim_class": "expert",
		"parameters": []string{"alkalinity", "alkalinity"}, "symptoms": []string{"tissue_recession"},
		"structured": map[string]any{
			"verifications":   []map[string]any{{"key": "retest_alk", "text": "Đo lại KH bằng bộ test khác."}},
			"recommendations": []map[string]any{{"key": "steady_dosing", "text": "Giữ liều ổn định.", "claim_class": "expert"}},
			"safety_warnings": []map[string]any{{"key": "no_big_jumps", "text": "Không tăng KH quá nhanh."}},
		},
	}
}

func TestWorkflow(t *testing.T) {
	e := apptest.New(t)
	alice := editor(t, e, "alice@example.com", "Alice")
	bob := editor(t, e, "bob@example.com", "Bob")
	reader, _ := e.SignUp("lan@example.com", "Lan")

	// Readers can't edit; there is nothing published yet.
	reader.Expect(reader.Do("POST", "/admin/knowledge/articles", map[string]any{"slug": "alk", "category": "water_chemistry"}), http.StatusForbidden, "forbidden")
	reader.Expect(reader.Do("GET", "/admin/knowledge/articles", nil), http.StatusForbidden, "forbidden")
	if n := len(items(reader.Expect(reader.Do("GET", "/knowledge", nil), http.StatusOK, ""))); n != 0 {
		t.Fatalf("published = %d", n)
	}
	e.Browser().Expect(e.Browser().Do("GET", "/knowledge", nil), http.StatusUnauthorized, "")

	// Sources and articles.
	src := alice.Expect(alice.Do("POST", "/admin/knowledge/sources", map[string]any{
		"title": "Test kit manual", "publisher": "Example Labs", "url": "https://example.org/manual", "accessed_on": "2026-10-01",
	}), http.StatusCreated, "").Body
	if src["accessed_on"] != "2026-10-01" {
		t.Fatalf("source: %v", src)
	}
	r := alice.Expect(alice.Do("POST", "/admin/knowledge/sources", map[string]any{"title": "x", "url": "http://insecure.example"}), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["url"] != "https_only" {
		t.Fatalf("%v", r.FieldCodes())
	}
	art := alice.Expect(alice.Do("POST", "/admin/knowledge/articles", map[string]any{"slug": "alkalinity-stability", "category": "water_chemistry"}), http.StatusCreated, "").Body["id"].(string)
	alice.Expect(alice.Do("POST", "/admin/knowledge/articles", map[string]any{"slug": "alkalinity-stability", "category": "testing"}), http.StatusConflict, "slug_taken")
	r = alice.Expect(alice.Do("POST", "/admin/knowledge/articles", map[string]any{"slug": "Bad Slug", "category": "magic"}), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["slug"] != "invalid_slug" || c["category"] != "unsupported" {
		t.Fatalf("%v", c)
	}

	// Draft, edit, validation.
	body := draft("Độ ổn định KH")
	body["sources"] = []map[string]any{{"source_id": src["id"], "note": "page 4"}}
	v := alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", body), http.StatusCreated, "").Body
	vid := v["id"].(string)
	if v["status"] != "draft" || v["version_no"] != float64(1) || len(v["parameters"].([]any)) != 1 || len(v["sources"].([]any)) != 1 {
		t.Fatalf("draft: %v", v)
	}
	bad := draft("x")
	bad["parameters"] = []string{"unobtainium"}
	bad["structured"] = map[string]any{"recommendations": []map[string]any{{"key": "Bad Key", "text": "x", "claim_class": "rumour"}}}
	r = alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", bad), http.StatusUnprocessableEntity, "validation_failed")
	if c := r.FieldCodes(); c["parameters.0"] != "unknown_parameter" || c["structured.recommendations.0.key"] != "invalid_key" || c["structured.recommendations.0.claim_class"] != "unsupported" {
		t.Fatalf("%v", c)
	}
	edit := draft("Độ ổn định của KH")
	edit["revision"] = v["revision"]
	edit["claim_class"] = "established"
	alice.Expect(alice.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusOK, "")
	alice.Expect(alice.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusConflict, "version_conflict")

	// An established version needs a source, checked at submit.
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+vid+"/submit", nil), http.StatusUnprocessableEntity, "validation_failed")
	cur := alice.Do("GET", "/admin/knowledge/versions/"+vid, nil).Body
	edit["revision"] = cur["revision"]
	edit["sources"] = []map[string]any{{"source_id": src["id"], "note": "page 4"}}
	alice.Expect(alice.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusOK, "")

	// Only the author edits or submits a draft: another editor can't take it over.
	bob.Expect(bob.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusForbidden, "not_author")
	bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+vid+"/submit", nil), http.StatusForbidden, "not_author")

	// Submit; the author can't publish their own.
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+vid+"/submit", nil), http.StatusOK, "")
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+vid+"/publish", nil), http.StatusConflict, "own_review")
	alice.Expect(alice.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusConflict, "not_draft")

	// Rejected back to draft: the reviewer still can't rewrite it and publish it himself.
	rej := bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+vid+"/reject", map[string]any{"note": "Cite the table.\nAnd the page."}), http.StatusOK, "").Body
	if rej["status"] != "draft" || rej["review_note"] != "Cite the table.\nAnd the page." {
		t.Fatalf("rejected: %v", rej)
	}
	edit["revision"] = rej["revision"]
	bob.Expect(bob.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusForbidden, "not_author")
	alice.Expect(alice.Do("PATCH", "/admin/knowledge/versions/"+vid, edit), http.StatusOK, "")
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+vid+"/submit", nil), http.StatusOK, "")
	pub := bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+vid+"/publish", map[string]any{"note": "OK,\nthanks"}), http.StatusOK, "").Body
	if pub["status"] != "published" || pub["published_at"] == nil {
		t.Fatalf("published: %v", pub)
	}
	if rv, _ := pub["reviews"].([]any); len(rv) != 2 || rv[0].(map[string]any)["decision"] != "rejected" {
		t.Fatalf("review history: %v", pub["reviews"])
	}
	// The source note survived the edits.
	if srcs := pub["sources"].([]any); len(srcs) != 1 || srcs[0].(map[string]any)["note"] != "page 4" {
		t.Fatalf("sources: %v", pub["sources"])
	}

	// A recommendation can't claim more than its version.
	strong := draft("x")
	strong["structured"] = map[string]any{"recommendations": []map[string]any{{"key": "r", "text": "x", "claim_class": "established"}}}
	r = alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", strong), http.StatusUnprocessableEntity, "validation_failed")
	if r.FieldCodes()["structured.recommendations.0.claim_class"] != "stronger_than_version" {
		t.Fatalf("%v", r.FieldCodes())
	}

	// Readers see it, in their locale or the other one.
	list := items(reader.Expect(reader.Do("GET", "/knowledge?locale=en&parameter=alkalinity", nil), http.StatusOK, ""))
	if len(list) != 1 || list[0]["slug"] != "alkalinity-stability" || list[0]["locale"] != "vi" {
		t.Fatalf("list: %v", list)
	}
	if n := len(items(reader.Do("GET", "/knowledge?parameter=calcium", nil))); n != 0 {
		t.Fatalf("calcium filter = %d", n)
	}
	if n := len(items(reader.Do("GET", "/knowledge?q=%C4%91%E1%BB%99%20%E1%BB%95n", nil))); n != 1 { // "độ ổn"
		t.Fatalf("search = %d", n)
	}
	got := reader.Expect(reader.Do("GET", "/knowledge/alkalinity-stability", nil), http.StatusOK, "").Body
	if got["body_md"] == nil || got["structured"] == nil || len(got["sources"].([]any)) != 1 || got["author_id"] != nil {
		t.Fatalf("article: %v", got)
	}
	reader.Expect(reader.Do("GET", "/knowledge/nope", nil), http.StatusNotFound, "not_found")
	alice.Expect(alice.Do("GET", "/admin/knowledge/articles/"+src["id"].(string)+"/versions", nil), http.StatusNotFound, "not_found")

	// Published content is frozen in the database too.
	if _, err := e.Pool.Exec(context.Background(), `UPDATE knowledge_article_versions SET body_md = 'changed' WHERE id = $1`, vid); err == nil {
		t.Fatal("a published version was edited")
	}
	if _, err := e.Pool.Exec(context.Background(), `DELETE FROM knowledge_version_sources WHERE version_id = $1`, vid); err == nil {
		t.Fatal("a published version's sources were changed")
	}

	// A new version replaces the live one; the old one is retired, not deleted.
	v2 := draft("KH ổn định (bản 2)")
	v2["claim_class"] = "expert"
	vid2 := alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", v2), http.StatusCreated, "").Body["id"].(string)
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+vid2+"/submit", nil), http.StatusOK, "")
	bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+vid2+"/publish", nil), http.StatusOK, "")
	if got := reader.Do("GET", "/knowledge/alkalinity-stability", nil).Body; got["version_no"] != float64(2) {
		t.Fatalf("live version: %v", got["version_no"])
	}
	if n := count(t, e, `SELECT count(*) FROM knowledge_article_versions WHERE status = 'retired' AND id = $1`, vid); n != 1 {
		t.Fatal("previous version not retired")
	}
	// Publishing an older version over a newer live one is refused.
	v3 := alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", draft("v3")), http.StatusCreated, "").Body["id"].(string)
	v4 := alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", draft("v4")), http.StatusCreated, "").Body["id"].(string)
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+v3+"/submit", nil), http.StatusOK, "")
	alice.Expect(alice.Do("POST", "/admin/knowledge/versions/"+v4+"/submit", nil), http.StatusOK, "")
	bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+v4+"/publish", nil), http.StatusOK, "")
	bob.Expect(bob.Do("POST", "/admin/knowledge/versions/"+v3+"/publish", nil), http.StatusConflict, "newer_version_live")

	// The database enforces the workflow too.
	ctx := context.Background()
	if _, err := e.Pool.Exec(ctx, `INSERT INTO knowledge_article_versions (article_id, locale, version_no, title, summary, body_md, claim_class, status, submitted_at, reviewed_by, reviewed_at, published_at)
		VALUES ($1, 'en', 1, 't', 's', 'b', 'expert', 'published', now(), NULL, now(), now())`, art); err == nil {
		t.Fatal("a version was inserted as published")
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE knowledge_article_versions SET author_id = (SELECT id FROM users WHERE email = 'bob@example.com') WHERE id = $1`, v3); err == nil {
		t.Fatal("the author of a submitted version was changed")
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE knowledge_article_versions SET published_at = '2000-01-01' WHERE id = $1`, vid); err == nil {
		t.Fatal("a published review record was rewritten")
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE knowledge_articles SET slug = 'other' WHERE id = $1`, art); err == nil {
		t.Fatal("a slug was changed")
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE knowledge_sources SET url = 'https://evil.example' WHERE id = $1`, src["id"]); err == nil {
		t.Fatal("a cited source was changed")
	}

	vs := items(alice.Expect(alice.Do("GET", "/admin/knowledge/articles/"+art+"/versions", nil), http.StatusOK, ""))
	arts := items(alice.Expect(alice.Do("GET", "/admin/knowledge/articles", nil), http.StatusOK, ""))
	if len(vs) != 4 || len(arts) != 1 || len(arts[0]["latest"].([]any)) != 1 {
		t.Fatalf("editor views: %v / %v", vs, arts)
	}

	// Withdrawing the article hides it from readers.
	bob.Expect(bob.Do("POST", "/admin/knowledge/articles/"+art+"/retire", nil), http.StatusNoContent, "")
	reader.Expect(reader.Do("GET", "/knowledge/alkalinity-stability", nil), http.StatusNotFound, "not_found")
	alice.Expect(alice.Do("POST", "/admin/knowledge/articles/"+art+"/versions", draft("x")), http.StatusConflict, "article_retired")

	bob.Expect(bob.Do("POST", "/admin/knowledge/articles/"+art+"/retire", nil), http.StatusNoContent, "") // again: no new audit
	if n := count(t, e, `SELECT count(*) FROM audit_logs WHERE action IN ('knowledge.published', 'knowledge.rejected', 'knowledge.retired')`); n != 5 {
		t.Fatalf("audit = %d", n)
	}
	if n := count(t, e, `SELECT count(*) FROM audit_logs WHERE action = 'role.granted' AND metadata ? 'operator'`); n != 2 {
		t.Fatalf("role audit = %d", n)
	}
}

func TestRoleRevoked(t *testing.T) {
	e := apptest.New(t)
	b := editor(t, e, "alice@example.com", "Alice")
	b.Expect(b.Do("GET", "/admin/knowledge/sources", nil), http.StatusOK, "")
	if _, err := identity.SetRole(context.Background(), e.Pool, e.Clock, "alice@example.com", "administrator", false); err != nil {
		t.Fatal(err)
	}
	b.Expect(b.Do("GET", "/admin/knowledge/sources", nil), http.StatusForbidden, "forbidden")
	if _, err := identity.SetRole(context.Background(), e.Pool, e.Clock, "nobody@example.com", "administrator", true); err == nil {
		t.Fatal("unknown account")
	}
	if _, err := identity.SetRole(context.Background(), e.Pool, e.Clock, "alice@example.com", "rank_gold", true); err == nil {
		t.Fatal("unknown role")
	}
}
