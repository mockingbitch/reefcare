package waterparameters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/platform/textcheck"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
)

// Limits and windows.
const (
	maxFutureSkew  = 5 * time.Minute // US-WP-1 AC2
	maxTrendRange  = 2 * 366 * 24 * time.Hour
	defaultTrend   = 90 * 24 * time.Hour
	maxTrendPoints = 3000 // per series
	maxAnnotations = 500
)

var (
	earliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	minTime  = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime  = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

	methods = map[string]bool{"titration": true, "colorimetric": true, "photometer": true, "refractometer": true,
		"hydrometer": true, "probe": true, "icp": true, "other": true}

	errKeyReused    = apperr.Invalid("idempotency_key_reused", "This Idempotency-Key was already used for a different request.")
	errLostReplay   = errors.New("waterparameters: idempotent insert conflicted but no first row was found")
	errSuperseded   = apperr.Conflict("already_superseded", "This reading was already corrected; correct the newest one.")
	annotationTypes = []string{timeline.CategoryDosing, timeline.CategoryWaterChange, timeline.CategoryEquipment}
)

// Service records and reads water parameters.
type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
	authz tanks.Authorizer
	log   *slog.Logger
}

// NewService wires the service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, log: log}
}

// --- model ------------------------------------------------------------------------------

// Reading is one measured value, as entered and in canonical units.
type Reading struct {
	ID               uuid.UUID
	WaterTestID      uuid.UUID
	Parameter        string
	MeasuredAt       time.Time
	InputValue       decimal.Decimal
	InputUnit        string
	CanonicalValue   decimal.Decimal
	CanonicalUnit    string
	Estimate         bool
	Method           *string
	TestKit          *string
	Status           string
	SupersedesID     *uuid.UUID
	CorrectionReason *string
	CreatedAt        time.Time
	// TargetStatus is "below", "within" or "above" the target valid when it was measured
	// (nil without a target or for superseded readings).
	TargetStatus *string
	// Warnings are soft hints of a likely unit mistake (Parameter.Warnings); never blocking.
	Warnings []string
}

// Test is a testing session with its readings.
type Test struct {
	ID         uuid.UUID
	TankID     uuid.UUID
	MeasuredAt time.Time
	Source     string
	Notes      *string
	CreatedAt  time.Time
	Readings   []Reading
}

// Target is one parameter's target, valid from ValidFrom until ValidTo (nil: current).
type Target struct {
	Parameter      string
	Min            *decimal.Decimal
	Ideal          *decimal.Decimal
	Max            *decimal.Decimal
	FreshnessHours *int32
	ValidFrom      time.Time
	ValidTo        *time.Time
}

func (t Target) status(v decimal.Decimal) *string {
	s := "within"
	switch {
	case t.Min != nil && v.Cmp(*t.Min) < 0:
		s = "below"
	case t.Max != nil && v.Cmp(*t.Max) > 0:
		s = "above"
	case t.Min == nil && t.Max == nil:
		return nil // only freshness or an ideal: no range to compare with
	}
	return &s
}

// targetSet indexes a tank's target history by parameter.
type targetSet map[string][]Target

func (ts targetSet) at(parameter string, t time.Time) *Target {
	list := ts[parameter]
	for i := len(list) - 1; i >= 0; i-- {
		tg := list[i]
		if !t.Before(tg.ValidFrom) && (tg.ValidTo == nil || t.Before(*tg.ValidTo)) {
			return &list[i]
		}
	}
	return nil
}

func (ts targetSet) current(parameter string) *Target {
	list := ts[parameter]
	if n := len(list); n > 0 && list[n-1].ValidTo == nil {
		return &list[n-1]
	}
	return nil
}

func loadTargets(ctx context.Context, q *dbq.Queries, tankID uuid.UUID) (targetSet, error) {
	rows, err := q.ListTargetHistory(ctx, tankID)
	if err != nil {
		return nil, err
	}
	ts := targetSet{}
	for _, r := range rows {
		ts[r.Parameter] = append(ts[r.Parameter], Target{
			Parameter: r.Parameter, Min: r.MinValue, Ideal: r.IdealValue, Max: r.MaxValue,
			FreshnessHours: r.FreshnessHours, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo,
		})
	}
	return ts, nil
}

// --- validation helpers ---------------------------------------------------------------------

type checker struct{ v *apperr.Validator }

func (c checker) text(field string, s *string, maxLen int, multiLine bool) *string {
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
		c.v.Add(field, "length", map[string]string{"max": strconv.Itoa(maxLen)})
	} else if !ok {
		c.v.Add(field, "invalid_characters", nil)
	}
	return &t
}

func (c checker) instant(field string, t, now time.Time) {
	switch {
	case t.Before(earliest):
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01T00:00:00Z"})
	case t.After(now.Add(maxFutureSkew)):
		c.v.Add(field, "in_future", nil)
	}
}

// value parses and converts a reading value; it reports errors under prefix (e.g.
// "readings[0]") and returns ok=false when the reading can't be stored.
func (c checker) value(prefix string, p *Parameter, unitCode, raw string) (in, canonical decimal.Decimal, u Unit, ok bool) {
	u, found := p.Unit(unitCode)
	if !found {
		codes := make([]string, len(p.Units))
		for i, x := range p.Units {
			codes[i] = x.Code
		}
		c.v.Add(prefix+".unit", "unsupported_unit", map[string]string{"allowed": strings.Join(codes, ",")})
		return in, canonical, u, false
	}
	in, err := decimal.Parse(strings.TrimSpace(raw))
	if err != nil {
		c.v.Add(prefix+".value", "invalid_decimal", nil)
		return in, canonical, u, false
	}
	if !in.FitsNumeric(12, 4) {
		c.v.Add(prefix+".value", "out_of_range", map[string]string{"max_integer_digits": "8", "max_decimals": "4"})
		return in, canonical, u, false
	}
	canonical, err = p.ToCanonical(u, in)
	if err != nil || !p.InBounds(canonical) {
		c.v.Add(prefix+".value", "out_of_range", map[string]string{
			"min": p.HardMin.String(), "max": p.HardMax.String(), "unit": p.CanonicalUnit,
		})
		return in, canonical, u, false
	}
	return in, canonical, u, true
}

func (c checker) method(field string, m *string) *string {
	if m == nil || strings.TrimSpace(*m) == "" {
		return nil
	}
	t := strings.TrimSpace(*m)
	c.v.Check(methods[t], field, "unsupported", map[string]string{
		"allowed": "titration,colorimetric,photometer,refractometer,hydrometer,probe,icp,other",
	})
	return &t
}

func stored(t time.Time) time.Time { return t.Truncate(time.Microsecond) }

// --- record a water test --------------------------------------------------------------------

// ReadingInput is one parameter in a new water test.
type ReadingInput struct {
	Parameter string
	Value     string
	Unit      string
	Method    *string
	TestKit   *string
}

// TestInput is a new water test (US-WP-1).
type TestInput struct {
	MeasuredAt *time.Time
	Notes      *string
	Readings   []ReadingInput
}

type parsedReading struct {
	p         *Parameter
	in, canon decimal.Decimal
	unit      Unit
	method    *string
	kit       *string
}

// Record stores a water test with one or more readings. A retry with the same
// Idempotency-Key returns the first test (created=false).
func (s *Service) Record(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in TestInput, key *string, requestHash []byte) (out Test, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	at := now
	if in.MeasuredAt != nil {
		c.instant("measured_at", *in.MeasuredAt, now)
		at = stored(*in.MeasuredAt)
	}
	notes := c.text("notes", in.Notes, 2000, true)
	if key != nil && !idempotency.Valid(*key) {
		v.Add(idempotency.Header, "invalid", map[string]string{"format": "8-128 printable ASCII characters"})
	}
	switch {
	case len(in.Readings) == 0:
		v.Add("readings", "required", nil)
	case len(in.Readings) > len(Catalog):
		v.Add("readings", "length", map[string]string{"max": strconv.Itoa(len(Catalog))})
	}
	parsed := make([]parsedReading, 0, len(in.Readings))
	seen := map[string]bool{}
	for i, r := range in.Readings {
		prefix := "readings[" + strconv.Itoa(i) + "]"
		p, ok := Lookup(r.Parameter)
		if !ok {
			v.Add(prefix+".parameter", "unsupported", nil)
			continue
		}
		if seen[p.Code] {
			v.Add(prefix+".parameter", "duplicate", nil)
			continue
		}
		seen[p.Code] = true
		inV, canon, u, ok := c.value(prefix, p, r.Unit, r.Value)
		method := c.method(prefix+".method", r.Method)
		kit := c.text(prefix+".test_kit", r.TestKit, 100, false)
		if ok {
			parsed = append(parsed, parsedReading{p: p, in: inV, canon: canon, unit: u, method: method, kit: kit})
		}
	}
	if err := v.Err(); err != nil {
		return Test{}, false, err
	}

	var testID uuid.UUID
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if key != nil { // a retry returns the first test even if the tank was archived since
			if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
				return err
			}
			if id, found, err := replay(ctx, q, a, *key, tankID, requestHash); err != nil || found {
				testID = id
				return err
			}
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		id, err := q.CreateWaterTest(ctx, dbq.CreateWaterTestParams{
			TankID: tankID, MeasuredAt: at, Notes: notes, RecordedBy: &a.UserID, IdempotencyKey: key,
			RequestHash: hashIf(key, requestHash), Now: now,
		})
		if database.IsNoRows(err) && key != nil { // lost a race with the same key
			first, found, err := replay(ctx, q, a, *key, tankID, requestHash)
			if err == nil && !found {
				err = errLostReplay
			}
			testID = first
			return err
		}
		if err != nil {
			return err
		}
		testID, created = id, true
		codes := make([]string, len(parsed))
		for i, r := range parsed {
			codes[i] = r.p.Code
			if _, err := q.CreateReading(ctx, dbq.CreateReadingParams{
				WaterTestID: id, TankID: tankID, Parameter: r.p.Code, MeasuredAt: at, InputValue: r.in,
				InputUnit: r.unit.Code, CanonicalValue: r.canon, Estimate: r.unit.Estimate, Method: r.method,
				TestKit: r.kit, RecordedBy: &a.UserID, Now: now,
			}); err != nil {
				return err
			}
		}
		_, err = timeline.Record(ctx, q, now, timeline.Event{
			TankID: tankID, OccurredAt: at, Category: timeline.CategoryMeasurement, EventType: "measurement.recorded",
			SourceType: "water_test", SourceID: id, ActorID: &a.UserID,
			Summary: map[string]any{"parameters": codes},
		})
		return err
	})
	if err != nil {
		return Test{}, false, err
	}
	out, err = s.loadTest(ctx, dbq.New(s.pool), testID)
	return out, created, err
}

func replay(ctx context.Context, q *dbq.Queries, a tanks.Actor, key string, tankID uuid.UUID, requestHash []byte) (uuid.UUID, bool, error) {
	first, err := q.GetWaterTestByIdempotencyKey(ctx, dbq.GetWaterTestByIdempotencyKeyParams{RecordedBy: &a.UserID, IdempotencyKey: &key})
	if database.IsNoRows(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	if first.TankID != tankID || !bytes.Equal(first.RequestHash, requestHash) {
		return uuid.Nil, false, errKeyReused
	}
	return first.ID, true, nil
}

func hashIf(key *string, h []byte) []byte {
	if key == nil {
		return nil
	}
	return h
}

// --- read tests -------------------------------------------------------------------------------

// GetTest returns one water test with all its readings (corrections included).
func (s *Service) GetTest(ctx context.Context, a tanks.Actor, id uuid.UUID) (Test, error) {
	q := dbq.New(s.pool)
	row, err := q.GetWaterTest(ctx, id)
	if database.IsNoRows(err) {
		return Test{}, apperr.NotFound()
	}
	if err != nil {
		return Test{}, err
	}
	if _, err := s.authz.Require(ctx, q, a.UserID, row.TankID, tanks.PermRead); err != nil {
		return Test{}, err
	}
	return s.loadTest(ctx, q, id)
}

func (s *Service) loadTest(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Test, error) {
	row, err := q.GetWaterTest(ctx, id)
	if err != nil {
		return Test{}, err
	}
	tests := []Test{{ID: row.ID, TankID: row.TankID, MeasuredAt: row.MeasuredAt, Source: row.Source, Notes: row.Notes, CreatedAt: row.CreatedAt}}
	if err := s.attachReadings(ctx, q, row.TankID, tests); err != nil {
		return Test{}, err
	}
	return tests[0], nil
}

func (s *Service) attachReadings(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, tests []Test) error {
	if len(tests) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(tests))
	index := make(map[uuid.UUID]int, len(tests))
	for i, t := range tests {
		ids[i] = t.ID
		index[t.ID] = i
	}
	rows, err := q.ListReadingsForTests(ctx, ids)
	if err != nil {
		return err
	}
	targets, err := loadTargets(ctx, q, tankID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		rd := Reading{
			ID: r.ID, WaterTestID: r.WaterTestID, Parameter: r.Parameter, MeasuredAt: r.MeasuredAt,
			InputValue: r.InputValue, InputUnit: r.InputUnit, CanonicalValue: r.CanonicalValue,
			Estimate: r.Estimate, Method: r.Method, TestKit: r.TestKit, Status: r.Status,
			SupersedesID: r.SupersedesID, CorrectionReason: r.CorrectionReason, CreatedAt: r.CreatedAt,
		}
		if p, ok := Lookup(r.Parameter); ok {
			rd.CanonicalUnit = p.CanonicalUnit
			rd.Warnings = p.Warnings(r.InputUnit, r.InputValue, r.CanonicalValue)
		}
		if r.Status == "valid" {
			if tg := targets.at(r.Parameter, r.MeasuredAt); tg != nil {
				rd.TargetStatus = tg.status(r.CanonicalValue)
			}
		}
		i := index[r.WaterTestID]
		tests[i].Readings = append(tests[i].Readings, rd)
	}
	return nil
}

// ListFilter selects water tests.
type ListFilter struct {
	Parameter string // "" for all
	From, To  *time.Time
	Limit     int32
	Cursor    *ListCursor
}

// ListCursor is the keyset of the test list.
type ListCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// List returns the tank's water tests, newest first.
func (s *Service) List(ctx context.Context, a tanks.Actor, tankID uuid.UUID, f ListFilter) ([]Test, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	if f.Parameter != "" {
		if _, ok := Lookup(f.Parameter); !ok {
			return nil, apperr.BadRequest("invalid_query", "Unknown parameter.",
				apperr.FieldError{Field: "parameter", Code: "unsupported"})
		}
	}
	from, to := minTime, maxTime
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}
	cursorAt, cursorID := maxTime, uuid.Max
	if f.Cursor != nil {
		cursorAt, cursorID = f.Cursor.At, f.Cursor.ID
	}
	rows, err := q.ListWaterTests(ctx, dbq.ListWaterTestsParams{
		TankID: tankID, FromAt: from, ToAt: to, CursorAt: cursorAt, CursorID: cursorID, Parameter: f.Parameter,
		RowLimit: f.Limit,
	})
	if err != nil {
		return nil, err
	}
	tests := make([]Test, len(rows))
	for i, r := range rows {
		tests[i] = Test{ID: r.ID, TankID: r.TankID, MeasuredAt: r.MeasuredAt, Source: r.Source, Notes: r.Notes, CreatedAt: r.CreatedAt}
	}
	return tests, s.attachReadings(ctx, q, tankID, tests)
}

// --- corrections ------------------------------------------------------------------------------

// CorrectionInput replaces a reading's value (US-WP-2).
type CorrectionInput struct {
	Value  string
	Unit   string
	Reason *string
}

// Correct inserts a new reading that supersedes readingID; the original stays as history.
func (s *Service) Correct(ctx context.Context, a tanks.Actor, readingID uuid.UUID, in CorrectionInput) (Reading, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	reason := c.text("reason", in.Reason, 500, false)
	if reason == nil {
		v.Add("reason", "required", nil)
	}
	var testID uuid.UUID
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tid, err := q.GetReadingTank(ctx, readingID)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tid, tanks.PermWrite); err != nil {
			return err
		}
		old, err := q.LockReading(ctx, readingID)
		if err != nil {
			return err
		}
		if old.Status != "valid" {
			return errSuperseded
		}
		p, known := Lookup(old.Parameter)
		if !known { // TestCatalogMatchesDatabase keeps the catalog and parameter_types in sync
			return fmt.Errorf("correct reading: parameter %q missing from the catalog", old.Parameter)
		}
		inV, canon, u, ok := c.value("", p, in.Unit, in.Value)
		if err := v.Err(); err != nil || !ok {
			if err == nil {
				err = v.Err()
			}
			return err
		}
		if _, err := q.SupersedeReading(ctx, readingID); err != nil {
			return err
		}
		id, err := q.CreateReading(ctx, dbq.CreateReadingParams{
			WaterTestID: old.WaterTestID, TankID: old.TankID, Parameter: old.Parameter, MeasuredAt: old.MeasuredAt,
			InputValue: inV, InputUnit: u.Code, CanonicalValue: canon, Estimate: u.Estimate, Method: old.Method,
			TestKit: old.TestKit, SupersedesID: &readingID, CorrectionReason: reason, RecordedBy: &a.UserID, Now: now,
		})
		if err != nil {
			return err
		}
		testID = old.WaterTestID
		_, err = timeline.Record(ctx, q, now, timeline.Event{
			TankID: old.TankID, OccurredAt: old.MeasuredAt, Category: timeline.CategoryMeasurement,
			EventType: "measurement.corrected", SourceType: "water_test_reading", SourceID: id, ActorID: &a.UserID,
			Summary: map[string]any{"parameter": old.Parameter},
		})
		readingID = id
		return err
	})
	if err != nil {
		return Reading{}, fixFieldPrefix(err)
	}
	t, err := s.loadTest(ctx, dbq.New(s.pool), testID)
	if err != nil {
		return Reading{}, err
	}
	for _, r := range t.Readings {
		if r.ID == readingID {
			return r, nil
		}
	}
	return Reading{}, errors.New("waterparameters: corrected reading not found")
}

// fixFieldPrefix turns ".value"/".unit" (from an empty prefix) into "value"/"unit".
func fixFieldPrefix(err error) error {
	e, ok := apperr.As(err)
	if !ok || e.Kind != apperr.KindValidation {
		return err
	}
	for i := range e.Fields {
		e.Fields[i].Field = strings.TrimPrefix(e.Fields[i].Field, ".")
	}
	return e
}

// --- targets ------------------------------------------------------------------------------------

// TargetInput is one parameter's target in canonical units (US-WP-4).
type TargetInput struct {
	Parameter      string
	Min            *string
	Ideal          *string
	Max            *string
	FreshnessHours *int32
}

// Targets returns the tank's current targets in catalog order.
func (s *Service) Targets(ctx context.Context, a tanks.Actor, tankID uuid.UUID) ([]Target, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	ts, err := loadTargets(ctx, q, tankID)
	if err != nil {
		return nil, err
	}
	out := []Target{}
	for _, p := range Catalog {
		if t := ts.current(p.Code); t != nil {
			out = append(out, *t)
		}
	}
	return out, nil
}

type parsedTarget struct {
	p              *Parameter
	min, ideal, mx *decimal.Decimal
	fresh          *int32
}

func (c checker) bound(field string, p *Parameter, s *string) *decimal.Decimal {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	d, err := decimal.Parse(strings.TrimSpace(*s))
	if err != nil {
		c.v.Add(field, "invalid_decimal", nil)
		return nil
	}
	if !p.InBounds(d) || !d.FitsNumeric(12, 4) {
		c.v.Add(field, "out_of_range", map[string]string{"min": p.HardMin.String(), "max": p.HardMax.String(), "unit": p.CanonicalUnit})
		return nil
	}
	return &d
}

// ReplaceTargets sets the complete target list: listed parameters get a new version when
// they changed, unlisted ones lose their target. History stays (each reading keeps being
// compared with the target valid when it was measured).
func (s *Service) ReplaceTargets(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in []TargetInput) ([]Target, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	want := map[string]parsedTarget{}
	if len(in) > len(Catalog) {
		v.Add("targets", "length", map[string]string{"max": strconv.Itoa(len(Catalog))})
		return nil, v.Err()
	}
	for i, t := range in {
		prefix := "targets[" + strconv.Itoa(i) + "]"
		p, ok := Lookup(t.Parameter)
		if !ok {
			v.Add(prefix+".parameter", "unsupported", nil)
			continue
		}
		if _, dup := want[p.Code]; dup {
			v.Add(prefix+".parameter", "duplicate", nil)
			continue
		}
		pt := parsedTarget{p: p, min: c.bound(prefix+".min", p, t.Min), ideal: c.bound(prefix+".ideal", p, t.Ideal), mx: c.bound(prefix+".max", p, t.Max), fresh: t.FreshnessHours}
		if pt.fresh != nil && (*pt.fresh < 1 || *pt.fresh > 8760) {
			v.Add(prefix+".freshness_hours", "out_of_range", map[string]string{"min": "1", "max": "8760"})
		}
		if pt.min != nil && pt.mx != nil && pt.min.Cmp(*pt.mx) > 0 {
			v.Add(prefix+".max", "below_min", nil)
		}
		if pt.ideal != nil && ((pt.min != nil && pt.ideal.Cmp(*pt.min) < 0) || (pt.mx != nil && pt.ideal.Cmp(*pt.mx) > 0)) {
			v.Add(prefix+".ideal", "outside_range", nil)
		}
		if pt.min == nil && pt.ideal == nil && pt.mx == nil && pt.fresh == nil {
			v.Add(prefix, "required", nil)
		}
		want[p.Code] = pt
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		// Lock before Require: Require takes FOR SHARE, and upgrading it would deadlock two
		// concurrent replacements (db/queries/tanks.sql LockTankRow). An unknown tank locks
		// nothing and Require answers 404.
		if err := q.LockTankRow(ctx, tankID); err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		ts, err := loadTargets(ctx, q, tankID)
		if err != nil {
			return err
		}
		for _, p := range Catalog {
			cur := ts.current(p.Code)
			next, listed := want[p.Code]
			if cur != nil && listed && sameTarget(*cur, next) {
				continue
			}
			if cur != nil {
				if _, err := q.CloseTarget(ctx, dbq.CloseTargetParams{Now: now, TankID: tankID, Parameter: p.Code}); err != nil {
					return err
				}
			}
			if listed {
				if err := q.CreateTarget(ctx, dbq.CreateTargetParams{
					TankID: tankID, Parameter: p.Code, MinValue: next.min, IdealValue: next.ideal, MaxValue: next.mx,
					FreshnessHours: next.fresh, Now: now, CreatedBy: &a.UserID,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Targets(ctx, a, tankID)
}

func sameTarget(cur Target, next parsedTarget) bool {
	eq := func(a, b *decimal.Decimal) bool { return (a == nil) == (b == nil) && (a == nil || a.Cmp(*b) == 0) }
	fe := (cur.FreshnessHours == nil) == (next.fresh == nil) && (cur.FreshnessHours == nil || *cur.FreshnessHours == *next.fresh)
	return eq(cur.Min, next.min) && eq(cur.Ideal, next.ideal) && eq(cur.Max, next.mx) && fe
}

// --- latest and trends -----------------------------------------------------------------------

// Latest is one parameter's newest reading and freshness (US-WP-5).
type Latest struct {
	Parameter      string
	CanonicalUnit  string
	Reading        *Reading
	FreshnessHours int
	Stale          bool
	Target         *Target
}

// LatestReadings returns every catalog parameter with its newest valid reading (if any), its
// freshness window (the tank's target override or the catalog default) and the current target.
func (s *Service) LatestReadings(ctx context.Context, a tanks.Actor, tankID uuid.UUID) ([]Latest, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.LatestValidReadings(ctx, tankID)
	if err != nil {
		return nil, err
	}
	ts, err := loadTargets(ctx, q, tankID)
	if err != nil {
		return nil, err
	}
	byParam := make(map[string]dbq.LatestValidReadingsRow, len(rows))
	for _, r := range rows {
		byParam[r.Parameter] = r
	}
	now := s.clock.Now()
	out := make([]Latest, 0, len(Catalog))
	for _, p := range Catalog {
		l := Latest{Parameter: p.Code, CanonicalUnit: p.CanonicalUnit, FreshnessHours: p.FreshnessHours, Target: ts.current(p.Code)}
		if l.Target != nil && l.Target.FreshnessHours != nil {
			l.FreshnessHours = int(*l.Target.FreshnessHours)
		}
		if r, ok := byParam[p.Code]; ok {
			rd := Reading{
				ID: r.ID, Parameter: r.Parameter, MeasuredAt: r.MeasuredAt, InputValue: r.InputValue,
				InputUnit: r.InputUnit, CanonicalValue: r.CanonicalValue, CanonicalUnit: p.CanonicalUnit,
				Estimate: r.Estimate, Status: "valid", Warnings: p.Warnings(r.InputUnit, r.InputValue, r.CanonicalValue),
			}
			// "Is it fine now?": the latest value is compared with the current target, unlike
			// history, where each reading keeps the target in effect when it was measured.
			if l.Target != nil {
				rd.TargetStatus = l.Target.status(r.CanonicalValue)
			}
			l.Reading = &rd
			l.Stale = now.Sub(r.MeasuredAt) > time.Duration(l.FreshnessHours)*time.Hour
		} else {
			l.Stale = true
		}
		out = append(out, l)
	}
	return out, nil
}

// Point is one trend value.
type Point struct {
	At        time.Time
	Value     decimal.Decimal
	ReadingID uuid.UUID
	Estimate  bool
}

// Series is one parameter's trend with the targets valid during the range.
type Series struct {
	Parameter     string
	CanonicalUnit string
	Points        []Point
	Targets       []Target
	Truncated     bool // more than maxTrendPoints in range: only the newest are returned
}

// Annotation marks a dosing, water-change or equipment event on the chart.
type Annotation struct {
	At        time.Time
	Category  string
	EventType string
	Summary   json.RawMessage
}

// Trends is the response of the trend endpoint (US-WP-3).
type Trends struct {
	From, To    time.Time
	Series      []Series
	Annotations []Annotation
	// AnnotationsTruncated: more than maxAnnotations events in range; the newest are kept.
	AnnotationsTruncated bool
}

// TrendsFor returns valid readings in canonical units for the parameters in [from, to).
func (s *Service) TrendsFor(ctx context.Context, a tanks.Actor, tankID uuid.UUID, params []string, from, to *time.Time) (Trends, error) {
	now := s.clock.Now()
	end := now.Add(maxFutureSkew)
	if to != nil {
		end = *to
	}
	start := end.Add(-defaultTrend)
	if from != nil {
		start = *from
	}
	v := apperr.NewValidator()
	if len(params) == 0 {
		v.Add("parameters", "required", nil)
	}
	seen := map[string]bool{}
	for _, p := range params {
		if _, ok := Lookup(p); !ok {
			v.Add("parameters", "unsupported", map[string]string{"value": p})
		}
		seen[p] = true
	}
	if !start.Before(end) {
		v.Add("from", "before_to", nil)
	} else if end.Sub(start) > maxTrendRange {
		v.Add("from", "range_too_long", map[string]string{"max_days": "732"})
	}
	if err := v.Err(); err != nil {
		e, _ := apperr.As(err)
		return Trends{}, apperr.BadRequest("invalid_query", "Invalid trend query.", e.Fields...)
	}
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return Trends{}, err
	}
	codes := make([]string, 0, len(seen))
	for _, p := range Catalog {
		if seen[p.Code] {
			codes = append(codes, p.Code)
		}
	}
	rows, err := q.ListValidReadingsInRange(ctx, dbq.ListValidReadingsInRangeParams{
		TankID: tankID, Parameters: codes, FromAt: start, ToAt: end, PerParameter: maxTrendPoints + 1,
	})
	if err != nil {
		return Trends{}, err
	}
	out := Trends{From: start, To: end}
	ts, err := loadTargets(ctx, q, tankID)
	if err != nil {
		return Trends{}, err
	}
	for _, code := range codes {
		p, _ := Lookup(code)
		sr := Series{Parameter: code, CanonicalUnit: p.CanonicalUnit, Points: []Point{}, Targets: []Target{}}
		for _, r := range rows { // newest first
			if r.Parameter != code {
				continue
			}
			if len(sr.Points) == maxTrendPoints {
				sr.Truncated = true
				break
			}
			sr.Points = append(sr.Points, Point{At: r.MeasuredAt, Value: r.CanonicalValue, ReadingID: r.ID, Estimate: r.Estimate})
		}
		slices.Reverse(sr.Points)
		for _, t := range ts[code] {
			if t.ValidFrom.Before(end) && (t.ValidTo == nil || t.ValidTo.After(start)) {
				sr.Targets = append(sr.Targets, t)
			}
		}
		out.Series = append(out.Series, sr)
	}
	ann, err := q.ListTankEventsForAnnotations(ctx, dbq.ListTankEventsForAnnotationsParams{
		TankID: tankID, Categories: annotationTypes, FromAt: start, ToAt: end, RowLimit: maxAnnotations + 1,
	})
	if err != nil {
		return Trends{}, err
	}
	if len(ann) > maxAnnotations { // newest first: drop the oldest
		ann, out.AnnotationsTruncated = ann[:maxAnnotations], true
	}
	slices.Reverse(ann)
	out.Annotations = make([]Annotation, len(ann))
	for i, e := range ann {
		out.Annotations[i] = Annotation{At: e.OccurredAt, Category: e.Category, EventType: e.EventType, Summary: e.Summary}
	}
	return out, nil
}

// --- erasure --------------------------------------------------------------------------------

// EraseUserData deletes the erased account's water tests, readings and targets
// (identity.Eraser); it must run before tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.DeleteOwnerReadings(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerWaterTests(ctx, userID); err != nil {
		return err
	}
	return q.DeleteOwnerTargets(ctx, userID)
}
