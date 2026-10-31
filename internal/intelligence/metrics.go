package intelligence

import (
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/platform/decimal"
	"reefcare/internal/waterparameters"
)

// Metrics are pure functions of a Snapshot (docs/knowledge-engine.md §3 step 4). Decimal
// arithmetic is exact; divisions round to 6 places and are only used for derived values.

const (
	day        = 24 * time.Hour
	divPlaces  = 6
	minTrendPt = 3
)

var (
	zero      = decimal.MustParse("0")
	secPerDay = decimal.MustParse("86400")
)

// ParameterMetrics summarizes one parameter's readings.
type ParameterMetrics struct {
	Parameter string
	Unit      string
	Count     int // readings in the snapshot
	Latest    *Reading
	Previous  *Reading
	// Age of the latest reading; Fresh when within the freshness window (the target's
	// freshness_hours, else the catalog default).
	Age            time.Duration
	FreshnessHours int
	Fresh          bool
	// DeltaFromPrevious = latest − previous; DeltaPerDay divides by the days between them.
	DeltaFromPrevious *decimal.Decimal
	DeltaPerDay       *decimal.Decimal
	DaysFromPrevious  *decimal.Decimal
	Window7           Window
	Window30          Window
	// SlopePerDay is the least-squares slope over the last 30 days (≥ 3 points), and the
	// span of days it covers.
	SlopePerDay *decimal.Decimal
	SlopeSpan   *decimal.Decimal
	SlopePoints int
	// MaxStepChange is the largest |change| between consecutive readings in the last 7
	// days (a "day-to-day" swing when readings are daily).
	MaxStepChange *decimal.Decimal
	// Target valid when the latest reading was taken, and the latest's status against it.
	Target       *Target
	TargetStatus string // below | within | above | "" (no range)
}

// Window is a rolling window's statistics (nil fields without readings).
type Window struct {
	Count int
	Mean  *decimal.Decimal
	Min   *decimal.Decimal
	Max   *decimal.Decimal
	Range *decimal.Decimal
}

// byTime returns the readings of parameter in time order (oldest first; ties by id).
func byTime(s Snapshot, parameter string) []Reading {
	var out []Reading
	for _, r := range s.Readings {
		if r.Parameter == parameter && !r.MeasuredAt.After(s.Now) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b Reading) int {
		if c := a.MeasuredAt.Compare(b.MeasuredAt); c != 0 {
			return c
		}
		return slices.Compare(a.ID[:], b.ID[:])
	})
	return out
}

// targetAt is the target of parameter valid at t.
func targetAt(s Snapshot, parameter string, t time.Time) *Target {
	for i := range s.Targets {
		tg := &s.Targets[i]
		if tg.Parameter == parameter && !t.Before(tg.ValidFrom) && (tg.ValidTo == nil || t.Before(*tg.ValidTo)) {
			return tg
		}
	}
	return nil
}

// targetStatus compares v with the target's range.
func targetStatus(tg *Target, v decimal.Decimal) string {
	switch {
	case tg == nil || (tg.Min == nil && tg.Max == nil):
		return ""
	case tg.Min != nil && v.Cmp(*tg.Min) < 0:
		return "below"
	case tg.Max != nil && v.Cmp(*tg.Max) > 0:
		return "above"
	default:
		return "within"
	}
}

// cyclingFreshnessHours: while a tank cycles, ammonia and nitrite change by the day.
const cyclingFreshnessHours = 72

// Cycling reports a tank marked cycling or set up less than cyclingAge ago.
func Cycling(s Snapshot) bool {
	return s.TankStatus == "cycling" || (s.SetupDate != nil && s.Now.Sub(*s.SetupDate) < cyclingAge)
}

// freshnessHours is the target's window, else the catalog default (0 for unknown parameters);
// ammonia and nitrite of a cycling tank go stale after 3 days.
func freshnessHours(s Snapshot, parameter string) int {
	if tg := targetAt(s, parameter, s.Now); tg != nil && tg.FreshnessHours != nil {
		return int(*tg.FreshnessHours)
	}
	if (parameter == "ammonia" || parameter == "nitrite") && Cycling(s) {
		return cyclingFreshnessHours
	}
	if p, ok := waterparameters.Lookup(parameter); ok {
		return p.FreshnessHours
	}
	return 0
}

func days(d time.Duration) decimal.Decimal {
	out, _ := decimal.MustParse(itoa(int64(d/time.Second))).DivRound(secPerDay, divPlaces)
	return out
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func ptr(d decimal.Decimal) *decimal.Decimal { return &d }

func abs(d decimal.Decimal) decimal.Decimal {
	if d.Sign() < 0 {
		return zero.Sub(d)
	}
	return d
}

// window computes statistics over readings measured in [from, now].
func window(rs []Reading, from time.Time) Window {
	var w Window
	sum := zero
	for _, r := range rs {
		if r.MeasuredAt.Before(from) {
			continue
		}
		w.Count++
		sum = sum.Add(r.Value)
		if w.Min == nil || r.Value.Cmp(*w.Min) < 0 {
			w.Min = ptr(r.Value)
		}
		if w.Max == nil || r.Value.Cmp(*w.Max) > 0 {
			w.Max = ptr(r.Value)
		}
	}
	if w.Count > 0 {
		mean, _ := sum.DivRound(decimal.MustParse(itoa(int64(w.Count))), divPlaces)
		w.Mean = &mean
		w.Range = ptr(w.Max.Sub(*w.Min))
	}
	return w
}

// slope is the least-squares slope (value per day) of the points, with x in days since the
// first point: (nΣxy − ΣxΣy) / (nΣx² − (Σx)²). Nil with fewer than minTrendPt points or
// when every point has the same time.
func slope(rs []Reading) (*decimal.Decimal, *decimal.Decimal) {
	if len(rs) < minTrendPt {
		return nil, nil
	}
	t0 := rs[0].MeasuredAt
	n := decimal.MustParse(itoa(int64(len(rs))))
	sx, sy, sxy, sxx := zero, zero, zero, zero
	for _, r := range rs {
		x := days(r.MeasuredAt.Sub(t0))
		sx, sy = sx.Add(x), sy.Add(r.Value)
		sxy, sxx = sxy.Add(x.Mul(r.Value)), sxx.Add(x.Mul(x))
	}
	den := n.Mul(sxx).Sub(sx.Mul(sx))
	if den.Sign() == 0 {
		return nil, nil
	}
	s, err := n.Mul(sxy).Sub(sx.Mul(sy)).DivRound(den, divPlaces)
	if err != nil {
		return nil, nil
	}
	return &s, ptr(days(rs[len(rs)-1].MeasuredAt.Sub(t0)))
}

// MetricsFor computes the metrics of one parameter.
func MetricsFor(s Snapshot, parameter string) ParameterMetrics {
	rs := byTime(s, parameter)
	m := ParameterMetrics{Parameter: parameter, Count: len(rs), FreshnessHours: freshnessHours(s, parameter)}
	if p, ok := waterparameters.Lookup(parameter); ok {
		m.Unit = p.CanonicalUnit
	}
	if len(rs) == 0 {
		return m
	}
	latest := rs[len(rs)-1]
	m.Latest = &latest
	m.Age = s.Now.Sub(latest.MeasuredAt)
	m.Fresh = m.FreshnessHours > 0 && m.Age <= time.Duration(m.FreshnessHours)*time.Hour
	m.Target = targetAt(s, parameter, latest.MeasuredAt)
	m.TargetStatus = targetStatus(m.Target, latest.Value)
	if len(rs) >= 2 {
		prev := rs[len(rs)-2]
		m.Previous = &prev
		m.DeltaFromPrevious = ptr(latest.Value.Sub(prev.Value))
		if gap := days(latest.MeasuredAt.Sub(prev.MeasuredAt)); gap.Sign() > 0 {
			m.DaysFromPrevious = &gap
			if per, err := m.DeltaFromPrevious.DivRound(gap, divPlaces); err == nil {
				m.DeltaPerDay = &per
			}
		}
	}
	m.Window7 = window(rs, s.Now.Add(-7*day))
	m.Window30 = window(rs, s.Now.Add(-30*day))

	var last30 []Reading
	for _, r := range rs {
		if !r.MeasuredAt.Before(s.Now.Add(-30 * day)) {
			last30 = append(last30, r)
		}
	}
	m.SlopePerDay, m.SlopeSpan = slope(last30)
	if m.SlopePerDay != nil {
		m.SlopePoints = len(last30)
	}

	for i := 1; i < len(rs); i++ {
		if rs[i].MeasuredAt.Before(s.Now.Add(-7 * day)) {
			continue
		}
		if step := abs(rs[i].Value.Sub(rs[i-1].Value)); m.MaxStepChange == nil || step.Cmp(*m.MaxStepChange) > 0 {
			m.MaxStepChange = &step
		}
	}
	return m
}

// AllMetrics computes metrics for every catalog parameter, in catalog order.
func AllMetrics(s Snapshot) []ParameterMetrics {
	out := make([]ParameterMetrics, 0, len(waterparameters.Catalog))
	for _, p := range waterparameters.Catalog {
		out = append(out, MetricsFor(s, p.Code))
	}
	return out
}

// DosingAdherence counts manual-plan occurrences scheduled in the window. Pump plans are
// left out: ReefCare can't know whether a pump delivered ("unconfirmed" is not a miss).
type DosingAdherence struct {
	Due       int // scheduled in [from, now) and past their time
	Completed int // completed or partial
	Missed    int // missed, skipped or failed
	Open      int // still scheduled (inside their grace)
	// Ratio = Completed / (Due − Open); nil when nothing was resolvable.
	Ratio *decimal.Decimal
}

// adherenceEvents are the occurrences adherence is measured on: active manual plans,
// scheduled in [now − window, now), with a known status. Pump plans are left out (ReefCare
// can't know whether a pump delivered).
func adherenceEvents(s Snapshot, window time.Duration) []DosingEvent {
	manual := map[uuid.UUID]bool{}
	for _, p := range s.DosingPlans {
		if p.Delivery != "dosing_pump" && p.Status == "active" {
			manual[p.ID] = true
		}
	}
	from := s.Now.Add(-window)
	var out []DosingEvent
	for _, e := range s.DosingEvents {
		if e.PlanID == nil || e.ScheduledFor == nil || !manual[*e.PlanID] ||
			e.ScheduledFor.Before(from) || !e.ScheduledFor.Before(s.Now) {
			continue
		}
		switch e.Status {
		case "completed", "partial", "missed", "skipped", "failed", "scheduled":
			out = append(out, e)
		}
	}
	return out
}

// AdherenceFor computes dosing adherence over the last `window`.
func AdherenceFor(s Snapshot, window time.Duration) DosingAdherence {
	var a DosingAdherence
	for _, e := range adherenceEvents(s, window) {
		a.Due++
		switch e.Status {
		case "completed", "partial":
			a.Completed++
		case "missed", "skipped", "failed":
			a.Missed++
		case "scheduled":
			a.Open++
		}
	}
	if resolved := a.Due - a.Open; resolved > 0 {
		r, _ := decimal.MustParse(itoa(int64(a.Completed))).DivRound(decimal.MustParse(itoa(int64(resolved))), 4)
		a.Ratio = &r
	}
	return a
}

// WaterChangeCadence summarizes recent water changes.
type WaterChangeCadence struct {
	Count30       int
	LastAt        *time.Time
	DaysSinceLast *decimal.Decimal
	// ReplacedFraction30 is Σ(volume / system volume) over 30 days: an estimate (assumes the
	// recorded system volume and no dilution between changes), never an exact turnover.
	ReplacedFraction30 *decimal.Decimal
}

// CadenceFor summarizes water changes.
func CadenceFor(s Snapshot) WaterChangeCadence {
	var c WaterChangeCadence
	frac := zero
	from := s.Now.Add(-30 * day)
	for _, w := range s.WaterChanges {
		if w.CompletedAt.After(s.Now) {
			continue
		}
		if c.LastAt == nil || w.CompletedAt.After(*c.LastAt) {
			at := w.CompletedAt
			c.LastAt = &at
		}
		if !w.CompletedAt.Before(from) {
			c.Count30++
			if w.SystemVolumeL.Sign() > 0 {
				f, _ := w.VolumeL.DivRound(w.SystemVolumeL, divPlaces)
				frac = frac.Add(f)
			}
		}
	}
	if c.LastAt != nil {
		c.DaysSinceLast = ptr(days(s.Now.Sub(*c.LastAt)))
	}
	if c.Count30 > 0 {
		c.ReplacedFraction30 = &frac
	}
	return c
}

// LivestockSummary counts livestock events in a window, following the loss definitions in
// docs/knowledge-engine.md §5: only "died" is a death; "missing" is unaccounted for; sold
// and transferred_out are not losses; a negative reconciliation is a count correction.
type LivestockSummary struct {
	Deaths           int // died events
	DeathCount       int // individuals
	Missing          int // individuals
	CountCorrections int // negative reconciliations (individuals)
	Arrivals         int // added + transferred_in events (from outside the system)
	Deaths7          []LivestockEvent
}

// LivestockFor summarizes livestock events in [now−window, now].
func LivestockFor(s Snapshot, window time.Duration) LivestockSummary {
	var l LivestockSummary
	from := s.Now.Add(-window)
	for _, e := range s.LivestockEvents {
		if e.OccurredAt.Before(from) || e.OccurredAt.After(s.Now) {
			continue
		}
		switch e.Type {
		case "died":
			l.Deaths++
			l.DeathCount += int(-e.QuantityDelta)
			l.Deaths7 = append(l.Deaths7, e)
		case "missing":
			l.Missing += int(-e.QuantityDelta)
		case "reconciliation":
			if e.QuantityDelta < 0 {
				l.CountCorrections += int(-e.QuantityDelta)
			}
		case "added", "transferred_in":
			l.Arrivals++
		}
	}
	return l
}
