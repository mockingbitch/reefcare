package waterchanges

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/notifications"
)

// Reminder windows: a change is announced up to reminderAhead before its time, and an
// overdue one is reminded if it became overdue within reminderLookback.
const (
	reminderAhead    = 24 * time.Hour
	reminderLookback = 7 * 24 * time.Hour
)

// Reminders is the notifications source for water changes: "coming up" within a day, and
// "overdue" once its grace passed. Both once per occurrence.
func Reminders(ctx context.Context, q *dbq.Queries, now time.Time, emit func(notifications.Reminder) error) error {
	afterAt, afterID := time.Unix(0, 0).UTC(), uuid.Nil
	for {
		// overdue_after ≥ scheduled_for and grace ≤ 24 h, so every change due within a day
		// has overdue_after < now + 48 h.
		rows, err := q.ListWaterChangesForReminders(ctx, dbq.ListWaterChangesForRemindersParams{
			Since: now.Add(-reminderLookback), Until: now.Add(reminderAhead + OverdueGrace), AfterAt: afterAt, AfterID: afterID, Batch: 500,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			kind := ""
			switch {
			case !r.OverdueAfter.After(now):
				kind = notifications.KindWaterChangeOverdue
			case r.ScheduledFor.After(now) && !r.ScheduledFor.After(now.Add(reminderAhead)):
				kind = notifications.KindWaterChangeDueSoon
			default:
				continue
			}
			payload := map[string]any{"tank_name": r.TankName, "scheduled_for": r.ScheduledFor.UTC()}
			if r.PlannedVolumeL != nil {
				payload["volume_l"] = r.PlannedVolumeL.String()
			}
			if err := emit(notifications.Reminder{
				UserID: r.OwnerID, TankID: r.TankID, Category: notifications.CategoryWaterChange, Kind: kind,
				DedupeKey: kind + ":" + r.ID.String(), OccurredAt: r.ScheduledFor, Payload: payload,
			}); err != nil {
				return err
			}
		}
		if len(rows) < 500 {
			return nil
		}
		afterAt, afterID = rows[len(rows)-1].OverdueAfter, rows[len(rows)-1].ID
	}
}
