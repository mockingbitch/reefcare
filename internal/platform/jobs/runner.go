package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/clock"
)

// Config tunes a Runner.
type Config struct {
	Workers      int           // concurrent jobs; default 4
	PollInterval time.Duration // idle wait between polls; default 1 s
	// Timeout bounds each handler run; the lease is Timeout plus a margin, so a job is
	// only re-claimed after its original run must have stopped. Default 2 min.
	Timeout time.Duration
	// SchedulerInterval is how often periodic jobs are enqueued and expired leases
	// buried; default 30 s.
	SchedulerInterval time.Duration
}

func (c Config) withDefaults() Config {
	if c.Workers <= 0 {
		c.Workers = 4
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Minute
	}
	if c.SchedulerInterval <= 0 {
		c.SchedulerInterval = 30 * time.Second
	}
	return c
}

// Observer receives job outcomes, e.g. for metrics. Outcome is "succeeded", "retry" or
// "dead".
type Observer interface {
	JobFinished(kind, outcome string, d time.Duration)
}

type periodic struct {
	kind     string
	interval time.Duration
}

// Runner claims and runs jobs. Register handlers and periodic jobs before Run.
type Runner struct {
	pool     *pgxpool.Pool
	clock    clock.Clock
	log      *slog.Logger
	cfg      Config
	worker   string
	observer Observer

	handlers map[string]Handler
	kinds    []string
	periodic []periodic

	// abort cancels running handlers at the shutdown deadline (Abort).
	abortCtx context.Context
	abort    context.CancelFunc
}

// NewRunner creates a runner. Each runner has a unique worker id used to fence its leases.
func NewRunner(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger, cfg Config) *Runner {
	host, _ := os.Hostname()
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	worker := truncate(fmt.Sprintf("%s-%d-%s", host, os.Getpid(), hex.EncodeToString(suffix)), 100)
	abortCtx, abort := context.WithCancel(context.Background())
	r := &Runner{
		pool: pool, clock: clk, log: log, cfg: cfg.withDefaults(), worker: worker,
		handlers: map[string]Handler{}, abortCtx: abortCtx, abort: abort,
	}
	r.Handle(cleanupKind, r.cleanup)
	r.Every(cleanupKind, 24*time.Hour)
	return r
}

// Observe sets the outcome observer (optional).
func (r *Runner) Observe(o Observer) { r.observer = o }

// Handle registers the handler for kind. Kinds without a handler are never claimed, so
// several binaries can share one queue.
func (r *Runner) Handle(kind string, h Handler) {
	if _, dup := r.handlers[kind]; dup {
		panic("jobs: duplicate handler for " + kind)
	}
	r.handlers[kind] = h
	r.kinds = append(r.kinds, kind)
}

// Every enqueues kind once per interval, aligned to UTC multiples of interval. The time
// bucket is the dedupe key, so any number of runners enqueue it exactly once. A bucket
// missed while no runner was up is skipped; periodic handlers must therefore catch up on
// everything due ("purge all expired rows"), not only the last interval.
func (r *Runner) Every(kind string, interval time.Duration) {
	r.periodic = append(r.periodic, periodic{kind: kind, interval: interval})
}

// Abort cancels the contexts of running handlers. Call it when the shutdown deadline is
// reached and Run has not returned; interrupted jobs are retried after their lease.
func (r *Runner) Abort() { r.abort() }

// Run polls and runs jobs until ctx is cancelled, then waits for running handlers to
// finish (each is bounded by Config.Timeout, or cut short by Abort).
func (r *Runner) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { r.scheduleLoop(ctx) })
	for range r.cfg.Workers {
		wg.Go(func() { r.workLoop(ctx) })
	}
	wg.Wait()
}

// RunDue runs every job that is due now, one at a time, and returns how many ran. It is
// meant for tests and one-off maintenance; production uses Run.
func (r *Runner) RunDue(ctx context.Context) (int, error) {
	if err := r.schedule(ctx); err != nil {
		return 0, err
	}
	n := 0
	for {
		ran, err := r.runOne(ctx)
		if err != nil || !ran {
			return n, err
		}
		n++
	}
}

func (r *Runner) scheduleLoop(ctx context.Context) {
	for {
		if err := r.schedule(ctx); err != nil && ctx.Err() == nil {
			r.log.Error("schedule jobs", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.cfg.SchedulerInterval):
		}
	}
}

func (r *Runner) schedule(ctx context.Context) error {
	q := dbq.New(r.pool)
	now := r.clock.Now()
	if n, err := q.RequeueExpiredJobs(ctx, now); err != nil {
		return fmt.Errorf("requeue expired jobs: %w", err)
	} else if n > 0 {
		r.log.Warn("jobs re-queued after their worker's lease expired", "jobs", n)
	}
	if n, err := q.BuryExpiredJobs(ctx, now); err != nil {
		return fmt.Errorf("bury expired jobs: %w", err)
	} else if n > 0 {
		r.log.Error("jobs died with their worker on the final attempt", "jobs", n)
	}
	for _, p := range r.periodic {
		bucket := now.UTC().Truncate(p.interval)
		if _, err := Enqueue(ctx, q, now, Spec{
			Kind: p.kind, DedupeKey: "every:" + bucket.Format(time.RFC3339), RunAt: bucket, MaxAttempts: 3,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) workLoop(ctx context.Context) {
	for ctx.Err() == nil {
		ran, err := r.runOne(ctx)
		if err != nil && ctx.Err() == nil {
			r.log.Error("job worker", "err", err)
		}
		if ran {
			continue
		}
		// Idle (or the database is unreachable): wait, with jitter so workers spread out.
		wait := time.Duration(float64(r.cfg.PollInterval) * (0.75 + 0.5*randomJitter()))
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

// runOne claims and runs at most one job. Claiming uses ctx; the handler gets its own
// timeout detached from ctx, so a shutdown lets a started job finish instead of
// abandoning it half-done.
func (r *Runner) runOne(ctx context.Context) (bool, error) {
	if len(r.kinds) == 0 {
		return false, nil
	}
	now := r.clock.Now()
	lease := now.Add(r.cfg.Timeout + 30*time.Second)
	rows, err := dbq.New(r.pool).ClaimJobs(ctx, dbq.ClaimJobsParams{
		Kinds: r.kinds, Now: now, Batch: 1, Worker: r.worker, LeaseUntil: lease,
	})
	if err != nil {
		return false, fmt.Errorf("claim jobs: %w", err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	row := rows[0]
	job := Job{ID: row.ID, Kind: row.Kind, Payload: row.Payload, Attempt: row.Attempts, MaxAttempts: row.MaxAttempts}
	hctx, cancel := context.WithTimeout(r.abortCtx, r.cfg.Timeout)
	defer cancel()
	start := time.Now()
	runErr := r.call(hctx, job)
	// Recording the outcome gets its own short deadline: it must not hang shutdown, and if
	// it fails the lease expires and the job is retried.
	fctx, fcancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer fcancel()
	r.finish(fctx, job, runErr, time.Since(start))
	return true, nil
}

func (r *Runner) call(ctx context.Context, j Job) (err error) {
	defer func() {
		if p := recover(); p != nil {
			r.log.Error("job panicked", "kind", j.Kind, "job_id", j.ID, "panic", p, "stack", string(debug.Stack()))
			err = errors.New("panic") // the value may hold anything; it stays in the logs only
		}
	}()
	return r.handlers[j.Kind](ctx, j)
}

func (r *Runner) finish(ctx context.Context, j Job, runErr error, d time.Duration) {
	q := dbq.New(r.pool)
	now := r.clock.Now()
	var (
		n       int64
		err     error
		outcome string
	)
	switch {
	case runErr == nil:
		outcome = "succeeded"
		n, err = q.CompleteJob(ctx, dbq.CompleteJobParams{ID: j.ID, Worker: r.worker, Attempt: j.Attempt, Now: now})
	case IsPermanent(runErr) || j.Attempt >= j.MaxAttempts:
		outcome = "dead"
		n, err = q.BuryJob(ctx, dbq.BuryJobParams{ID: j.ID, Worker: r.worker, Attempt: j.Attempt, Now: now, LastError: truncate(runErr.Error(), 1000)})
		r.log.Error("job dead", "kind", j.Kind, "job_id", j.ID, "attempt", j.Attempt, "err", runErr)
	default:
		outcome = "retry"
		next := now.Add(Backoff(j.Attempt, randomJitter()))
		n, err = q.RetryJob(ctx, dbq.RetryJobParams{ID: j.ID, Worker: r.worker, Attempt: j.Attempt, Now: now, RunAt: next, LastError: truncate(runErr.Error(), 1000)})
		r.log.Warn("job failed; will retry", "kind", j.Kind, "job_id", j.ID, "attempt", j.Attempt, "retry_at", next, "err", runErr)
	}
	switch {
	case err != nil:
		r.log.Error("record job outcome", "kind", j.Kind, "job_id", j.ID, "err", err)
	case n == 0:
		// The lease expired and another worker re-claimed the job; it owns the outcome.
		r.log.Warn("job lease lost before completion", "kind", j.Kind, "job_id", j.ID)
	}
	if r.observer != nil {
		r.observer.JobFinished(j.Kind, outcome, d)
	}
}

const cleanupKind = "jobs.cleanup"

// cleanup deletes finished jobs: succeeded after 7 days, dead after 30 (time to inspect
// them). Dedupe keys of periodic buckets disappear with their rows, which is fine
// because a bucket is never enqueued again once its time has passed.
func (r *Runner) cleanup(ctx context.Context, _ Job) error {
	q := dbq.New(r.pool)
	now := r.clock.Now()
	for {
		n, err := q.DeleteFinishedJobs(ctx, dbq.DeleteFinishedJobsParams{
			SucceededBefore: now.Add(-7 * 24 * time.Hour), DeadBefore: now.Add(-30 * 24 * time.Hour), Batch: 1000,
		})
		if err != nil {
			return fmt.Errorf("delete finished jobs: %w", err)
		}
		if n < 1000 {
			return nil
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
