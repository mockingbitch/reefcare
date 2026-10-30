package equipment

import (
	"context"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/notifications"
	"reefcare/internal/platform/recurrence"
)

// startOfDay is local midnight of d in tz.
func startOfDay(d recurrence.Date, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// reminderLookbackDays bounds overdue reminders to plans that became overdue recently.
const reminderLookbackDays = 7

// Reminders is the notifications source for maintenance: "due today" and "overdue" in the
// plan's zone, once per plan and due date (performing it moves the due date, so the next
// cycle is reminded again).
func Reminders(ctx context.Context, q *dbq.Queries, now time.Time, emit func(notifications.Reminder) error) error {
	utc := timeToDate(now)
	since := dateToTime(utc).AddDate(0, 0, -reminderLookbackDays-1) // ±1 day covers every zone
	until := dateToTime(utc).AddDate(0, 0, 1)
	afterOn, afterID := since.AddDate(0, 0, -1), uuid.Nil
	for {
		rows, err := q.ListMaintenanceForReminders(ctx, dbq.ListMaintenanceForRemindersParams{
			Since: since, Until: until, AfterOn: afterOn, AfterID: afterID, Batch: 500,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			today := localToday(now, r.Timezone)
			due := timeToDate(r.NextDueOn)
			kind := ""
			switch {
			case due == today:
				kind = notifications.KindMaintenanceDue
			case due.Before(today) && !due.Before(timeToDate(dateToTime(today).AddDate(0, 0, -reminderLookbackDays))):
				kind = notifications.KindMaintenanceOverdue
			default:
				continue
			}
			payload := map[string]any{"tank_name": r.TankName, "title": r.Title, "due_on": due.String()}
			if r.EquipmentName != nil {
				payload["equipment_name"] = *r.EquipmentName
			}
			if err := emit(notifications.Reminder{
				UserID: r.OwnerID, TankID: r.TankID, Category: notifications.CategoryMaintenance, Kind: kind,
				DedupeKey:  kind + ":" + r.ID.String() + ":" + due.String(),
				OccurredAt: startOfDay(due, r.Timezone), Payload: payload,
			}); err != nil {
				return err
			}
		}
		if len(rows) < 500 {
			return nil
		}
		afterOn, afterID = rows[len(rows)-1].NextDueOn, rows[len(rows)-1].ID
	}
}
