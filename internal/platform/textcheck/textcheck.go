// Package textcheck validates user-written text that other people will see.
package textcheck

import "unicode"

// SingleLine reports whether s is safe as a one-line label (names, titles): it contains
// no control characters (Cc, including CR/LF and tab), no line/paragraph separators
// (U+2028, U+2029) and no format characters (Cf, including bidi overrides and
// zero-width spaces that can disguise text) except the joiners emoji sequences need.
func SingleLine(s string) bool {
	for _, r := range s {
		if r == '\u200c' || r == '\u200d' {
			continue
		}
		if unsafeRune(r) {
			return false
		}
	}
	return true
}

func unsafeRune(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

// MultiLine is SingleLine for free text (bios, descriptions): CR, LF and tab are also
// allowed. Bidi overrides, U+2028/U+2029 and other format characters are still rejected.
func MultiLine(s string) bool {
	for _, r := range s {
		switch r {
		case '\n', '\r', '\t', '\u200c', '\u200d':
			continue
		}
		if unsafeRune(r) {
			return false
		}
	}
	return true
}
