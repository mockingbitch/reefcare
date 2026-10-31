package intelligence

import (
	"slices"
	"time"

	"reefcare/internal/platform/decimal"
)

// Rule set v1.0.0 (docs/knowledge-engine.md §5). Every threshold below is PROVISIONAL
// (decision D5): hobby defaults reviewed by the reef-domain-expert agent, not yet by the
// named human reviewer. Thresholds live here, versioned with RuleVersion, never in user
// config. All values are in canonical units (internal/waterparameters/catalog.go).

// tolerance is how far two readings of a parameter may differ and still agree, given hobby
// test-kit resolution: an absolute floor and, for some, a share of the value.
type tolerance struct {
	abs decimal.Decimal
	rel decimal.Decimal // fraction of the larger value; 0 = none
}

var tolerances = map[string]tolerance{
	"alkalinity":  {abs: decimal.MustParse("0.5")},
	"calcium":     {abs: decimal.MustParse("25")},
	"magnesium":   {abs: decimal.MustParse("100")},
	"ph":          {abs: decimal.MustParse("0.25")},
	"temperature": {abs: decimal.MustParse("1")},
	"salinity":    {abs: decimal.MustParse("1.5")}, // refractometers resolve ≈ 0.001 SG ≈ 1.3 ppt
	"nitrate":     {abs: decimal.MustParse("2.5"), rel: decimal.MustParse("0.2")},
	"phosphate":   {abs: decimal.MustParse("0.05"), rel: decimal.MustParse("0.15")},
	"ammonia":     {abs: decimal.MustParse("0.1")},
	"nitrite":     {abs: decimal.MustParse("0.1")},
}

// estimateSalinityTolerance applies when either reading was converted from SG.
var estimateSalinityTolerance = decimal.MustParse("2")

func toleranceFor(param string, a, b Reading) decimal.Decimal {
	tl := tolerances[param]
	out := tl.abs
	if param == "salinity" && (a.Estimate || b.Estimate) {
		out = estimateSalinityTolerance
	}
	if tl.rel.Sign() > 0 {
		hi := a.Value
		if b.Value.Cmp(hi) > 0 {
			hi = b.Value
		}
		if rel := abs(hi).Mul(tl.rel); rel.Cmp(out) > 0 {
			out = rel
		}
	}
	return out
}

const conflictWindow = 2 * time.Hour

// fastChangePerDay: a change faster than this (per day, over at least a day) between readings
// ≤ 7 days apart is unusual; it may be a test error or a real event, so it is confirmed by a
// re-test before anything else is concluded. Temperature is left out: a fast temperature
// change is usually real (heater or chiller) and WB-WELFARE-BOUNDS speaks for it.
var fastChangePerDay = map[string]decimal.Decimal{
	"alkalinity": decimal.MustParse("1.5"), "calcium": decimal.MustParse("50"), "magnesium": decimal.MustParse("150"),
	"ph": decimal.MustParse("1"), "salinity": decimal.MustParse("2"),
}

// Ammonia / nitrite tiers: "detected" asks for a re-test and a look; "urgent" means act now.
var (
	nhDetected = map[string]decimal.Decimal{"ammonia": decimal.MustParse("0.05"), "nitrite": decimal.MustParse("0.1")}
	nhUrgent   = map[string]decimal.Decimal{
		"ammonia": decimal.MustParse("0.25"), // mg/L as NH3 (total ammonia; free NH3 is a few % at reef pH)
		"nitrite": decimal.MustParse("1"),    // mg/L as NO2
	}
)

// bound is a fixed welfare bound, applied with or without the keeper's targets.
type bound struct {
	param        string
	urgentBelow  *decimal.Decimal
	urgentAbove  *decimal.Decimal
	warnBelow    *decimal.Decimal
	warnAbove    *decimal.Decimal
	reefOnly     bool
	intentionLow bool // a low value may be intentional (hyposalinity treatment in fish-only tanks)
}

func dp(s string) *decimal.Decimal { v := decimal.MustParse(s); return &v }

var welfareBounds = []bound{
	{param: "temperature", urgentBelow: dp("20"), urgentAbove: dp("30"), warnAbove: dp("28.5")},
	{param: "salinity", urgentBelow: dp("28"), urgentAbove: dp("40"), intentionLow: true},
	{param: "ph", urgentBelow: dp("7.5"), urgentAbove: dp("8.7")},
	{param: "alkalinity", warnBelow: dp("5"), warnAbove: dp("14"), reefOnly: true},
}

var (
	minAdherence    = decimal.MustParse("0.8")
	trendHorizon    = decimal.MustParse("7") // days ahead
	minTrendSpan    = decimal.MustParse("5") // days covered by the trend
	trendWindow     = 14 * day
	lossLink        = 72 * time.Hour // a loss this soon after a change is reported as coinciding
	cyclingAge      = 60 * day       // a tank younger than this may still be cycling
	recentWindow    = 30 * day       // data-quality findings look at the last 30 days
	minAdherenceDue = 3
)

func reefType(s Snapshot) bool { return s.TankType != "fish_only" && s.TankType != "fowlr" }

// coreParameters are the parameters every keeper of this tank type is expected to test
// ("custom" tanks are treated as reef tanks).
func coreParameters(c *context) []string {
	s := c.s
	core := []string{"salinity", "temperature", "nitrate", "phosphate"}
	if reefType(s) {
		core = append([]string{"alkalinity", "calcium", "magnesium", "ph"}, core...)
	} else {
		core = append(core, "ammonia") // fish loads and quarantine make it relevant
	}
	for _, p := range []string{"ammonia", "nitrite"} {
		elevated := c.metrics[p].Latest != nil && c.metrics[p].Latest.Value.Cmp(nhDetected[p]) >= 0
		if (Cycling(s) || elevated) && !slices.Contains(core, p) {
			core = append(core, p) // until a clean reading exists
		}
	}
	return core
}

func reading(r *Reading) EvidenceRef { return EvidenceRef{Kind: "reading", ID: r.ID} }

func p(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func t(key string, kv ...string) Text { return Text{Key: key, Params: p(kv...)} }

func retest(param string) Verification {
	return Verification{Text: t("verify.retest", "parameter", param), Parameter: param}
}

// correctionWarning: temperature is fixed promptly (heater/chiller); chemistry slowly.
func correctionWarning(param string) SafetyWarning {
	if param == "temperature" {
		return SafetyWarning{Text: t("safety.check_heater_chiller")}
	}
	return SafetyWarning{Text: t("safety.no_fast_corrections", "parameter", param)}
}

// context precomputes what several rules share.
type context struct {
	s          Snapshot
	metrics    map[string]ParameterMetrics
	suspect    map[string]bool // latest reading changed unusually fast: confirm first
	conflicts  map[string]bool // latest readings disagree
	waterAlarm bool            // a welfare or ammonia/nitrite finding fired
}

func newContext(s Snapshot) *context {
	c := &context{s: s, metrics: map[string]ParameterMetrics{}, suspect: map[string]bool{}, conflicts: map[string]bool{}}
	for _, m := range AllMetrics(s) {
		c.metrics[m.Parameter] = m
	}
	return c
}

// doubtful reports a parameter whose latest readings changed unusually fast or disagree:
// instability rules wait for a re-test, and other findings about it have low confidence.
func (c *context) doubtful(param string) bool { return c.suspect[param] || c.conflicts[param] }

// rule is one versioned rule.
type rule struct {
	id   string
	eval func(*context) []Finding
}

// rules run in this order: data quality first (later rules read its flags), welfare before
// losses (losses escalate when water quality is also alarming).
var rules = []rule{
	{"DQ-FAST-CHANGE", ruleFastChange},
	{"DQ-CONFLICT", ruleConflict},
	{"DQ-STALE", ruleStale},
	{"WB-WELFARE-BOUNDS", ruleWelfareBounds},
	{"NH-AMMONIA-NITRITE", ruleAmmoniaNitrite},
	{"PD-OUT-OF-TARGET", ruleOutOfTarget},
	{"PI-ALK-SWING", ruleAlkSwing},
	{"PI-TREND", ruleTrend},
	{"DM-ADHERENCE", ruleAdherence},
	{"LC-LOSSES", ruleLosses},
	{"NM-NEXT", ruleNextMeasurements},
}

// RuleIDs lists the rules of RuleVersion in evaluation order.
func RuleIDs() []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.id
	}
	return out
}

// Evaluate runs every rule over the snapshot and validates the result (step 8).
func Evaluate(s Snapshot) Result {
	c := newContext(s)
	var fs []Finding
	for _, r := range rules {
		for _, f := range r.eval(c) {
			f.RuleID = r.id
			fs = append(fs, f)
		}
	}
	return finish(s, fs)
}

// --- data quality ---

func ruleFastChange(c *context) []Finding {
	var out []Finding
	for _, param := range sortedKeys(fastChangePerDay) {
		m := c.metrics[param]
		if m.Latest == nil || m.Previous == nil || m.DeltaFromPrevious == nil || c.s.Now.Sub(m.Latest.MeasuredAt) > recentWindow {
			continue
		}
		gap := m.Latest.MeasuredAt.Sub(m.Previous.MeasuredAt)
		if gap <= conflictWindow || gap > 7*day {
			continue // within 2 h it is a conflict (DQ-CONFLICT); beyond a week, too sparse
		}
		change := abs(*m.DeltaFromPrevious)
		if change.Cmp(toleranceFor(param, *m.Previous, *m.Latest)) <= 0 {
			continue // within kit resolution
		}
		perDay, err := change.DivRound(days(max(gap, day)), divPlaces) // at least a day: no inflation of short gaps
		if err != nil || perDay.Cmp(fastChangePerDay[param]) <= 0 {
			continue
		}
		c.suspect[param] = true
		out = append(out, Finding{
			Type: "data_quality", Severity: SeverityWarning, Parameter: param,
			Summary: t("dq.fast_change.summary", "parameter", param),
			Observations: []Observation{{
				Text: t("dq.fast_change.observation", "parameter", param, "from", m.Previous.Value.String(), "to", m.Latest.Value.String(),
					"unit", m.Unit, "days", days(gap).Round(1).String()),
				Evidence: []EvidenceRef{reading(m.Previous), reading(m.Latest)},
			}},
			// It may be real (a stuck doser, an ATO failure): confirm first, then act on it.
			Verifications:  []Verification{retest(param), {Text: t("verify.check_equipment_dosing", "parameter", param), Parameter: param}},
			SafetyWarnings: []SafetyWarning{{Text: t("safety.confirm_before_acting", "parameter", param)}},
			Confidence:     ConfidenceLow,
		})
	}
	return out
}

func ruleConflict(c *context) []Finding {
	var out []Finding
	for _, param := range sortedKeys(tolerances) {
		rs := byTime(c.s, param)
		if len(rs) < 2 {
			continue
		}
		// Only the latest pair: a later agreeing re-test resolves an older conflict.
		a, b := rs[len(rs)-2], rs[len(rs)-1]
		if b.MeasuredAt.Sub(a.MeasuredAt) > conflictWindow || c.s.Now.Sub(b.MeasuredAt) > recentWindow {
			continue
		}
		if abs(b.Value.Sub(a.Value)).Cmp(toleranceFor(param, a, b)) <= 0 {
			continue
		}
		c.conflicts[param] = true
		out = append(out, Finding{
			Type: "data_quality", Severity: SeverityNotice, Parameter: param,
			Summary: t("dq.conflict.summary", "parameter", param),
			Observations: []Observation{{
				Text:     t("dq.conflict.observation", "parameter", param, "a", a.Value.String(), "b", b.Value.String(), "unit", c.metrics[param].Unit),
				Evidence: []EvidenceRef{reading(&a), reading(&b)},
			}},
			Verifications: []Verification{retest(param), {Text: t("verify.check_kit", "parameter", param), Parameter: param}},
			Confidence:    ConfidenceLow,
		})
	}
	return out
}

func ruleStale(c *context) []Finding {
	var missing []MissingItem
	for _, param := range coreParameters(c) {
		m := c.metrics[param]
		switch {
		case m.Latest == nil:
			missing = append(missing, MissingItem{Text: t("dq.stale.never", "parameter", param), Parameter: param})
		case !m.Fresh:
			missing = append(missing, MissingItem{Text: t("dq.stale.old", "parameter", param, "days", itoa(int64(m.Age/day))), Parameter: param})
		}
	}
	if len(missing) == 0 {
		return nil
	}
	// What to test is NM-NEXT's job; this finding records what is missing.
	return []Finding{{
		Type: "data_quality", Severity: SeverityInfo,
		Summary: t("dq.stale.summary", "count", itoa(int64(len(missing)))),
		Missing: missing, Confidence: ConfidenceNotAssessed,
	}}
}

// --- welfare and deviations ---

func ruleWelfareBounds(c *context) []Finding {
	var out []Finding
	for _, b := range welfareBounds {
		m := c.metrics[b.param]
		if m.Latest == nil || !m.Fresh || (b.reefOnly && !reefType(c.s)) {
			continue
		}
		v := m.Latest.Value
		severity, side, limit := "", "", (*decimal.Decimal)(nil)
		switch {
		case b.urgentBelow != nil && v.Cmp(*b.urgentBelow) < 0:
			severity, side, limit = SeverityUrgent, "low", b.urgentBelow
		case b.urgentAbove != nil && v.Cmp(*b.urgentAbove) > 0:
			severity, side, limit = SeverityUrgent, "high", b.urgentAbove
		case b.warnBelow != nil && v.Cmp(*b.warnBelow) < 0:
			severity, side, limit = SeverityWarning, "low", b.warnBelow
		case b.warnAbove != nil && v.Cmp(*b.warnAbove) > 0:
			severity, side, limit = SeverityWarning, "high", b.warnAbove
		default:
			continue
		}
		key := "wb." + b.param + "." + side
		if side == "low" && b.intentionLow && !reefType(c.s) {
			severity, key = SeverityWarning, key+".if_unintended" // e.g. hyposalinity treatment
		}
		c.waterAlarm = c.waterAlarm || severity == SeverityUrgent
		ws := []SafetyWarning{correctionWarning(b.param)}
		if severity == SeverityUrgent {
			ws = append([]SafetyWarning{{Text: t("safety.welfare_" + b.param), Urgent: true}}, append(ws, SafetyWarning{Text: t("safety.escalate"), Urgent: true})...)
		}
		out = append(out, Finding{
			Type: "parameter_deviation", Severity: severity, Parameter: b.param,
			Summary: t(key+".summary", "parameter", b.param),
			Observations: []Observation{{
				Text:     t("wb.observation", "parameter", b.param, "value", v.String(), "unit", m.Unit, "limit", limit.String()),
				Evidence: []EvidenceRef{reading(m.Latest)},
			}},
			Verifications:  []Verification{retest(b.param)},
			SafetyWarnings: ws,
			Confidence:     confidence(c, b.param, []*Reading{m.Latest}),
		})
	}
	return out
}

func ruleAmmoniaNitrite(c *context) []Finding {
	if c.s.TankStatus == "archived" {
		return nil
	}
	// A fishless cycle is supposed to show ammonia and nitrite; with animals present it is
	// urgent whatever the tank's status (fish-in cycles included).
	fishless := c.s.TankStatus == "cycling" && c.s.LivestockCount == 0
	var out []Finding
	for _, param := range []string{"ammonia", "nitrite"} {
		m := c.metrics[param]
		if m.Latest == nil || m.Latest.Value.Cmp(nhDetected[param]) < 0 {
			continue
		}
		f := Finding{
			Type: "parameter_deviation", Parameter: param,
			Observations: []Observation{{
				Text:     t("nh.observation", "parameter", param, "value", m.Latest.Value.String(), "unit", m.Unit, "threshold", nhUrgent[param].String()),
				Evidence: []EvidenceRef{reading(m.Latest)},
			}},
			Verifications:  []Verification{{Text: t("verify.second_test", "parameter", param), Parameter: param}},
			KnowledgeSlugs: []string{"ammonia-nitrite-established-tank"},
			Confidence:     confidence(c, param, []*Reading{m.Latest}),
		}
		urgent := m.Latest.Value.Cmp(nhUrgent[param]) >= 0
		switch {
		case fishless:
			f.Severity, f.Summary = SeverityInfo, t("nh.summary.cycle", "parameter", param)
			f.Confidence = ConfidenceNotAssessed
		case !m.Fresh:
			// Elevated last time and not re-tested since: ask for a test, don't assume it cleared.
			f.Severity, f.Summary = SeverityWarning, t("nh.summary.stale", "parameter", param)
			f.Missing = []MissingItem{{Text: t("missing.repeat_test", "parameter", param), Parameter: param}}
		case urgent:
			c.waterAlarm = true
			f.Severity, f.Summary = SeverityUrgent, t("nh.summary.urgent", "parameter", param)
			f.SafetyWarnings = []SafetyWarning{
				{Text: t("safety.welfare_ammonia_nitrite", "parameter", param), Urgent: true},
				{Text: t("safety.escalate"), Urgent: true},
			}
		default:
			f.Severity, f.Summary = SeverityWarning, t("nh.summary.detected", "parameter", param)
			f.Verifications = append(f.Verifications, Verification{Text: t("verify.look_for_cause_nh")})
		}
		out = append(out, f)
	}
	return out
}

func ruleOutOfTarget(c *context) []Finding {
	var out []Finding
	for _, m := range AllMetrics(c.s) {
		if m.Latest == nil || !m.Fresh || m.TargetStatus == "" || m.TargetStatus == "within" {
			continue
		}
		if m.Parameter == "ammonia" || m.Parameter == "nitrite" {
			continue // NH-AMMONIA-NITRITE speaks for these
		}
		// Readings outside the same side of the target support the deviation; readings within
		// it since the deviation began support a measurement problem.
		freshFrom := c.s.Now.Add(-time.Duration(m.FreshnessHours) * time.Hour)
		var outside, within []*Reading
		for _, r := range byTime(c.s, m.Parameter) {
			if r.MeasuredAt.Before(freshFrom) {
				continue
			}
			r := r
			switch targetStatus(targetAt(c.s, m.Parameter, r.MeasuredAt), r.Value) {
			case m.TargetStatus:
				outside = append(outside, &r)
			case "within":
				if len(outside) > 0 {
					within = append(within, &r)
				}
			}
		}
		refs := func(rs []*Reading) []EvidenceRef {
			out := []EvidenceRef{}
			for _, r := range rs {
				out = append(out, reading(r))
			}
			return out
		}
		limit := m.Target.Min
		if m.TargetStatus == "above" {
			limit = m.Target.Max
		}
		conf := confidence(c, m.Parameter, outside, within...)
		f := Finding{
			Type: "parameter_deviation", Severity: deviationSeverity(m), Parameter: m.Parameter,
			Summary: t("pd.summary."+m.TargetStatus, "parameter", m.Parameter),
			Observations: []Observation{{
				Text:     t("pd.observation."+m.TargetStatus, "parameter", m.Parameter, "value", m.Latest.Value.String(), "unit", m.Unit, "bound", limit.String()),
				Evidence: refs(outside),
			}},
			Verifications:  []Verification{retest(m.Parameter)},
			SafetyWarnings: []SafetyWarning{correctionWarning(m.Parameter)},
			KnowledgeSlugs: []string{m.Parameter + "-basics"},
			Confidence:     conf,
		}
		if conf != ConfidenceHigh {
			// Measurement error stays a possibility until a second kit or method confirms it.
			f.Hypotheses = []Hypothesis{{
				Key: "measurement", Text: t("pd.hypothesis.measurement", "parameter", m.Parameter),
				Supporting: refs(within), Contradicting: refs(outside[1:]),
				Missing: []MissingItem{{Text: t("missing.second_kit", "parameter", m.Parameter), Parameter: m.Parameter}},
			}}
		}
		if len(outside) == 1 {
			f.Missing = append(f.Missing, MissingItem{Text: t("missing.repeat_test", "parameter", m.Parameter), Parameter: m.Parameter})
		}
		out = append(out, f)
	}
	return out
}

// deviationSeverity grows with the distance outside the target: more than half the target's
// width (or 10 % of the bound without a width) is a warning.
func deviationSeverity(m ParameterMetrics) string {
	tg, v := m.Target, m.Latest.Value
	var dist, scale decimal.Decimal
	switch m.TargetStatus {
	case "below":
		dist = tg.Min.Sub(v)
	case "above":
		dist = v.Sub(*tg.Max)
	}
	if tg.Min != nil && tg.Max != nil && tg.Max.Cmp(*tg.Min) > 0 {
		scale, _ = tg.Max.Sub(*tg.Min).DivRound(decimal.MustParse("2"), divPlaces)
	} else if tg.Min != nil {
		scale = abs(*tg.Min).Mul(decimal.MustParse("0.1"))
	} else {
		scale = abs(*tg.Max).Mul(decimal.MustParse("0.1"))
	}
	if scale.Sign() > 0 && dist.Cmp(scale) > 0 {
		return SeverityWarning
	}
	return SeverityNotice
}

// --- instability ---

// agreeing drops readings that disagree with another reading taken within conflictWindow
// (both readings of the pair are unreliable); instability is judged on the rest.
func agreeing(param string, rs []Reading) []Reading {
	bad := map[int]bool{}
	for i := range rs {
		for j := i + 1; j < len(rs) && rs[j].MeasuredAt.Sub(rs[i].MeasuredAt) <= conflictWindow; j++ {
			if abs(rs[j].Value.Sub(rs[i].Value)).Cmp(toleranceFor(param, rs[i], rs[j])) > 0 {
				bad[i], bad[j] = true, true
			}
		}
	}
	out := make([]Reading, 0, len(rs))
	for i, r := range rs {
		if !bad[i] {
			out = append(out, r)
		}
	}
	return out
}

func ruleAlkSwing(c *context) []Finding {
	m := c.metrics["alkalinity"]
	if c.doubtful("alkalinity") {
		return nil
	}
	var rs []Reading
	for _, r := range agreeing("alkalinity", byTime(c.s, "alkalinity")) {
		if !r.MeasuredAt.Before(c.s.Now.Add(-7 * day)) {
			rs = append(rs, r)
		}
	}
	w := window(rs, c.s.Now.Add(-7*day))
	if w.Count < 3 || w.Range == nil {
		return nil
	}
	// A swing goes up and down; a steady move in one direction (e.g. a slow correction) is
	// a trend, not instability.
	ups, downs := false, false
	for i := 1; i < len(rs); i++ {
		d := rs[i].Value.Sub(rs[i-1].Value)
		if d.Cmp(tolerances["alkalinity"].abs) > 0 {
			ups = true
		}
		if zero.Sub(d).Cmp(tolerances["alkalinity"].abs) > 0 {
			downs = true
		}
	}
	if !ups || !downs || w.Range.Cmp(decimal.MustParse("1.0")) <= 0 {
		return nil
	}
	var ev []EvidenceRef
	var lo, hi *Reading
	for i := range rs {
		r := &rs[i]
		ev = append(ev, reading(r))
		if lo == nil || r.Value.Cmp(lo.Value) < 0 {
			lo = r
		}
		if hi == nil || r.Value.Cmp(hi.Value) > 0 {
			hi = r
		}
	}
	conf := confidence(c, "alkalinity", []*Reading{lo, hi})
	if conf == ConfidenceHigh {
		conf = ConfidenceModerate // built from two readings: never more than moderate
	}
	f := Finding{
		Type: "parameter_instability", Severity: SeverityWarning, Parameter: "alkalinity",
		Summary: t("pi.alk_swing.summary"),
		Observations: []Observation{{
			Text:     t("pi.alk_swing.observation", "range", w.Range.String(), "min", lo.Value.String(), "max", hi.Value.String(), "unit", m.Unit),
			Evidence: ev,
		}},
		Verifications:  []Verification{retest("alkalinity")},
		SafetyWarnings: []SafetyWarning{correctionWarning("alkalinity")},
		KnowledgeSlugs: []string{"alkalinity-stability"},
		Confidence:     conf,
	}
	// Possible links, labelled as coincidences.
	a := AdherenceFor(c.s, 7*day)
	if miss := missedDoses(c.s, 7*day); a.Ratio != nil && a.Ratio.Cmp(minAdherence) < 0 && len(miss) > 0 {
		f.Hypotheses = append(f.Hypotheses, Hypothesis{
			Key: "dosing_adherence", Text: t("pi.alk_swing.link.dosing", "done", itoa(int64(a.Completed)), "due", itoa(int64(a.Due-a.Open))),
			Correlation: true, Supporting: miss,
		})
	}
	var wcs []EvidenceRef
	for _, w := range c.s.WaterChanges {
		if !w.CompletedAt.Before(c.s.Now.Add(-7*day)) && !w.CompletedAt.After(c.s.Now) {
			wcs = append(wcs, EvidenceRef{Kind: "water_change", ID: w.ID})
		}
	}
	if len(wcs) > 0 {
		f.Hypotheses = append(f.Hypotheses, Hypothesis{Key: "water_change", Text: t("pi.alk_swing.link.water_change"), Correlation: true, Supporting: wcs})
	}
	if len(f.Hypotheses) == 0 {
		f.Missing = append(f.Missing, MissingItem{Text: t("missing.dosing_records")})
	}
	return []Finding{f}
}

func ruleTrend(c *context) []Finding {
	var out []Finding
	for _, m := range AllMetrics(c.s) {
		if m.Latest == nil || !m.Fresh || m.TargetStatus != "within" || c.doubtful(m.Parameter) {
			continue
		}
		var pts []Reading
		for _, r := range agreeing(m.Parameter, byTime(c.s, m.Parameter)) {
			if !r.MeasuredAt.Before(c.s.Now.Add(-trendWindow)) {
				pts = append(pts, r)
			}
		}
		slopePerDay, span := slope(pts)
		if slopePerDay == nil || span.Cmp(minTrendSpan) < 0 {
			continue
		}
		move := slopePerDay.Mul(trendHorizon)
		if abs(move).Cmp(toleranceFor(m.Parameter, *m.Latest, *m.Latest)) <= 0 {
			continue // within kit noise
		}
		projected := m.Latest.Value.Add(move)
		side := targetStatus(targetAt(c.s, m.Parameter, c.s.Now), projected)
		if side == "within" || side == "" {
			continue
		}
		ev := make([]EvidenceRef, len(pts))
		for i := range pts {
			ev[i] = reading(&pts[i])
		}
		out = append(out, Finding{
			Type: "parameter_instability", Severity: SeverityNotice, Parameter: m.Parameter,
			Summary: t("pi.trend.summary."+side, "parameter", m.Parameter),
			Observations: []Observation{{
				Text: t("pi.trend.observation", "parameter", m.Parameter, "slope", slopePerDay.Round(3).String(), "unit", m.Unit,
					"points", itoa(int64(len(pts))), "days", span.Round(0).String(), "projected", projected.Round(2).String()),
				Evidence: ev,
			}},
			Verifications:  []Verification{retest(m.Parameter)},
			SafetyWarnings: []SafetyWarning{correctionWarning(m.Parameter)},
			Confidence:     ConfidenceLow, // a projection is never more than an early notice
		})
	}
	return out
}

// --- dosing, livestock ---

// missedDoses are the missed, skipped or failed occurrences counted by AdherenceFor.
func missedDoses(s Snapshot, window time.Duration) []EvidenceRef {
	var out []EvidenceRef
	for _, e := range adherenceEvents(s, window) {
		if slices.Contains([]string{"missed", "skipped", "failed"}, e.Status) {
			out = append(out, EvidenceRef{Kind: "dosing_event", ID: e.ID})
		}
	}
	return out
}

// ruleAdherence reports incomplete dosing records. Docs §5 also want "the plan's target
// parameter trending away"; plans have no target parameter yet (open item DS2), so the rule
// reports the records alone, as a records finding.
func ruleAdherence(c *context) []Finding {
	a := AdherenceFor(c.s, 7*day)
	miss := missedDoses(c.s, 7*day)
	if a.Ratio == nil || a.Due-a.Open < minAdherenceDue || a.Ratio.Cmp(minAdherence) >= 0 || len(miss) == 0 {
		return nil
	}
	return []Finding{{
		Type: "dosing_mismatch", Severity: SeverityNotice,
		Summary: t("dm.summary"),
		Observations: []Observation{{
			Text:     t("dm.observation", "done", itoa(int64(a.Completed)), "due", itoa(int64(a.Due-a.Open))),
			Evidence: miss,
		}},
		Verifications: []Verification{{Text: t("verify.dosing_records")}},
		// The finding is about the records themselves, which are the evidence: ≥ 3 resolved
		// occurrences of the keeper's own log (docs §4 "moderate": ≥ 2 independent facts).
		Confidence: ConfidenceModerate,
	}}
}

func ruleLosses(c *context) []Finding {
	l := LivestockFor(c.s, 7*day)
	type change struct {
		ref EvidenceRef
		at  time.Time
	}
	var changes []change
	for _, e := range c.s.EquipmentChanges {
		changes = append(changes, change{EvidenceRef{Kind: "equipment_change", ID: e.ID}, e.At})
	}
	for _, w := range c.s.WaterChanges {
		changes = append(changes, change{EvidenceRef{Kind: "water_change", ID: w.ID}, w.CompletedAt})
	}
	for _, e := range c.s.LivestockEvents {
		if e.Type == "added" || e.Type == "transferred_in" {
			changes = append(changes, change{EvidenceRef{Kind: "livestock_event", ID: e.ID}, e.OccurredAt})
		}
	}
	var linked, deaths []EvidenceRef
	for _, d := range l.Deaths7 {
		deaths = append(deaths, EvidenceRef{Kind: "livestock_event", ID: d.ID})
		for _, ch := range changes {
			if !ch.at.After(d.OccurredAt) && d.OccurredAt.Sub(ch.at) <= lossLink && !slices.Contains(linked, ch.ref) {
				linked = append(linked, ch.ref)
			}
		}
	}
	several := l.DeathCount >= 2 || l.Deaths >= 2
	if !several && len(linked) == 0 {
		return nil
	}
	// Two deaths can be routine in a large clean-up crew; together with alarming water
	// quality they are urgent.
	urgent := several && c.waterAlarm
	severity := SeverityWarning
	if urgent {
		severity = SeverityUrgent
	}
	f := Finding{
		Type: "livestock_change", Severity: severity,
		Summary: t("lc.summary", "deaths", itoa(int64(l.DeathCount))),
		Observations: []Observation{{
			Text:     t("lc.observation", "events", itoa(int64(l.Deaths)), "individuals", itoa(int64(l.DeathCount))),
			Evidence: deaths,
		}},
		SafetyWarnings: []SafetyWarning{{Text: t("safety.welfare_losses"), Urgent: urgent}, {Text: t("safety.escalate"), Urgent: urgent}},
		Verifications:  []Verification{{Text: t("verify.full_test")}, {Text: t("verify.observe_livestock")}},
		Confidence:     ConfidenceLow,
	}
	if len(linked) > 0 {
		f.Hypotheses = append(f.Hypotheses, Hypothesis{Key: "recent_change", Text: t("lc.link.recent_change", "hours", itoa(int64(lossLink/time.Hour))), Correlation: true, Supporting: linked})
	}
	if l.Missing > 0 {
		var miss []EvidenceRef
		for _, e := range c.s.LivestockEvents {
			if e.Type == "missing" && !e.OccurredAt.Before(c.s.Now.Add(-7*day)) && !e.OccurredAt.After(c.s.Now) {
				miss = append(miss, EvidenceRef{Kind: "livestock_event", ID: e.ID})
			}
		}
		// Unaccounted for, never counted as deaths (docs/knowledge-engine.md §5).
		f.Observations = append(f.Observations, Observation{Text: t("lc.observation.missing", "individuals", itoa(int64(l.Missing))), Evidence: miss})
	}
	return []Finding{f}
}

func ruleNextMeasurements(c *context) []Finding {
	var vs []Verification
	for _, param := range coreParameters(c) {
		m := c.metrics[param]
		if m.Latest == nil || !m.Fresh || c.doubtful(param) {
			vs = append(vs, Verification{Text: t("verify.test", "parameter", param), Parameter: param})
		}
	}
	if len(vs) == 0 {
		return nil
	}
	return []Finding{{
		Type: "next_measurements", Severity: SeverityInfo,
		Summary:       t("nm.summary", "count", itoa(int64(len(vs)))),
		Verifications: vs, Confidence: ConfidenceNotAssessed,
	}}
}

// confidence applies the documented basis (docs/knowledge-engine.md §4):
// low — one supporting reading, any contradicting one, a stale or doubtful input;
// moderate — ≥ 2 supporting readings, all fresh, none contradicting;
// high — moderate and confirmed by a second known method or kit (readings converted with an
// assumption, e.g. from SG, never count as confirmation).
func confidence(c *context, param string, support []*Reading, contra ...*Reading) string {
	m := c.metrics[param]
	if len(support) == 0 {
		return ConfidenceNotAssessed
	}
	if len(support) < 2 || len(contra) > 0 || !m.Fresh || c.doubtful(param) {
		return ConfidenceLow
	}
	kits := map[string]bool{}
	for _, r := range support {
		if r.Estimate || (r.Method == "" && r.TestKit == "") {
			continue
		}
		kits[r.Method+"|"+r.TestKit] = true
	}
	if len(kits) >= 2 {
		return ConfidenceHigh
	}
	return ConfidenceModerate
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
