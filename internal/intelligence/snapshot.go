// Package intelligence is the deterministic Reef Intelligence engine (docs/knowledge-engine.md,
// .claude/rules/intelligence-safety.md, Phase 3): metrics and versioned rules over an
// authorized, immutable snapshot of one tank's recent history, producing provisional
// assessments — never diagnoses, never actions.
//
// This file holds the input snapshot. Metrics (metrics.go) and rules (rules.go) are pure
// functions of it, so every assessment can be reproduced from its stored snapshot.
package intelligence

import (
	"time"

	"github.com/google/uuid"

	"reefcare/internal/platform/decimal"
)

// SnapshotVersion is the version of the Snapshot layout stored with each assessment.
const SnapshotVersion = 1

// Snapshot is everything the engine may look at for one tank. It is loaded only after the
// caller was authorized to read the tank, and contains only that tank's data.
type Snapshot struct {
	TankID     uuid.UUID `json:"tank_id"`
	TankType   string    `json:"tank_type"`
	TankStatus string    `json:"tank_status"`
	// SetupDate is when the tank was set up (nil when unknown); young tanks are still cycling.
	SetupDate *time.Time `json:"setup_date,omitempty"`
	Now       time.Time  `json:"now"`
	// LivestockCount is the number of animals and corals present now (Σ current quantities).
	LivestockCount int `json:"livestock_count"`
	// Readings are valid (not superseded) readings in canonical units, any order.
	Readings []Reading `json:"readings"`
	// Targets is the tank's target history.
	Targets          []Target          `json:"targets"`
	DosingPlans      []DosingPlan      `json:"dosing_plans"`
	DosingEvents     []DosingEvent     `json:"dosing_events"`
	WaterChanges     []WaterChange     `json:"water_changes"`
	EquipmentChanges []EquipmentChange `json:"equipment_changes"`
	LivestockEvents  []LivestockEvent  `json:"livestock_events"`
}

// Reading is one measured value in the parameter's canonical unit.
type Reading struct {
	ID         uuid.UUID       `json:"id"`
	Parameter  string          `json:"parameter"`
	MeasuredAt time.Time       `json:"measured_at"`
	Value      decimal.Decimal `json:"value"`
	Unit       string          `json:"unit"`
	Estimate   bool            `json:"estimate,omitempty"` // converted with an assumption (e.g. SG)
	Method     string          `json:"method,omitempty"`
	TestKit    string          `json:"test_kit,omitempty"`
}

// Target is a parameter target valid in [ValidFrom, ValidTo).
type Target struct {
	Parameter      string           `json:"parameter"`
	Min            *decimal.Decimal `json:"min,omitempty"`
	Max            *decimal.Decimal `json:"max,omitempty"`
	FreshnessHours *int32           `json:"freshness_hours,omitempty"`
	ValidFrom      time.Time        `json:"valid_from"`
	ValidTo        *time.Time       `json:"valid_to,omitempty"`
}

// DosingPlan is an active or past dosing plan.
type DosingPlan struct {
	ID      uuid.UUID `json:"id"`
	Product string    `json:"product"`
	Status  string    `json:"status"` // active | paused | completed
	// Delivery is "manual" or "dosing_pump" (pump doses are never confirmed by ReefCare).
	Delivery string `json:"delivery"`
}

// DosingEvent is one planned occurrence or ad-hoc dose.
type DosingEvent struct {
	ID           uuid.UUID  `json:"id"`
	PlanID       *uuid.UUID `json:"plan_id,omitempty"`
	Status       string     `json:"status"` // scheduled | completed | partial | skipped | failed | missed | unconfirmed
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	ExecutedAt   *time.Time `json:"executed_at,omitempty"`
}

// WaterChange is a completed water change.
type WaterChange struct {
	ID          uuid.UUID       `json:"id"`
	CompletedAt time.Time       `json:"completed_at"`
	VolumeL     decimal.Decimal `json:"volume_l"`
	// SystemVolumeL is the tank's system volume when it was done (an estimate).
	SystemVolumeL decimal.Decimal `json:"system_volume_l"`
}

// EquipmentChange is a device installed or removed, or a configuration change.
type EquipmentChange struct {
	ID       uuid.UUID `json:"id"`
	Kind     string    `json:"kind"` // installed | removed | configuration_changed
	Category string    `json:"category"`
	At       time.Time `json:"at"`
}

// LivestockEvent is one livestock history event.
type LivestockEvent struct {
	ID            uuid.UUID `json:"id"`
	EntryID       uuid.UUID `json:"entry_id"`
	Type          string    `json:"type"` // added, died, missing, … (docs/knowledge-engine.md §5)
	QuantityDelta int32     `json:"quantity_delta"`
	OccurredAt    time.Time `json:"occurred_at"`
}
