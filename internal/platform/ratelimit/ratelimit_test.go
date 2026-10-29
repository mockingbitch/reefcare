package ratelimit

import (
	"strings"
	"testing"
	"time"

	"reefcare/internal/platform/clock"
)

func TestLimiterBurstRefillAndKeys(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	l := New(10, 15*time.Minute, clk)

	for i := range 10 {
		if ok, _ := l.Allow("a@example.com"); !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	ok, retry := l.Allow("a@example.com")
	if ok {
		t.Fatal("11th attempt should be denied")
	}
	if retry <= 0 || retry > 90*time.Second {
		t.Errorf("retry-after = %v, want (0, 90s]", retry)
	}

	if ok, _ := l.Allow("b@example.com"); !ok {
		t.Error("other keys must have their own bucket")
	}

	// One token refills every 90s (15m / 10).
	clk.Advance(90 * time.Second)
	if ok, _ := l.Allow("a@example.com"); !ok {
		t.Error("token should refill after 90s")
	}
	if ok, _ := l.Allow("a@example.com"); ok {
		t.Error("only one token should have refilled")
	}
}

func TestDeniedAttemptsDoNotConsumeTokens(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	l := New(1, time.Minute, clk)
	l.Allow("k")
	for range 5 {
		l.Allow("k") // denied; must not push the next token further out
	}
	clk.Advance(time.Minute)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("denied attempts must not delay refill")
	}
}

func TestKeyCapEvictsLeastRecentlySeen(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	l := New(5, time.Minute, clk).WithMaxKeys(2)
	l.Allow("old")
	clk.Advance(time.Second)
	l.Allow("recent")
	if ok, _ := l.Allow("new"); !ok {
		t.Fatal("a new key must be admitted when the table is full")
	}
	if n := l.Len(); n != 2 {
		t.Fatalf("table size = %d, want cap 2", n)
	}
	// "recent" kept its bucket: exhausting it still denies.
	for range 4 {
		l.Allow("recent")
	}
	if ok, _ := l.Allow("recent"); ok {
		t.Fatal("the recently seen key should have kept its (exhausted) bucket")
	}
}

func TestHugeKeysUseFixedMemory(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	l := New(1, time.Minute, clk)
	huge := strings.Repeat("x", 1<<20)
	l.Allow(huge)
	if ok, _ := l.Allow(huge); ok {
		t.Fatal("same huge key must share a bucket")
	}
	for k := range l.buckets {
		if len(k) != 32 {
			t.Fatalf("stored key length %d, want 32", len(k))
		}
	}
}

func TestNewRejectsInvalidLimits(t *testing.T) {
	for _, tc := range []struct {
		n int
		w time.Duration
	}{{0, time.Minute}, {-1, time.Minute}, {10, 0}, {10, 5 * time.Nanosecond}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%d, %v) should panic", tc.n, tc.w)
				}
			}()
			New(tc.n, tc.w, clock.System{})
		}()
	}
}

func TestIdleBucketsAreCollected(t *testing.T) {
	clk := clock.NewFixed(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	l := New(5, time.Minute, clk)
	for _, k := range []string{"a", "b", "c"} {
		l.Allow(k)
	}
	clk.Advance(2 * time.Minute)
	l.Allow("d")
	if n := l.Len(); n != 1 {
		t.Fatalf("tracked keys = %d, want 1", n)
	}
}
