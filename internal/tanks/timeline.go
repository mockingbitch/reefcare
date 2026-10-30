package tanks

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/timeline"
)

// TimelineEvent is one entry of a tank's unified timeline (US-TL-1).
type TimelineEvent struct {
	ID         uuid.UUID
	OccurredAt time.Time
	Category   string
	EventType  string
	SourceType string
	SourceID   uuid.UUID
	ActorID    *uuid.UUID
	ActorName  *string // the actor's current display name ("Deleted user" after erasure)
	Summary    json.RawMessage
}

// TimelineFilter narrows the feed. From is inclusive, To exclusive; zero values mean
// unbounded.
type TimelineFilter struct {
	Categories []string
	From, To   time.Time
	Limit      int32
	Cursor     *TimelineCursor
}

// TimelineCursor is the keyset of the feed.
type TimelineCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

var (
	feedStart = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	feedEnd   = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
)

// Timeline returns the tank's events newest first, for anyone who may read the tank.
func (s *Service) Timeline(ctx context.Context, a Actor, tankID uuid.UUID, f TimelineFilter) ([]TimelineEvent, error) {
	cats := make([]string, 0, len(f.Categories))
	seen := map[string]bool{}
	for _, c := range f.Categories {
		if !timeline.IsCategory(c) {
			return nil, apperr.BadRequest("invalid_query", "Unknown timeline category.", apperr.FieldError{
				Field: "category", Code: "unsupported", Params: map[string]string{"allowed": strings.Join(timeline.Categories, ",")},
			})
		}
		if !seen[c] {
			seen[c] = true
			cats = append(cats, c)
		}
	}
	from, to := feedStart, feedEnd
	if !f.From.IsZero() {
		from = f.From
	}
	if !f.To.IsZero() {
		to = f.To
	}
	if !f.From.IsZero() && !f.To.IsZero() && f.To.Before(f.From) {
		return nil, apperr.BadRequest("invalid_query", "The time range ends before it starts.",
			apperr.FieldError{Field: "to", Code: "before_from"})
	}
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, PermRead); err != nil {
		return nil, err
	}
	// The exclusive "to" bound is folded into the keyset: (t, id) < (to, uuid.Nil) ⇔ t < to.
	curAt, curID := to, uuid.Nil
	if f.Cursor != nil && f.Cursor.At.Before(to) {
		curAt, curID = f.Cursor.At, f.Cursor.ID
	}
	if len(cats) == len(timeline.Categories) {
		cats = nil // every category: the plain feed is cheaper
	}
	var rows []dbq.ListTankEventsRow
	var err error
	if len(cats) == 0 {
		rows, err = q.ListTankEvents(ctx, dbq.ListTankEventsParams{
			TankID: tankID, FromAt: from, CursorAt: curAt, CursorID: curID, RowLimit: f.Limit,
		})
	} else {
		var byCat []dbq.ListTankEventsByCategoryRow
		byCat, err = q.ListTankEventsByCategory(ctx, dbq.ListTankEventsByCategoryParams{
			TankID: tankID, Categories: cats, FromAt: from, CursorAt: curAt, CursorID: curID, RowLimit: f.Limit,
		})
		rows = make([]dbq.ListTankEventsRow, len(byCat))
		for i, r := range byCat {
			rows[i] = dbq.ListTankEventsRow(r)
		}
	}
	if err != nil {
		return nil, err
	}
	out := make([]TimelineEvent, len(rows))
	for i, r := range rows {
		out[i] = TimelineEvent{
			ID: r.ID, OccurredAt: r.OccurredAt, Category: r.Category, EventType: r.EventType, SourceType: r.SourceType,
			SourceID: r.SourceID, ActorID: r.ActorID, ActorName: r.ActorName, Summary: r.Summary,
		}
	}
	return out, nil
}
