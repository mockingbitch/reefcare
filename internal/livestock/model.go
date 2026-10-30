// Package livestock tracks the animals and corals in a tank as entries whose quantity is
// derived from an append-only event history (docs/user-stories.md US-LS-1..3, milestone 2.7).
//
// Business rules:
//   - An entry is a group of one kind (a pair of clownfish, a frag). Adding it records an
//     `added` event; its current quantity is the sum of its event deltas. Convention: a colony,
//     a frag plug or a rock of zoanthids counts as 1; partial die-off is an observation.
//   - Arrivals: `added` (from outside, again later for more of the same), `transferred_in`
//     (from another tank), `propagated` (fragging, splitting, reproduction in the system),
//     `found` (a missing animal turned up). Losses: `died` (seen), `missing` (fate unknown —
//     never reported as a death), `transferred_out` (moved, rehomed, traded), `sold`.
//   - No running balance may go below zero in event order, also when an event is recorded
//     late with an earlier time (409 quantity_underflow). Nothing happens before `added`.
//   - History is never edited. A wrong count is corrected by reconciling: the keeper enters
//     the actual count and a `reconciliation` event records the difference.
//   - Health observations and treatments carry a note and no quantity. ReefCare records
//     them; it never diagnoses.
//   - A group that reaches 0 is "gone" and stays in history.
package livestock

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/textcheck"
)

// Limits.
const (
	MaxEntriesPerTank  = 500  // present groups
	MaxEntriesEver     = 5000 // all entries of a tank, gone ones included (never deleted)
	MaxMovesPerEntry   = 2000 // quantity events, all loaded for the balance check
	MaxObservations    = 5000 // per entry
	MaxOwnSpecies      = 500
	MaxQuantity        = 100000 // a whole entry
	MaxEventQuantity   = 10000  // one event
	maxFutureSkew      = 5 * time.Minute
	defaultSearchLimit = 50
)

var (
	earliest   = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	categories = map[string]bool{"fish": true, "coral": true, "invertebrate": true, "algae": true, "other": true}
	quarantine = map[string]bool{"in_quarantine": true, "completed": true}
	coralTypes = map[string]bool{"sps": true, "lps": true, "soft": true, "non_photosynthetic": true}
	forms      = map[string]bool{"individual": true, "group": true, "frag": true, "colony": true}

	// Event types a keeper records directly, with the sign of their quantity.
	eventSigns = map[string]int32{
		"added": 1, "transferred_in": 1, "propagated": 1, "found": 1,
		"died": -1, "missing": -1, "transferred_out": -1, "sold": -1,
		"health_observation": 0, "treatment": 0,
	}
)

// Species is a reference kind of animal or coral.
type Species struct {
	ID             uuid.UUID
	Curated        bool
	Category       string
	CoralType      *string
	CommonNameVi   *string
	CommonNameEn   *string
	ScientificName *string
	CreatedAt      time.Time
}

// Entry is a group of one kind in a tank.
type Entry struct {
	ID               uuid.UUID
	TankID           uuid.UUID
	SpeciesID        *uuid.UUID
	Species          *SpeciesRef
	Category         string
	CoralType        *string
	DisplayName      string
	Form             *string
	QuarantineStatus *string
	LocationNote     *string
	SourceNote       *string
	Notes            *string
	CurrentQuantity  int32
	AddedAt          time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int32
}

// SpeciesRef is the species shown with an entry.
type SpeciesRef struct {
	CommonNameVi   *string
	CommonNameEn   *string
	ScientificName *string
}

// Event is one fact in an entry's history.
type Event struct {
	ID            uuid.UUID
	EntryID       uuid.UUID
	TankID        uuid.UUID
	Type          string
	QuantityDelta int32
	OccurredAt    time.Time
	Note          *string
	CreatedAt     time.Time
}

// move is one quantity change in event order.
type move struct {
	At    time.Time
	ID    uuid.UUID
	Delta int32
}

func before(a, b move) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	return bytes.Compare(a.ID[:], b.ID[:]) < 0 // PostgreSQL orders uuid bytewise too
}

// balanceAt is the quantity just before m would take effect, given the entry's moves.
func balanceAt(moves []move, m move) int64 {
	var sum int64
	for _, x := range moves {
		if before(x, m) {
			sum += int64(x.Delta)
		}
	}
	return sum
}

// balance outcomes of insert.
const (
	balanceOK = iota
	balanceUnderflow
	balanceOverflow
)

// insert checks that adding m keeps every running balance within 0..MaxQuantity and returns
// the new final quantity.
func insert(moves []move, m move) (int32, int) {
	all := append(append(make([]move, 0, len(moves)+1), moves...), m)
	sort.SliceStable(all, func(i, j int) bool { return before(all[i], all[j]) })
	var sum int64
	for _, x := range all {
		sum += int64(x.Delta)
		switch {
		case sum < 0:
			return 0, balanceUnderflow
		case sum > MaxQuantity:
			return 0, balanceOverflow
		}
	}
	return int32(sum), balanceOK
}

// stored is t as PostgreSQL keeps it (microseconds), so the order checked here is the order
// stored.
func stored(t time.Time) time.Time { return t.Truncate(time.Microsecond) }

// likePattern turns a search into a lower-case LIKE pattern with wildcards escaped.
func likePattern(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

// --- validation ---

type checker struct{ v *apperr.Validator }

func (c checker) name(field, s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(t); n < 1 || n > maxLen {
		c.v.Add(field, "length", map[string]string{"min": "1", "max": strconv.Itoa(maxLen)})
	} else if !textcheck.SingleLine(t) {
		c.v.Add(field, "invalid_characters", nil)
	}
	return t
}

func (c checker) text(field string, s *string, maxLen int, multiLine bool) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	ok := textcheck.SingleLine(t)
	if multiLine {
		ok = textcheck.MultiLine(t)
	}
	if utf8.RuneCountInString(t) > maxLen {
		c.v.Add(field, "length", map[string]string{"max": strconv.Itoa(maxLen)})
	} else if !ok {
		c.v.Add(field, "invalid_characters", nil)
	}
	return &t
}

func (c checker) category(s string) string {
	c.v.Check(categories[s], "category", "unsupported", map[string]string{"allowed": "fish,coral,invertebrate,algae,other"})
	return s
}

func (c checker) quarantineStatus(s *string) *string {
	if s != nil && !quarantine[*s] {
		c.v.Add("quarantine_status", "unsupported", map[string]string{"allowed": "in_quarantine,completed"})
	}
	return s
}

func (c checker) coralType(s *string, category string) *string {
	if s == nil {
		return nil
	}
	switch {
	case !coralTypes[*s]:
		c.v.Add("coral_type", "unsupported", map[string]string{"allowed": "sps,lps,soft,non_photosynthetic"})
	case category != "coral":
		c.v.Add("coral_type", "coral_only", nil)
	}
	return s
}

func (c checker) form(s *string) *string {
	if s != nil && !forms[*s] {
		c.v.Add("form", "unsupported", map[string]string{"allowed": "individual,group,frag,colony"})
	}
	return s
}

// instant checks an event time: after 2000-01-01 and not in the future.
func (c checker) instant(field string, t, now time.Time) {
	switch {
	case t.Before(earliest):
		c.v.Add(field, "out_of_range", map[string]string{"min": "2000-01-01T00:00:00Z"})
	case t.After(now.Add(maxFutureSkew)):
		c.v.Add(field, "in_future", nil)
	}
}

func (c checker) quantity(field string, n int32, minimum int32) {
	c.v.Check(n >= minimum && n <= MaxEventQuantity, field, "out_of_range",
		map[string]string{"min": strconv.Itoa(int(minimum)), "max": strconv.Itoa(MaxEventQuantity)})
}
