package intelligence

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"reefcare/internal/audit"
	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/apperr"
	"reefcare/internal/platform/database"
	"reefcare/internal/platform/textcheck"
	"reefcare/internal/tanks"
	"reefcare/internal/timeline"
)

// Feedback is one user's rating of an assessment (docs/knowledge-engine.md §10).
type Feedback struct {
	Rating      string
	OutcomeNote *string
	UpdatedAt   time.Time
}

var (
	ratings             = map[string]bool{"useful": true, "not_useful": true, "wrong": true}
	errAlreadyRetracted = apperr.Conflict("already_retracted", "This assessment was already retracted.")
)

func cleanText(v *apperr.Validator, field string, s *string, maxLen int, required bool) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		if required {
			v.Add(field, "required", nil)
		}
		return nil
	}
	t := strings.TrimSpace(*s)
	if utf8.RuneCountInString(t) > maxLen {
		v.Add(field, "length", map[string]string{"max": itoa(int64(maxLen))})
	} else if !textcheck.MultiLine(t) {
		v.Add(field, "invalid_characters", nil)
	}
	return &t
}

// requireAssessment resolves the assessment's tank and authorizes the caller on it.
func (s *Service) requireAssessment(ctx context.Context, q *dbq.Queries, a tanks.Actor, id uuid.UUID, perm tanks.Perm) (uuid.UUID, error) {
	tankID, err := q.GetAssessmentTank(ctx, id)
	if database.IsNoRows(err) {
		return uuid.Nil, errNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.authz.Require(ctx, q, a.UserID, tankID, perm)
	return tankID, err
}

// GiveFeedback records (or replaces) the caller's rating of an assessment. Anyone who may read
// the tank may rate it; the assessment itself never changes.
func (s *Service) GiveFeedback(ctx context.Context, a tanks.Actor, id uuid.UUID, rating string, note *string) (Feedback, error) {
	v := apperr.NewValidator()
	if !ratings[rating] {
		v.Add("rating", "unsupported", map[string]string{"allowed": "useful,not_useful,wrong"})
	}
	n := cleanText(v, "outcome_note", note, 2000, false)
	if err := v.Err(); err != nil {
		return Feedback{}, err
	}
	now := s.clock.Now()
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if _, err := s.requireAssessment(ctx, q, a, id, tanks.PermRead); err != nil {
			return err
		}
		return q.UpsertAssessmentFeedback(ctx, dbq.UpsertAssessmentFeedbackParams{AssessmentID: id, UserID: a.UserID, Rating: rating, OutcomeNote: n, Now: now})
	})
	if err != nil {
		return Feedback{}, err
	}
	return Feedback{Rating: rating, OutcomeNote: n, UpdatedAt: now}, nil
}

// MyFeedback returns the caller's feedback on an assessment, if any (call after authorization).
func (s *Service) myFeedback(ctx context.Context, q *dbq.Queries, userID, id uuid.UUID) (*Feedback, error) {
	r, err := q.GetAssessmentFeedback(ctx, dbq.GetAssessmentFeedbackParams{AssessmentID: id, UserID: userID})
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Feedback{Rating: r.Rating, OutcomeNote: r.OutcomeNote, UpdatedAt: r.UpdatedAt}, nil
}

// Retract withdraws an assessment with a reason (owner). It stays readable, marked retracted;
// its content never changes. Audited and on the timeline.
func (s *Service) Retract(ctx context.Context, a tanks.Actor, id uuid.UUID, reason *string) (Assessment, error) {
	v := apperr.NewValidator()
	r := cleanText(v, "reason", reason, 500, true)
	if err := v.Err(); err != nil {
		return Assessment{}, err
	}
	now := s.clock.Now()
	err := database.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		tankID, err := s.requireAssessment(ctx, q, a, id, tanks.PermWrite)
		if err != nil {
			return err
		}
		if _, err := q.LockAssessment(ctx, id); err != nil {
			return err
		}
		n, err := q.RetractAssessment(ctx, dbq.RetractAssessmentParams{ID: id, Reason: r, Now: now})
		if err != nil {
			return err
		}
		if n == 0 {
			return errAlreadyRetracted
		}
		if err := audit.Record(ctx, q, now, a.Origin, audit.Entry{ActorID: &a.UserID, Action: "assessment.retracted", TargetType: "assessment", TargetID: &id}); err != nil {
			return err
		}
		_, err = timeline.Record(ctx, q, now, timeline.Event{
			TankID: tankID, OccurredAt: now, Category: timeline.CategoryAssessment, EventType: "assessment.retracted",
			SourceType: "assessment", SourceID: id, ActorID: &a.UserID,
		})
		return err
	})
	if err != nil {
		return Assessment{}, err
	}
	return s.Get(ctx, a, id)
}
