package waterchanges

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/decimal"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/patch"
	"reefcare/internal/platform/recurrence"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
)

// Service implements water-change workflows. Every method authorizes through
// tanks.Authorizer: reads need PermRead, writes PermWrite (owner, tank not archived).
type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
	authz tanks.Authorizer
	log   *slog.Logger
}

// NewService wires the service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, log: log}
}

var (
	errVersionConflict = apperr.Conflict("version_conflict", "The schedule was changed elsewhere. Reload and try again.")
	errAlreadyResolved = apperr.Conflict("already_resolved", "This water change was already completed or skipped.")
	errScheduleEnded   = apperr.Conflict("schedule_ended", "This schedule has ended and can no longer change.")
)

// --- schedules ---

// ScheduleInput creates a schedule. Decimals and dates are strings so every malformed
// value is reported against its field.
type ScheduleInput struct {
	RRule          string
	Timezone       string
	StartsOn       string
	EndsOn         *string
	PlannedVolumeL string
	SaltMix        *string
	Notes          *string
}

// CreateSchedule adds a recurring schedule and creates its first occurrences (US-WC-1).
func (s *Service) CreateSchedule(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in ScheduleInput) (Schedule, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	rule := c.rrule(in.RRule)
	tz := c.timezone(in.Timezone)
	starts := c.date("starts_on", in.StartsOn, now)
	ends := c.optDate("ends_on", in.EndsOn, now)
	if ends != nil && !v.HasErrors() && ends.Before(starts) {
		v.Add("ends_on", "before_start", nil)
	}
	volume := c.volume("planned_volume_l", in.PlannedVolumeL)
	salt := c.text("salt_mix", in.SaltMix, 100, false)
	notes := c.text("notes", in.Notes, 2000, true)
	if err := v.Err(); err != nil {
		return Schedule{}, err
	}
	var out Schedule
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		p := dbq.CreateWaterChangeScheduleParams{
			TankID: tankID, Rrule: rule, Timezone: tz, StartsOn: dateToTime(starts), PlannedVolumeL: volume,
			SaltMix: salt, Notes: notes, CreatedBy: &a.UserID, Now: now,
		}
		if ends != nil {
			t := dateToTime(*ends)
			p.EndsOn = &t
		}
		id, err := q.CreateWaterChangeSchedule(ctx, p)
		if err != nil {
			return err
		}
		row, err := q.LockWaterChangeSchedule(ctx, id)
		if err != nil {
			return err
		}
		if err := s.materialize(ctx, q, scheduleFromLocked(row), now); err != nil {
			return err
		}
		out, err = s.loadSchedule(ctx, q, id, now)
		return err
	})
	return out, err
}

// ListSchedules returns the tank's schedules, oldest first.
func (s *Service) ListSchedules(ctx context.Context, a tanks.Actor, tankID uuid.UUID) ([]Schedule, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListWaterChangeSchedules(ctx, tankID)
	if err != nil {
		return nil, err
	}
	system, err := q.GetTankSystemVolume(ctx, tankID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]Schedule, len(rows))
	for i, r := range rows {
		out[i] = s.decorate(scheduleFromRow(dbq.GetWaterChangeScheduleRow(r)), system, now)
	}
	return out, nil
}

// SchedulePatch is a merge patch of a schedule; Version is required.
type SchedulePatch struct {
	Version        *int32
	RRule          patch.Field[string]
	Timezone       patch.Field[string]
	StartsOn       patch.Field[string]
	EndsOn         patch.Field[string]
	PlannedVolumeL patch.Field[string]
	SaltMix        patch.Field[string]
	Notes          patch.Field[string]
	Status         patch.Field[string] // active, paused, ended (ended is terminal)
}

// UpdateSchedule edits a schedule. Upcoming occurrences nobody acted on are replaced to
// match; overdue and resolved ones are history and stay untouched.
func (s *Service) UpdateSchedule(ctx context.Context, a tanks.Actor, id uuid.UUID, in SchedulePatch) (Schedule, error) {
	now := s.clock.Now()
	var out Schedule
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		cur, err := q.GetWaterChangeSchedule(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, cur.TankID, tanks.PermWrite); err != nil {
			return err
		}
		row, err := q.LockWaterChangeSchedule(ctx, id)
		if err != nil {
			return err
		}
		sch := scheduleFromLocked(row)
		v := apperr.NewValidator()
		if in.Version == nil {
			v.Add("version", "required", nil)
			return v.Err()
		}
		if *in.Version != sch.Version {
			return errVersionConflict
		}
		if sch.Status == StatusEnded {
			return errScheduleEnded
		}
		next := applyPatch(checker{v}, sch, in, now)
		if err := v.Err(); err != nil {
			return err
		}
		// Anything that shapes occurrences invalidates the upcoming ones.
		reshape := next.RRule != sch.RRule || next.Timezone != sch.Timezone || next.StartsOn != sch.StartsOn ||
			!sameDate(next.EndsOn, sch.EndsOn) || next.PlannedVolumeL.Cmp(sch.PlannedVolumeL) != 0 ||
			!sameText(next.SaltMix, sch.SaltMix) || next.Status != sch.Status
		var until *time.Time
		if !reshape {
			until = row.MaterializedUntil
		} else if _, err := q.DeleteFutureWaterChangeOccurrences(ctx, dbq.DeleteFutureWaterChangeOccurrencesParams{ScheduleID: id, After: now}); err != nil {
			return err
		}
		p := dbq.UpdateWaterChangeScheduleParams{
			ID: id, Version: sch.Version, Rrule: next.RRule, Timezone: next.Timezone, StartsOn: dateToTime(next.StartsOn),
			PlannedVolumeL: next.PlannedVolumeL, SaltMix: next.SaltMix, Notes: next.Notes, Status: next.Status,
			MaterializedUntil: until, Now: now,
		}
		if next.EndsOn != nil {
			t := dateToTime(*next.EndsOn)
			p.EndsOn = &t
		}
		n, err := q.UpdateWaterChangeSchedule(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 {
			return errVersionConflict
		}
		if reshape && next.Status == StatusActive {
			// The upcoming window was just emptied: materialize it again from now.
			next.ID, next.TankID, next.materializedUntil = id, sch.TankID, nil
			if err := s.materialize(ctx, q, next, now); err != nil {
				return err
			}
		}
		out, err = s.loadSchedule(ctx, q, id, now)
		return err
	})
	return out, err
}

func applyPatch(c checker, cur Schedule, in SchedulePatch, now time.Time) Schedule {
	next := cur
	required := func(field string, f patch.Field[string]) bool {
		if f.Set && f.Null {
			c.v.Add(field, "required", nil)
			return false
		}
		return f.Set
	}
	if required("rrule", in.RRule) {
		next.RRule = c.rrule(in.RRule.Value)
	}
	if required("timezone", in.Timezone) {
		next.Timezone = c.timezone(in.Timezone.Value)
	}
	if required("starts_on", in.StartsOn) {
		next.StartsOn = c.date("starts_on", in.StartsOn.Value, now)
	}
	if in.EndsOn.Set {
		next.EndsOn = c.optDate("ends_on", in.EndsOn.Apply(nil), now)
	}
	if required("planned_volume_l", in.PlannedVolumeL) {
		next.PlannedVolumeL = c.volume("planned_volume_l", in.PlannedVolumeL.Value)
	}
	if in.SaltMix.Set {
		next.SaltMix = c.text("salt_mix", in.SaltMix.Apply(nil), 100, false)
	}
	if in.Notes.Set {
		next.Notes = c.text("notes", in.Notes.Apply(nil), 2000, true)
	}
	if required("status", in.Status) {
		switch in.Status.Value {
		case StatusActive, StatusPaused, StatusEnded:
			next.Status = in.Status.Value
		default:
			c.v.Add("status", "unsupported", nil)
		}
	}
	if next.EndsOn != nil && !c.v.HasErrors() && next.EndsOn.Before(next.StartsOn) {
		c.v.Add("ends_on", "before_start", nil)
	}
	return next
}

func sameDate(a, b *recurrence.Date) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// materialize creates the schedule's occurrences from where the previous pass stopped
// (or from now, so a new or reshaped schedule never starts with overdue rows) up to the
// horizon. Re-running is a no-op thanks to the (schedule_id, scheduled_for) unique index.
// A schedule whose rule or zone no longer loads (tzdata change) is logged and skipped up
// to the horizon so it doesn't hold back the job.
func (s *Service) materialize(ctx context.Context, q *dbq.Queries, sch Schedule, now time.Time) error {
	if sch.Status != StatusActive {
		return nil
	}
	to := now.Add(Horizon)
	series, err := sch.series()
	if err != nil {
		s.log.ErrorContext(ctx, "water-change schedule cannot be materialized", "schedule_id", sch.ID, "err", err)
		return q.SetWaterChangeMaterializedUntil(ctx, dbq.SetWaterChangeMaterializedUntilParams{ID: sch.ID, Until: to})
	}
	g := grace(series.Rule)
	from := now
	if sch.materializedUntil != nil && sch.materializedUntil.After(from.Add(-Horizon)) {
		from = *sch.materializedUntil
	}
	occurrences := series.Between(from, to, maxPerPass)
	for _, at := range occurrences {
		if _, err := q.InsertWaterChangeOccurrence(ctx, dbq.InsertWaterChangeOccurrenceParams{
			TankID: sch.TankID, ScheduleID: sch.ID, ScheduledFor: at.UTC(), OverdueAfter: at.Add(g).UTC(),
			PlannedVolumeL: sch.PlannedVolumeL, SaltMix: sch.SaltMix, Now: now,
		}); err != nil {
			return err
		}
	}
	until := to
	if len(occurrences) == maxPerPass { // continue after the last one next time
		until = occurrences[len(occurrences)-1].Add(time.Nanosecond)
	}
	return q.SetWaterChangeMaterializedUntil(ctx, dbq.SetWaterChangeMaterializedUntilParams{ID: sch.ID, Until: until})
}

func (s *Service) loadSchedule(ctx context.Context, q *dbq.Queries, id uuid.UUID, now time.Time) (Schedule, error) {
	row, err := q.GetWaterChangeSchedule(ctx, id)
	if err != nil {
		return Schedule{}, err
	}
	system, err := q.GetTankSystemVolume(ctx, row.TankID)
	if err != nil {
		return Schedule{}, err
	}
	return s.decorate(scheduleFromRow(row), system, now), nil
}

func (s *Service) decorate(sch Schedule, system decimal.Decimal, now time.Time) Schedule {
	vol := sch.PlannedVolumeL
	sch.Estimate = estimate(&vol, system, BasisPlanned)
	if sch.Status == StatusActive {
		if series, err := sch.series(); err == nil {
			sch.Next = series.Between(now, now.Add(400*24*time.Hour), 3)
		}
	}
	return sch
}

// --- events ---

// RecordInput is a water change entered directly: either already done (CompletedAt set)
// or planned once (ScheduledFor set).
type RecordInput struct {
	ScheduledFor   *time.Time
	PlannedVolumeL *string
	CompletedAt    *time.Time
	ActualVolumeL  *string
	SaltMix        *string
	Notes          *string
}

// Record adds a one-time planned water change or one done without a schedule.
func (s *Service) Record(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in RecordInput) (Event, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	p := dbq.CreateWaterChangeEventParams{TankID: tankID, RecordedBy: &a.UserID, Now: now}
	p.SaltMix = c.text("salt_mix", in.SaltMix, 100, false)
	p.Notes = c.text("notes", in.Notes, 2000, true)
	switch {
	case in.CompletedAt != nil && in.ScheduledFor != nil:
		v.Add("completed_at", "exactly_one_of", map[string]string{"fields": "completed_at,scheduled_for"})
	case in.CompletedAt != nil:
		c.instant("completed_at", *in.CompletedAt, now, maxFutureSkew)
		if in.ActualVolumeL == nil {
			v.Add("actual_volume_l", "required", nil)
		}
		p.Status, p.CompletedAt, p.ResolvedAt = EventCompleted, in.CompletedAt, &now
		p.ActualVolumeL = c.optVolume("actual_volume_l", in.ActualVolumeL)
		p.PlannedVolumeL = c.optVolume("planned_volume_l", in.PlannedVolumeL)
	case in.ScheduledFor != nil:
		c.instant("scheduled_for", *in.ScheduledFor, now, maxPlanAhead)
		after := in.ScheduledFor.Add(OverdueGrace)
		p.OverdueAfter = &after
		if in.PlannedVolumeL == nil {
			v.Add("planned_volume_l", "required", nil)
		}
		if in.ActualVolumeL != nil {
			v.Add("actual_volume_l", "not_allowed", nil)
		}
		p.Status, p.ScheduledFor = EventScheduled, in.ScheduledFor
		p.PlannedVolumeL = c.optVolume("planned_volume_l", in.PlannedVolumeL)
	default:
		v.Add("completed_at", "exactly_one_of", map[string]string{"fields": "completed_at,scheduled_for"})
	}
	if err := v.Err(); err != nil {
		return Event{}, err
	}
	var out Event
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		id, err := q.CreateWaterChangeEvent(ctx, p)
		if err != nil {
			return err
		}
		if p.Status == EventCompleted {
			if err := recordTimeline(ctx, q, now, tankID, id, "water_change.completed", *p.CompletedAt, a.UserID, p.ActualVolumeL); err != nil {
				return err
			}
		}
		out, err = s.loadEvent(ctx, q, id, now)
		return err
	})
	return out, err
}

// ListFilter selects events. Zero value: everything.
type ListFilter struct {
	Overdue, Upcoming, Completed, Skipped bool // none set means all
	From, To                              *time.Time
	Limit                                 int32
	Cursor                                *ListCursor
}

// ListCursor is the keyset of the event list.
type ListCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// List returns the tank's water changes, newest event time first (US-WC-2 AC1: overdue
// ones are listed as overdue).
func (s *Service) List(ctx context.Context, a tanks.Actor, tankID uuid.UUID, f ListFilter) ([]Event, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	all := !f.Overdue && !f.Upcoming && !f.Completed && !f.Skipped
	// Unbounded ranges and the first page use sentinels so every bound stays an index
	// condition under generic plans.
	from, to := minTime, maxTime
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}
	cursorAt, cursorID := maxTime, maxUUID
	if f.Cursor != nil {
		cursorAt, cursorID = f.Cursor.At, f.Cursor.ID
	}
	var rows []dbq.GetWaterChangeEventRow
	if !all && !f.Completed && !f.Skipped {
		list, err := q.ListScheduledWaterChangeEvents(ctx, dbq.ListScheduledWaterChangeEventsParams{
			TankID: tankID, WantOverdue: f.Overdue, WantUpcoming: f.Upcoming, Now: now,
			FromAt: from, ToAt: to, CursorAt: cursorAt, CursorID: cursorID, RowLimit: f.Limit,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range list {
			rows = append(rows, dbq.GetWaterChangeEventRow(r))
		}
	} else {
		list, err := q.ListWaterChangeEvents(ctx, dbq.ListWaterChangeEventsParams{
			TankID: tankID, WantCompleted: all || f.Completed, WantSkipped: all || f.Skipped,
			WantOverdue: all || f.Overdue, WantUpcoming: all || f.Upcoming, Now: now,
			FromAt: from, ToAt: to, CursorAt: cursorAt, CursorID: cursorID, RowLimit: f.Limit,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range list {
			rows = append(rows, dbq.GetWaterChangeEventRow(r))
		}
	}
	system, err := q.GetTankSystemVolume(ctx, tankID)
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = decorateEvent(eventFromRow(r), system, now)
	}
	return out, nil
}

var (
	minTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	maxUUID = uuid.Max
)

// CompleteInput resolves a scheduled change as done.
type CompleteInput struct {
	CompletedAt   *time.Time // default now
	ActualVolumeL string
	SaltMix       *string
	Notes         *string
}

// Complete marks a scheduled (possibly overdue) water change done and adds it to the
// timeline (US-WC-2). Repeating the same completion returns the stored result
// (idempotent); a different one after resolution is a conflict.
func (s *Service) Complete(ctx context.Context, a tanks.Actor, id uuid.UUID, in CompleteInput) (Event, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	at := now
	if in.CompletedAt != nil {
		at = *in.CompletedAt
		c.instant("completed_at", at, now, maxFutureSkew)
	}
	volume := c.volume("actual_volume_l", in.ActualVolumeL)
	salt := c.text("salt_mix", in.SaltMix, 100, false)
	notes := c.text("notes", in.Notes, 2000, true)
	if err := v.Err(); err != nil {
		return Event{}, err
	}
	return s.resolve(ctx, a, id, now, func(q *dbq.Queries, ev Event) (bool, error) {
		if ev.Status == EventCompleted { // a retry of the same request is satisfied
			same := ev.CompletedAt != nil && ev.ActualVolumeL != nil && ev.ActualVolumeL.Cmp(volume) == 0 &&
				(in.CompletedAt == nil || ev.CompletedAt.Equal(at)) && sameText(ev.SaltMix, salt) && sameText(ev.Notes, notes)
			return same, nil
		}
		if ev.Status != EventScheduled {
			return false, nil
		}
		n, err := q.CompleteWaterChangeEvent(ctx, dbq.CompleteWaterChangeEventParams{
			ID: id, CompletedAt: at, ActualVolumeL: volume, SaltMix: salt, Notes: notes, RecordedBy: &a.UserID, Now: now,
		})
		if err != nil || n == 0 {
			return false, err
		}
		return true, recordTimeline(ctx, q, now, ev.TankID, id, "water_change.completed", at, a.UserID, &volume)
	})
}

// Skip marks a scheduled (possibly overdue) water change as deliberately not done; a
// reason is required.
func (s *Service) Skip(ctx context.Context, a tanks.Actor, id uuid.UUID, reason string) (Event, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	r := checker{v}.text("reason", &reason, 500, true)
	if r == nil {
		v.Add("reason", "required", nil)
	}
	if err := v.Err(); err != nil {
		return Event{}, err
	}
	return s.resolve(ctx, a, id, now, func(q *dbq.Queries, ev Event) (bool, error) {
		if ev.Status == EventSkipped {
			return ev.SkipReason != nil && *ev.SkipReason == *r, nil
		}
		if ev.Status != EventScheduled {
			return false, nil
		}
		n, err := q.SkipWaterChangeEvent(ctx, dbq.SkipWaterChangeEventParams{ID: id, SkipReason: *r, RecordedBy: &a.UserID, Now: now})
		if err != nil || n == 0 {
			return false, err
		}
		return true, recordTimeline(ctx, q, now, ev.TankID, id, "water_change.skipped", *ev.ScheduledFor, a.UserID, nil)
	})
}

// resolve authorizes, locks the event row (so concurrent identical requests see each
// other's result), and maps "not satisfied" to already_resolved. fn reports whether the
// request is satisfied.
func (s *Service) resolve(ctx context.Context, a tanks.Actor, id uuid.UUID, now time.Time, fn func(*dbq.Queries, Event) (bool, error)) (Event, error) {
	var out Event
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := q.GetWaterChangeEventTank(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		row, err := q.LockWaterChangeEvent(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound() // removed by a schedule edit meanwhile
		}
		if err != nil {
			return err
		}
		ev := eventFromRow(dbq.GetWaterChangeEventRow(row))
		ok, err := fn(q, ev)
		if err != nil {
			return err
		}
		if !ok {
			return errAlreadyResolved
		}
		out, err = s.loadEvent(ctx, q, id, now)
		return err
	})
	return out, err
}

func recordTimeline(ctx context.Context, q *dbq.Queries, now time.Time, tankID, eventID uuid.UUID, eventType string, at time.Time, actor uuid.UUID, volume *decimal.Decimal) error {
	summary := map[string]any{}
	if volume != nil {
		summary["volume_l"] = volume.String()
	}
	_, err := timeline.Record(ctx, q, now, timeline.Event{
		TankID: tankID, OccurredAt: at, Category: timeline.CategoryWaterChange, EventType: eventType,
		SourceType: "water_change_event", SourceID: eventID, ActorID: &actor, Summary: summary,
	})
	return err
}

func (s *Service) loadEvent(ctx context.Context, q *dbq.Queries, id uuid.UUID, now time.Time) (Event, error) {
	row, err := q.GetWaterChangeEvent(ctx, id)
	if database.IsNoRows(err) {
		return Event{}, apperr.NotFound()
	}
	if err != nil {
		return Event{}, err
	}
	system, err := q.GetTankSystemVolume(ctx, row.TankID)
	if err != nil {
		return Event{}, err
	}
	return decorateEvent(eventFromRow(row), system, now), nil
}

// decorateEvent derives overdue and the estimate. Completed changes use the system volume
// captured at completion, so correcting the tank volume later never changes their history.
func decorateEvent(ev Event, currentSystem decimal.Decimal, now time.Time) Event {
	ev.Overdue = overdue(ev.Status, ev.OverdueAfter, now)
	if ev.Status == EventCompleted && ev.SystemVolumeL != nil {
		ev.Estimate = estimate(ev.ActualVolumeL, *ev.SystemVolumeL, BasisActual)
	} else {
		ev.Estimate = estimate(ev.PlannedVolumeL, currentSystem, BasisPlanned)
	}
	return ev
}

// --- jobs and erasure ---

const jobMaterialize = "waterchanges.materialize"

// RegisterJobs adds the hourly materialization of upcoming occurrences.
func (s *Service) RegisterJobs(r *jobs.Runner) {
	r.Handle(jobMaterialize, s.runMaterialize)
	r.Every(jobMaterialize, time.Hour)
}

// runMaterialize extends every active schedule whose occurrences end within a day of the
// horizon, so each schedule is touched about once a day. One transaction per schedule
// keeps locks short; a failing schedule doesn't block the others. If occurrences stopped
// more than a Horizon ago (long outage), materialization restarts at now instead of
// back-filling a backlog of overdue rows.
func (s *Service) runMaterialize(ctx context.Context, _ jobs.Job) error {
	const batch = 500
	now := s.clock.Now()
	q := dbq.New(s.pool)
	after := uuid.Nil
	var failed, total int
	for {
		ids, err := q.ListWaterChangeSchedulesToMaterialize(ctx, dbq.ListWaterChangeSchedulesToMaterializeParams{
			Horizon: now.Add(Horizon - 24*time.Hour), AfterID: after, Batch: batch,
		})
		if err != nil {
			return fmt.Errorf("list schedules: %w", err)
		}
		for _, id := range ids {
			total++
			err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
				q := dbq.New(tx)
				row, err := q.LockWaterChangeSchedule(ctx, id)
				if database.IsNoRows(err) {
					return nil // erased meanwhile
				}
				if err != nil {
					return err
				}
				if row.TankStatus == tanks.StatusArchived || row.OwnerStatus != "active" {
					return nil // changed since listed
				}
				return s.materialize(ctx, q, scheduleFromLocked(row), now)
			})
			if err != nil {
				failed++
				s.log.ErrorContext(ctx, "materialize water-change schedule", "schedule_id", id, "err", err)
			}
		}
		if len(ids) < batch {
			break
		}
		after = ids[len(ids)-1]
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d schedules failed to materialize", failed, total)
	}
	return nil
}

// OnTankArchived is the tanks archive hook: an archived tank is read-only, so its
// schedules end, upcoming occurrences are removed and changes still open on it
// (overdue ones included) are resolved as skipped with reason code "tank_archived".
func OnTankArchived(ctx context.Context, q *dbq.Queries, tankID, actorID uuid.UUID, now time.Time) error {
	ids, err := q.EndTankWaterChangeSchedules(ctx, dbq.EndTankWaterChangeSchedulesParams{TankID: tankID, Now: now})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := q.DeleteFutureWaterChangeOccurrences(ctx, dbq.DeleteFutureWaterChangeOccurrencesParams{ScheduleID: id, After: now}); err != nil {
			return err
		}
	}
	_, err = q.SkipTankScheduledWaterChanges(ctx, dbq.SkipTankScheduledWaterChangesParams{TankID: tankID, RecordedBy: &actorID, Now: now})
	return err
}

// EraseUserData deletes water changes of the erased account's tanks (identity.Eraser).
// It must run before tanks.EraseUserData, which deletes the tanks themselves.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.LockOwnerWaterChangeSchedules(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerWaterChangeEvents(ctx, userID); err != nil {
		return err
	}
	return q.DeleteOwnerWaterChangeSchedules(ctx, userID)
}

// --- row mapping ---

func scheduleFromRow(r dbq.GetWaterChangeScheduleRow) Schedule {
	s := Schedule{
		ID: r.ID, TankID: r.TankID, RRule: r.Rrule, Timezone: r.Timezone, StartsOn: timeToDate(r.StartsOn),
		PlannedVolumeL: r.PlannedVolumeL, SaltMix: r.SaltMix, Notes: r.Notes, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version, materializedUntil: r.MaterializedUntil,
	}
	if r.EndsOn != nil {
		d := timeToDate(*r.EndsOn)
		s.EndsOn = &d
	}
	return s
}

func scheduleFromLocked(r dbq.LockWaterChangeScheduleRow) Schedule {
	return scheduleFromRow(dbq.GetWaterChangeScheduleRow{
		ID: r.ID, TankID: r.TankID, Rrule: r.Rrule, Timezone: r.Timezone, StartsOn: r.StartsOn, EndsOn: r.EndsOn,
		PlannedVolumeL: r.PlannedVolumeL, SaltMix: r.SaltMix, Notes: r.Notes, Status: r.Status,
		MaterializedUntil: r.MaterializedUntil, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	})
}

func eventFromRow(r dbq.GetWaterChangeEventRow) Event {
	return Event{
		ID: r.ID, TankID: r.TankID, ScheduleID: r.ScheduleID, ScheduledFor: r.ScheduledFor, OverdueAfter: r.OverdueAfter,
		PlannedVolumeL: r.PlannedVolumeL, SystemVolumeL: r.SystemVolumeL,
		Status: r.Status, CompletedAt: r.CompletedAt, ActualVolumeL: r.ActualVolumeL, SaltMix: r.SaltMix, Notes: r.Notes,
		SkipReason: r.SkipReason, ResolvedAt: r.ResolvedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
}
