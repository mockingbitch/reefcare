package livestock

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestInsertKeepsEveryBalance(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }
	id := func(b byte) uuid.UUID { return uuid.UUID{15: b} }
	// Added 3 on the 1st, 2 died on the 10th: balances 3, 1.
	history := []move{{day(1), id(1), 3}, {day(10), id(2), -2}}
	for _, c := range []struct {
		name string
		m    move
		want int32
		res  int
	}{
		{"one more dies today", move{day(20), id(9), -1}, 0, balanceOK},
		{"two more die today", move{day(20), id(9), -2}, 0, balanceUnderflow},
		{"backdated death fits before the later one", move{day(5), id(9), -1}, 0, balanceOK},
		{"backdated death breaks the later balance", move{day(5), id(9), -2}, 0, balanceUnderflow},
		{"backdated addition", move{day(5), id(9), 4}, 5, balanceOK},
		{"same instant, earlier id: goes first and leaves the later death short", move{day(10), id(1), -3}, 0, balanceUnderflow},
		{"same instant, later id: goes after", move{day(10), id(3), -1}, 0, balanceOK},
		{"above the maximum", move{day(20), id(9), MaxQuantity}, 0, balanceOverflow},
	} {
		got, res := insert(history, c.m)
		if res != c.res || (res == balanceOK && got != c.want) {
			t.Errorf("%s: got %d %v, want %d %v", c.name, got, res, c.want, c.res)
		}
	}
	if b := balanceAt(history, move{At: day(5), ID: id(9)}); b != 3 {
		t.Errorf("balance on the 5th = %d", b)
	}
	if b := balanceAt(history, move{At: day(30), ID: id(9)}); b != 1 {
		t.Errorf("balance now = %d", b)
	}
	if len(history) != 2 {
		t.Fatal("insert must not modify the history")
	}
}

func TestStoredTruncatesToMicroseconds(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 1_234_567, time.UTC)
	if got := stored(at); got.Nanosecond() != 1_234_000 {
		t.Fatalf("stored = %v", got)
	}
}

func TestLikePattern(t *testing.T) {
	for in, want := range map[string]string{"": "%%", " Cá Hề ": "%cá hề%", "50%_off\\": `%50\%\_off\\%`} {
		if got := likePattern(in); got != want {
			t.Errorf("likePattern(%q) = %q, want %q", in, got, want)
		}
	}
}
