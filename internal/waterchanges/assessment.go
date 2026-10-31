package waterchanges

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/intelligence"
)

// ForAssessment fills the completed water changes of an assessment snapshot
// (intelligence.Source). The caller has authorized the tank.
func ForAssessment(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since time.Time, s *intelligence.Snapshot) error {
	rows, err := q.ListCompletedWaterChangesForAssessment(ctx, dbq.ListCompletedWaterChangesForAssessmentParams{
		TankID: tankID, Since: since, Until: s.Now, RowLimit: intelligence.LimitWaterChanges,
	})
	if err != nil {
		return err
	}
	s.MarkTruncated("water_changes", len(rows), intelligence.LimitWaterChanges)
	for _, r := range intelligence.Reverse(rows) {
		if r.ActualVolumeL == nil || r.SystemVolumeL == nil {
			continue
		}
		s.WaterChanges = append(s.WaterChanges, intelligence.WaterChange{ID: r.ID, CompletedAt: r.CompletedAt, VolumeL: *r.ActualVolumeL, SystemVolumeL: *r.SystemVolumeL})
	}
	return nil
}
