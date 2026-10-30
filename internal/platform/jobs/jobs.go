// Package jobs is the durable background job queue (docs/architecture.md §5, decision
// D3). Jobs live in PostgreSQL: they are enqueued inside the transaction of the change
// that needs them (so a rolled-back change never leaves a job behind), claimed with
// FOR UPDATE SKIP LOCKED under a time-limited lease, retried with exponential backoff and
// buried as "dead" after their last attempt.
//
// Delivery is at least once: a worker can crash after a handler's side effect but before
// recording success, so every handler must be idempotent (re-check state, rely on
// unique constraints).
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"reefcare/internal/db/dbq"
)

// DefaultMaxAttempts is used when Spec.MaxAttempts is zero.
const DefaultMaxAttempts = 10

// Spec describes a job to enqueue.
type Spec struct {
	Kind string // e.g. "identity.email"; must be registered with Runner.Handle
	// Payload is stored as JSON. Never put secrets (tokens, passwords) or more personal
	// data than an id in it; load what you need when the job runs.
	Payload any
	// DedupeKey makes enqueuing idempotent: a second job with the same (Kind, DedupeKey)
	// is not created while the first row exists.
	DedupeKey   string
	RunAt       time.Time // zero means now
	MaxAttempts int32     // zero means DefaultMaxAttempts
	// Completed inserts the row as already succeeded, so it is never claimed. Use it when
	// the insert must happen for uniform timing (enumeration safety) but there is no work.
	Completed bool
}

// Enqueue adds a job using q, normally bound to the caller's transaction. It reports
// false when an equal DedupeKey already exists.
func Enqueue(ctx context.Context, q *dbq.Queries, now time.Time, s Spec) (bool, error) {
	payload := []byte("{}")
	if s.Payload != nil {
		var err error
		if payload, err = json.Marshal(s.Payload); err != nil {
			return false, fmt.Errorf("encode %s payload: %w", s.Kind, err)
		}
	}
	runAt := s.RunAt
	if runAt.IsZero() {
		runAt = now
	}
	maxAttempts := s.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = DefaultMaxAttempts
	}
	var dedupe *string
	if s.DedupeKey != "" {
		dedupe = &s.DedupeKey
	}
	n, err := q.EnqueueJob(ctx, dbq.EnqueueJobParams{
		Kind: s.Kind, DedupeKey: dedupe, Payload: payload, RunAt: runAt, MaxAttempts: maxAttempts, Now: now,
		Completed: s.Completed,
	})
	if err != nil {
		return false, fmt.Errorf("enqueue %s: %w", s.Kind, err)
	}
	return n == 1, nil
}

// Job is a claimed job handed to a Handler.
type Job struct {
	ID          int64
	Kind        string
	Payload     json.RawMessage
	Attempt     int32 // 1 on the first run
	MaxAttempts int32
}

// Decode unmarshals the payload; a malformed payload is a permanent failure.
func (j Job) Decode(dst any) error {
	if err := json.Unmarshal(j.Payload, dst); err != nil {
		return Permanent(fmt.Errorf("decode %s payload: %w", j.Kind, err))
	}
	return nil
}

// Handler runs one job. Returning nil marks it succeeded; an error schedules a retry,
// unless it wraps Permanent. The error text is stored in jobs.last_error, so it must not
// contain personal data or secrets.
type Handler func(ctx context.Context, j Job) error

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as not worth retrying (bad payload, missing configuration, …).
func Permanent(err error) error { return permanentError{err} }

// IsPermanent reports whether err was marked with Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// Backoff is the delay before retry number attempt (1-based): 15 s doubling per attempt,
// capped at 1 h, with ±20 % jitter so failed jobs don't retry in lockstep. jitter is a
// value in [0, 1).
func Backoff(attempt int32, jitter float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := 15 * time.Second * time.Duration(math.Pow(2, float64(min(attempt-1, 20))))
	base = min(base, time.Hour)
	factor := 0.8 + 0.4*jitter
	return time.Duration(float64(base) * factor)
}

func randomJitter() float64 { return rand.Float64() } //nolint:gosec // retry jitter, not security

// Revive gives a dead job with this kind and dedupe key a fresh set of attempts. It
// reports whether a dead job was found. Use it for work that must eventually happen.
func Revive(ctx context.Context, q *dbq.Queries, now time.Time, kind, dedupeKey string) (bool, error) {
	n, err := q.ReviveDeadJob(ctx, dbq.ReviveDeadJobParams{Kind: kind, DedupeKey: &dedupeKey, Now: now})
	if err != nil {
		return false, fmt.Errorf("revive %s: %w", kind, err)
	}
	return n == 1, nil
}
