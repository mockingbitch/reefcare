package dosing

import (
	"bytes"
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
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/platform/jobs"
	"reefcare/internal/platform/patch"
	"reefcare/internal/platform/recurrence"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
)

// Service implements dosing workflows; tank data is authorized through tanks.Authorizer.
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
	errVersionConflict = apperr.Conflict("version_conflict", "The plan was changed elsewhere. Reload and try again.")
	errAlreadyResolved = apperr.Conflict("already_resolved", "This dose was already recorded differently.")
	errPlanCompleted   = apperr.Conflict("plan_completed", "This plan is completed and can no longer change.")
	errProductLimit    = apperr.Conflict("limit_reached", "You have reached the maximum number of products.")
	errPlanLimit       = apperr.Conflict("limit_reached", "This tank has the maximum number of dosing plans.")
	errKeyReused       = apperr.Invalid("idempotency_key_reused", "This Idempotency-Key was already used for a different request.")
)

// --- products ---

// ProductInput creates a private product.
type ProductInput struct {
	Name        string
	Brand       *string
	Form        string
	DefaultUnit string
	Notes       *string
}

// ListProducts returns the curated catalog and the caller's own products.
func (s *Service) ListProducts(ctx context.Context, a tanks.Actor) ([]Product, error) {
	rows, err := dbq.New(s.pool).ListDosingProducts(ctx, &a.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]Product, len(rows))
	for i, r := range rows {
		out[i] = productFromRow(r)
	}
	return out, nil
}

// CreateProduct adds a private product, usable only on the caller's tanks.
func (s *Service) CreateProduct(ctx context.Context, a tanks.Actor, in ProductInput) (Product, error) {
	v := apperr.NewValidator()
	c := checker{v}
	name := c.name("name", in.Name, 100)
	brand := c.text("brand", in.Brand, 100, false)
	notes := c.text("notes", in.Notes, 2000, true)
	if _, ok := unitsByForm[in.Form]; !ok {
		v.Add("form", "unsupported", map[string]string{"allowed": "liquid,powder,tablet,other"})
	} else {
		c.unit("default_unit", in.DefaultUnit)
		c.unitForForm("default_unit", in.DefaultUnit, in.Form)
	}
	if err := v.Err(); err != nil {
		return Product{}, err
	}
	var out Product
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockUserRow(ctx, a.UserID); err != nil { // serializes the limit check
			return err
		}
		n, err := q.CountOwnDosingProducts(ctx, &a.UserID)
		if err != nil {
			return err
		}
		if n >= MaxOwnProducts {
			return errProductLimit
		}
		row, err := q.CreateDosingProduct(ctx, dbq.CreateDosingProductParams{
			OwnerID: &a.UserID, Name: name, Brand: brand, Form: in.Form, DefaultUnit: in.DefaultUnit, Notes: notes, Now: s.clock.Now(),
		})
		out = productFromRow(row)
		return err
	})
	return out, err
}

// usableProduct loads a product the tank owner may dose: curated, or their own. Anything
// else is reported as not found so private products of others stay invisible.
func usableProduct(ctx context.Context, q *dbq.Queries, v *apperr.Validator, id uuid.UUID, owner uuid.UUID) (dbq.DosingProduct, bool, error) {
	p, err := q.GetDosingProduct(ctx, id)
	if err != nil && !database.IsNoRows(err) {
		return dbq.DosingProduct{}, false, err
	}
	if err != nil || (p.OwnerID != nil && *p.OwnerID != owner) {
		v.Add("product_id", "product_not_found", nil)
		return dbq.DosingProduct{}, false, nil
	}
	return p, true, nil
}

// --- plans ---

// PlanInput creates a plan.
type PlanInput struct {
	ProductID      uuid.UUID
	Amount         string
	Unit           string
	RRule          string
	Timezone       string
	StartsOn       string
	EndsOn         *string
	DeliveryMethod string // default manual
	Notes          *string
}

// CreatePlan adds a recurring dose and materializes its first occurrences.
func (s *Service) CreatePlan(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in PlanInput) (Plan, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	amount := c.amount("amount", in.Amount)
	unit := c.unit("amount_unit", in.Unit)
	rule := c.rrule(in.RRule)
	tz := c.timezone(in.Timezone)
	starts := c.date("starts_on", in.StartsOn, now)
	ends := c.optDate("ends_on", in.EndsOn, now)
	if ends != nil && !v.HasErrors() && ends.Before(starts) {
		v.Add("ends_on", "before_start", nil)
	}
	method := in.DeliveryMethod
	if method == "" {
		method = "manual"
	}
	v.Check(deliveryMethods[method], "delivery_method", "unsupported", map[string]string{"allowed": "manual,dosing_pump"})
	notes := c.text("notes", in.Notes, 2000, true)
	var out Plan
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, tankID); err != nil { // serializes the per-tank plan limit
			return err
		}
		acc, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite)
		if err != nil {
			return err
		}
		product, ok, err := usableProduct(ctx, q, v, in.ProductID, acc.OwnerID)
		if err != nil {
			return err
		}
		if ok {
			c.unitForForm("amount_unit", unit, product.Form)
		}
		if err := v.Err(); err != nil {
			return err
		}
		if n, err := q.CountTankDosingPlans(ctx, tankID); err != nil {
			return err
		} else if n >= MaxPlansPerTank {
			return errPlanLimit
		}
		p := dbq.CreateDosingPlanParams{
			TankID: tankID, ProductID: in.ProductID, Amount: amount, AmountUnit: unit, Rrule: rule, Timezone: tz,
			StartsOn: dateToTime(starts), DeliveryMethod: method, Notes: notes, CreatedBy: &a.UserID, Now: now,
		}
		if ends != nil {
			t := dateToTime(*ends)
			p.EndsOn = &t
		}
		id, err := q.CreateDosingPlan(ctx, p)
		if err != nil {
			return err
		}
		row, err := q.LockDosingPlan(ctx, id)
		if err != nil {
			return err
		}
		if err := s.materialize(ctx, q, planFromLocked(row), now); err != nil {
			return err
		}
		out, err = s.loadPlan(ctx, q, id, now)
		return err
	})
	return out, err
}

// ListPlans returns the tank's plans, oldest first.
func (s *Service) ListPlans(ctx context.Context, a tanks.Actor, tankID uuid.UUID) ([]Plan, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListDosingPlans(ctx, tankID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]Plan, len(rows))
	for i, r := range rows {
		out[i] = withNext(planFromRow(dbq.GetDosingPlanRow(r)), now)
	}
	return out, nil
}

// PlanPatch edits a plan (merge patch, Version required). Status changes use SetStatus.
type PlanPatch struct {
	Version        *int32
	Amount         patch.Field[string]
	Unit           patch.Field[string]
	RRule          patch.Field[string]
	Timezone       patch.Field[string]
	StartsOn       patch.Field[string]
	EndsOn         patch.Field[string]
	DeliveryMethod patch.Field[string]
	Notes          patch.Field[string]
}

// UpdatePlan edits a plan. Changes to amount, unit or timing replace the upcoming
// occurrences nobody acted on; missed and resolved doses are history and stay.
func (s *Service) UpdatePlan(ctx context.Context, a tanks.Actor, id uuid.UUID, in PlanPatch) (Plan, error) {
	now := s.clock.Now()
	return s.editPlan(ctx, a, id, now, false, func(c checker, cur Plan, form string) (Plan, error) {
		if in.Version == nil {
			c.v.Add("version", "required", nil)
			return cur, c.v.Err()
		}
		if *in.Version != cur.Version {
			return cur, errVersionConflict
		}
		next := cur
		required := func(field string, f patch.Field[string]) bool {
			if f.Set && f.Null {
				c.v.Add(field, "required", nil)
				return false
			}
			return f.Set
		}
		if required("amount", in.Amount) {
			next.Amount = c.amount("amount", in.Amount.Value)
		}
		if required("amount_unit", in.Unit) {
			next.Unit = c.unit("amount_unit", in.Unit.Value)
		}
		c.unitForForm("amount_unit", next.Unit, form)
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
		if required("delivery_method", in.DeliveryMethod) {
			next.DeliveryMethod = in.DeliveryMethod.Value
			c.v.Check(deliveryMethods[next.DeliveryMethod], "delivery_method", "unsupported", nil)
		}
		if in.Notes.Set {
			next.Notes = c.text("notes", in.Notes.Apply(nil), 2000, true)
		}
		if next.EndsOn != nil && !c.v.HasErrors() && next.EndsOn.Before(next.StartsOn) {
			c.v.Add("ends_on", "before_start", nil)
		}
		return next, c.v.Err()
	})
}

// SetStatus pauses, resumes or completes a plan. Repeating the current status is a no-op;
// completed is terminal.
func (s *Service) SetStatus(ctx context.Context, a tanks.Actor, id uuid.UUID, status string) (Plan, error) {
	now := s.clock.Now()
	// Completing a completed plan again is a no-op; any other change to it is a conflict.
	return s.editPlan(ctx, a, id, now, status == PlanCompleted, func(_ checker, cur Plan, _ string) (Plan, error) {
		next := cur
		next.Status = status
		return next, nil
	})
}

// editPlan authorizes, locks the plan, applies fn and, when the occurrences' shape
// changed, replaces the upcoming ones and materializes again from now.
func (s *Service) editPlan(ctx context.Context, a tanks.Actor, id uuid.UUID, now time.Time, completeAgainOK bool, fn func(checker, Plan, string) (Plan, error)) (Plan, error) {
	var out Plan
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		cur, err := q.GetDosingPlan(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, cur.TankID, tanks.PermWrite); err != nil {
			return err
		}
		row, err := q.LockDosingPlan(ctx, id)
		if err != nil {
			return err
		}
		plan := planFromLocked(row)
		plan.Product = ProductRef{ID: cur.ProductID, Name: cur.ProductName, Brand: cur.ProductBrand, Form: cur.ProductForm}
		if plan.Status == PlanCompleted {
			if completeAgainOK {
				out, err = s.loadPlan(ctx, q, id, now)
				return err
			}
			return errPlanCompleted
		}
		next, err := fn(checker{apperr.NewValidator()}, plan, cur.ProductForm)
		if err != nil {
			return err
		}
		reshape := next.Amount.Cmp(plan.Amount) != 0 || next.Unit != plan.Unit || next.RRule != plan.RRule ||
			next.Timezone != plan.Timezone || next.StartsOn != plan.StartsOn || !sameDate(next.EndsOn, plan.EndsOn) ||
			next.Status != plan.Status
		if !reshape && next.DeliveryMethod == plan.DeliveryMethod && sameText(next.Notes, plan.Notes) {
			out, err = s.loadPlan(ctx, q, id, now) // nothing to change
			return err
		}
		until := plan.materializedUntil
		if reshape {
			until = nil
			// Also drop doses still inside their grace that nobody recorded: after a timing
			// change or a pause they would otherwise stay open next to the new ones (a
			// double-dose risk) or turn "missed" under a plan the keeper just paused.
			after := now.Add(-MissedGrace)
			if series, err := plan.series(); err == nil {
				after = now.Add(-grace(series.Rule))
			}
			if _, err := q.DeleteFutureDosingOccurrences(ctx, dbq.DeleteFutureDosingOccurrencesParams{PlanID: id, After: after}); err != nil {
				return err
			}
		}
		p := dbq.UpdateDosingPlanParams{
			ID: id, Version: plan.Version, Amount: next.Amount, AmountUnit: next.Unit, Rrule: next.RRule,
			Timezone: next.Timezone, StartsOn: dateToTime(next.StartsOn), DeliveryMethod: next.DeliveryMethod,
			Notes: next.Notes, Status: next.Status, MaterializedUntil: until, Now: now,
		}
		if next.EndsOn != nil {
			t := dateToTime(*next.EndsOn)
			p.EndsOn = &t
		}
		n, err := q.UpdateDosingPlan(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 {
			return errVersionConflict
		}
		if reshape && next.Status == PlanActive {
			next.ID, next.TankID, next.materializedUntil = id, plan.TankID, nil
			if err := s.materialize(ctx, q, next, now); err != nil {
				return err
			}
		}
		out, err = s.loadPlan(ctx, q, id, now)
		return err
	})
	return out, err
}

func sameDate(a, b *recurrence.Date) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// materialize creates occurrences from where the previous pass stopped (or from now) to
// the horizon; idempotent through the (plan_id, scheduled_for) unique index.
func (s *Service) materialize(ctx context.Context, q *dbq.Queries, p Plan, now time.Time) error {
	if p.Status != PlanActive {
		return nil
	}
	to := now.Add(Horizon)
	series, err := p.series()
	if err != nil {
		s.log.ErrorContext(ctx, "dosing plan cannot be materialized", "plan_id", p.ID, "err", err)
		return q.SetDosingMaterializedUntil(ctx, dbq.SetDosingMaterializedUntilParams{ID: p.ID, Until: to})
	}
	g := grace(series.Rule)
	from := now
	if p.materializedUntil != nil && p.materializedUntil.After(from.Add(-Horizon)) {
		from = *p.materializedUntil
	}
	occurrences := series.Between(from, to, maxPerPass)
	for _, at := range occurrences {
		if _, err := q.InsertDosingOccurrence(ctx, dbq.InsertDosingOccurrenceParams{
			TankID: p.TankID, PlanID: p.ID, ProductID: p.Product.ID, ScheduledFor: at.UTC(), MissedAfter: at.Add(g).UTC(),
			PlannedAmount: p.Amount, AmountUnit: p.Unit, Now: now,
		}); err != nil {
			return err
		}
	}
	until := to
	if len(occurrences) == maxPerPass {
		until = occurrences[len(occurrences)-1].Add(time.Nanosecond)
	}
	return q.SetDosingMaterializedUntil(ctx, dbq.SetDosingMaterializedUntilParams{ID: p.ID, Until: until})
}

func (s *Service) loadPlan(ctx context.Context, q *dbq.Queries, id uuid.UUID, now time.Time) (Plan, error) {
	row, err := q.GetDosingPlan(ctx, id)
	if err != nil {
		return Plan{}, err
	}
	return withNext(planFromRow(row), now), nil
}

func withNext(p Plan, now time.Time) Plan {
	if p.Status == PlanActive {
		if series, err := p.series(); err == nil {
			p.Next = series.Between(now, now.Add(400*24*time.Hour), 3)
		}
	}
	return p
}

// --- events ---

// HistoryFilter selects dose events; empty Statuses means all.
type HistoryFilter struct {
	Statuses []string
	From, To *time.Time
	Limit    int32
	Cursor   *HistoryCursor
}

// HistoryCursor is the keyset of the history.
type HistoryCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

var (
	minTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
)

// History lists a tank's doses, newest event time first.
func (s *Service) History(ctx context.Context, a tanks.Actor, tankID uuid.UUID, f HistoryFilter) ([]Event, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	p := dbq.ListDosingEventsParams{
		TankID: tankID, Statuses: f.Statuses, FromAt: minTime, ToAt: maxTime, CursorAt: maxTime, CursorID: uuid.Max, RowLimit: f.Limit,
	}
	if len(p.Statuses) == 0 {
		p.Statuses = AllStatuses
	}
	if f.From != nil {
		p.FromAt = *f.From
	}
	if f.To != nil {
		p.ToAt = *f.To
	}
	if f.Cursor != nil {
		p.CursorAt, p.CursorID = f.Cursor.At, f.Cursor.ID
	}
	rows, err := q.ListDosingEvents(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = eventFromRow(dbq.GetDosingEventRow(r))
	}
	return out, nil
}

// ResolveInput records what happened to a scheduled, missed or unconfirmed dose.
type ResolveInput struct {
	Status       string // completed, partial, skipped, failed
	ActualAmount *string
	ExecutedAt   *time.Time // default now for completed/partial of a dose not yet late
	Reason       *string
	Notes        *string
	// ConfirmOverPlanned must be true to record more than 1.5× the planned amount.
	ConfirmOverPlanned bool
}

// Resolve records the outcome of a dose. Rules (domain review 2026-10-30):
//   - completed means the planned amount was given (actual defaults to planned, recorded as
//     actual_source=planned_default); less is partial; more than 1.5× needs confirmation;
//   - a missed or unconfirmed dose resolved as given needs an explicit executed_at (did it
//     happen on time and was not logged, or is it being given late?), never "now";
//   - executed_at may not be more than a day before the scheduled time;
//   - failed may record the amount that went in.
//
// Repeating the same outcome returns the stored event (idempotent); a different one after
// resolution is a conflict. Resolving a missed/unconfirmed dose is marked as a correction on
// the timeline.
func (s *Service) Resolve(ctx context.Context, a tanks.Actor, id uuid.UUID, in ResolveInput) (Event, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	var actual *decimal.Decimal
	if in.ActualAmount != nil {
		d := c.amount("actual_amount", *in.ActualAmount)
		actual = &d
	}
	reason := c.text("reason", in.Reason, 500, true)
	notes := c.text("notes", in.Notes, 2000, true)
	if in.ExecutedAt != nil {
		c.instant("executed_at", *in.ExecutedAt, now)
	}
	switch in.Status {
	case Completed, Partial:
		if in.Status == Partial && actual == nil {
			v.Add("actual_amount", "required", nil)
		}
		if reason != nil {
			v.Add("reason", "not_allowed", nil)
		}
	case Skipped, Failed:
		if reason == nil {
			v.Add("reason", "required", nil)
		}
		if in.Status == Skipped && actual != nil {
			v.Add("actual_amount", "not_allowed", nil)
		}
		if in.ExecutedAt != nil {
			v.Add("executed_at", "not_allowed", nil)
		}
	default:
		v.Add("status", "unsupported", map[string]string{"allowed": "completed,partial,skipped,failed"})
	}
	if err := v.Err(); err != nil {
		return Event{}, err
	}
	var out Event
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := q.GetDosingEventTank(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		row, err := q.LockDosingEvent(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		given := in.Status == Completed || in.Status == Partial
		amount, source := actual, "entered"
		if in.Status == Completed && amount == nil {
			amount, source = row.PlannedAmount, "planned_default"
		}
		var src *string
		if amount != nil {
			src = &source
		}
		if planned := row.PlannedAmount; planned != nil && amount != nil && given {
			switch {
			case in.Status == Completed && amount.Cmp(*planned) < 0:
				v.Add("status", "use_partial", map[string]string{"planned": planned.String()})
			case in.Status == Partial && amount.Cmp(*planned) >= 0:
				v.Add("actual_amount", "not_less_than_planned", map[string]string{"planned": planned.String()})
			case amount.Cmp(mul(*planned, overConfirm)) > 0 && !in.ConfirmOverPlanned:
				v.Add("actual_amount", "confirm_over_planned", map[string]string{"planned": planned.String()})
			}
		}
		late := row.Status == Missed || row.Status == Unconfirmed
		executed := in.ExecutedAt
		if given && executed == nil {
			if late {
				v.Add("executed_at", "required_for_late_dose", nil)
			} else {
				executed = &now
			}
		}
		if executed != nil && row.ScheduledFor != nil && executed.Before(row.ScheduledFor.Add(-lateWindow)) {
			v.Add("executed_at", "before_scheduled", nil)
		}
		if row.Status != Scheduled && !late { // already resolved: same request is a no-op
			same := row.Status == in.Status && sameAmount(row.ActualAmount, amount) && sameText(row.Reason, reason) &&
				sameText(row.Notes, notes) && (in.ExecutedAt == nil || (row.ExecutedAt != nil && row.ExecutedAt.Equal(*in.ExecutedAt)))
			if !same {
				return errAlreadyResolved
			}
			out, err = s.loadEvent(ctx, q, id)
			return err
		}
		if err := v.Err(); err != nil {
			return err
		}
		if !given {
			executed = nil
		}
		if _, err := q.ResolveDosingEvent(ctx, dbq.ResolveDosingEventParams{
			ID: id, Status: in.Status, ActualAmount: amount, ActualSource: src, ExecutedAt: executed, Reason: reason,
			Notes: notes, RecordedBy: &a.UserID, Now: now,
		}); err != nil {
			return err
		}
		occurred := now
		if executed != nil {
			occurred = *executed
		} else if row.ScheduledFor != nil {
			occurred = *row.ScheduledFor
		}
		extra := map[string]any{}
		if late {
			extra["previous_status"], extra["corrected_at"] = row.Status, now.UTC().Format(time.RFC3339)
		}
		if d := deviation(row.PlannedAmount, amount); d != "" {
			extra["deviation"] = d
		}
		if err := recordTimeline(ctx, q, now, tankID, id, "dosing."+in.Status, occurred, &a.UserID, amount, row.AmountUnit, extra); err != nil {
			return err
		}
		out, err = s.loadEvent(ctx, q, id)
		return err
	})
	return out, err
}

func mul(d decimal.Decimal, factor string) decimal.Decimal { return d.Mul(decimal.MustParse(factor)) }

// AdHocInput is a dose given without a plan.
type AdHocInput struct {
	ProductID  uuid.UUID
	Amount     string
	Unit       string
	ExecutedAt *time.Time // default now
	Notes      *string
}

// RecordAdHoc stores a dose given without a plan. With an idempotency key (the
// Idempotency-Key header) a retry returns the first dose instead of a duplicate; the same
// key with a different request is rejected. created reports whether a new dose was stored.
func (s *Service) RecordAdHoc(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in AdHocInput, key *string, requestHash []byte) (ev Event, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	amount := c.amount("amount", in.Amount)
	unit := c.unit("amount_unit", in.Unit)
	at := now
	if in.ExecutedAt != nil {
		at = *in.ExecutedAt
		c.instant("executed_at", at, now)
	}
	notes := c.text("notes", in.Notes, 2000, true)
	if key != nil && !idempotency.Valid(*key) {
		v.Add("Idempotency-Key", "invalid", map[string]string{"format": "8-128 printable ASCII characters"})
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		acc, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite)
		if err != nil {
			return err
		}
		product, ok, err := usableProduct(ctx, q, v, in.ProductID, acc.OwnerID)
		if err != nil {
			return err
		}
		if ok {
			c.unitForForm("amount_unit", unit, product.Form)
		}
		if err := v.Err(); err != nil {
			return err
		}
		var hash []byte
		if key != nil {
			hash = requestHash
		}
		id, err := q.CreateAdHocDose(ctx, dbq.CreateAdHocDoseParams{
			TankID: tankID, ProductID: in.ProductID, ActualAmount: amount, AmountUnit: unit, ExecutedAt: at,
			Notes: notes, Now: now, RecordedBy: &a.UserID, IdempotencyKey: key, RequestHash: hash,
		})
		if database.IsNoRows(err) && key != nil { // a retry: return the first dose
			first, err := q.GetDosingEventByIdempotencyKey(ctx, dbq.GetDosingEventByIdempotencyKeyParams{RecordedBy: &a.UserID, IdempotencyKey: key})
			if err != nil {
				return err
			}
			if first.TankID != tankID || !bytes.Equal(first.RequestHash, requestHash) {
				return errKeyReused
			}
			ev, err = s.loadEvent(ctx, q, first.ID)
			return err
		}
		if err != nil {
			return err
		}
		created = true
		if err := recordTimeline(ctx, q, now, tankID, id, "dosing.completed", at, &a.UserID, &amount, unit, nil); err != nil {
			return err
		}
		ev, err = s.loadEvent(ctx, q, id)
		return err
	})
	return ev, created, err
}

func recordTimeline(ctx context.Context, q *dbq.Queries, now time.Time, tankID, eventID uuid.UUID, eventType string, at time.Time, actor *uuid.UUID, amount *decimal.Decimal, unit string, extra map[string]any) error {
	summary := map[string]any{}
	for k, v := range extra {
		summary[k] = v
	}
	if amount != nil {
		summary["amount"], summary["unit"] = amount.String(), unit
	}
	_, err := timeline.Record(ctx, q, now, timeline.Event{
		TankID: tankID, OccurredAt: at, Category: timeline.CategoryDosing, EventType: eventType,
		SourceType: "dosing_event", SourceID: eventID, ActorID: actor, Summary: summary,
	})
	return err
}

func (s *Service) loadEvent(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Event, error) {
	row, err := q.GetDosingEvent(ctx, id)
	if database.IsNoRows(err) {
		return Event{}, apperr.NotFound()
	}
	if err != nil {
		return Event{}, err
	}
	return eventFromRow(row), nil
}

// --- jobs, archive and erasure ---

const (
	jobMaterialize = "dosing.materialize"
	jobMarkMissed  = "dosing.mark_missed"
)

// RegisterJobs adds hourly materialization and the 15-minute missed-dose scheduler.
func (s *Service) RegisterJobs(r *jobs.Runner) {
	r.Handle(jobMaterialize, s.runMaterialize)
	r.Every(jobMaterialize, time.Hour)
	r.Handle(jobMarkMissed, s.runMarkMissed)
	r.Every(jobMarkMissed, 15*time.Minute)
}

func (s *Service) runMaterialize(ctx context.Context, _ jobs.Job) error {
	const batch = 500
	now := s.clock.Now()
	q := dbq.New(s.pool)
	after := uuid.Nil
	var failed, total int
	for {
		ids, err := q.ListDosingPlansToMaterialize(ctx, dbq.ListDosingPlansToMaterializeParams{
			Horizon: now.Add(Horizon - 6*time.Hour), AfterID: after, Batch: batch,
		})
		if err != nil {
			return fmt.Errorf("list plans: %w", err)
		}
		for _, id := range ids {
			total++
			err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
				q := dbq.New(tx)
				row, err := q.LockDosingPlan(ctx, id)
				if database.IsNoRows(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if row.TankStatus == tanks.StatusArchived || row.OwnerStatus != "active" {
					return nil
				}
				return s.materialize(ctx, q, planFromLocked(row), now)
			})
			if err != nil {
				failed++
				s.log.ErrorContext(ctx, "materialize dosing plan", "plan_id", id, "err", err)
			}
		}
		if len(ids) < batch {
			break
		}
		after = ids[len(ids)-1]
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d plans failed to materialize", failed, total)
	}
	return nil
}

// runMarkMissed turns late scheduled doses into missed ones (manual plans, recorded on the
// timeline) or unconfirmed ones (pump plans, not on the timeline). It never completes
// anything.
func (s *Service) runMarkMissed(ctx context.Context, _ jobs.Job) error {
	const batch = 500
	for {
		now := s.clock.Now()
		var n int
		err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
			q := dbq.New(tx)
			rows, err := q.MarkMissedDosingEvents(ctx, dbq.MarkMissedDosingEventsParams{Now: now, Batch: batch})
			if err != nil {
				return err
			}
			n = len(rows)
			for _, r := range rows {
				if r.Status != Missed {
					continue // unconfirmed pump doses don't flood the timeline
				}
				if err := recordTimeline(ctx, q, now, r.TankID, r.ID, "dosing.missed", *r.ScheduledFor, nil, nil, "", nil); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("mark missed doses: %w", err)
		}
		if n < batch {
			return nil
		}
	}
}

// OnTankArchived is the tanks archive hook: plans complete, upcoming occurrences go and
// still-scheduled doses are resolved as skipped with reason code "tank_archived".
func OnTankArchived(ctx context.Context, q *dbq.Queries, tankID, actorID uuid.UUID, now time.Time) error {
	ids, err := q.EndTankDosingPlans(ctx, dbq.EndTankDosingPlansParams{TankID: tankID, Now: now})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := q.DeleteFutureDosingOccurrences(ctx, dbq.DeleteFutureDosingOccurrencesParams{PlanID: id, After: now}); err != nil {
			return err
		}
	}
	_, err = q.SkipTankScheduledDoses(ctx, dbq.SkipTankScheduledDosesParams{TankID: tankID, RecordedBy: &actorID, Now: now})
	return err
}

// EraseUserData deletes the erased account's dosing data (identity.Eraser); it must run
// before tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.LockOwnerDosingPlans(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerDosingEvents(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerDosingPlans(ctx, userID); err != nil {
		return err
	}
	return q.DeleteOwnerDosingProducts(ctx, &userID)
}

// --- row mapping ---

func productFromRow(r dbq.DosingProduct) Product {
	return Product{ID: r.ID, OwnerID: r.OwnerID, Name: r.Name, Brand: r.Brand, Form: r.Form, DefaultUnit: r.DefaultUnit, Notes: r.Notes, CreatedAt: r.CreatedAt}
}

func planFromRow(r dbq.GetDosingPlanRow) Plan {
	p := Plan{
		ID: r.ID, TankID: r.TankID, Product: ProductRef{ID: r.ProductID, Name: r.ProductName, Brand: r.ProductBrand, Form: r.ProductForm},
		Amount: r.Amount, Unit: r.AmountUnit, RRule: r.Rrule, Timezone: r.Timezone, StartsOn: timeToDate(r.StartsOn),
		DeliveryMethod: r.DeliveryMethod, Status: r.Status, Notes: r.Notes, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Version: r.Version, materializedUntil: r.MaterializedUntil,
	}
	if r.EndsOn != nil {
		d := timeToDate(*r.EndsOn)
		p.EndsOn = &d
	}
	return p
}

func planFromLocked(r dbq.LockDosingPlanRow) Plan {
	p := Plan{
		ID: r.ID, TankID: r.TankID, Product: ProductRef{ID: r.ProductID}, Amount: r.Amount, Unit: r.AmountUnit,
		RRule: r.Rrule, Timezone: r.Timezone, StartsOn: timeToDate(r.StartsOn), DeliveryMethod: r.DeliveryMethod,
		Status: r.Status, Notes: r.Notes, Version: r.Version, materializedUntil: r.MaterializedUntil,
	}
	if r.EndsOn != nil {
		d := timeToDate(*r.EndsOn)
		p.EndsOn = &d
	}
	return p
}

func eventFromRow(r dbq.GetDosingEventRow) Event {
	return Event{
		ID: r.ID, TankID: r.TankID, PlanID: r.PlanID, Product: ProductRef{ID: r.ProductID, Name: r.ProductName},
		ScheduledFor: r.ScheduledFor, MissedAfter: r.MissedAfter, PlannedAmount: r.PlannedAmount, ActualAmount: r.ActualAmount,
		ActualSource: r.ActualSource, Unit: r.AmountUnit, Status: r.Status, ExecutedAt: r.ExecutedAt, Reason: r.Reason,
		ReasonCode: r.ReasonCode, Notes: r.Notes,
		ReportedBy: r.ReportedBy, MissedAt: r.MissedAt, ResolvedAt: r.ResolvedAt, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
}
