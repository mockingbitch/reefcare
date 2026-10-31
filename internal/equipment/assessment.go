package equipment

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/intelligence"
)

// ForAssessment fills the equipment changes of an assessment snapshot (intelligence.Source).
// The caller has authorized the tank.
func ForAssessment(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since time.Time, s *intelligence.Snapshot) error {
	rows, err := q.ListEquipmentChangesForAssessment(ctx, dbq.ListEquipmentChangesForAssessmentParams{
		TankID: tankID, Since: since, Until: s.Now, RowLimit: intelligence.LimitEquipmentChanges,
	})
	if err != nil {
		return err
	}
	s.MarkTruncated("equipment_changes", len(rows), intelligence.LimitEquipmentChanges)
	for _, r := range intelligence.Reverse(rows) {
		s.EquipmentChanges = append(s.EquipmentChanges, intelligence.EquipmentChange{ID: r.ID, Kind: r.Kind, Category: r.Category, At: r.At})
	}
	return nil
}
