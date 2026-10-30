//go:build integration

package notifications_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"reefcare/internal/apptest"
)

func TestEmailFailureAndQuietHours(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	tank := owner.Expect(owner.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	patchMe(t, owner, map[string]any{"email_reminders": true})
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/maintenance-plans", map[string]any{
		"title": "Clean skimmer cup", "interval_days": 1, "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29",
	}), http.StatusCreated, "")

	// The SMTP server is down for longer than the job retries: the reminder stays pending
	// and a later scan sends it.
	e.Outbox.FailNext(100)
	e.Clock.Advance(ScanStep)
	e.RunJobs()
	for range 6 { // retries back off; let them all run and die
		e.Clock.Advance(time.Hour)
		e.RunJobs()
	}
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE email_status = 'pending'`); n != 1 {
		t.Fatalf("pending = %d", n)
	}
	e.Outbox.FailNext(0)
	sent := len(e.Outbox.All())
	e.Clock.Advance(ScanStep) // 2026-10-29 ~09:30 UTC = 16:30 local
	e.RunJobs()
	e.Outbox.WaitFor(t, "reminders", "lan@example.com", sent)

	// Due again tomorrow: the reminder appears after local midnight, the email waits for 07:00.
	sent = len(e.Outbox.All())
	e.Clock.Advance(8 * time.Hour) // ~17:30 UTC = 00:30 local
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE kind = 'maintenance.overdue'`); n != 1 {
		t.Fatalf("overdue reminders = %d", n)
	}
	if n := len(e.Outbox.All()); n != sent {
		t.Fatal("email at night")
	}
	e.Clock.Advance(7 * time.Hour) // 07:30 local
	e.RunJobs()
	e.Outbox.WaitFor(t, "reminders", "lan@example.com", sent)
}

// The test clock starts at 2026-10-29 03:00 UTC = Thursday 10:00 in Ho Chi Minh City.

// ScanStep moves the clock into the next scan bucket.
const ScanStep = 15 * time.Minute

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

func kinds(t *testing.T, e *apptest.Env) map[string]int {
	t.Helper()
	rows, err := e.Pool.Query(context.Background(), `SELECT kind, count(*) FROM notifications GROUP BY kind`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			t.Fatal(err)
		}
		out[k] = n
	}
	return out
}

func patchMe(t *testing.T, b *apptest.Browser, prefs map[string]any) {
	t.Helper()
	v := b.Expect(b.Do("GET", "/me", nil), http.StatusOK, "").Body["version"]
	b.Expect(b.Do("PATCH", "/me", map[string]any{"version": v, "notification_prefs": prefs}), http.StatusOK, "")
}

func TestReminders(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	viewer, _ := e.SignUp("minh@example.com", "Minh")
	tank := owner.Expect(owner.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/shares", map[string]string{"email": "minh@example.com"}), http.StatusCreated, "")

	// Daily water change at 20:00 local (13:00 UTC), a twice-daily dose, maintenance due today.
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/water-change-schedules", map[string]any{
		"rrule": "FREQ=DAILY;BYHOUR=20", "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29", "planned_volume_l": "6",
	}), http.StatusCreated, "")
	product := owner.Expect(owner.Do("POST", "/dosing-products", map[string]any{"name": "Kalkwasser", "form": "powder", "default_unit": "g"}), http.StatusCreated, "").Body["id"].(string)
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/dosing-plans", map[string]any{
		"product_id": product, "amount": "2", "amount_unit": "g", "rrule": "FREQ=DAILY;BYHOUR=8,20",
		"timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29",
	}), http.StatusCreated, "")
	owner.Expect(owner.Do("POST", "/tanks/"+tank+"/maintenance-plans", map[string]any{
		"title": "Clean skimmer cup", "interval_days": 7, "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29",
	}), http.StatusCreated, "")
	e.Clock.Advance(ScanStep) // the 03:00 scan ran when the app started
	e.RunJobs()
	if k := kinds(t, e); k["water_change.upcoming"] != 1 || k["maintenance.due"] != 1 || len(k) != 2 {
		t.Fatalf("first scan: %v", k)
	}
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE email_status IS NOT NULL`); n != 0 {
		t.Fatal("email is opt-in")
	}

	// Opt in to email, then a day passes without doing anything.
	patchMe(t, owner, map[string]any{"email_reminders": true})
	sent := len(e.Outbox.All())
	e.Clock.Advance(26 * time.Hour) // 2026-10-30 05:15 UTC = 12:15 local
	e.RunJobs()
	// A dose marked missed after this scan ran is picked up by the next one.
	e.Clock.Advance(ScanStep)
	e.RunJobs()
	k := kinds(t, e)
	if k["water_change.overdue"] != 1 || k["water_change.upcoming"] != 2 || k["maintenance.overdue"] != 1 || k["dosing.missed"] != 1 { // the 08:00 dose today is still within its grace
		t.Fatalf("second scan: %v", k)
	}
	e.Outbox.WaitFor(t, "reminders", "lan@example.com", sent)
	// The missed dose may come one scan later than the others; its email then waits for the
	// 3-hour gap. Either way every reminder is emailed once, in at most two emails.
	e.Clock.Advance(3 * time.Hour)
	e.RunJobs()
	var mails []string
	for _, m := range e.Outbox.All()[sent:] {
		if m.Kind == "reminders" && m.To == "lan@example.com" {
			mails = append(mails, m.Subject+"\n"+m.Body)
		}
	}
	all := strings.Join(mails, "\n")
	if len(mails) < 1 || len(mails) > 2 || !strings.Contains(all, "Dosing: ") || // one or two missed doses, depending on job order
		!strings.Contains(all, "Thay nước: 2") ||
		!strings.Contains(all, "Bảo trì: 1") || !strings.Contains(all, "/notifications") {
		t.Fatalf("%d emails:\n%s", len(mails), all)
	}
	// No user-written text in mail: whoever set up the account may not own the mailbox.
	for _, s := range []string{"Kalkwasser", "Clean skimmer cup", "Nano", "Lan", "minh@"} {
		if strings.Contains(all, s) {
			t.Fatalf("email contains %q:\n%s", s, all)
		}
	}
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE email_status = 'pending'`); n != 0 {
		t.Fatalf("pending = %d", n)
	}

	// The next scan repeats nothing: existing reminders are neither re-created nor re-emailed.
	snapshot := func(before time.Time) string {
		var out string
		if err := e.Pool.QueryRow(context.Background(), `SELECT coalesce(string_agg(id::text || ':' || coalesce(emailed_at::text, '-'), ',' ORDER BY id), '')
			FROM notifications WHERE created_at <= $1`, before).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	cut := e.Clock.Now()
	was := snapshot(cut)
	e.Clock.Advance(ScanStep)
	e.RunJobs()
	if now := snapshot(cut); now != was {
		t.Fatalf("existing reminders changed:\n%s\n%s", was, now)
	}
	if n := count(t, e, `SELECT count(*) - count(DISTINCT dedupe_key) FROM notifications`); n != 0 {
		t.Fatal("duplicate reminders")
	}

	// Only the owner is reminded.
	if n := len(items(viewer.Expect(viewer.Do("GET", "/notifications", nil), http.StatusOK, ""))); n != 0 {
		t.Fatalf("viewer has %d notifications", n)
	}

	// A category can be turned off.
	owner.Login("lan@example.com", apptest.Password)
	patchMe(t, owner, map[string]any{"reminders_dosing": false, "email_reminders": false})
	missed := kinds(t, e)["dosing.missed"]
	e.Clock.Advance(13 * time.Hour)
	e.RunJobs()
	if n := kinds(t, e)["dosing.missed"]; n != missed {
		t.Fatalf("dosing reminders off, still %d → %d", missed, n)
	}
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE email_status IS NOT NULL AND created_at > $1`, e.Clock.Now().Add(-time.Minute)); n != 0 {
		t.Fatal("email after opting out")
	}
}

func TestReadingAndErasure(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.SignUp("lan@example.com", "Lan")
	other, _ := e.SignUp("eve@example.com", "Eve")
	tank := owner.Expect(owner.Do("POST", "/tanks", map[string]any{
		"name": "Nano", "tank_type": "mixed_reef", "display_volume_l": "60", "system_volume_l": "72",
	}), http.StatusCreated, "").Body["id"].(string)
	for _, title := range []string{"Clean skimmer cup", "Replace carbon", "Calibrate pH probe"} {
		owner.Expect(owner.Do("POST", "/tanks/"+tank+"/maintenance-plans", map[string]any{
			"title": title, "interval_days": 7, "timezone": "Asia/Ho_Chi_Minh", "starts_on": "2026-10-29",
		}), http.StatusCreated, "")
	}
	e.Clock.Advance(ScanStep)
	e.RunJobs()

	list := owner.Expect(owner.Do("GET", "/notifications", nil), http.StatusOK, "")
	ns := items(list)
	if len(ns) != 3 || ns[0]["kind"] != "maintenance.due" || ns[0]["tank_id"] != tank || ns[0]["read_at"] != nil {
		t.Fatalf("list: %v", list.Raw)
	}
	if p := ns[0]["payload"].(map[string]any); p["tank_name"] != "Nano" || p["due_on"] != "2026-10-29" {
		t.Fatalf("payload: %v", p)
	}
	if c := owner.Do("GET", "/notifications/unread-count", nil).Body; c["count"] != float64(3) || c["capped"] != false {
		t.Fatalf("count: %v", c)
	}

	id := ns[0]["id"].(string)
	other.Expect(other.Do("POST", "/notifications/"+id+"/read", nil), http.StatusNotFound, "not_found")
	other.Expect(other.Do("POST", "/notifications/not-a-uuid/read", nil), http.StatusNotFound, "not_found")
	owner.Expect(owner.Do("POST", "/notifications/"+id+"/read", nil), http.StatusNoContent, "")
	owner.Expect(owner.Do("POST", "/notifications/"+id+"/read", nil), http.StatusNoContent, "")
	if n := len(items(owner.Do("GET", "/notifications?unread=true", nil))); n != 2 {
		t.Fatalf("unread = %d", n)
	}
	owner.Expect(owner.Do("GET", "/notifications?unread=maybe", nil), http.StatusBadRequest, "invalid_query")

	page := owner.Expect(owner.Do("GET", "/notifications?limit=2", nil), http.StatusOK, "")
	cursor, _ := page.Body["next_cursor"].(string)
	if len(items(page)) != 2 || cursor == "" || len(items(owner.Do("GET", "/notifications?limit=2&cursor="+cursor, nil))) != 1 {
		t.Fatalf("pagination: %v", page.Raw)
	}

	owner.Expect(owner.Do("POST", "/notifications/read-all", nil), http.StatusNoContent, "")
	if c := owner.Do("GET", "/notifications/unread-count", nil).Body["count"]; c != float64(0) {
		t.Fatalf("after read-all: %v", c)
	}
	e.Browser().Expect(e.Browser().Do("GET", "/notifications", nil), http.StatusUnauthorized, "")

	// Retention: read ones go after 90 days.
	e.Clock.Advance(91 * 24 * time.Hour)
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM notifications WHERE read_at IS NOT NULL`); n != 0 {
		t.Fatalf("%d read notifications kept past retention", n)
	}

	// Erasure removes the account's notifications.
	owner.Login("lan@example.com", apptest.Password)
	e.RunJobs()
	owner.Expect(owner.Do("DELETE", "/me", map[string]string{"password": apptest.Password}), http.StatusAccepted, "")
	e.Clock.Advance(31 * 24 * time.Hour)
	e.RunJobs()
	if n := count(t, e, `SELECT count(*) FROM notifications`); n != 0 {
		t.Fatalf("%d notifications left after erasure", n)
	}
}
