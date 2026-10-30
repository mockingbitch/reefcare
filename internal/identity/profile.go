package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/patch"
	"reefcare/internal/platform/textcheck"
)

// AccountEraseDelay is how long a deactivated account is kept before its personal data
// is erased or anonymized (docs/security.md §9).
const AccountEraseDelay = 30 * 24 * time.Hour

// unitPrefOptions are the display-unit choices per quantity (US-ID-4). They only change
// how values are shown; stored measurements always keep their original value and unit.
// Codes must match the parameter catalog when it ships (milestone 2.1).
var unitPrefOptions = map[string][]string{
	"volume":      {"L", "gal_us"},
	"length":      {"cm", "in"},
	"temperature": {"C", "F"},
	"alkalinity":  {"dKH", "meq/L", "ppm"},
	"salinity":    {"SG", "ppt"},
}

// notificationPrefKeys: email_* are opt-in email channels (default off); reminders_<category>
// turn a category of in-app reminders off (default on; package notifications). Security
// emails (password changed, account deactivated) cannot be turned off. email_assessments and
// email_digest are reserved for Phase 3 / open item NT3.
var notificationPrefKeys = map[string]bool{
	"email_reminders":        true,
	"email_assessments":      true,
	"email_digest":           true,
	"reminders_dosing":       true,
	"reminders_water_change": true,
	"reminders_maintenance":  true,
}

var (
	usernameRe          = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)
	experienceLevels    = map[string]bool{"beginner": true, "intermediate": true, "advanced": true, "professional": true}
	errVersionConflict  = apperr.Conflict("version_conflict", "The profile was changed elsewhere. Reload and try again.")
	errUsernameTaken    = apperr.Conflict("username_taken", "This username is already taken.")
	errProfileNotActive = apperr.Unauthenticated("unauthenticated", "Sign in to continue.")
)

func validateDisplayName(v *apperr.Validator, name string) {
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		v.Add("display_name", "length", map[string]string{"min": "1", "max": "60"})
	} else if !textcheck.SingleLine(name) {
		v.Add("display_name", "invalid_characters", nil)
	}
}

func decodePrefs(unitRaw, notifyRaw []byte) (map[string]string, map[string]bool, error) {
	units := map[string]string{}
	notify := map[string]bool{}
	if len(unitRaw) > 0 {
		if err := json.Unmarshal(unitRaw, &units); err != nil {
			return nil, nil, fmt.Errorf("decode unit_prefs: %w", err)
		}
	}
	if len(notifyRaw) > 0 {
		if err := json.Unmarshal(notifyRaw, &notify); err != nil {
			return nil, nil, fmt.Errorf("decode notification_prefs: %w", err)
		}
	}
	return units, notify, nil
}

// ProfilePatch is a merge patch of the caller's profile. Version is required. In the
// preference maps a null value removes the key (back to the default).
type ProfilePatch struct {
	Version           *int32
	DisplayName       patch.Field[string]
	Username          patch.Field[string]
	Bio               patch.Field[string]
	ExperienceLevel   patch.Field[string]
	Locale            patch.Field[string]
	Timezone          patch.Field[string]
	UnitPrefs         patch.Field[map[string]*string]
	NotificationPrefs patch.Field[map[string]*bool]
}

// UpdateProfile applies a merge patch to the caller's profile under optimistic locking
// (US-ID-4). Changing the timezone never changes stored timestamps; it only changes how
// they are rendered.
func (s *Service) UpdateProfile(ctx context.Context, p Principal, in ProfilePatch) (User, error) {
	q := dbq.New(s.pool)
	cur, err := s.user(ctx, q, p.UserID)
	if err != nil {
		return User{}, err
	}
	v := apperr.NewValidator()
	if in.Version == nil {
		v.Add("version", "required", nil)
	} else if *in.Version != cur.Version {
		return User{}, errVersionConflict // never validate a patch against state the client hasn't seen
	}
	// Required attributes reject null.
	required := func(field string, f patch.Field[string]) bool {
		if f.Set && f.Null {
			v.Add(field, "required", nil)
			return false
		}
		return f.Set
	}

	name := cur.DisplayName
	if required("display_name", in.DisplayName) {
		name = strings.TrimSpace(in.DisplayName.Value)
		validateDisplayName(v, name)
	}
	username := cur.Username
	if in.Username.Set && !in.Username.Null {
		u := strings.ToLower(strings.TrimSpace(in.Username.Value))
		v.Check(usernameRe.MatchString(u), "username", "invalid_format", map[string]string{"pattern": usernameRe.String()})
		username = &u
	} else if in.Username.Null {
		username = nil
	}
	bio := cur.Bio
	if in.Bio.Set {
		bio = nil
		if b := strings.TrimSpace(in.Bio.Value); !in.Bio.Null && b != "" {
			if utf8.RuneCountInString(b) > 1000 {
				v.Add("bio", "length", map[string]string{"max": "1000"})
			} else if !textcheck.MultiLine(b) {
				v.Add("bio", "invalid_characters", nil)
			}
			bio = &b
		}
	}
	level := in.ExperienceLevel.Apply(cur.ExperienceLevel)
	if level != nil {
		v.Check(experienceLevels[*level], "experience_level", "unsupported", nil)
	}
	locale := cur.Locale
	if required("locale", in.Locale) {
		locale = in.Locale.Value
		v.Check(locale == "vi" || locale == "en", "locale", "unsupported", nil)
	}
	tz := cur.Timezone
	if required("timezone", in.Timezone) {
		tz = in.Timezone.Value
		v.Check(validTimezone(tz), "timezone", "invalid_timezone", nil)
	}

	units := cur.UnitPrefs
	if in.UnitPrefs.Set {
		units = mergeUnitPrefs(v, units, in.UnitPrefs)
	}
	notify := cur.NotifyPrefs
	if in.NotificationPrefs.Set {
		notify = mergeNotifyPrefs(v, notify, in.NotificationPrefs)
	}
	if err := v.Err(); err != nil {
		return User{}, err
	}

	unitJSON, err := json.Marshal(units)
	if err != nil {
		return User{}, err
	}
	notifyJSON, err := json.Marshal(notify)
	if err != nil {
		return User{}, err
	}
	n, err := q.UpdateUserProfile(ctx, dbq.UpdateUserProfileParams{
		ID: p.UserID, Version: *in.Version, DisplayName: name, Username: username, Bio: bio,
		ExperienceLevel: level, Locale: locale, Timezone: tz, UnitPrefs: unitJSON, NotificationPrefs: notifyJSON,
		Now: s.clock.Now(),
	})
	if database.IsUniqueViolation(err, "users_username_key") {
		return User{}, errUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	if n == 0 {
		return User{}, errVersionConflict
	}
	return s.user(ctx, q, p.UserID)
}

func mergeUnitPrefs(v *apperr.Validator, cur map[string]string, f patch.Field[map[string]*string]) map[string]string {
	out := map[string]string{}
	if f.Null {
		return out
	}
	maps.Copy(out, cur)
	for _, k := range slices.Sorted(maps.Keys(f.Value)) { // stable error order
		val := f.Value[k]
		options, known := unitPrefOptions[k]
		switch {
		case !known:
			v.Add("unit_prefs."+k, "unknown_field", nil)
		case val == nil:
			delete(out, k)
		case !slices.Contains(options, *val):
			v.Add("unit_prefs."+k, "unsupported_unit", map[string]string{"allowed": strings.Join(options, ",")})
		default:
			out[k] = *val
		}
	}
	return out
}

func mergeNotifyPrefs(v *apperr.Validator, cur map[string]bool, f patch.Field[map[string]*bool]) map[string]bool {
	out := map[string]bool{}
	if f.Null {
		return out
	}
	maps.Copy(out, cur)
	for _, k := range slices.Sorted(maps.Keys(f.Value)) {
		val := f.Value[k]
		switch {
		case !notificationPrefKeys[k]:
			v.Add("notification_prefs."+k, "unknown_field", nil)
		case val == nil:
			delete(out, k)
		default:
			out[k] = *val
		}
	}
	return out
}

// DeleteAccount handles an account deletion request (US-ID-5): it re-checks the
// password, deactivates the account, signs out every session and revokes pending email
// links, all in one transaction. Personal data is erased after AccountEraseDelay by a
// background job (open item J4). Returns the time after which erasure may happen.
func (s *Service) DeleteAccount(ctx context.Context, c Client, p Principal, password string) (time.Time, error) {
	if err := allow(s.limits.PasswordChangePerUser, p.UserID.String()); err != nil {
		return time.Time{}, err
	}
	q := dbq.New(s.pool)
	creds, err := q.GetUserCredentialsByID(ctx, p.UserID)
	if err != nil {
		return time.Time{}, err
	}
	match, _, err := s.hasher.Verify(ctx, password, creds.PasswordHash)
	if apperr.IsKind(err, apperr.KindUnavailable) {
		return time.Time{}, err
	}
	if err != nil || !match {
		v := apperr.NewValidator()
		v.Add("password", "incorrect", nil)
		return time.Time{}, v.Err()
	}
	now := s.clock.Now()
	eraseAfter := now.Add(AccountEraseDelay)
	err = database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		n, err := q.DeactivateUser(ctx, dbq.DeactivateUserParams{ID: p.UserID, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return errProfileNotActive
		}
		if _, err := q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: p.UserID, Now: now}); err != nil {
			return err
		}
		if err := q.RevokeAllLiveAuthTokens(ctx, dbq.RevokeAllLiveAuthTokensParams{UserID: p.UserID, Now: now}); err != nil {
			return err
		}
		if err := enqueueEmail(ctx, q, now, &p.UserID, templateAccountDeactivated, c.Origin); err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{
			ActorID: &p.UserID, Action: "account.deactivated", TargetType: "user", TargetID: &p.UserID,
			Metadata: map[string]any{"erase_after": eraseAfter.UTC().Format(time.RFC3339)},
		})
	})
	if err != nil {
		return time.Time{}, err
	}
	return eraseAfter, nil
}

// LogoutAll revokes every session of the caller, including the current one.
func (s *Service) LogoutAll(ctx context.Context, c Client, p Principal) error {
	now := s.clock.Now()
	return database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		n, err := q.RevokeOtherUserSessions(ctx, dbq.RevokeOtherUserSessionsParams{UserID: p.UserID, Now: now})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, now, c.Origin, audit.Entry{
			ActorID: &p.UserID, Action: "auth.logout_all", TargetType: "user", TargetID: &p.UserID,
			Metadata: map[string]any{"sessions_revoked": n},
		})
	})
}
