package dosing

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/notifications"
)

// reminderLookback bounds the reminder scan to recently missed doses.
const reminderLookback = 48 * time.Hour

// Reminders is the notifications source for dosing: one reminder per missed dose
// (manual plans; pump doses become "unconfirmed" and are not reminded). Upcoming doses
// are not reminded (open item NT2).
func Reminders(ctx context.Context, q *dbq.Queries, now time.Time, emit func(notifications.Reminder) error) error {
	afterAt, afterID := time.Unix(0, 0).UTC(), uuid.Nil
	for {
		rows, err := q.ListMissedDosesForReminders(ctx, dbq.ListMissedDosesForRemindersParams{
			Since: now.Add(-reminderLookback), Until: now.Add(time.Microsecond), AfterAt: afterAt, AfterID: afterID, Batch: 500,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			payload := map[string]any{
				"tank_name": r.TankName, "product_name": r.ProductName, "scheduled_for": r.ScheduledFor.UTC(), "unit": r.AmountUnit,
			}
			if r.PlannedAmount != nil {
				payload["amount"] = r.PlannedAmount.String()
			}
			if err := emit(notifications.Reminder{
				UserID: r.OwnerID, TankID: r.TankID, Category: notifications.CategoryDosing, Kind: notifications.KindDosingMissed,
				DedupeKey: notifications.KindDosingMissed + ":" + r.ID.String(), OccurredAt: r.ScheduledFor, Payload: payload,
			}); err != nil {
				return err
			}
		}
		if len(rows) < 500 {
			return nil
		}
		afterAt, afterID = rows[len(rows)-1].MissedAt, rows[len(rows)-1].ID
	}
}
