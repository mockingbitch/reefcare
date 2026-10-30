package jobs

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int32
		want    time.Duration // without jitter (jitter 0.5 → factor 1.0)
	}{
		{0, 15 * time.Second},
		{1, 15 * time.Second},
		{2, 30 * time.Second},
		{3, time.Minute},
		{8, 32 * time.Minute},
		{9, time.Hour},
		{50, time.Hour},
	}
	for _, c := range cases {
		if got := Backoff(c.attempt, 0.5); got != c.want {
			t.Errorf("Backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
	if lo, hi := Backoff(3, 0), Backoff(3, 0.999999); lo != 48*time.Second || hi < 71*time.Second || hi > 72*time.Second {
		t.Errorf("jitter range for 1 min: %v..%v, want 48s..72s", lo, hi)
	}
}

func TestPermanent(t *testing.T) {
	base := errors.New("bad payload")
	wrapped := fmt.Errorf("handler: %w", Permanent(base))
	if !IsPermanent(wrapped) || !errors.Is(wrapped, base) {
		t.Fatal("Permanent must survive wrapping and keep its cause")
	}
	if IsPermanent(base) {
		t.Fatal("plain errors are retryable")
	}
}

func TestTruncateKeepsUTF8(t *testing.T) {
	if got := truncate("ăăă", 3); got != "ă" {
		t.Fatalf("got %q", got)
	}
}
