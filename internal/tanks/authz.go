package tanks

import (
	"context"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/database"
)

// Perm is what a caller wants to do with a tank or its data.
type Perm int

const (
	// PermRead allows viewing the tank and everything recorded on it (owner or viewer).
	PermRead Perm = iota + 1
	// PermManage allows owner-only actions that are still valid on an archived tank
	// (archiving, managing shares).
	PermManage
	// PermWrite allows changing the tank or recording new data on it: owner only, and
	// never on an archived tank.
	PermWrite
)

// Access is the outcome of a successful authorization.
type Access struct {
	TankID   uuid.UUID
	OwnerID  uuid.UUID
	IsOwner  bool
	Archived bool
}

var (
	errReadOnly = apperr.Forbidden("forbidden", "You can view this tank but not change it.")
	errArchived = apperr.Conflict("tank_archived", "This tank is archived and can no longer be changed.")
)

// Authorizer decides access to tanks and every tank-owned resource
// (docs/architecture.md §Authorization). Every service method touching tank data calls
// Require before reading or writing; resources addressed by their own ID resolve their
// tank_id first.
//
// Callers who may not see a tank get 404 (never 403), so tank IDs cannot be probed.
// A viewer attempting a write gets 403: they already know the tank exists.
type Authorizer struct{}

// Require checks that userID may perform perm on tankID. For PermWrite and PermManage
// pass the querier of the transaction the check guards: the tank row is then share-locked
// until that transaction ends, so the tank cannot be archived (or change owner state)
// between the check and the write. Workflows that update the tank row itself take
// LockTankRow first.
func (Authorizer) Require(ctx context.Context, q *dbq.Queries, userID, tankID uuid.UUID, perm Perm) (Access, error) {
	var row dbq.GetTankAccessRow
	var err error
	if perm == PermRead {
		row, err = q.GetTankAccess(ctx, dbq.GetTankAccessParams{TankID: tankID, UserID: userID})
	} else {
		var w dbq.GetTankAccessForWriteRow
		w, err = q.GetTankAccessForWrite(ctx, dbq.GetTankAccessForWriteParams{TankID: tankID, UserID: userID})
		row = dbq.GetTankAccessRow(w)
	}
	if database.IsNoRows(err) {
		return Access{}, apperr.NotFound()
	}
	if err != nil {
		return Access{}, err
	}
	return decide(row, userID, tankID, perm)
}

// decide is the pure policy behind Require.
func decide(row dbq.GetTankAccessRow, userID, tankID uuid.UUID, perm Perm) (Access, error) {
	a := Access{TankID: tankID, OwnerID: row.OwnerID, IsOwner: row.OwnerID == userID, Archived: row.Status == StatusArchived}
	if !a.IsOwner && (!row.Viewer || row.OwnerStatus == "deactivated") {
		return Access{}, apperr.NotFound()
	}
	switch perm {
	case PermRead:
		return a, nil
	case PermManage, PermWrite:
		if !a.IsOwner {
			return Access{}, errReadOnly
		}
		if perm == PermWrite && a.Archived {
			return Access{}, errArchived
		}
		return a, nil
	default:
		return Access{}, apperr.NotFound()
	}
}
