package identity

import (
	"context"
	"errors"
	"fmt"
	osuser "os/user"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
)

// Roles that grant permissions (docs/security.md §4). Community rank never does.
var grantableRoles = map[string]bool{"moderator": true, "administrator": true}

// SetRole grants or revokes an authorization role from the command line (`api roles grant
// <email> <role>`); there is no HTTP endpoint for it. The change is audited without an actor.
// It reports whether anything changed.
func SetRole(ctx context.Context, pool *pgxpool.Pool, clk clock.Clock, email, role string, grant bool) (bool, error) {
	if !grantableRoles[role] {
		return false, fmt.Errorf("unknown role %q (moderator, administrator)", role)
	}
	changed := false
	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		u, err := q.GetUserCredentialsByEmail(ctx, normalizeEmail(email))
		if database.IsNoRows(err) {
			return errors.New("no account with this email")
		}
		if err != nil {
			return err
		}
		if grant && (u.Status != "active" || u.EmailVerifiedAt == nil) {
			return errors.New("the account must be active and verified")
		}
		now := clk.Now()
		var n int64
		action := "role.granted"
		if grant {
			n, err = q.GrantUserRole(ctx, dbq.GrantUserRoleParams{UserID: u.ID, Role: role, Now: now})
		} else {
			action = "role.revoked"
			n, err = q.RevokeUserRole(ctx, dbq.RevokeUserRoleParams{UserID: u.ID, Role: role})
		}
		if err != nil || n == 0 {
			return err
		}
		changed = true
		// No account acts here: record the operating-system user who ran the command.
		return audit.Record(ctx, q, now, audit.Origin{RequestID: "cli"}, audit.Entry{
			Action: action, TargetType: "user", TargetID: &u.ID, Metadata: map[string]any{"role": role, "operator": operator()},
		})
	})
	return changed, err
}

func operator() string {
	if u, err := osuser.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}
