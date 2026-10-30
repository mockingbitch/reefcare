package tanks

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/ratelimit"
	"reefcare/internal/timeline"
)

// Abuse limits. Shares are also rate limited because a share lookup reveals whether an
// email or username belongs to an account.
const (
	MaxTanksPerOwner = 100
	MaxSharesPerTank = 20
)

// Actor is the authenticated caller.
type Actor struct {
	UserID        uuid.UUID
	EmailVerified bool
	Origin        audit.Origin
}

// Limits are the tank module's rate limiters.
type Limits struct {
	SharesPerUser *ratelimit.Limiter
}

// DefaultLimits returns the documented limits.
func DefaultLimits(clk clock.Clock) Limits {
	return Limits{SharesPerUser: ratelimit.New(30, time.Hour, clk)}
}

// Service implements tank workflows. Every method authorizes through Authorizer.
type Service struct {
	pool   *pgxpool.Pool
	clock  clock.Clock
	authz  Authorizer
	limits Limits
	log    *slog.Logger
}

// NewService wires the tank service.
func NewService(pool *pgxpool.Pool, clk clock.Clock, limits Limits, log *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, limits: limits, log: log}
}

var (
	errVersionConflict = apperr.Conflict("version_conflict", "The tank was changed elsewhere. Reload and try again.")
	errTankLimit       = apperr.Conflict("limit_reached", "You have reached the maximum number of tanks.")
	errShareLimit      = apperr.Conflict("limit_reached", "This tank is shared with the maximum number of people.")
	errDuplicateShare  = apperr.Conflict("duplicate", "This tank is already shared with that person.")
	errNotVerified     = apperr.Forbidden("email_not_verified", "Verify your email address before sharing a tank.")
)

func (s *Service) parser(v *apperr.Validator) parser {
	return parser{v: v, today: s.clock.Now().UTC().Truncate(24 * time.Hour)}
}

// Create adds a private tank owned by the actor and records tank.created on its
// timeline (US-TK-1).
func (s *Service) Create(ctx context.Context, a Actor, in CreateInput) (Tank, error) {
	v := apperr.NewValidator()
	f := s.parser(v).create(in)
	if err := v.Err(); err != nil {
		return Tank{}, err
	}
	now := s.clock.Now()
	var t Tank
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockUserRow(ctx, a.UserID); err != nil {
			return err
		}
		n, err := q.CountOwnedTanks(ctx, a.UserID)
		if err != nil {
			return err
		}
		if n >= MaxTanksPerOwner {
			return errTankLimit
		}
		p := dbq.CreateTankParams{
			OwnerID: a.UserID, Name: f.Name, Description: f.Description, TankType: f.TankType, Status: f.Status,
			DisplayVolumeL: f.DisplayVolumeL, SumpVolumeL: f.SumpVolumeL, SystemVolumeL: f.SystemVolumeL,
			SetupDate: f.SetupDate, WaterSource: f.WaterSource, SaltMix: f.SaltMix, Now: now,
		}
		if d := f.Dimensions; d != nil {
			p.LengthCm, p.WidthCm, p.HeightCm = &d.Length, &d.Width, &d.Height
		}
		id, err := q.CreateTank(ctx, p)
		if err != nil {
			return err
		}
		if _, err := timeline.Record(ctx, q, now, timeline.Event{
			TankID: id, OccurredAt: now, Category: timeline.CategoryMilestone, EventType: "tank.created",
			SourceType: "tank", SourceID: id, ActorID: &a.UserID,
			Summary: map[string]any{"name": f.Name, "tank_type": f.TankType},
		}); err != nil {
			return err
		}
		t, err = s.load(ctx, q, a.UserID, id)
		return err
	})
	return t, err
}

// Get returns a tank the actor may read (US-TK-2).
func (s *Service) Get(ctx context.Context, a Actor, id uuid.UUID) (Tank, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, id, PermRead); err != nil {
		return Tank{}, err
	}
	return s.load(ctx, q, a.UserID, id)
}

func (s *Service) load(ctx context.Context, q *dbq.Queries, userID, id uuid.UUID) (Tank, error) {
	row, err := q.GetTank(ctx, dbq.GetTankParams{ID: id, UserID: userID})
	if database.IsNoRows(err) {
		return Tank{}, apperr.NotFound()
	}
	if err != nil {
		return Tank{}, err
	}
	return fromRow(row), nil
}

func fromRow(r dbq.GetTankRow) Tank {
	t := Tank{
		ID: r.ID, OwnerID: r.OwnerID, Access: r.Access, Name: r.Name, Description: r.Description,
		TankType: r.TankType, Status: r.Status, DisplayVolumeL: r.DisplayVolumeL, SumpVolumeL: r.SumpVolumeL,
		SystemVolumeL: r.SystemVolumeL, SetupDate: r.SetupDate, WaterSource: r.WaterSource, SaltMix: r.SaltMix,
		ArchivedAt: r.ArchivedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
	if r.LengthCm != nil && r.WidthCm != nil && r.HeightCm != nil {
		t.Dimensions = &Dimensions{Length: *r.LengthCm, Width: *r.WidthCm, Height: *r.HeightCm}
	}
	return t
}

// ListCursor is the keyset of the tank list.
type ListCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

// ListParams filters the tank list.
type ListParams struct {
	IncludeShared   bool
	IncludeArchived bool
	Limit           int32 // rows to fetch
	Cursor          *ListCursor
}

// List returns the actor's tanks, newest first, plus shared tanks on request (US-TK-2).
func (s *Service) List(ctx context.Context, a Actor, p ListParams) ([]Tank, error) {
	arg := dbq.ListTanksForUserParams{
		UserID: a.UserID, IncludeShared: p.IncludeShared, IncludeArchived: p.IncludeArchived, RowLimit: p.Limit,
	}
	if p.Cursor != nil {
		arg.CursorCreatedAt, arg.CursorID = &p.Cursor.CreatedAt, &p.Cursor.ID
	}
	rows, err := dbq.New(s.pool).ListTanksForUser(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]Tank, len(rows))
	for i, r := range rows {
		out[i] = fromRow(dbq.GetTankRow(r))
	}
	return out, nil
}

// Update applies a merge patch under optimistic locking (US-TK-3). Archived tanks are
// read-only.
func (s *Service) Update(ctx context.Context, a Actor, id uuid.UUID, pt Patch) (Tank, error) {
	now := s.clock.Now()
	var t Tank
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, id); err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, id, PermWrite); err != nil {
			return err
		}
		cur, err := s.load(ctx, q, a.UserID, id)
		if err != nil {
			return err
		}
		v := apperr.NewValidator()
		if pt.Version == nil {
			v.Add("version", "required", nil)
		} else if *pt.Version != cur.Version {
			return errVersionConflict // never validate a patch against state the client hasn't seen
		}
		f := s.parser(v).apply(cur, pt)
		if err := v.Err(); err != nil {
			return err
		}
		p := dbq.UpdateTankParams{
			ID: id, Version: *pt.Version, Name: f.Name, Description: f.Description, TankType: f.TankType,
			Status: f.Status, DisplayVolumeL: f.DisplayVolumeL, SumpVolumeL: f.SumpVolumeL,
			SystemVolumeL: f.SystemVolumeL, SetupDate: f.SetupDate, WaterSource: f.WaterSource, SaltMix: f.SaltMix,
			Now: now,
		}
		if d := f.Dimensions; d != nil {
			p.LengthCm, p.WidthCm, p.HeightCm = &d.Length, &d.Width, &d.Height
		}
		n, err := q.UpdateTank(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 { // unreachable while the row lock is held; kept as a guard
			return errVersionConflict
		}
		t, err = s.load(ctx, q, a.UserID, id)
		return err
	})
	return t, err
}

// Archive makes a tank read-only for good (US-TK-3 AC2: tanks with history are never
// hard-deleted). Archiving an archived tank is a no-op.
func (s *Service) Archive(ctx context.Context, a Actor, id uuid.UUID) (Tank, error) {
	now := s.clock.Now()
	var t Tank
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, id); err != nil {
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, id, PermManage); err != nil {
			return err
		}
		n, err := q.ArchiveTank(ctx, dbq.ArchiveTankParams{ID: id, Now: now})
		if err != nil {
			return err
		}
		if n == 1 {
			if _, err := timeline.Record(ctx, q, now, timeline.Event{
				TankID: id, OccurredAt: now, Category: timeline.CategoryMilestone, EventType: "tank.archived",
				SourceType: "tank", SourceID: id, ActorID: &a.UserID,
			}); err != nil {
				return err
			}
		}
		t, err = s.load(ctx, q, a.UserID, id)
		return err
	})
	return t, err
}

// Share is a read-only grant. The grantee's email is never exposed.
type Share struct {
	UserID      uuid.UUID
	DisplayName string
	Username    *string
	Permission  string
	CreatedAt   time.Time
}

// ListShares returns who can view the tank (owner only).
func (s *Service) ListShares(ctx context.Context, a Actor, id uuid.UUID) ([]Share, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, id, PermManage); err != nil {
		return nil, err
	}
	return listShares(ctx, q, id)
}

func listShares(ctx context.Context, q *dbq.Queries, id uuid.UUID) ([]Share, error) {
	rows, err := q.ListTankShares(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]Share, len(rows))
	for i, r := range rows {
		out[i] = Share{UserID: r.UserID, DisplayName: r.DisplayName, Username: r.Username, Permission: r.Permission, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// ShareInput names the grantee by exactly one of email or username.
type ShareInput struct {
	Email    *string
	Username *string
}

// Grant shares the tank read-only with another active account whose email is verified
// (US-TK-4, decision D4); unverified accounts are reported as not found.
// The owner must have a verified email, and grants are rate limited, because the
// answer reveals whether an address or username has an account.
func (s *Service) Grant(ctx context.Context, a Actor, id uuid.UUID, in ShareInput) (Share, error) {
	q := dbq.New(s.pool)
	if _, err := s.authz.Require(ctx, q, a.UserID, id, PermManage); err != nil {
		return Share{}, err
	}
	if !a.EmailVerified {
		return Share{}, errNotVerified
	}
	v := apperr.NewValidator()
	field := "email"
	switch {
	case (in.Email == nil) == (in.Username == nil):
		v.Add("email", "exactly_one_of", map[string]string{"fields": "email,username"})
	case in.Username != nil:
		field = "username"
		v.Check(strings.TrimSpace(*in.Username) != "", "username", "required", nil)
	default:
		v.Check(strings.TrimSpace(*in.Email) != "", "email", "required", nil)
	}
	if err := v.Err(); err != nil {
		return Share{}, err
	}
	if ok, retry := s.limits.SharesPerUser.Allow(a.UserID.String()); !ok {
		return Share{}, apperr.RateLimited(retry)
	}

	var grantee uuid.UUID
	var err error
	if field == "email" {
		grantee, err = q.FindShareGranteeByEmail(ctx, strings.ToLower(strings.TrimSpace(*in.Email)))
	} else {
		grantee, err = q.FindShareGranteeByUsername(ctx, strings.ToLower(strings.TrimSpace(*in.Username)))
	}
	if database.IsNoRows(err) {
		v.Add(field, "grantee_not_found", nil)
		return Share{}, v.Err()
	}
	if err != nil {
		return Share{}, err
	}
	if grantee == a.UserID {
		v.Add(field, "cannot_share_with_self", nil)
		return Share{}, v.Err()
	}

	now := s.clock.Now()
	var share Share
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := q.LockTankRow(ctx, id); err != nil { // serializes the share-limit check
			return err
		}
		if _, err := s.authz.Require(ctx, q, a.UserID, id, PermManage); err != nil {
			return err
		}
		count, err := q.CountTankShares(ctx, id)
		if err != nil {
			return err
		}
		if count >= MaxSharesPerTank {
			return errShareLimit
		}
		n, err := q.CreateTankShare(ctx, dbq.CreateTankShareParams{TankID: id, UserID: grantee, GrantedBy: &a.UserID, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return errDuplicateShare
		}
		if err := audit.Record(ctx, q, now, a.Origin, audit.Entry{
			ActorID: &a.UserID, Action: "tank.share.grant", TargetType: "tank", TargetID: &id,
			Metadata: map[string]any{"grantee_id": grantee.String(), "permission": "viewer"},
		}); err != nil {
			return err
		}
		shares, err := listShares(ctx, q, id)
		if err != nil {
			return err
		}
		for _, sh := range shares {
			if sh.UserID == grantee {
				share = sh
				return nil
			}
		}
		return errors.New("share not found after insert")
	})
	return share, err
}

// Revoke removes a share. The owner can revoke anyone; a viewer can remove themselves.
// Revocation takes effect on the next request (US-TK-4 AC2).
func (s *Service) Revoke(ctx context.Context, a Actor, id, grantee uuid.UUID) error {
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		acc, err := s.authz.Require(ctx, q, a.UserID, id, PermRead)
		if err != nil {
			return err
		}
		if !acc.IsOwner && grantee != a.UserID {
			return errReadOnly
		}
		n, err := q.DeleteTankShare(ctx, dbq.DeleteTankShareParams{TankID: id, UserID: grantee})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound()
		}
		return audit.Record(ctx, q, now, a.Origin, audit.Entry{
			ActorID: &a.UserID, Action: "tank.share.revoke", TargetType: "tank", TargetID: &id,
			Metadata: map[string]any{"grantee_id": grantee.String()},
		})
	})
}
