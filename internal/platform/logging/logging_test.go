package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactsSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo, "json")
	log.Info("login", "email_domain", "example.com", "Password", "hunter2", slog.Group("req", "token", "abc123"))

	out := buf.String()
	for _, secret := range []string{"hunter2", "abc123"} {
		if strings.Contains(out, secret) {
			t.Errorf("log output leaked %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "example.com") {
		t.Errorf("non-sensitive attribute missing: %s", out)
	}
}
