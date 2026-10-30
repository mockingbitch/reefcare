// Package waterparameters records water tests, corrections and per-tank targets, and serves
// trends and the latest values (docs/user-stories.md US-WP-1..5, milestones 2.1/2.2).
//
// PROVISIONAL (decision D5, open item L3): conversion factors, hard bounds and freshness
// windows below come from standard chemistry and the design docs and were reviewed by the
// reef-domain-expert agent, not yet by the named human domain reviewer. They must be
// confirmed before launch.
package waterparameters

import (
	"errors"
	"math"
	"strconv"

	"reefcare/internal/platform/decimal"
)

// Unit is an accepted input unit and its exact conversion to the canonical unit.
type Unit struct {
	Code string
	// Estimate marks conversions that rely on assumptions (e.g. SG → ppt at 25 °C);
	// readings converted with them are flagged as estimates.
	Estimate    bool
	toCanonical func(decimal.Decimal) (decimal.Decimal, error)
}

// Parameter is one catalog entry.
type Parameter struct {
	Code           string
	CanonicalUnit  string
	Units          []Unit
	HardMin        decimal.Decimal // canonical; outside is physically/analytically impossible
	HardMax        decimal.Decimal
	FreshnessHours int // default age after which a reading counts as stale
	Decimals       int // canonical values are rounded to this many places after conversion
}

func identity(d decimal.Decimal) (decimal.Decimal, error) { return d, nil }

func times(f string) func(decimal.Decimal) (decimal.Decimal, error) {
	k := decimal.MustParse(f)
	return func(d decimal.Decimal) (decimal.Decimal, error) { return d.Mul(k), nil }
}

func dividedBy(f string) func(decimal.Decimal) (decimal.Decimal, error) {
	k := decimal.MustParse(f)
	return func(d decimal.Decimal) (decimal.Decimal, error) { return d.DivRound(k, 6) }
}

var (
	c32  = decimal.MustParse("32")
	c5   = decimal.MustParse("5")
	c9   = decimal.MustParse("9")
	same = func(code string) Unit { return Unit{Code: code, toCanonical: identity} }
)

// fahrenheitToCelsius is exact up to the final rounding: (°F − 32) × 5 / 9.
func fahrenheitToCelsius(f decimal.Decimal) (decimal.Decimal, error) {
	return f.Sub(c32).Mul(c5).DivRound(c9, 6)
}

// Catalog lists the system parameters in display order. Codes match the rows seeded by
// migration 00011 (an integration test checks they agree).
var Catalog = []Parameter{
	{
		Code: "alkalinity", CanonicalUnit: "dKH", Decimals: 2, FreshnessHours: 7 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("30"),
		Units: []Unit{
			same("dKH"),
			// 1 dKH = 10 mg/L CaO = 0.35665 meq/L (CaO 56.077 g/mol, 2 eq/mol), so
			// 1 meq/L = 2.804 dKH and 1 dKH = 0.35665 × 50.04 = 17.848 mg/L as CaCO3.
			{Code: "meq/L", toCanonical: times("2.804")},
			{Code: "ppm CaCO3", toCanonical: dividedBy("17.848")}, // also what kits call "ppm KH"
		},
	},
	{
		Code: "calcium", CanonicalUnit: "mg/L", Decimals: 0, FreshnessHours: 14 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("1000"),
		Units: []Unit{same("mg/L"), same("ppm")}, // hobby kits report mg/L and label it ppm
	},
	{
		Code: "magnesium", CanonicalUnit: "mg/L", Decimals: 0, FreshnessHours: 30 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("3000"),
		Units: []Unit{same("mg/L"), same("ppm")},
	},
	{
		Code: "ph", CanonicalUnit: "pH", Decimals: 2, FreshnessHours: 7 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("14"),
		Units: []Unit{same("pH")},
	},
	{
		Code: "temperature", CanonicalUnit: "C", Decimals: 1, FreshnessHours: 7 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("45"),
		Units: []Unit{same("C"), {Code: "F", toCanonical: fahrenheitToCelsius}},
	},
	{
		Code: "salinity", CanonicalUnit: "ppt", Decimals: 1, FreshnessHours: 7 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("60"),
		Units: []Unit{
			same("ppt"),
			same("PSU"), // practical salinity, numerically ≈ g/kg for seawater
			{Code: "SG", Estimate: true, toCanonical: specificGravityToSalinity},
		},
	},
	{
		Code: "nitrate", CanonicalUnit: "mg/L NO3", Decimals: 2, FreshnessHours: 14 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("500"),
		Units: []Unit{
			same("mg/L NO3"), same("ppm"),
			{Code: "mg/L NO3-N", toCanonical: times("4.427")}, // NO3 / N = 62.004 / 14.007
		},
	},
	{
		Code: "phosphate", CanonicalUnit: "mg/L PO4", Decimals: 4, FreshnessHours: 14 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("20"),
		Units: []Unit{
			same("mg/L PO4"), same("ppm"),
			{Code: "mg/L PO4-P", toCanonical: times("3.066")}, // PO4 / P = 94.971 / 30.974
			{Code: "ppb PO4", toCanonical: dividedBy("1000")},
			// Ultra-low-range phosphorus checkers (e.g. Hanna HI736) read ppb as P, not PO4.
			{Code: "ppb PO4-P", toCanonical: times("0.003066")},
		},
	},
	{
		// Total ammonia (NH3 + NH4+) expressed as NH3. Kits report it as NH3, NH3-N or NH4+,
		// up to 29 % apart for the same water. Free-NH3 indicators (pH/temperature-dependent
		// share of the total) are not accepted as total ammonia.
		Code: "ammonia", CanonicalUnit: "mg/L NH3", Decimals: 2, FreshnessHours: 30 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("20"),
		Units: []Unit{
			same("mg/L NH3"), same("ppm"),
			{Code: "mg/L NH3-N", toCanonical: times("1.216")}, // NH3 / N = 17.031 / 14.007
			{Code: "mg/L NH4+", toCanonical: times("0.9441")}, // NH3 / NH4+ = 17.031 / 18.039
		},
	},
	{
		Code: "nitrite", CanonicalUnit: "mg/L NO2", Decimals: 3, FreshnessHours: 30 * 24,
		HardMin: decimal.MustParse("0"), HardMax: decimal.MustParse("50"),
		Units: []Unit{
			same("mg/L NO2"), same("ppm"),
			{Code: "mg/L NO2-N", toCanonical: times("3.285")}, // NO2 / N = 46.006 / 14.007
			// Ultra-low-range nitrite checkers (e.g. Hanna HI764) read ppb as N.
			{Code: "ppb NO2-N", toCanonical: times("0.003285")},
		},
	},
}

var byCode = func() map[string]*Parameter {
	m := make(map[string]*Parameter, len(Catalog))
	for i := range Catalog {
		m[Catalog[i].Code] = &Catalog[i]
	}
	return m
}()

// Lookup returns the catalog entry for code.
func Lookup(code string) (*Parameter, bool) {
	p, ok := byCode[code]
	return p, ok
}

// Unit returns the parameter's accepted unit with this code.
func (p *Parameter) Unit(code string) (Unit, bool) {
	for _, u := range p.Units {
		if u.Code == code {
			return u, true
		}
	}
	return Unit{}, false
}

// ToCanonical converts a value entered in u, rounded to the parameter's precision.
func (p *Parameter) ToCanonical(u Unit, v decimal.Decimal) (decimal.Decimal, error) {
	c, err := u.toCanonical(v)
	if err != nil {
		return decimal.Decimal{}, err
	}
	return c.Round(p.Decimals), nil
}

// Warnings returns soft, non-blocking hints that a value was probably entered in the wrong
// unit (.claude/rules/reef-domain.md: flag for confirmation, never silently clamp).
func (p *Parameter) Warnings(unit string, input, canonical decimal.Decimal) []string {
	var w []string
	switch p.Code {
	case "salinity":
		// A specific gravity typed into a ppt/PSU field ("1.026 ppt").
		if unit != "SG" && input.Cmp(sgLow) >= 0 && input.Cmp(sgHigh) <= 0 {
			w = append(w, "looks_like_specific_gravity")
		}
	case "phosphate":
		// ppb typed as mg/L (e.g. "15"): unusual outside heavily fed fish-only systems.
		if canonical.Cmp(phosphateHigh) > 0 {
			w = append(w, "unusually_high")
		}
	}
	return w
}

var (
	sgLow         = decimal.MustParse("0.99")
	sgHigh        = decimal.MustParse("1.04")
	phosphateHigh = decimal.MustParse("2")
)

// InBounds reports whether a canonical value is physically possible.
func (p *Parameter) InBounds(v decimal.Decimal) bool {
	return v.Cmp(p.HardMin) >= 0 && v.Cmp(p.HardMax) <= 0
}

// --- salinity from specific gravity -----------------------------------------------------

var errSGRange = errors.New("specific gravity outside the seawater range")

// specificGravityToSalinity estimates practical salinity from specific gravity read at (or
// compensated to) 25 °C, relative to pure water at 25 °C — what ATC refractometers and
// most hobby hydrometers report (SG assumed 25/25 °C; a 60/60 °F reading of the same water
// is about 0.0005 higher, ≈ 0.6 ppt, below the instruments' 0.001 resolution). It inverts the UNESCO 1981 (EOS-80) equation of state at
// atmospheric pressure by bisection. The result is an estimate: other reference
// temperatures (e.g. 60/60 °F) give different salinities for the same reading.
func specificGravityToSalinity(sg decimal.Decimal) (decimal.Decimal, error) {
	x, err := strconv.ParseFloat(sg.String(), 64)
	if err != nil {
		return decimal.Decimal{}, err
	}
	const t = 25.0
	target := x * pureWaterDensity(t)
	lo, hi := 0.0, 60.0
	if target < seawaterDensity(lo, t) || target > seawaterDensity(hi, t) {
		return decimal.Decimal{}, errSGRange
	}
	for range 60 {
		mid := (lo + hi) / 2
		if seawaterDensity(mid, t) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return decimal.Parse(strconv.FormatFloat(math.Round((lo+hi)/2*1000)/1000, 'f', 3, 64))
}

// pureWaterDensity is the SMOW density in kg/m³ at temperature t (°C), EOS-80.
func pureWaterDensity(t float64) float64 {
	return 999.842594 + 6.793952e-2*t - 9.095290e-3*t*t + 1.001685e-4*t*t*t -
		1.120083e-6*t*t*t*t + 6.536332e-9*t*t*t*t*t
}

// seawaterDensity is the EOS-80 density in kg/m³ at salinity s and temperature t (°C), 0 dbar.
func seawaterDensity(s, t float64) float64 {
	a := 8.24493e-1 - 4.0899e-3*t + 7.6438e-5*t*t - 8.2467e-7*t*t*t + 5.3875e-9*t*t*t*t
	b := -5.72466e-3 + 1.0227e-4*t - 1.6546e-6*t*t
	const c = 4.8314e-4
	return pureWaterDensity(t) + a*s + b*s*math.Sqrt(s) + c*s*s
}
