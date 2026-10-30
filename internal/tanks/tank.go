// Package tanks owns tanks, read-only sharing and the tank authorization policy
// (docs/user-stories.md US-TK-1..4, docs/architecture.md §Authorization).
package tanks

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/patch"
	"reefcare/internal/platform/textcheck"
)

// Tank statuses. Archived is terminal and reachable only through Service.Archive.
const (
	StatusCycling  = "cycling"
	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusArchived = "archived"
)

var (
	tankTypes = map[string]bool{
		"fish_only": true, "fowlr": true, "mixed_reef": true, "sps_dominant": true,
		"lps_dominant": true, "soft_coral": true, "custom": true,
	}
	editableStatuses = map[string]bool{StatusCycling: true, StatusActive: true, StatusPaused: true}
	earliestSetup    = time.Date(1950, 1, 1, 0, 0, 0, 0, time.UTC)
)

// Column precisions (db/migrations/00004_tanks.sql). Values that don't fit are rejected,
// never rounded.
const (
	volumePrecision, volumeScale       = 9, 2
	dimensionPrecision, dimensionScale = 7, 1
)

// Dimensions are the inside measurements of the display tank.
type Dimensions struct {
	Length, Width, Height decimal.Decimal
}

// Tank is a tank as seen by one caller.
type Tank struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	Access         string // "owner" or "viewer"
	Name           string
	Description    *string
	TankType       string
	Status         string
	Dimensions     *Dimensions
	DisplayVolumeL decimal.Decimal
	SumpVolumeL    *decimal.Decimal
	// SystemVolumeL is the keeper's estimate of the total water volume. Calculations
	// (dose per volume, water-change fraction) use it and label it as an estimate.
	SystemVolumeL decimal.Decimal
	SetupDate     *time.Time
	WaterSource   *string
	SaltMix       *string
	ArchivedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int32
}

// DimensionsInput carries dimensions as decimal strings.
type DimensionsInput struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

// CreateInput is the new-tank form (US-TK-1). Decimals and the date are strings so
// every malformed value is reported against its field.
type CreateInput struct {
	Name           string
	Description    *string
	TankType       string
	Status         *string // default cycling
	Dimensions     *DimensionsInput
	DisplayVolumeL string
	SumpVolumeL    *string
	SystemVolumeL  string
	SetupDate      *string // YYYY-MM-DD
	WaterSource    *string
	SaltMix        *string
}

// Patch is a merge patch of a tank (US-TK-3). Version is required; null clears an
// optional attribute.
type Patch struct {
	Version        *int32
	Name           patch.Field[string]
	Description    patch.Field[string]
	TankType       patch.Field[string]
	Status         patch.Field[string]
	Dimensions     patch.Field[DimensionsInput]
	DisplayVolumeL patch.Field[string]
	SumpVolumeL    patch.Field[string]
	SystemVolumeL  patch.Field[string]
	SetupDate      patch.Field[string]
	WaterSource    patch.Field[string]
	SaltMix        patch.Field[string]
}

// fields is the validated, storable state of a tank's editable attributes.
type fields struct {
	Name           string
	Description    *string
	TankType       string
	Status         string
	Dimensions     *Dimensions
	DisplayVolumeL decimal.Decimal
	SumpVolumeL    *decimal.Decimal
	SystemVolumeL  decimal.Decimal
	SetupDate      *time.Time
	WaterSource    *string
	SaltMix        *string
}

// parser turns raw input into fields, collecting every problem in v.
type parser struct {
	v     *apperr.Validator
	today time.Time // UTC date of "now"; setup dates may be at most one day ahead (timezones)
}

func (p parser) name(s string) string {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > 80 {
		p.v.Add("name", "length", map[string]string{"min": "1", "max": "80"})
	} else if !textcheck.SingleLine(s) {
		p.v.Add("name", "invalid_characters", nil)
	}
	return s
}

// optText trims s; empty becomes nil.
func (p parser) optText(field string, s *string, maxLen int, multiLine bool) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	ok := textcheck.SingleLine(t)
	if multiLine {
		ok = textcheck.MultiLine(t)
	}
	if utf8.RuneCountInString(t) > maxLen {
		p.v.Add(field, "length", map[string]string{"max": itoa(maxLen)})
	} else if !ok {
		p.v.Add(field, "invalid_characters", nil)
	}
	return &t
}

func (p parser) tankType(s string) string {
	p.v.Check(tankTypes[s], "tank_type", "unsupported", nil)
	return s
}

func (p parser) status(s string) string {
	p.v.Check(editableStatuses[s], "status", "unsupported", nil)
	return s
}

// quantity parses a decimal that must fit numeric(precision, scale) and be > 0
// (or ≥ 0 when zeroOK).
func (p parser) quantity(field, s string, precision, scale int, zeroOK bool) decimal.Decimal {
	d, err := decimal.Parse(strings.TrimSpace(s))
	switch {
	case err != nil:
		p.v.Add(field, "invalid_decimal", nil)
	case d.Sign() < 0 || (d.Sign() == 0 && !zeroOK):
		code := "must_be_positive"
		if zeroOK {
			code = "must_not_be_negative"
		}
		p.v.Add(field, code, nil)
	case !d.FitsNumeric(precision, scale):
		p.v.Add(field, "out_of_range", map[string]string{"max_integer_digits": itoa(precision - scale), "max_decimals": itoa(scale)})
	}
	return d
}

func (p parser) dimensions(in *DimensionsInput) *Dimensions {
	if in == nil {
		return nil
	}
	return &Dimensions{
		Length: p.quantity("dimensions_cm.length", in.Length, dimensionPrecision, dimensionScale, false),
		Width:  p.quantity("dimensions_cm.width", in.Width, dimensionPrecision, dimensionScale, false),
		Height: p.quantity("dimensions_cm.height", in.Height, dimensionPrecision, dimensionScale, false),
	}
}

func (p parser) setupDate(s *string) *time.Time {
	if s == nil {
		return nil
	}
	d, err := time.Parse(time.DateOnly, strings.TrimSpace(*s))
	switch {
	case err != nil:
		p.v.Add("setup_date", "invalid_date", map[string]string{"format": "YYYY-MM-DD"})
		return nil
	case d.Before(earliestSetup) || d.After(p.today.AddDate(0, 0, 1)):
		p.v.Add("setup_date", "out_of_range", map[string]string{"min": earliestSetup.Format(time.DateOnly)})
		return nil
	}
	return &d
}

func (p parser) create(in CreateInput) fields {
	f := fields{
		Name:           p.name(in.Name),
		Description:    p.optText("description", in.Description, 4000, true),
		TankType:       p.tankType(in.TankType),
		Status:         StatusCycling,
		Dimensions:     p.dimensions(in.Dimensions),
		DisplayVolumeL: p.quantity("display_volume_l", in.DisplayVolumeL, volumePrecision, volumeScale, false),
		SystemVolumeL:  p.quantity("system_volume_l", in.SystemVolumeL, volumePrecision, volumeScale, false),
		SetupDate:      p.setupDate(in.SetupDate),
		WaterSource:    p.optText("water_source", in.WaterSource, 100, false),
		SaltMix:        p.optText("salt_mix", in.SaltMix, 100, false),
	}
	if in.Status != nil {
		f.Status = p.status(*in.Status)
	}
	if in.SumpVolumeL != nil {
		s := p.quantity("sump_volume_l", *in.SumpVolumeL, volumePrecision, volumeScale, true)
		f.SumpVolumeL = &s
	}
	return f
}

// apply merges pt onto the current state. Required attributes reject null.
func (p parser) apply(cur Tank, pt Patch) fields {
	f := fields{
		Name: cur.Name, Description: cur.Description, TankType: cur.TankType, Status: cur.Status,
		Dimensions: cur.Dimensions, DisplayVolumeL: cur.DisplayVolumeL, SumpVolumeL: cur.SumpVolumeL,
		SystemVolumeL: cur.SystemVolumeL, SetupDate: cur.SetupDate, WaterSource: cur.WaterSource, SaltMix: cur.SaltMix,
	}
	required := func(field string, pf patch.Field[string]) bool {
		if pf.Set && pf.Null {
			p.v.Add(field, "required", nil)
			return false
		}
		return pf.Set
	}
	if required("name", pt.Name) {
		f.Name = p.name(pt.Name.Value)
	}
	if pt.Description.Set {
		f.Description = p.optText("description", pt.Description.Apply(nil), 4000, true)
	}
	if required("tank_type", pt.TankType) {
		f.TankType = p.tankType(pt.TankType.Value)
	}
	if required("status", pt.Status) {
		f.Status = p.status(pt.Status.Value)
	}
	if pt.Dimensions.Set {
		f.Dimensions = p.dimensions(pt.Dimensions.Apply(nil))
	}
	if required("display_volume_l", pt.DisplayVolumeL) {
		f.DisplayVolumeL = p.quantity("display_volume_l", pt.DisplayVolumeL.Value, volumePrecision, volumeScale, false)
	}
	if pt.SumpVolumeL.Set {
		f.SumpVolumeL = nil
		if !pt.SumpVolumeL.Null {
			s := p.quantity("sump_volume_l", pt.SumpVolumeL.Value, volumePrecision, volumeScale, true)
			f.SumpVolumeL = &s
		}
	}
	if required("system_volume_l", pt.SystemVolumeL) {
		f.SystemVolumeL = p.quantity("system_volume_l", pt.SystemVolumeL.Value, volumePrecision, volumeScale, false)
	}
	if pt.SetupDate.Set {
		f.SetupDate = p.setupDate(pt.SetupDate.Apply(nil))
	}
	if pt.WaterSource.Set {
		f.WaterSource = p.optText("water_source", pt.WaterSource.Apply(nil), 100, false)
	}
	if pt.SaltMix.Set {
		f.SaltMix = p.optText("salt_mix", pt.SaltMix.Apply(nil), 100, false)
	}
	return f
}

func itoa(n int) string { return strconv.Itoa(n) }
