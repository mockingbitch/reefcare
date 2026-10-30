package equipment

import (
	"bytes"
	"context"
	"encoding/json"
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
	"reefcare/internal/platform/idempotency"
	"reefcare/internal/platform/patch"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
)

// Service implements equipment and maintenance workflows; tank data is authorized through
// tanks.Authorizer (reads PermRead, writes PermWrite).
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
	errVersionConflict = apperr.Conflict("version_conflict", "This was changed elsewhere. Reload and try again.")
	errRemoved         = apperr.Conflict("equipment_removed", "This equipment was removed and can no longer change.")
	errEquipmentLimit  = apperr.Conflict("limit_reached", "This tank has the maximum number of devices.")
	errPlanLimit       = apperr.Conflict("limit_reached", "This tank has the maximum number of maintenance plans.")
	errKeyReused       = apperr.Invalid("idempotency_key_reused", "This Idempotency-Key was already used for a different request.")
)

// --- equipment ---

// DeviceInput adds a device.
type DeviceInput struct {
	Category      string
	Name          string
	Manufacturer  *string
	Model         *string
	SerialNumber  *string
	PurchasedOn   *string
	WarrantyUntil *string
	InstalledAt   *time.Time // default now
	Notes         *string
}

// AddEquipment adds a device to the tank and records equipment.installed on the timeline.
func (s *Service) AddEquipment(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in DeviceInput) (Equipment, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	p := dbq.CreateEquipmentParams{
		TankID: tankID, Category: c.category(in.Category), Name: c.name("name", in.Name, 100),
		Manufacturer: c.text("manufacturer", in.Manufacturer, 100, false), Model: c.text("model", in.Model, 100, false),
		SerialNumber: c.text("serial_number", in.SerialNumber, 100, false), Notes: c.text("notes", in.Notes, 2000, true),
		CreatedBy: &a.UserID, Now: now, InstalledAt: now,
	}
	purchased := c.optDate("purchased_on", in.PurchasedOn, now)
	warranty := c.optDate("warranty_until", in.WarrantyUntil, now)
	if purchased != nil && warranty != nil && !v.HasErrors() && warranty.Before(*purchased) {
		v.Add("warranty_until", "before_purchase", nil)
	}
	p.PurchasedOn, p.WarrantyUntil = optDateToTime(purchased), optDateToTime(warranty)
	if in.InstalledAt != nil {
		c.instant("installed_at", *in.InstalledAt, now)
		p.InstalledAt = *in.InstalledAt
	}
	if err := v.Err(); err != nil {
		return Equipment{}, err
	}
	var out Equipment
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, tankID); err != nil { // serializes the per-tank limit
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		if n, err := q.CountTankEquipment(ctx, tankID); err != nil {
			return err
		} else if n >= MaxEquipmentPerTank {
			return errEquipmentLimit
		}
		id, err := q.CreateEquipment(ctx, p)
		if err != nil {
			return err
		}
		if err := record(ctx, q, now, tankID, "tank_equipment", id, "equipment.installed", p.InstalledAt, a.UserID,
			map[string]any{"name": p.Name, "category": p.Category}); err != nil {
			return err
		}
		out, err = s.loadEquipment(ctx, q, id)
		return err
	})
	return out, err
}

// ListEquipment returns the tank's devices, newest installation first.
func (s *Service) ListEquipment(ctx context.Context, a tanks.Actor, tankID uuid.UUID, includeRemoved bool) ([]Equipment, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListEquipment(ctx, dbq.ListEquipmentParams{TankID: tankID, IncludeRemoved: includeRemoved})
	if err != nil {
		return nil, err
	}
	out := make([]Equipment, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = equipmentFromRow(dbq.GetEquipmentRow(r))
		ids[i] = r.ID
	}
	return out, attachConfigs(ctx, q, out, ids)
}

// attachConfigs fills Current for each device with one query.
func attachConfigs(ctx context.Context, q *dbq.Queries, list []Equipment, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.ListCurrentConfigurations(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]Configuration, len(rows))
	for _, r := range rows {
		byID[r.EquipmentID] = configFromRow(dbq.ListConfigurationsRow(r))
	}
	for i := range list {
		if c, ok := byID[list[i].ID]; ok {
			list[i].Current = &c
		}
	}
	return nil
}

// GetEquipment returns one device.
func (s *Service) GetEquipment(ctx context.Context, a tanks.Actor, id uuid.UUID) (Equipment, error) {
	q := dbq.New(s.pool)
	if _, err := s.requireEquipment(ctx, q, a, id, tanks.PermRead); err != nil {
		return Equipment{}, err
	}
	return s.loadEquipment(ctx, q, id)
}

func (s *Service) requireEquipment(ctx context.Context, q *dbq.Queries, a tanks.Actor, id uuid.UUID, perm tanks.Perm) (uuid.UUID, error) {
	tankID, err := q.GetEquipmentTank(ctx, id)
	if database.IsNoRows(err) {
		return uuid.Nil, apperr.NotFound()
	}
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.authz.Require(ctx, q, a.UserID, tankID, perm)
	return tankID, err
}

// DevicePatch edits a device (merge patch, Version required).
type DevicePatch struct {
	Version       *int32
	Category      patch.Field[string]
	Name          patch.Field[string]
	Manufacturer  patch.Field[string]
	Model         patch.Field[string]
	SerialNumber  patch.Field[string]
	PurchasedOn   patch.Field[string]
	WarrantyUntil patch.Field[string]
	Notes         patch.Field[string]
}

// UpdateEquipment edits a device that is still installed. installed_at is fixed at creation:
// the timeline and later configurations and maintenance are placed against it (open item EQ5).
func (s *Service) UpdateEquipment(ctx context.Context, a tanks.Actor, id uuid.UUID, in DevicePatch) (Equipment, error) {
	now := s.clock.Now()
	var out Equipment
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := s.requireEquipment(ctx, q, a, id, tanks.PermWrite); err != nil {
			return err
		}
		lock, err := q.LockEquipment(ctx, id)
		if err != nil {
			return err
		}
		if lock.RemovedAt != nil {
			return errRemoved
		}
		v := apperr.NewValidator()
		if in.Version == nil {
			v.Add("version", "required", nil)
			return v.Err()
		}
		if *in.Version != lock.Version {
			return errVersionConflict
		}
		cur, err := s.loadEquipment(ctx, q, id)
		if err != nil {
			return err
		}
		c := checker{v}
		required := func(field string, set, null bool) bool {
			if set && null {
				v.Add(field, "required", nil)
				return false
			}
			return set
		}
		p := dbq.UpdateEquipmentParams{
			ID: id, Version: lock.Version, Category: cur.Category, Name: cur.Name, Manufacturer: cur.Manufacturer,
			Model: cur.Model, SerialNumber: cur.SerialNumber, PurchasedOn: optDateToTime(cur.PurchasedOn),
			WarrantyUntil: optDateToTime(cur.WarrantyUntil), Notes: cur.Notes, Now: now,
		}
		if required("category", in.Category.Set, in.Category.Null) {
			p.Category = c.category(in.Category.Value)
		}
		if required("name", in.Name.Set, in.Name.Null) {
			p.Name = c.name("name", in.Name.Value, 100)
		}
		if in.Manufacturer.Set {
			p.Manufacturer = c.text("manufacturer", in.Manufacturer.Apply(nil), 100, false)
		}
		if in.Model.Set {
			p.Model = c.text("model", in.Model.Apply(nil), 100, false)
		}
		if in.SerialNumber.Set {
			p.SerialNumber = c.text("serial_number", in.SerialNumber.Apply(nil), 100, false)
		}
		if in.PurchasedOn.Set {
			p.PurchasedOn = optDateToTime(c.optDate("purchased_on", in.PurchasedOn.Apply(nil), now))
		}
		if in.WarrantyUntil.Set {
			p.WarrantyUntil = optDateToTime(c.optDate("warranty_until", in.WarrantyUntil.Apply(nil), now))
		}
		if in.Notes.Set {
			p.Notes = c.text("notes", in.Notes.Apply(nil), 2000, true)
		}
		if p.PurchasedOn != nil && p.WarrantyUntil != nil && !v.HasErrors() && p.WarrantyUntil.Before(*p.PurchasedOn) {
			v.Add("warranty_until", "before_purchase", nil)
		}
		if err := v.Err(); err != nil {
			return err
		}
		n, err := q.UpdateEquipment(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 {
			return errVersionConflict
		}
		out, err = s.loadEquipment(ctx, q, id)
		return err
	})
	return out, err
}

// RemoveEquipment marks a device removed (it stays in history), deactivates its
// maintenance plans and records equipment.removed. Removing it again is a no-op.
func (s *Service) RemoveEquipment(ctx context.Context, a tanks.Actor, id uuid.UUID, at *time.Time, reason *string) (Equipment, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	removedAt := now
	if at != nil {
		c.instant("removed_at", *at, now)
		removedAt = *at
	}
	why := c.text("reason", reason, 500, true)
	if err := v.Err(); err != nil {
		return Equipment{}, err
	}
	var out Equipment
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := s.requireEquipment(ctx, q, a, id, tanks.PermWrite)
		if err != nil {
			return err
		}
		lock, err := q.LockEquipment(ctx, id)
		if err != nil {
			return err
		}
		if lock.RemovedAt == nil {
			cur, err := s.loadEquipment(ctx, q, id)
			if err != nil {
				return err
			}
			if removedAt.Before(cur.InstalledAt) {
				v.Add("removed_at", "before_installed", nil)
				return v.Err()
			}
			if _, err := q.RemoveEquipment(ctx, dbq.RemoveEquipmentParams{ID: id, RemovedAt: removedAt, RemovalReason: why, Now: now}); err != nil {
				return err
			}
			if err := q.DeactivateEquipmentMaintenancePlans(ctx, dbq.DeactivateEquipmentMaintenancePlansParams{EquipmentID: id, Now: now}); err != nil {
				return err
			}
			if err := record(ctx, q, now, tankID, "tank_equipment", id, "equipment.removed", removedAt, a.UserID,
				removedSummary(cur.Name, cur.Category, why)); err != nil {
				return err
			}
		}
		out, err = s.loadEquipment(ctx, q, id)
		return err
	})
	return out, err
}

// AddConfiguration records a new configuration version (US-EQ-2) and the
// equipment.configuration_changed timeline event. Versions are never edited.
func (s *Service) AddConfiguration(ctx context.Context, a tanks.Actor, id uuid.UUID, settings json.RawMessage, effective *time.Time, note *string) (Configuration, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	clean := c.settings(settings)
	at := now
	if effective != nil {
		c.instant("effective_at", *effective, now)
		at = *effective
	}
	n := c.text("note", note, 500, true)
	if err := v.Err(); err != nil {
		return Configuration{}, err
	}
	var out Configuration
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := s.requireEquipment(ctx, q, a, id, tanks.PermWrite)
		if err != nil {
			return err
		}
		lock, err := q.LockEquipment(ctx, id) // serializes version numbers
		if err != nil {
			return err
		}
		if lock.RemovedAt != nil {
			return errRemoved
		}
		next, err := q.NextConfigurationVersion(ctx, id)
		if err != nil {
			return err
		}
		// Versions follow time: a backdated version would otherwise become "current".
		switch {
		case at.Before(lock.InstalledAt):
			v.Add("effective_at", "before_installed", nil)
		case at.Before(next.LatestEffectiveAt):
			v.Add("effective_at", "before_current", map[string]string{"min": next.LatestEffectiveAt.UTC().Format(time.RFC3339)})
		}
		if err := v.Err(); err != nil {
			return err
		}
		version := next.VersionNo
		row, err := q.CreateConfiguration(ctx, dbq.CreateConfigurationParams{
			EquipmentID: id, TankID: tankID, VersionNo: version, Settings: clean, EffectiveAt: at, Note: n, CreatedBy: &a.UserID, Now: now,
		})
		if err != nil {
			return err
		}
		out = configFromRow(dbq.ListConfigurationsRow(row))
		return record(ctx, q, now, tankID, "equipment_configuration", row.ID, "equipment.configuration_changed", at, a.UserID,
			map[string]any{"equipment_id": id.String(), "version": version})
	})
	return out, err
}

// ListConfigurations returns every version, newest first.
func (s *Service) ListConfigurations(ctx context.Context, a tanks.Actor, id uuid.UUID) ([]Configuration, error) {
	q := dbq.New(s.pool)
	if _, err := s.requireEquipment(ctx, q, a, id, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListConfigurations(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]Configuration, len(rows))
	for i, r := range rows {
		out[i] = configFromRow(r)
	}
	return out, nil
}

func (s *Service) loadEquipment(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Equipment, error) {
	row, err := q.GetEquipment(ctx, id)
	if database.IsNoRows(err) {
		return Equipment{}, apperr.NotFound()
	}
	if err != nil {
		return Equipment{}, err
	}
	list := []Equipment{equipmentFromRow(row)}
	if err := attachConfigs(ctx, q, list, []uuid.UUID{id}); err != nil {
		return Equipment{}, err
	}
	return list[0], nil
}

// --- maintenance ---

// PlanInput creates a maintenance plan.
type PlanInput struct {
	Title        string
	IntervalDays int32
	Timezone     string
	StartsOn     string
	EquipmentID  *uuid.UUID
	Notes        *string
}

// CreatePlan adds a maintenance plan, first due on its start date.
func (s *Service) CreatePlan(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in PlanInput) (Plan, error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	title := c.name("title", in.Title, 100)
	interval := c.interval(in.IntervalDays)
	tz := c.timezone(in.Timezone)
	starts := c.date("starts_on", in.StartsOn, now)
	notes := c.text("notes", in.Notes, 2000, true)
	if err := v.Err(); err != nil {
		return Plan{}, err
	}
	var out Plan
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, tankID); err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		if err := s.checkPlanEquipment(ctx, q, v, tankID, in.EquipmentID); err != nil {
			return err
		}
		if n, err := q.CountTankMaintenancePlans(ctx, tankID); err != nil {
			return err
		} else if n >= MaxPlansPerTank {
			return errPlanLimit
		}
		id, err := q.CreateMaintenancePlan(ctx, dbq.CreateMaintenancePlanParams{
			TankID: tankID, EquipmentID: in.EquipmentID, Title: title, IntervalDays: interval, Timezone: tz,
			StartsOn: dateToTime(starts), Notes: notes, CreatedBy: &a.UserID, Now: now,
		})
		if err != nil {
			return err
		}
		out, err = s.loadPlan(ctx, q, id, now)
		return err
	})
	return out, err
}

// checkPlanEquipment requires the device to be on the same tank and still installed.
func (s *Service) checkPlanEquipment(ctx context.Context, q *dbq.Queries, v *apperr.Validator, tankID uuid.UUID, id *uuid.UUID) error {
	if id == nil {
		return v.Err()
	}
	e, err := q.ShareLockEquipment(ctx, *id)
	switch {
	case database.IsNoRows(err) || (err == nil && e.TankID != tankID):
		v.Add("equipment_id", "equipment_not_found", nil)
	case err != nil:
		return err
	case e.RemovedAt != nil:
		v.Add("equipment_id", "equipment_removed", nil)
	}
	return v.Err()
}

// ListPlans returns the tank's plans, active and soonest due first, with derived due state.
func (s *Service) ListPlans(ctx context.Context, a tanks.Actor, tankID uuid.UUID) ([]Plan, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListMaintenancePlans(ctx, tankID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]Plan, len(rows))
	for i, r := range rows {
		out[i] = decorate(planFromRow(dbq.GetMaintenancePlanRow(r)), now)
	}
	return out, nil
}

// PlanPatch edits a plan (merge patch, Version required).
type PlanPatch struct {
	Version      *int32
	Title        patch.Field[string]
	IntervalDays patch.Field[int32]
	Timezone     patch.Field[string]
	StartsOn     patch.Field[string]
	EquipmentID  patch.Field[uuid.UUID]
	Active       patch.Field[bool]
	Notes        patch.Field[string]
}

// UpdatePlan edits a plan; the due date is recomputed from the last performance.
func (s *Service) UpdatePlan(ctx context.Context, a tanks.Actor, id uuid.UUID, in PlanPatch) (Plan, error) {
	now := s.clock.Now()
	var out Plan
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		cur, err := q.GetMaintenancePlan(ctx, id)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, cur.TankID, tanks.PermWrite); err != nil {
			return err
		}
		// Lock order is tank, equipment, plan (as in RemoveEquipment): share-lock the device the
		// plan will point to before the plan. If the plan changes meanwhile, version catches it.
		target := cur.EquipmentID
		if in.EquipmentID.Set {
			target = in.EquipmentID.Apply(nil)
		}
		if target != nil {
			if _, err := q.ShareLockEquipment(ctx, *target); err != nil && !database.IsNoRows(err) {
				return err
			}
		}
		row, err := q.LockMaintenancePlan(ctx, id)
		if err != nil {
			return err
		}
		v := apperr.NewValidator()
		if in.Version == nil {
			v.Add("version", "required", nil)
			return v.Err()
		}
		if *in.Version != row.Version {
			return errVersionConflict
		}
		c := checker{v}
		p := dbq.UpdateMaintenancePlanParams{
			ID: id, Version: row.Version, EquipmentID: row.EquipmentID, Title: row.Title, IntervalDays: row.IntervalDays,
			Timezone: row.Timezone, StartsOn: row.StartsOn, Active: row.Active, Notes: row.Notes,
			LastPerformedAt: row.LastPerformedAt, Now: now,
		}
		required := func(field string, set, null bool) bool {
			if set && null {
				v.Add(field, "required", nil)
				return false
			}
			return set
		}
		if required("title", in.Title.Set, in.Title.Null) {
			p.Title = c.name("title", in.Title.Value, 100)
		}
		if required("interval_days", in.IntervalDays.Set, in.IntervalDays.Null) {
			p.IntervalDays = c.interval(in.IntervalDays.Value)
		}
		if required("timezone", in.Timezone.Set, in.Timezone.Null) {
			p.Timezone = c.timezone(in.Timezone.Value)
		}
		if required("starts_on", in.StartsOn.Set, in.StartsOn.Null) {
			p.StartsOn = dateToTime(c.date("starts_on", in.StartsOn.Value, now))
		}
		if required("active", in.Active.Set, in.Active.Null) {
			p.Active = in.Active.Value
		}
		if in.Notes.Set {
			p.Notes = c.text("notes", in.Notes.Apply(nil), 2000, true)
		}
		if in.EquipmentID.Set {
			p.EquipmentID = in.EquipmentID.Apply(nil)
		}
		// An active plan needs an installed device: covers relinking and re-activating a plan
		// whose device was removed.
		if p.Active && (in.EquipmentID.Set || !row.Active) {
			if err := s.checkPlanEquipment(ctx, q, v, row.TankID, p.EquipmentID); err != nil && !apperr.IsKind(err, apperr.KindValidation) {
				return err
			}
		}
		if err := v.Err(); err != nil {
			return err
		}
		p.NextDueOn = dateToTime(nextDue(p.LastPerformedAt, timeToDate(p.StartsOn), p.IntervalDays, p.Timezone))
		n, err := q.UpdateMaintenancePlan(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 {
			return errVersionConflict
		}
		out, err = s.loadPlan(ctx, q, id, now)
		return err
	})
	return out, err
}

// EventInput records maintenance that was done.
type EventInput struct {
	Title       string // required for unplanned maintenance; plans use their title
	EquipmentID *uuid.UUID
	PerformedAt *time.Time // default now
	Notes       *string
}

// Perform records a performance of a plan and moves its due date to interval_days after the
// latest performance (an older one recorded late never moves it back).
//
// With an idempotency key, a retry returns the first event (created = false) and changes
// nothing; the same key with a different request is rejected. An inactive plan may still be
// recorded (work done before it was turned off).
func (s *Service) Perform(ctx context.Context, a tanks.Actor, planID uuid.UUID, in EventInput, key *string, requestHash []byte) (out Event, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	at := now
	if in.PerformedAt != nil {
		c.instant("performed_at", *in.PerformedAt, now)
		at = *in.PerformedAt
	}
	notes := c.text("notes", in.Notes, 2000, true)
	checkKey(v, key)
	if err := v.Err(); err != nil {
		return Event{}, false, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		cur, err := q.GetMaintenancePlan(ctx, planID)
		if database.IsNoRows(err) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, cur.TankID, tanks.PermWrite); err != nil {
			return err
		}
		plan, err := q.LockMaintenancePlan(ctx, planID)
		if err != nil {
			return err
		}
		ev, err := q.CreateMaintenanceEvent(ctx, dbq.CreateMaintenanceEventParams{
			TankID: plan.TankID, PlanID: &planID, EquipmentID: plan.EquipmentID, Title: plan.Title, PerformedAt: at,
			Notes: notes, RecordedBy: &a.UserID, IdempotencyKey: key, RequestHash: hashIf(key, requestHash), Now: now,
		})
		if database.IsNoRows(err) && key != nil {
			out, err = replay(ctx, q, a, *key, plan.TankID, requestHash)
			return err
		}
		if err != nil {
			return err
		}
		created = true
		latest, err := q.LatestPlanPerformance(ctx, &planID)
		if err != nil {
			return err
		}
		next := nextDue(&latest, timeToDate(plan.StartsOn), plan.IntervalDays, plan.Timezone)
		if n, err := q.UpdateMaintenancePlan(ctx, dbq.UpdateMaintenancePlanParams{
			ID: planID, Version: plan.Version, EquipmentID: plan.EquipmentID, Title: plan.Title, IntervalDays: plan.IntervalDays,
			Timezone: plan.Timezone, StartsOn: plan.StartsOn, NextDueOn: dateToTime(next), Active: plan.Active,
			Notes: plan.Notes, LastPerformedAt: &latest, Now: now,
		}); err != nil {
			return err
		} else if n == 0 {
			return fmt.Errorf("perform: plan %s changed under its lock", planID)
		}
		out = eventFromRow(dbq.ListMaintenanceEventsRow(ev))
		return record(ctx, q, now, plan.TankID, "maintenance_event", ev.ID, "maintenance.performed", at, a.UserID,
			performedSummary(plan.Title, &planID, plan.EquipmentID))
	})
	return out, created, err
}

// RecordUnplanned records maintenance done without a plan.
// It honours an idempotency key like Perform.
func (s *Service) RecordUnplanned(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in EventInput, key *string, requestHash []byte) (out Event, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	title := c.name("title", in.Title, 100)
	at := now
	if in.PerformedAt != nil {
		c.instant("performed_at", *in.PerformedAt, now)
		at = *in.PerformedAt
	}
	notes := c.text("notes", in.Notes, 2000, true)
	checkKey(v, key)
	if err := v.Err(); err != nil {
		return Event{}, false, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite); err != nil {
			return err
		}
		if in.EquipmentID != nil {
			e, err := q.ShareLockEquipment(ctx, *in.EquipmentID)
			if database.IsNoRows(err) || (err == nil && e.TankID != tankID) {
				v.Add("equipment_id", "equipment_not_found", nil)
				return v.Err()
			}
			if err != nil {
				return err
			}
			// Past maintenance on a removed device may still be recorded, within its lifetime.
			if at.Before(e.InstalledAt) {
				v.Add("performed_at", "before_installed", nil)
			} else if e.RemovedAt != nil && at.After(*e.RemovedAt) {
				v.Add("performed_at", "after_removed", nil)
			}
			if err := v.Err(); err != nil {
				return err
			}
		}
		ev, err := q.CreateMaintenanceEvent(ctx, dbq.CreateMaintenanceEventParams{
			TankID: tankID, EquipmentID: in.EquipmentID, Title: title, PerformedAt: at, Notes: notes, RecordedBy: &a.UserID,
			IdempotencyKey: key, RequestHash: hashIf(key, requestHash), Now: now,
		})
		if database.IsNoRows(err) && key != nil {
			out, err = replay(ctx, q, a, *key, tankID, requestHash)
			return err
		}
		if err != nil {
			return err
		}
		created = true
		out = eventFromRow(dbq.ListMaintenanceEventsRow(ev))
		return record(ctx, q, now, tankID, "maintenance_event", ev.ID, "maintenance.performed", at, a.UserID, performedSummary(title, nil, in.EquipmentID))
	})
	return out, created, err
}

func checkKey(v *apperr.Validator, key *string) {
	if key != nil && !idempotency.Valid(*key) {
		v.Add(idempotency.Header, "invalid", map[string]string{"format": "8-128 printable ASCII characters"})
	}
}

func hashIf(key *string, h []byte) []byte {
	if key == nil {
		return nil
	}
	return h
}

// replay returns the event first stored under key, or rejects a key reused for another request.
func replay(ctx context.Context, q *dbq.Queries, a tanks.Actor, key string, tankID uuid.UUID, requestHash []byte) (Event, error) {
	first, err := q.GetMaintenanceEventByIdempotencyKey(ctx, dbq.GetMaintenanceEventByIdempotencyKeyParams{RecordedBy: &a.UserID, IdempotencyKey: &key})
	if err != nil {
		return Event{}, err
	}
	if first.TankID != tankID || !bytes.Equal(first.RequestHash, requestHash) {
		return Event{}, errKeyReused
	}
	return eventFromRow(dbq.ListMaintenanceEventsRow{
		ID: first.ID, TankID: first.TankID, PlanID: first.PlanID, EquipmentID: first.EquipmentID, Title: first.Title,
		PerformedAt: first.PerformedAt, Notes: first.Notes, CreatedAt: first.CreatedAt,
	}), nil
}

// EventFilter narrows the maintenance history.
type EventFilter struct {
	PlanID, EquipmentID *uuid.UUID
	Limit               int32
	Cursor              *EventCursor
}

// EventCursor is the keyset of the history.
type EventCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// ListEvents returns maintenance done on the tank, newest first.
func (s *Service) ListEvents(ctx context.Context, a tanks.Actor, tankID uuid.UUID, f EventFilter) ([]Event, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	p := dbq.ListMaintenanceEventsParams{TankID: tankID, PlanID: f.PlanID, EquipmentID: f.EquipmentID,
		CursorAt: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), CursorID: uuid.Max, RowLimit: f.Limit}
	if f.Cursor != nil {
		p.CursorAt, p.CursorID = f.Cursor.At, f.Cursor.ID
	}
	rows, err := q.ListMaintenanceEvents(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = eventFromRow(r)
	}
	return out, nil
}

func (s *Service) loadPlan(ctx context.Context, q *dbq.Queries, id uuid.UUID, now time.Time) (Plan, error) {
	row, err := q.GetMaintenancePlan(ctx, id)
	if err != nil {
		return Plan{}, err
	}
	return decorate(planFromRow(row), now), nil
}

func record(ctx context.Context, q *dbq.Queries, now time.Time, tankID uuid.UUID, sourceType string, sourceID uuid.UUID, eventType string, at time.Time, actor uuid.UUID, summary map[string]any) error {
	category := timeline.CategoryEquipment
	if sourceType == "maintenance_event" {
		category = timeline.CategoryMaintenance
	}
	_, err := timeline.Record(ctx, q, now, timeline.Event{
		TankID: tankID, OccurredAt: at, Category: category, EventType: eventType, SourceType: sourceType,
		SourceID: sourceID, ActorID: &actor, Summary: summary,
	})
	return err
}

// --- archive and erasure ---

// OnTankArchived is the tanks archive hook: maintenance plans stop being due. Equipment and
// history stay as they are.
func OnTankArchived(ctx context.Context, q *dbq.Queries, tankID, _ uuid.UUID, now time.Time) error {
	return q.DeactivateTankMaintenancePlans(ctx, dbq.DeactivateTankMaintenancePlansParams{TankID: tankID, Now: now})
}

// EraseUserData deletes the erased account's equipment and maintenance data
// (identity.Eraser); it must run before tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.LockOwnerTanksForErasure(ctx, userID); err != nil {
		return err
	}
	for _, del := range []func(context.Context, uuid.UUID) error{
		q.DeleteOwnerMaintenance, q.DeleteOwnerMaintenancePlans, q.DeleteOwnerConfigurations, q.DeleteOwnerEquipment,
	} {
		if err := del(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

// --- row mapping ---

func equipmentFromRow(r dbq.GetEquipmentRow) Equipment {
	e := Equipment{
		ID: r.ID, TankID: r.TankID, Category: r.Category, Name: r.Name, Manufacturer: r.Manufacturer, Model: r.Model,
		SerialNumber: r.SerialNumber, PurchasedOn: optTimeToDate(r.PurchasedOn), WarrantyUntil: optTimeToDate(r.WarrantyUntil),
		InstalledAt: r.InstalledAt, RemovedAt: r.RemovedAt, RemovalReason: r.RemovalReason, Notes: r.Notes,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
	return e
}

func configFromRow(r dbq.ListConfigurationsRow) Configuration {
	return Configuration{ID: r.ID, VersionNo: r.VersionNo, Settings: r.Settings, EffectiveAt: r.EffectiveAt, Note: r.Note, CreatedAt: r.CreatedAt}
}

func planFromRow(r dbq.GetMaintenancePlanRow) Plan {
	return Plan{
		ID: r.ID, TankID: r.TankID, EquipmentID: r.EquipmentID, EquipmentName: r.EquipmentName, Title: r.Title,
		IntervalDays: r.IntervalDays, Timezone: r.Timezone, StartsOn: timeToDate(r.StartsOn), LastPerformedAt: r.LastPerformedAt,
		NextDueOn: timeToDate(r.NextDueOn), Active: r.Active, Notes: r.Notes, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Version: r.Version,
	}
}

func eventFromRow(r dbq.ListMaintenanceEventsRow) Event {
	return Event{ID: r.ID, TankID: r.TankID, PlanID: r.PlanID, EquipmentID: r.EquipmentID, Title: r.Title,
		PerformedAt: r.PerformedAt, Notes: r.Notes, CreatedAt: r.CreatedAt}
}

func removedSummary(name, category string, reason *string) map[string]any {
	m := map[string]any{"name": name, "category": category}
	if reason != nil {
		m["reason"] = *reason
	}
	return m
}

func performedSummary(title string, planID, equipmentID *uuid.UUID) map[string]any {
	m := map[string]any{"title": title}
	if planID != nil {
		m["plan_id"] = planID.String()
	}
	if equipmentID != nil {
		m["equipment_id"] = equipmentID.String()
	}
	return m
}
