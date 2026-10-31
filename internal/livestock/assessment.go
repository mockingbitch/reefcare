package livestock

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/intelligence"
)

// ForAssessment fills the livestock part of an assessment snapshot (intelligence.Source):
// recent events and the number of animals and corals present. The caller has authorized
// the tank.
func ForAssessment(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since time.Time, s *intelligence.Snapshot) error {
	rows, err := q.ListLivestockEventsForAssessment(ctx, dbq.ListLivestockEventsForAssessmentParams{
		TankID: tankID, Since: since, Until: s.Now, RowLimit: intelligence.LimitLivestockEvents,
	})
	if err != nil {
		return err
	}
	s.MarkTruncated("livestock_events", len(rows), intelligence.LimitLivestockEvents)
	for _, r := range intelligence.Reverse(rows) {
		s.LivestockEvents = append(s.LivestockEvents, intelligence.LivestockEvent{ID: r.ID, EntryID: r.EntryID, Type: r.EventType, QuantityDelta: r.QuantityDelta, OccurredAt: r.OccurredAt})
	}
	n, err := q.CountLivestockPresent(ctx, tankID)
	if err != nil {
		return err
	}
	s.LivestockCount = int(n)
	return nil
}
