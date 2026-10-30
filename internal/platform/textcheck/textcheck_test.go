package textcheck

import "testing"

func TestSingleLine(t *testing.T) {
	for _, s := range []string{"Lan\nNguyen", "Lan\r", "a\u202Eb", "tab\there", "zero\u200Bwidth", "line\u2028sep", "para\u2029sep"} {
		if SingleLine(s) {
			t.Errorf("%q should be rejected", s)
		}
	}
	for _, s := range []string{"Lan Nguyễn", "Rạn san hô 🐠", "O'Brien-Smith", "👨\u200d👩\u200d👧 family"} {
		if !SingleLine(s) {
			t.Errorf("%q should be allowed", s)
		}
	}
}

func TestMultiLine(t *testing.T) {
	for _, s := range []string{"line one\nline two", "a\r\nb", "col\tcol", "🧑\u200d🔬 reef keeper"} {
		if !MultiLine(s) {
			t.Errorf("%q should be allowed", s)
		}
	}
	for _, s := range []string{"a\u202Eb", "bell\a", "nul\x00", "zero\u200Bwidth", "line\u2028sep"} {
		if MultiLine(s) {
			t.Errorf("%q should be rejected", s)
		}
	}
}
