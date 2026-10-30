package tanks

import (
	"testing"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
)

func TestDecide(t *testing.T) {
	owner, viewer, stranger, tank := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	row := func(status, ownerStatus string, isViewer bool) dbq.GetTankAccessRow {
		return dbq.GetTankAccessRow{OwnerID: owner, Status: status, OwnerStatus: ownerStatus, Viewer: isViewer}
	}
	const ok = apperr.Kind(0)
	tests := []struct {
		name string
		row  dbq.GetTankAccessRow
		user uuid.UUID
		perm Perm
		want apperr.Kind
	}{
		{"owner reads", row("active", "active", false), owner, PermRead, ok},
		{"owner writes", row("active", "active", false), owner, PermWrite, ok},
		{"owner manages archived", row("archived", "active", false), owner, PermManage, ok},
		{"owner cannot write archived", row("archived", "active", false), owner, PermWrite, apperr.KindConflict},
		{"viewer reads", row("active", "active", true), viewer, PermRead, ok},
		{"viewer reads archived", row("archived", "active", true), viewer, PermRead, ok},
		{"viewer cannot write", row("active", "active", true), viewer, PermWrite, apperr.KindForbidden},
		{"viewer cannot manage", row("active", "active", true), viewer, PermManage, apperr.KindForbidden},
		{"stranger read is 404", row("active", "active", false), stranger, PermRead, apperr.KindNotFound},
		{"stranger write is 404", row("active", "active", false), stranger, PermWrite, apperr.KindNotFound},
		{"deactivated owner hides tank", row("active", "deactivated", true), viewer, PermRead, apperr.KindNotFound},
		{"suspended owner still visible", row("active", "suspended", true), viewer, PermRead, ok},
		{"unknown perm denied", row("active", "active", false), owner, Perm(99), apperr.KindNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := decide(tt.row, tt.user, tank, tt.perm)
			if tt.want == ok {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				if a.TankID != tank || a.IsOwner != (tt.user == owner) {
					t.Fatalf("access %+v", a)
				}
				return
			}
			if !apperr.IsKind(err, tt.want) {
				t.Fatalf("got %v, want kind %d", err, tt.want)
			}
		})
	}
}
