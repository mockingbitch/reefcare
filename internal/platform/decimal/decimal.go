// Package decimal holds exact decimal quantities (volumes, measurements, doses) without
// ever passing through float64. A Decimal keeps a canonical text form: optional "-",
// no leading zeros, no trailing fractional zeros ("60.00" → "60", "0.50" → "0.5").
//
// Decimals cross the API as JSON strings (docs/api-guidelines.md §1) and map to
// PostgreSQL numeric through pgx's NumericScanner/NumericValuer interfaces.
package decimal

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Decimal is an exact decimal number. The zero value is 0.
type Decimal struct {
	s string // canonical form; "" means 0
}

// ErrSyntax reports a string that is not a plain decimal number.
var ErrSyntax = errors.New("decimal: invalid syntax")

// maxLen bounds input so parsing cost stays trivial.
const maxLen = 64

// Parse accepts plain decimal notation: an optional leading "-", digits, and an
// optional "." followed by digits ("8.3", "-0.5", "1025"). Exponents, "+", spaces,
// thousands separators and a leading or trailing "." are rejected.
func Parse(s string) (Decimal, error) {
	if s == "" || len(s) > maxLen {
		return Decimal{}, ErrSyntax
	}
	neg := strings.HasPrefix(s, "-")
	body := strings.TrimPrefix(s, "-")
	intPart, frac, hasDot := strings.Cut(body, ".")
	if intPart == "" || (hasDot && frac == "") || !allDigits(intPart) || !allDigits(frac) {
		return Decimal{}, ErrSyntax
	}
	return canonical(neg, intPart, frac), nil
}

// MustParse is Parse for constants; it panics on invalid input.
func MustParse(s string) Decimal {
	d, err := Parse(s)
	if err != nil {
		panic(fmt.Sprintf("decimal.MustParse(%q): %v", s, err))
	}
	return d
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func canonical(neg bool, intPart, frac string) Decimal {
	intPart = strings.TrimLeft(intPart, "0")
	frac = strings.TrimRight(frac, "0")
	if intPart == "" {
		intPart = "0"
	}
	s := intPart
	if frac != "" {
		s += "." + frac
	}
	if s == "0" {
		return Decimal{}
	}
	if neg {
		s = "-" + s
	}
	return Decimal{s: s}
}

// String returns the canonical text form.
func (d Decimal) String() string {
	if d.s == "" {
		return "0"
	}
	return d.s
}

// Sign returns -1, 0 or +1.
func (d Decimal) Sign() int {
	switch {
	case d.s == "":
		return 0
	case d.s[0] == '-':
		return -1
	default:
		return 1
	}
}

// Digits returns the number of significant integer digits and of fractional digits,
// which is what a numeric(p, s) column constrains: intDigits ≤ p−s and fracDigits ≤ s.
func (d Decimal) Digits() (intDigits, fracDigits int) {
	intPart, frac, _ := strings.Cut(strings.TrimPrefix(d.String(), "-"), ".")
	if intPart != "0" {
		intDigits = len(intPart)
	}
	return intDigits, len(frac)
}

// FitsNumeric reports whether d can be stored in numeric(precision, scale) without
// rounding. Values are never rounded silently; callers reject them instead.
func (d Decimal) FitsNumeric(precision, scale int) bool {
	i, f := d.Digits()
	return i <= precision-scale && f <= scale
}

// Cmp compares d and o and returns -1, 0 or +1.
func (d Decimal) Cmp(o Decimal) int { return d.rat().Cmp(o.rat()) }

func (d Decimal) rat() *big.Rat {
	r, ok := new(big.Rat).SetString(d.String())
	if !ok { // unreachable: the canonical form always parses
		panic("decimal: corrupt value " + d.s)
	}
	return r
}

// MarshalJSON encodes d as a JSON string, preserving every digit.
func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

// MarshalText implements encoding.TextMarshaler.
func (d Decimal) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// ScanNumeric implements pgtype.NumericScanner.
func (d *Decimal) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid {
		return errors.New("decimal: cannot scan NULL into Decimal (use *Decimal)")
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return errors.New("decimal: NaN and infinity are not supported")
	}
	digits := new(big.Int).Abs(n.Int).String()
	neg := n.Int.Sign() < 0
	switch exp := int(n.Exp); {
	case exp >= 0:
		*d = canonical(neg, digits+strings.Repeat("0", exp), "")
	default:
		scale := -exp
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale-len(digits)+1) + digits
		}
		cut := len(digits) - scale
		*d = canonical(neg, digits[:cut], digits[cut:])
	}
	return nil
}

// NumericValue implements pgtype.NumericValuer.
func (d Decimal) NumericValue() (pgtype.Numeric, error) {
	intPart, frac, _ := strings.Cut(d.String(), ".")
	i, ok := new(big.Int).SetString(intPart+frac, 10)
	if !ok {
		return pgtype.Numeric{}, fmt.Errorf("decimal: corrupt value %q", d.s)
	}
	return pgtype.Numeric{Int: i, Exp: int32(-len(frac)), Valid: true}, nil //nolint:gosec // len(frac) < maxLen
}

// DivRound returns d / o rounded half away from zero to places decimal digits. Dividing
// by zero is an error. Use it only for derived, displayed values (ratios, estimates),
// never to rewrite a stored measurement.
func (d Decimal) DivRound(o Decimal, places int) (Decimal, error) {
	if o.Sign() == 0 {
		return Decimal{}, errors.New("decimal: division by zero")
	}
	q := new(big.Rat).Quo(d.rat(), o.rat())
	return Parse(q.FloatString(places))
}

// Mul returns the exact product d × o (a product of finite decimals is a finite decimal).
func (d Decimal) Mul(o Decimal) Decimal {
	_, fd := d.Digits()
	_, fo := o.Digits()
	out, err := Parse(new(big.Rat).Mul(d.rat(), o.rat()).FloatString(fd + fo))
	if err != nil { // unreachable: FloatString yields plain notation
		panic("decimal: " + err.Error())
	}
	return out
}
