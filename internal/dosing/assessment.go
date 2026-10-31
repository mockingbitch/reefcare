package dosing

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/intelligence"
)

// ForAssessment fills the dosing part of an assessment snapshot (intelligence.Source). The
// caller has authorized the tank.
func ForAssessment(ctx context.Context, q *dbq.Queries, tankID uuid.UUID, since time.Time, s *intelligence.Snapshot) error {
	plans, err := q.ListDosingPlansForAssessment(ctx, tankID)
	if err != nil {
		return err
	}
	for _, p := range plans {
		s.DosingPlans = append(s.DosingPlans, intelligence.DosingPlan{ID: p.ID, Product: p.ProductName, Status: p.Status, Delivery: p.DeliveryMethod})
	}
	events, err := q.ListDosingEventsForAssessment(ctx, dbq.ListDosingEventsForAssessmentParams{
		TankID: tankID, Since: since, Until: s.Now, RowLimit: intelligence.LimitDosingEvents,
	})
	if err != nil {
		return err
	}
	s.MarkTruncated("dosing_events", len(events), intelligence.LimitDosingEvents)
	for _, e := range intelligence.Reverse(events) {
		s.DosingEvents = append(s.DosingEvents, intelligence.DosingEvent{ID: e.ID, PlanID: e.PlanID, Status: e.Status, ScheduledFor: e.ScheduledFor, ExecutedAt: e.ExecutedAt})
	}
	return nil
}
