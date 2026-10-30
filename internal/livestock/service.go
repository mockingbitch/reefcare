package livestock

import (
	"bytes"
	"context"
	"errors"
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

// Service implements livestock workflows; tank data is authorized through tanks.Authorizer
// (reads PermRead, writes PermWrite).
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
	errUnderflow       = apperr.Conflict("quantity_underflow", "This would leave fewer than zero at some point in the history.")
	errOverflow        = apperr.Conflict("quantity_overflow", "This would exceed the maximum quantity of an entry.")
	errEntryLimit      = apperr.Conflict("limit_reached", "This tank has the maximum number of livestock entries.")
	errEventLimit      = apperr.Conflict("limit_reached", "This entry has the maximum number of events of this kind.")
	errLostReplay      = errors.New("livestock: idempotent insert conflicted but no first row was found")
	errSpeciesLimit    = apperr.Conflict("limit_reached", "You have the maximum number of own species.")
	errKeyReused       = apperr.Invalid("idempotency_key_reused", "This Idempotency-Key was already used for a different request.")
)

// --- species ---

// SpeciesInput creates a private species.
type SpeciesInput struct {
	Category       string
	CoralType      *string
	CommonNameVi   *string
	CommonNameEn   *string
	ScientificName *string
}

// SearchSpecies returns curated and own species whose names contain q.
func (s *Service) SearchSpecies(ctx context.Context, a tanks.Actor, q string, category *string, limit int32) ([]Species, error) {
	if category != nil && !categories[*category] {
		v := apperr.NewValidator()
		v.Add("category", "unsupported", nil)
		return nil, v.Err()
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	rows, err := dbq.New(s.pool).SearchSpecies(ctx, dbq.SearchSpeciesParams{
		UserID: &a.UserID, Category: category, Pattern: likePattern(q), RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Species, len(rows))
	for i, r := range rows {
		out[i] = speciesFromRow(dbq.Species{ID: r.ID, OwnerID: r.OwnerID, Category: r.Category, CoralType: r.CoralType,
			CommonNameVi: r.CommonNameVi, CommonNameEn: r.CommonNameEn, ScientificName: r.ScientificName, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// CreateSpecies adds a private species, usable only on the caller's tanks.
func (s *Service) CreateSpecies(ctx context.Context, a tanks.Actor, in SpeciesInput) (Species, error) {
	v := apperr.NewValidator()
	c := checker{v}
	cat := c.category(in.Category)
	c.coralType(in.CoralType, cat)
	vi := c.text("common_name_vi", in.CommonNameVi, 100, false)
	en := c.text("common_name_en", in.CommonNameEn, 100, false)
	sci := c.text("scientific_name", in.ScientificName, 150, false)
	if vi == nil && en == nil && sci == nil {
		v.Add("common_name_vi", "name_required", nil)
	}
	if err := v.Err(); err != nil {
		return Species{}, err
	}
	var out Species
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockUserRow(ctx, a.UserID); err != nil { // serializes the limit check
			return err
		}
		if n, err := q.CountOwnSpecies(ctx, &a.UserID); err != nil {
			return err
		} else if n >= MaxOwnSpecies {
			return errSpeciesLimit
		}
		row, err := q.CreateSpecies(ctx, dbq.CreateSpeciesParams{
			OwnerID: &a.UserID, Category: cat, CoralType: in.CoralType, CommonNameVi: vi, CommonNameEn: en,
			ScientificName: sci, Now: s.clock.Now(),
		})
		if err != nil {
			return err
		}
		out = speciesFromRow(row)
		return nil
	})
	return out, err
}

// usableSpecies loads a species the tank owner may use: curated, or their own. Anything else
// is reported as not found so private species of others stay invisible.
func usableSpecies(ctx context.Context, q *dbq.Queries, v *apperr.Validator, id uuid.UUID, owner uuid.UUID) (dbq.Species, bool, error) {
	sp, err := q.GetSpecies(ctx, id)
	if err != nil && !database.IsNoRows(err) {
		return dbq.Species{}, false, err
	}
	if err != nil || (sp.OwnerID != nil && *sp.OwnerID != owner) {
		v.Add("species_id", "species_not_found", nil)
		return dbq.Species{}, false, nil
	}
	return sp, true, nil
}

// --- entries ---

// EntryInput adds livestock to a tank.
type EntryInput struct {
	SpeciesID        *uuid.UUID
	Category         *string // defaults to the species' category; required without a species
	CoralType        *string // corals only; defaults to the species' coral type
	DisplayName      string
	Quantity         *int32
	AddedAt          *time.Time // default now
	Form             *string
	QuarantineStatus *string
	LocationNote     *string
	SourceNote       *string
	Notes            *string
}

// AddEntry creates an entry with its `added` event (US-LS-1). With an idempotency key a retry
// returns the first entry (created = false).
func (s *Service) AddEntry(ctx context.Context, a tanks.Actor, tankID uuid.UUID, in EntryInput, key *string, requestHash []byte) (out Entry, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	name := c.name("display_name", in.DisplayName, 100)
	var quantity int32
	if in.Quantity == nil {
		v.Add("quantity", "required", nil)
	} else {
		quantity = *in.Quantity
		c.quantity("quantity", quantity, 1)
	}
	at := now
	if in.AddedAt != nil {
		c.instant("added_at", *in.AddedAt, now)
		at = stored(*in.AddedAt)
	}
	form := c.form(in.Form)
	qs := c.quarantineStatus(in.QuarantineStatus)
	loc := c.text("location_note", in.LocationNote, 200, false)
	src := c.text("source_note", in.SourceNote, 200, false)
	notes := c.text("notes", in.Notes, 2000, true)
	if in.Category != nil {
		c.category(*in.Category)
	} else if in.SpeciesID == nil {
		v.Add("category", "required", nil)
	}
	checkKey(v, key)
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, tankID); err != nil { // serializes the per-tank limit
			return err
		}
		// A retry returns the first entry even if the tank was archived or filled up since.
		if key != nil {
			if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
				return err
			}
			if first, found, err := s.replayEntry(ctx, q, a, *key, tankID, requestHash); err != nil || found {
				out = first
				return err
			}
		}
		acc, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermWrite)
		if err != nil {
			return err
		}
		cat := ""
		if in.Category != nil {
			cat = *in.Category
		}
		coral := in.CoralType
		if in.SpeciesID != nil {
			if sp, ok, err := usableSpecies(ctx, q, v, *in.SpeciesID, acc.OwnerID); err != nil {
				return err
			} else if ok {
				if cat == "" {
					cat = sp.Category
				} else if cat != sp.Category {
					v.Add("category", "species_mismatch", map[string]string{"species": sp.Category})
				}
				coral = speciesCoralType(v, coral, sp.CoralType)
			}
		}
		c.coralType(coral, cat)
		if err := v.Err(); err != nil {
			return err
		}
		if n, err := q.CountTankLivestock(ctx, tankID); err != nil {
			return err
		} else if n.Present >= MaxEntriesPerTank || n.Total >= MaxEntriesEver {
			return errEntryLimit
		}
		id, err := q.CreateLivestockEntry(ctx, dbq.CreateLivestockEntryParams{
			TankID: tankID, SpeciesID: in.SpeciesID, Category: cat, CoralType: coral, DisplayName: name, Form: form,
			QuarantineStatus: qs, LocationNote: loc, SourceNote: src, Notes: notes, CurrentQuantity: quantity, AddedAt: at,
			CreatedBy: &a.UserID, IdempotencyKey: key, RequestHash: hashIf(key, requestHash), Now: now,
		})
		if database.IsNoRows(err) && key != nil { // lost a race with the same key
			first, found, err := s.replayEntry(ctx, q, a, *key, tankID, requestHash)
			if err == nil && !found {
				err = errLostReplay
			}
			out = first
			return err
		}
		if err != nil {
			return err
		}
		created = true
		ev, err := s.insertEvent(ctx, q, now, a, id, tankID, "added", quantity, at, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if err := recordTimeline(ctx, q, now, a, *ev, name); err != nil {
			return err
		}
		out, err = s.loadEntry(ctx, q, id)
		return err
	})
	return out, created, err
}

// replayEntry returns the entry first created under key, or rejects a key reused for
// another request.
func (s *Service) replayEntry(ctx context.Context, q *dbq.Queries, a tanks.Actor, key string, tankID uuid.UUID, requestHash []byte) (Entry, bool, error) {
	first, err := q.GetLivestockEntryByIdempotencyKey(ctx, dbq.GetLivestockEntryByIdempotencyKeyParams{CreatedBy: &a.UserID, IdempotencyKey: &key})
	if database.IsNoRows(err) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, err
	}
	if first.TankID != tankID || !bytes.Equal(first.RequestHash, requestHash) {
		return Entry{}, false, errKeyReused
	}
	e, err := s.loadEntry(ctx, q, first.ID)
	return e, err == nil, err
}

// speciesCoralType defaults an entry's coral type to its species' and rejects a different one.
func speciesCoralType(v *apperr.Validator, given, species *string) *string {
	switch {
	case species == nil:
		return given
	case given == nil:
		return species
	case *given != *species:
		v.Add("coral_type", "species_mismatch", map[string]string{"species": *species})
	}
	return given
}

// ListEntries returns the tank's livestock, newest first; gone groups only on request.
func (s *Service) ListEntries(ctx context.Context, a tanks.Actor, tankID uuid.UUID, includeGone bool) ([]Entry, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, tankID, tanks.PermRead); err != nil {
		return nil, err
	}
	rows, err := q.ListLivestockEntries(ctx, dbq.ListLivestockEntriesParams{TankID: tankID, IncludeGone: includeGone})
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = entryFromRow(dbq.GetLivestockEntryRow(r))
	}
	return out, nil
}

// GetEntry returns one entry.
func (s *Service) GetEntry(ctx context.Context, a tanks.Actor, id uuid.UUID) (Entry, error) {
	q := dbq.New(s.pool)
	if _, _, err := s.requireEntry(ctx, q, a, id, tanks.PermRead); err != nil {
		return Entry{}, err
	}
	return s.loadEntry(ctx, q, id)
}

func (s *Service) requireEntry(ctx context.Context, q *dbq.Queries, a tanks.Actor, id uuid.UUID, perm tanks.Perm) (uuid.UUID, tanks.Access, error) {
	tankID, err := q.GetLivestockEntryTank(ctx, id)
	if database.IsNoRows(err) {
		return uuid.Nil, tanks.Access{}, apperr.NotFound()
	}
	if err != nil {
		return uuid.Nil, tanks.Access{}, err
	}
	acc, err := s.authz.Require(ctx, q, a.UserID, tankID, perm)
	return tankID, acc, err
}

func (s *Service) loadEntry(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Entry, error) {
	row, err := q.GetLivestockEntry(ctx, id)
	if database.IsNoRows(err) {
		return Entry{}, apperr.NotFound()
	}
	if err != nil {
		return Entry{}, err
	}
	return entryFromRow(row), nil
}

// EntryPatch edits an entry's description (merge patch, Version required). Quantity changes
// only through events.
type EntryPatch struct {
	Version          *int32
	SpeciesID        patch.Field[uuid.UUID]
	Category         patch.Field[string]
	CoralType        patch.Field[string]
	DisplayName      patch.Field[string]
	Form             patch.Field[string]
	QuarantineStatus patch.Field[string]
	LocationNote     patch.Field[string]
	SourceNote       patch.Field[string]
	Notes            patch.Field[string]
}

// UpdateEntry edits an entry's description.
func (s *Service) UpdateEntry(ctx context.Context, a tanks.Actor, id uuid.UUID, in EntryPatch) (Entry, error) {
	now := s.clock.Now()
	var out Entry
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		_, acc, err := s.requireEntry(ctx, q, a, id, tanks.PermWrite)
		if err != nil {
			return err
		}
		lock, err := q.LockLivestockEntry(ctx, id)
		if err != nil {
			return err
		}
		v := apperr.NewValidator()
		if in.Version == nil {
			v.Add("version", "required", nil)
			return v.Err()
		}
		if *in.Version != lock.Version {
			return errVersionConflict
		}
		cur, err := q.GetLivestockEntry(ctx, id)
		if err != nil {
			return err
		}
		c := checker{v}
		p := dbq.UpdateLivestockEntryParams{
			ID: id, Version: lock.Version, SpeciesID: cur.SpeciesID, Category: cur.Category, CoralType: cur.CoralType,
			DisplayName: cur.DisplayName, Form: cur.Form, QuarantineStatus: cur.QuarantineStatus, LocationNote: cur.LocationNote,
			SourceNote: cur.SourceNote, Notes: cur.Notes, Now: now,
		}
		required := func(field string, set, null bool) bool {
			if set && null {
				v.Add(field, "required", nil)
				return false
			}
			return set
		}
		if required("display_name", in.DisplayName.Set, in.DisplayName.Null) {
			p.DisplayName = c.name("display_name", in.DisplayName.Value, 100)
		}
		if required("category", in.Category.Set, in.Category.Null) {
			p.Category = c.category(in.Category.Value)
		}
		if in.Form.Set {
			p.Form = c.form(in.Form.Apply(nil))
		}
		if in.QuarantineStatus.Set {
			p.QuarantineStatus = c.quarantineStatus(in.QuarantineStatus.Apply(nil))
		}
		if in.CoralType.Set {
			p.CoralType = in.CoralType.Apply(nil)
		}
		if in.LocationNote.Set {
			p.LocationNote = c.text("location_note", in.LocationNote.Apply(nil), 200, false)
		}
		if in.SourceNote.Set {
			p.SourceNote = c.text("source_note", in.SourceNote.Apply(nil), 200, false)
		}
		if in.Notes.Set {
			p.Notes = c.text("notes", in.Notes.Apply(nil), 2000, true)
		}
		if in.SpeciesID.Set {
			p.SpeciesID = in.SpeciesID.Apply(nil)
		}
		// Category and coral type must agree with the species (checked on any change); changing
		// only the species takes its values.
		if p.SpeciesID != nil && (in.SpeciesID.Set || in.Category.Set || in.CoralType.Set) {
			if sp, ok, err := usableSpecies(ctx, q, v, *p.SpeciesID, acc.OwnerID); err != nil {
				return err
			} else if ok {
				if sp.Category != p.Category {
					if in.Category.Set {
						v.Add("category", "species_mismatch", map[string]string{"species": sp.Category})
					} else {
						p.Category = sp.Category
					}
				}
				if !in.CoralType.Set && sp.CoralType != nil {
					p.CoralType = sp.CoralType
				}
				p.CoralType = speciesCoralType(v, p.CoralType, sp.CoralType)
			}
		}
		if p.Category != "coral" && !in.CoralType.Set {
			p.CoralType = nil // a category change away from coral drops the coral type
		}
		c.coralType(p.CoralType, p.Category)
		if err := v.Err(); err != nil {
			return err
		}
		if n, err := q.UpdateLivestockEntry(ctx, p); err != nil {
			return err
		} else if n == 0 {
			return errVersionConflict
		}
		out, err = s.loadEntry(ctx, q, id)
		return err
	})
	return out, err
}

// --- events ---

// EventInput records something that happened to an entry (US-LS-2).
type EventInput struct {
	Type       string
	Quantity   *int32 // required for quantity events, absent for observations
	OccurredAt *time.Time
	Note       *string
}

// RecordEvent adds an event. Quantity events are checked against every running balance in
// event order (quantity_underflow); observations need a note. With an idempotency key a
// retry returns the first event (created = false).
func (s *Service) RecordEvent(ctx context.Context, a tanks.Actor, entryID uuid.UUID, in EventInput, key *string, requestHash []byte) (out Event, created bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	sign, ok := eventSigns[in.Type]
	if !ok {
		v.Add("type", "unsupported", map[string]string{
			"allowed": "added,transferred_in,propagated,found,died,missing,transferred_out,sold,health_observation,treatment",
		})
	}
	var delta int32
	switch {
	case !ok:
	case sign == 0 && in.Quantity != nil:
		v.Add("quantity", "not_allowed", nil)
	case sign != 0 && in.Quantity == nil:
		v.Add("quantity", "required", nil)
	case sign != 0:
		c.quantity("quantity", *in.Quantity, 1)
		delta = sign * *in.Quantity
	}
	at := stored(now)
	if in.OccurredAt != nil {
		c.instant("occurred_at", *in.OccurredAt, now)
		at = stored(*in.OccurredAt)
	}
	note := c.text("note", in.Note, 2000, true)
	if ok && sign == 0 && note == nil {
		v.Add("note", "required", nil)
	}
	checkKey(v, key)
	if err := v.Err(); err != nil {
		return Event{}, false, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := s.requireEntryWrite(ctx, q, a, entryID, key, func(tankID uuid.UUID) (bool, error) {
			first, found, err := s.replay(ctx, q, a, *key, tankID, requestHash)
			out = first
			return found, err
		})
		if err != nil || tankID == uuid.Nil {
			return err
		}
		ev, err := s.applyMove(ctx, q, now, a, entryID, in.Type, delta, at, note, key, requestHash, nil)
		if err != nil {
			return err
		}
		if ev == nil { // lost a race with the same key
			first, found, err := s.replay(ctx, q, a, *key, tankID, requestHash)
			if err == nil && !found {
				err = errLostReplay
			}
			out = first
			return err
		}
		created = true
		out = *ev
		return nil
	})
	return out, created, err
}

// Reconcile records the actual count (US-LS-3): a `reconciliation` event with the difference
// from the balance at that time. History is not modified. A count equal to the balance
// records nothing (so a retry is harmless); changed reports whether an event was added.
//
// With an idempotency key a retry returns the entry as it is without reconciling again (a
// retry without one and without occurred_at reconciles at the new time).
func (s *Service) Reconcile(ctx context.Context, a tanks.Actor, entryID uuid.UUID, count int32, occurredAt *time.Time, note *string, key *string, requestHash []byte) (out Entry, changed bool, err error) {
	now := s.clock.Now()
	v := apperr.NewValidator()
	c := checker{v}
	c.v.Check(count >= 0 && count <= MaxQuantity, "count", "out_of_range", map[string]string{"min": "0", "max": fmt.Sprint(MaxQuantity)})
	at := stored(now)
	if occurredAt != nil {
		c.instant("occurred_at", *occurredAt, now)
		at = stored(*occurredAt)
	}
	n := c.text("note", note, 2000, true)
	checkKey(v, key)
	if err := v.Err(); err != nil {
		return Entry{}, false, err
	}
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := s.requireEntryWrite(ctx, q, a, entryID, key, func(tankID uuid.UUID) (bool, error) {
			_, found, err := s.replay(ctx, q, a, *key, tankID, requestHash)
			return found, err
		})
		if err != nil {
			return err
		}
		if tankID != uuid.Nil {
			ev, err := s.applyMove(ctx, q, now, a, entryID, "reconciliation", 0, at, n, key, requestHash, &count)
			if err != nil {
				return err
			}
			changed = ev != nil
		}
		out, err = s.loadEntry(ctx, q, entryID)
		return err
	})
	return out, changed, err
}

// requireEntryWrite authorizes a write on the entry's tank. With a key it first runs replay
// under read access, so a retry is answered even after the tank was archived; when replay
// finds the first result it returns uuid.Nil and no error.
func (s *Service) requireEntryWrite(ctx context.Context, q *dbq.Queries, a tanks.Actor, entryID uuid.UUID, key *string, replay func(uuid.UUID) (bool, error)) (uuid.UUID, error) {
	if key != nil {
		tankID, _, err := s.requireEntry(ctx, q, a, entryID, tanks.PermRead)
		if err != nil {
			return uuid.Nil, err
		}
		if found, err := replay(tankID); err != nil || found {
			return uuid.Nil, err
		}
	}
	tankID, _, err := s.requireEntry(ctx, q, a, entryID, tanks.PermWrite)
	return tankID, err
}

// applyMove locks the entry, checks the history with the new event and stores it with its
// timeline entry. With reconcileTo set, the delta is reconcileTo minus the balance at at;
// a zero difference stores nothing and returns nil. A nil event with a key means another
// request stored it first.
func (s *Service) applyMove(ctx context.Context, q *dbq.Queries, now time.Time, a tanks.Actor, entryID uuid.UUID, typ string, delta int32, at time.Time, note *string, key *string, requestHash []byte, reconcileTo *int32) (*Event, error) {
	lock, err := q.LockLivestockEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if at.Before(lock.AddedAt) {
		v := apperr.NewValidator()
		v.Add("occurred_at", "before_added", map[string]string{"min": lock.AddedAt.UTC().Format(time.RFC3339)})
		return nil, v.Err()
	}
	if n, err := q.CountLivestockEvents(ctx, entryID); err != nil {
		return nil, err
	} else if (reconcileTo != nil || delta != 0) && n.Moves >= MaxMovesPerEntry ||
		reconcileTo == nil && delta == 0 && n.Observations >= MaxObservations {
		return nil, errEventLimit
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	rows, err := q.LivestockMoves(ctx, entryID)
	if err != nil {
		return nil, err
	}
	moves := make([]move, len(rows))
	for i, r := range rows {
		moves[i] = move{At: r.OccurredAt, ID: r.ID, Delta: r.QuantityDelta}
	}
	m := move{At: at, ID: id}
	if reconcileTo != nil {
		diff := int64(*reconcileTo) - balanceAt(moves, m)
		switch {
		case diff == 0:
			return nil, nil
		case diff < -MaxQuantity || diff > MaxQuantity: // balances stay in 0..MaxQuantity
			return nil, errOverflow
		}
		m.Delta = int32(diff)
	} else {
		m.Delta = delta
	}
	quantity := lock.CurrentQuantity
	if m.Delta != 0 {
		var res int
		switch quantity, res = insert(moves, m); res {
		case balanceUnderflow:
			return nil, errUnderflow
		case balanceOverflow:
			return nil, errOverflow
		}
	}
	ev, err := s.insertEvent(ctx, q, now, a, entryID, lock.TankID, typ, m.Delta, at, note, key, requestHash, &id)
	if err != nil || ev == nil {
		return ev, err
	}
	if m.Delta != 0 {
		if err := q.SetLivestockQuantity(ctx, dbq.SetLivestockQuantityParams{ID: entryID, CurrentQuantity: quantity, Now: now}); err != nil {
			return nil, err
		}
	}
	entry, err := q.GetLivestockEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	return ev, recordTimeline(ctx, q, now, a, *ev, entry.DisplayName)
}

// insertEvent stores one event; with an idempotency key that was already used it returns nil.
func (s *Service) insertEvent(ctx context.Context, q *dbq.Queries, now time.Time, a tanks.Actor, entryID, tankID uuid.UUID, typ string, delta int32, at time.Time, note, key *string, requestHash []byte, id *uuid.UUID) (*Event, error) {
	if id == nil {
		v7, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		id = &v7
	}
	row, err := q.CreateLivestockEvent(ctx, dbq.CreateLivestockEventParams{
		ID: *id, EntryID: entryID, TankID: tankID, EventType: typ, QuantityDelta: delta, OccurredAt: at, Note: note,
		RecordedBy: &a.UserID, IdempotencyKey: key, RequestHash: hashIf(key, requestHash), Now: now,
	})
	if database.IsNoRows(err) && key != nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ev := eventFromRow(dbq.ListLivestockEventsRow(row))
	return &ev, nil
}

// replay returns the event first stored under key, or rejects a key reused for another request.
func (s *Service) replay(ctx context.Context, q *dbq.Queries, a tanks.Actor, key string, tankID uuid.UUID, requestHash []byte) (Event, bool, error) {
	first, err := q.GetLivestockEventByIdempotencyKey(ctx, dbq.GetLivestockEventByIdempotencyKeyParams{RecordedBy: &a.UserID, IdempotencyKey: &key})
	if database.IsNoRows(err) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}
	if first.TankID != tankID || !bytes.Equal(first.RequestHash, requestHash) {
		return Event{}, false, errKeyReused
	}
	return eventFromRow(dbq.ListLivestockEventsRow{
		ID: first.ID, EntryID: first.EntryID, TankID: first.TankID, EventType: first.EventType,
		QuantityDelta: first.QuantityDelta, OccurredAt: first.OccurredAt, Note: first.Note, CreatedAt: first.CreatedAt,
	}), true, nil
}

// EventCursor is the keyset of an entry's history.
type EventCursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

// ListEvents returns an entry's history, newest first.
func (s *Service) ListEvents(ctx context.Context, a tanks.Actor, entryID uuid.UUID, limit int32, cursor *EventCursor) ([]Event, error) {
	q := dbq.New(s.pool)
	if _, _, err := s.requireEntry(ctx, q, a, entryID, tanks.PermRead); err != nil {
		return nil, err
	}
	p := dbq.ListLivestockEventsParams{EntryID: entryID, CursorAt: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), CursorID: uuid.Max, RowLimit: limit}
	if cursor != nil {
		p.CursorAt, p.CursorID = cursor.At, cursor.ID
	}
	rows, err := q.ListLivestockEvents(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = eventFromRow(r)
	}
	return out, nil
}

func recordTimeline(ctx context.Context, q *dbq.Queries, now time.Time, a tanks.Actor, ev Event, name string) error {
	summary := map[string]any{"name": name, "entry_id": ev.EntryID.String()}
	if ev.QuantityDelta != 0 {
		summary["quantity_delta"] = ev.QuantityDelta
	}
	_, err := timeline.Record(ctx, q, now, timeline.Event{
		TankID: ev.TankID, OccurredAt: ev.OccurredAt, Category: timeline.CategoryLivestock, EventType: "livestock." + ev.Type,
		SourceType: "livestock_event", SourceID: ev.ID, ActorID: &a.UserID, Summary: summary,
	})
	return err
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

// --- erasure ---

// EraseUserData deletes the erased account's livestock and own species (identity.Eraser); it
// must run before tanks.EraseUserData.
func EraseUserData(ctx context.Context, q *dbq.Queries, userID uuid.UUID) error {
	if err := q.LockOwnerTanksForLivestockErasure(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerLivestockEvents(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteOwnerLivestockEntries(ctx, userID); err != nil {
		return err
	}
	if err := q.DetachOwnerSpecies(ctx, &userID); err != nil {
		return err
	}
	return q.DeleteOwnerSpecies(ctx, &userID)
}

// --- row mapping ---

func speciesFromRow(r dbq.Species) Species {
	return Species{
		ID: r.ID, Curated: r.OwnerID == nil, Category: r.Category, CoralType: r.CoralType, CommonNameVi: r.CommonNameVi,
		CommonNameEn: r.CommonNameEn, ScientificName: r.ScientificName, CreatedAt: r.CreatedAt,
	}
}

func entryFromRow(r dbq.GetLivestockEntryRow) Entry {
	e := Entry{
		ID: r.ID, TankID: r.TankID, SpeciesID: r.SpeciesID, Category: r.Category, CoralType: r.CoralType,
		DisplayName: r.DisplayName, Form: r.Form, QuarantineStatus: r.QuarantineStatus, LocationNote: r.LocationNote, SourceNote: r.SourceNote, Notes: r.Notes, CurrentQuantity: r.CurrentQuantity,
		AddedAt: r.AddedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
	if r.SpeciesID != nil {
		e.Species = &SpeciesRef{CommonNameVi: r.CommonNameVi, CommonNameEn: r.CommonNameEn, ScientificName: r.ScientificName}
	}
	return e
}

func eventFromRow(r dbq.ListLivestockEventsRow) Event {
	return Event{
		ID: r.ID, EntryID: r.EntryID, TankID: r.TankID, Type: r.EventType, QuantityDelta: r.QuantityDelta,
		OccurredAt: r.OccurredAt, Note: r.Note, CreatedAt: r.CreatedAt,
	}
}
