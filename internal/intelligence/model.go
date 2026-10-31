package intelligence

import "github.com/google/uuid"

// Output model (docs/knowledge-engine.md §4, .claude/rules/intelligence-safety.md). Every
// user-facing sentence is a Text (an i18n key + params) the client localizes; templates use
// "may be related to" / "coincides with", never causal language.

// RuleVersion is the semver of the rule set. Thresholds are PROVISIONAL until the domain
// reviewer (decision D5, open item L3) confirms them; assessments say so.
const RuleVersion = "1.0.0"

// Provisional marks rule thresholds that no domain reviewer has confirmed yet.
const Provisional = true

// Text is an i18n key with parameters.
type Text struct {
	Key    string            `json:"key"`
	Params map[string]string `json:"params,omitempty"`
}

// EvidenceRef points at a stored fact of the tank.
type EvidenceRef struct {
	Kind string    `json:"kind"` // reading | dosing_event | water_change | equipment_change | livestock_event
	ID   uuid.UUID `json:"id"`
}

// Observation is a factual statement backed by evidence.
type Observation struct {
	Text     Text          `json:"text"`
	Evidence []EvidenceRef `json:"evidence"`
}

// Hypothesis is a possible explanation. It lists what supports it, what contradicts it and
// what is missing; one without supporting evidence is dropped by validation.
type Hypothesis struct {
	Key           string        `json:"key"`
	Text          Text          `json:"text"`
	ClaimClass    string        `json:"claim_class"` // of the knowledge behind it; "" for rule-derived links
	Correlation   bool          `json:"correlation"` // a temporal coincidence, never a proven cause
	Supporting    []EvidenceRef `json:"supporting"`
	Contradicting []EvidenceRef `json:"contradicting"`
	Missing       []MissingItem `json:"missing"`
}

// MissingItem is information that would change or firm up the assessment.
type MissingItem struct {
	Text      Text   `json:"text"`
	Parameter string `json:"parameter,omitempty"`
}

// Verification is something the keeper can check (re-test, inspect a device…).
type Verification struct {
	Text      Text   `json:"text"`
	Parameter string `json:"parameter,omitempty"`
}

// SafetyWarning must survive into the final output (validation never removes it).
type SafetyWarning struct {
	Text   Text `json:"text"`
	Urgent bool `json:"urgent"`
}

// KnowledgeRef cites an exact published knowledge version (resolved by the service).
type KnowledgeRef struct {
	Slug      string    `json:"slug"`
	VersionID uuid.UUID `json:"version_id"`
	Title     string    `json:"title"`
	Locale    string    `json:"locale"`
}

// Confidence labels (documented basis, docs/knowledge-engine.md §4; no percentages).
const (
	ConfidenceNotAssessed = "not_assessed"
	ConfidenceLow         = "low"
	ConfidenceModerate    = "moderate"
	ConfidenceHigh        = "high"
)

// Severities order findings and decide notifications.
const (
	SeverityInfo    = "info"
	SeverityNotice  = "notice"
	SeverityWarning = "warning"
	SeverityUrgent  = "urgent"
)

// Finding is one rule's result.
type Finding struct {
	RuleID         string          `json:"rule_id"`
	Type           string          `json:"type"` // data_quality | parameter_deviation | parameter_instability | dosing_mismatch | livestock_change | next_measurements
	Severity       string          `json:"severity"`
	Parameter      string          `json:"parameter,omitempty"`
	Summary        Text            `json:"summary"`
	Observations   []Observation   `json:"observations"`
	Hypotheses     []Hypothesis    `json:"hypotheses"`
	Alternatives   []Hypothesis    `json:"alternative_explanations"`
	Missing        []MissingItem   `json:"missing_information"`
	Verifications  []Verification  `json:"recommended_verifications"`
	SafetyWarnings []SafetyWarning `json:"safety_warnings"`
	// KnowledgeSlugs are the articles the rule relies on; the service resolves them to
	// published versions (KnowledgeRefs) and drops the ones that don't exist.
	KnowledgeSlugs []string       `json:"-"`
	KnowledgeRefs  []KnowledgeRef `json:"knowledge_references"`
	Confidence     string         `json:"confidence_label"`
}

// Result is the engine's output for one snapshot.
type Result struct {
	Findings    []Finding `json:"findings"`
	RuleVersion string    `json:"rule_version"`
	Provisional bool      `json:"provisional"`
	// Confidence is the lowest confidence among findings that were assessed, or
	// not_assessed when no rule could assess anything.
	Confidence string `json:"confidence_label"`
	// Narrative is optional generated wording (null without an LLM, or when it failed).
	Narrative *Narrative `json:"narrative"`
}
