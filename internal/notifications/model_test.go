package notifications

import (
	"strings"
	"testing"
	"time"
)

func TestPreferences(t *testing.T) {
	if !wants(nil, CategoryDosing) || !wants(map[string]bool{"reminders_dosing": true}, CategoryDosing) {
		t.Fatal("in-app reminders default on")
	}
	if wants(map[string]bool{"reminders_dosing": false}, CategoryDosing) || !wants(map[string]bool{"reminders_dosing": false}, CategoryMaintenance) {
		t.Fatal("categories are independent")
	}
	if wantsEmail(nil) || !wantsEmail(map[string]bool{"email_reminders": true}) {
		t.Fatal("email is opt-in")
	}
}

func TestEmailBody(t *testing.T) {
	subject, body := emailBody("vi", "https://reef.example/notifications", map[string]int{CategoryDosing: 2, CategoryMaintenance: 1})
	for _, want := range []string{"Nhắc việc", "3 nhắc việc mới", "- Dosing: 2", "- Bảo trì: 1", "https://reef.example/notifications"} {
		if !strings.Contains(subject+body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Thay nước") {
		t.Error("empty categories are left out")
	}
	if _, b := emailBody("fr", "l", map[string]int{CategoryWaterChange: 1}); !strings.Contains(b, "Thay nước: 1") {
		t.Errorf("unknown locale falls back to vi:\n%s", b)
	}
}

func TestEmailLimits(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	epoch := time.Unix(0, 0)
	for _, c := range []struct {
		sent int32
		last time.Time
		ok   bool
	}{
		{0, epoch, true},
		{3, now.Add(-3 * time.Hour), true},
		{3, now.Add(-2 * time.Hour), false},
		{4, now.Add(-10 * time.Hour), false},
	} {
		if got := mayEmail(c.sent, c.last, now); got != c.ok {
			t.Errorf("sent %d, last %v: %v", c.sent, c.last, got)
		}
	}
}

func TestQuietHours(t *testing.T) {
	for utc, quiet := range map[int]bool{14: false, 15: true, 23: true, 0: false, 10: false} { // HCMC = UTC+7
		if got := quietHours(time.Date(2026, 10, 29, utc, 30, 0, 0, time.UTC), "Asia/Ho_Chi_Minh"); got != quiet {
			t.Errorf("%02d:30 UTC: quiet=%v", utc, got)
		}
	}
}
