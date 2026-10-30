// Package timeline records events on a tank's unified chronological timeline
// (docs/domain-model.md, tank_events). Modules call Record inside the transaction that
// changes the source data, so the event and its cause commit or roll back together.
// The feed endpoint arrives with milestone 2.8.
package timeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
)

// Categories group events for filtering (CHECK constraint on tank_events.category).
const (
	CategoryMeasurement = "measurement"
	CategoryDosing      = "dosing"
	CategoryWaterChange = "water_change"
	CategoryEquipment   = "equipment"
	CategoryMaintenance = "maintenance"
	CategoryLivestock   = "livestock"
	CategorySymptom     = "symptom"
	CategoryTreatment   = "treatment"
	CategoryMilestone   = "milestone"
	CategoryNote        = "note"
	CategoryAssessment  = "assessment"
)

// Event is one timeline entry. (SourceType, SourceID, EventType) identifies the fact:
// recording it again is a no-op, so retried workflows never duplicate events. SourceID
// must therefore name the fact itself (a status-change row, a configuration version, a
// dosing event), never a parent entity that can produce the same event type twice.
type Event struct {
	TankID     uuid.UUID
	OccurredAt time.Time // when it happened in the tank, not when it was entered
	Category   string
	EventType  string // e.g. "tank.created", "dosing.completed"
	SourceType string // table or entity name, e.g. "tank"
	SourceID   uuid.UUID
	ActorID    *uuid.UUID
	// Summary holds small render-ready params; the client localizes by EventType.
	// Never put private data here that the tank's viewers may not see.
	Summary map[string]any
}

// Record inserts e with q (normally bound to the caller's transaction). It reports
// whether a new event was written.
func Record(ctx context.Context, q *dbq.Queries, now time.Time, e Event) (bool, error) {
	var summary []byte
	if len(e.Summary) > 0 {
		var err error
		if summary, err = json.Marshal(e.Summary); err != nil {
			return false, fmt.Errorf("encode timeline summary: %w", err)
		}
	}
	n, err := q.InsertTankEvent(ctx, dbq.InsertTankEventParams{
		TankID: e.TankID, OccurredAt: e.OccurredAt, Category: e.Category, EventType: e.EventType,
		SourceType: e.SourceType, SourceID: e.SourceID, ActorID: e.ActorID, Summary: summary, Now: now,
	})
	if err != nil {
		return false, fmt.Errorf("record timeline event %s: %w", e.EventType, err)
	}
	return n == 1, nil
}
