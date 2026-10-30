package decimal

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseCanonical(t *testing.T) {
	for in, want := range map[string]string{
		"60": "60", "60.00": "60", "0.50": "0.5", "007.10": "7.1", "0": "0", "-0.0": "0",
		"-8.30": "-8.3", "1.0255": "1.0255", "0.0001": "0.0001",
	} {
		d, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if d.String() != want {
			t.Errorf("Parse(%q) = %q, want %q", in, d.String(), want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", "-", ".", "1.", ".5", "+1", "1e3", "1,5", " 1", "1 ", "NaN", "Infinity", "--1", "1.2.3", "٣"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestDigitsAndFits(t *testing.T) {
	cases := []struct {
		in     string
		i, f   int
		fits92 bool // numeric(9,2)
		fits71 bool // numeric(7,1)
	}{
		{"60", 2, 0, true, true},
		{"0.25", 0, 2, true, false},
		{"9999999.99", 7, 2, true, false},
		{"10000000", 8, 0, false, false},
		{"123456.7", 6, 1, true, true},
		{"1.005", 1, 3, false, false},
	}
	for _, c := range cases {
		d := MustParse(c.in)
		i, f := d.Digits()
		if i != c.i || f != c.f || d.FitsNumeric(9, 2) != c.fits92 || d.FitsNumeric(7, 1) != c.fits71 {
			t.Errorf("%s: digits (%d,%d) fits92 %v fits71 %v", c.in, i, f, d.FitsNumeric(9, 2), d.FitsNumeric(7, 1))
		}
	}
}

func TestCmpAndSign(t *testing.T) {
	if MustParse("72").Cmp(MustParse("60.5")) != 1 || MustParse("8.30").Cmp(MustParse("8.3")) != 0 ||
		MustParse("-1").Cmp(MustParse("0")) != -1 {
		t.Fatal("Cmp wrong")
	}
	if MustParse("-0.1").Sign() != -1 || MustParse("0.00").Sign() != 0 || MustParse("3").Sign() != 1 {
		t.Fatal("Sign wrong")
	}
}

func TestNumericRoundTrip(t *testing.T) {
	for _, s := range []string{"0", "60", "0.5", "1.0255", "-8.3", "9999999.99", "0.0001", "1200"} {
		n, err := MustParse(s).NumericValue()
		if err != nil {
			t.Fatal(err)
		}
		var back Decimal
		if err := back.ScanNumeric(n); err != nil {
			t.Fatal(err)
		}
		if back.String() != s {
			t.Errorf("round trip %s → %s", s, back.String())
		}
	}
	// What PostgreSQL returns for numeric(9,2) 60 and for 1.2E3.
	var d Decimal
	if err := d.ScanNumeric(pgtype.Numeric{Int: big.NewInt(6000), Exp: -2, Valid: true}); err != nil || d.String() != "60" {
		t.Fatalf("got %s, %v", d, err)
	}
	if err := d.ScanNumeric(pgtype.Numeric{Int: big.NewInt(12), Exp: 2, Valid: true}); err != nil || d.String() != "1200" {
		t.Fatalf("got %s, %v", d, err)
	}
	if err := d.ScanNumeric(pgtype.Numeric{Int: big.NewInt(-5), Exp: -3, Valid: true}); err != nil || d.String() != "-0.005" {
		t.Fatalf("got %s, %v", d, err)
	}
	if err := d.ScanNumeric(pgtype.Numeric{}); err == nil {
		t.Fatal("NULL must not scan into a non-pointer Decimal")
	}
	if err := d.ScanNumeric(pgtype.Numeric{NaN: true, Valid: true}); err == nil {
		t.Fatal("NaN must be rejected")
	}
}

func TestJSONIsString(t *testing.T) {
	b, err := json.Marshal(struct {
		V Decimal  `json:"v"`
		P *Decimal `json:"p"`
	}{V: MustParse("8.30")})
	if err != nil || string(b) != `{"v":"8.3","p":null}` {
		t.Fatalf("got %s, %v", b, err)
	}
}

func TestDivRound(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"10", "72", "0.139"},
		{"25", "100", "0.25"},
		{"2", "3", "0.667"},
		{"1", "8", "0.125"},
		{"0.0005", "1", "0.001"},
	}
	for _, c := range cases {
		got, err := MustParse(c.a).DivRound(MustParse(c.b), 3)
		if err != nil || got.String() != c.want {
			t.Errorf("%s/%s = %v %v, want %s", c.a, c.b, got, err, c.want)
		}
	}
	if _, err := MustParse("1").DivRound(MustParse("0"), 3); err == nil {
		t.Fatal("division by zero must fail")
	}
}

func TestMul(t *testing.T) {
	for _, c := range []struct{ a, b, want string }{
		{"5", "1.5", "7.5"}, {"0.0005", "1.5", "0.00075"}, {"-2", "3", "-6"}, {"12.25", "0", "0"},
	} {
		if got := MustParse(c.a).Mul(MustParse(c.b)).String(); got != c.want {
			t.Errorf("%s×%s = %s, want %s", c.a, c.b, got, c.want)
		}
	}
}

func TestAddSubRound(t *testing.T) {
	tests := []struct{ a, b, sum, diff string }{
		{"8.3", "0.25", "8.55", "8.05"},
		{"32", "77", "109", "-45"},
		{"-0.5", "0.5", "0", "-1"},
		{"1.0001", "0.0009", "1.001", "0.9992"},
	}
	for _, tt := range tests {
		a, b := MustParse(tt.a), MustParse(tt.b)
		if got := a.Add(b).String(); got != tt.sum {
			t.Errorf("%s + %s = %s, want %s", tt.a, tt.b, got, tt.sum)
		}
		if got := a.Sub(b).String(); got != tt.diff {
			t.Errorf("%s - %s = %s, want %s", tt.a, tt.b, got, tt.diff)
		}
	}
	for in, want := range map[string]string{"8.345": "8.35", "-8.345": "-8.35", "8.344": "8.34", "8": "8"} {
		if got := MustParse(in).Round(2).String(); got != want {
			t.Errorf("Round(%s, 2) = %s, want %s", in, got, want)
		}
	}
}
