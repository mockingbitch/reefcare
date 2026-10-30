//go:build integration

package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"reefcare/internal/db/dbq"
	"reefcare/internal/platform/clock"
	"reefcare/internal/platform/database/dbtest"
	"reefcare/internal/platform/jobs"
)

var t0 = time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func enqueue(t *testing.T, pool *pgxpool.Pool, now time.Time, s jobs.Spec) bool {
	t.Helper()
	ok, err := jobs.Enqueue(context.Background(), dbq.New(pool), now, s)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

type row struct {
	status   string
	attempts int
	runAt    time.Time
	lastErr  *string
}

func jobRow(t *testing.T, pool *pgxpool.Pool, kind string) row {
	t.Helper()
	var r row
	if err := pool.QueryRow(context.Background(), `SELECT status, attempts, run_at, last_error FROM jobs WHERE kind = $1`, kind).
		Scan(&r.status, &r.attempts, &r.runAt, &r.lastErr); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDedupeAndSuccess(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	if !enqueue(t, pool, t0, jobs.Spec{Kind: "test.once", DedupeKey: "k", Payload: map[string]string{"x": "1"}}) {
		t.Fatal("first enqueue must create the job")
	}
	if enqueue(t, pool, t0, jobs.Spec{Kind: "test.once", DedupeKey: "k"}) {
		t.Fatal("same dedupe key must be a no-op")
	}
	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	var got string
	r.Handle("test.once", func(_ context.Context, j jobs.Job) error {
		var p struct{ X string }
		if err := j.Decode(&p); err != nil {
			return err
		}
		got = p.X
		return nil
	})
	n, err := r.RunDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "1" || jobRow(t, pool, "test.once").status != "succeeded" {
		t.Fatalf("job not run: got %q, ran %d", got, n)
	}
}

func TestRetryWithBackoffThenDead(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.flaky", MaxAttempts: 3})
	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	calls := 0
	r.Handle("test.flaky", func(context.Context, jobs.Job) error { calls++; return errors.New("smtp reply 451") })
	ctx := context.Background()

	if _, err := r.RunDue(ctx); err != nil {
		t.Fatal(err)
	}
	j := jobRow(t, pool, "test.flaky")
	if j.status != "pending" || j.attempts != 1 || j.lastErr == nil || *j.lastErr != "smtp reply 451" {
		t.Fatalf("after first failure: %+v", j)
	}
	if delay := j.runAt.Sub(t0); delay < 12*time.Second || delay > 18*time.Second {
		t.Fatalf("first retry delay %v, want ~15s", delay)
	}
	// Not due yet: nothing runs.
	if _, _ = r.RunDue(ctx); calls != 1 {
		t.Fatalf("ran before its retry time (calls=%d)", calls)
	}
	clk.Advance(time.Hour)
	_, _ = r.RunDue(ctx)
	clk.Advance(time.Hour)
	_, _ = r.RunDue(ctx)
	if j := jobRow(t, pool, "test.flaky"); j.status != "dead" || j.attempts != 3 || calls != 3 {
		t.Fatalf("after max attempts: %+v calls=%d", j, calls)
	}
}

func TestPermanentErrorAndPanic(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.bad", Payload: map[string]int{"n": 1}})
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.panic"})
	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	r.Handle("test.bad", func(_ context.Context, j jobs.Job) error {
		var p struct{ N string } // wrong type: permanent decode failure
		return j.Decode(&p)
	})
	r.Handle("test.panic", func(context.Context, jobs.Job) error { panic("boom") })
	if _, err := r.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if j := jobRow(t, pool, "test.bad"); j.status != "dead" || j.attempts != 1 {
		t.Fatalf("permanent error must bury at once: %+v", j)
	}
	if j := jobRow(t, pool, "test.panic"); j.status != "pending" || j.lastErr == nil || *j.lastErr != "panic" {
		t.Fatalf("a panic is a retryable failure: %+v", j)
	}
}

// A worker that died mid-job leaves a running row; once its lease expires another worker
// claims it, and the dead worker's late completion is fenced off.
func TestExpiredLeaseIsReclaimedAndFenced(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	ctx := context.Background()
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.crash", MaxAttempts: 2})
	q := dbq.New(pool)
	rows, err := q.ClaimJobs(ctx, dbq.ClaimJobsParams{Kinds: []string{"test.crash"}, Now: t0, Batch: 1, Worker: "ghost", LeaseUntil: t0.Add(time.Minute)})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ghost claim: %v %v", rows, err)
	}

	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	runs := 0
	r.Handle("test.crash", func(context.Context, jobs.Job) error { runs++; return nil })
	if _, _ = r.RunDue(ctx); runs != 0 {
		t.Fatal("a live lease must not be stolen")
	}
	clk.Advance(2 * time.Minute)
	if _, _ = r.RunDue(ctx); runs != 1 {
		t.Fatal("an expired lease must be reclaimed")
	}
	if n, err := q.CompleteJob(ctx, dbq.CompleteJobParams{ID: rows[0].ID, Worker: "ghost", Attempt: rows[0].Attempts, Now: clk.Now()}); err != nil || n != 0 {
		t.Fatalf("the old worker must not overwrite the outcome: n=%d err=%v", n, err)
	}
	if j := jobRow(t, pool, "test.crash"); j.status != "succeeded" || j.attempts != 2 {
		t.Fatalf("%+v", j)
	}

	// A lease that expires on the final attempt buries the job.
	enqueue(t, pool, clk.Now(), jobs.Spec{Kind: "test.lastcrash", MaxAttempts: 1})
	if _, err := q.ClaimJobs(ctx, dbq.ClaimJobsParams{Kinds: []string{"test.lastcrash"}, Now: clk.Now(), Batch: 1, Worker: "ghost", LeaseUntil: clk.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	clk.Advance(2 * time.Minute)
	if _, err := r.RunDue(ctx); err != nil {
		t.Fatal(err)
	}
	if j := jobRow(t, pool, "test.lastcrash"); j.status != "dead" {
		t.Fatalf("%+v", j)
	}
}

// Several runners (as several API instances would) process every job exactly once.
func TestConcurrentRunnersRunEachJobOnce(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	const total = 200
	for range total {
		enqueue(t, pool, t0, jobs.Spec{Kind: "test.count"})
	}
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for range 3 {
		r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{Workers: 4, PollInterval: 5 * time.Millisecond})
		r.Handle("test.count", func(context.Context, jobs.Job) error { runs.Add(1); return nil })
		wg.Go(func() { r.Run(ctx) })
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		var left int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs WHERE kind = 'test.count' AND status <> 'succeeded'`).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()
	if runs.Load() != total {
		t.Fatalf("runs = %d, want %d", runs.Load(), total)
	}
}

func TestPeriodicJobsEnqueueOncePerBucket(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0.Add(17 * time.Minute))
	ctx := context.Background()
	var runs atomic.Int32
	newRunner := func() *jobs.Runner {
		r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
		r.Handle("test.hourly", func(context.Context, jobs.Job) error { runs.Add(1); return nil })
		r.Every("test.hourly", time.Hour)
		return r
	}
	a, b := newRunner(), newRunner()
	_, _ = a.RunDue(ctx)
	_, _ = b.RunDue(ctx)
	if runs.Load() != 1 {
		t.Fatalf("two runners in one bucket ran %d times", runs.Load())
	}
	clk.Advance(time.Hour)
	_, _ = b.RunDue(ctx)
	if runs.Load() != 2 {
		t.Fatalf("next bucket: runs = %d", runs.Load())
	}
}

func TestCleanupDeletesOldFinishedJobs(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	ctx := context.Background()
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.old"})
	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	r.Handle("test.old", func(context.Context, jobs.Job) error { return nil })
	_, _ = r.RunDue(ctx)
	clk.Advance(8 * 24 * time.Hour)
	_, _ = r.RunDue(ctx) // runs the daily jobs.cleanup of the new bucket
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE kind = 'test.old'`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("old succeeded job not cleaned up: %d %v", left, err)
	}
}

// The fence is per claim, not per process: a slow handler in the same runner whose lease
// expired cannot record an outcome over the newer claim.
func TestFenceIsPerClaim(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.slow"})
	q := dbq.New(pool)
	claim := func(at time.Time) dbq.ClaimJobsRow {
		rows, err := q.ClaimJobs(ctx, dbq.ClaimJobsParams{Kinds: []string{"test.slow"}, Now: at, Batch: 1, Worker: "same", LeaseUntil: at.Add(time.Minute)})
		if err != nil || len(rows) != 1 {
			t.Fatalf("claim: %v %v", rows, err)
		}
		return rows[0]
	}
	first := claim(t0)
	// The first claim's lease expires and the job is re-queued, then claimed again by
	// the same worker id.
	if n, err := q.RequeueExpiredJobs(ctx, t0.Add(2*time.Minute)); err != nil || n != 1 {
		t.Fatalf("requeue: %d %v", n, err)
	}
	second := claim(t0.Add(2 * time.Minute))
	if n, _ := q.CompleteJob(ctx, dbq.CompleteJobParams{ID: first.ID, Worker: "same", Attempt: first.Attempts, Now: t0.Add(3 * time.Minute)}); n != 0 {
		t.Fatal("a stale claim of the same worker must not complete the job")
	}
	if n, _ := q.RetryJob(ctx, dbq.RetryJobParams{ID: second.ID, Worker: "same", Attempt: second.Attempts, Now: t0.Add(3 * time.Minute), RunAt: t0.Add(4 * time.Minute), LastError: "x"}); n != 1 {
		t.Fatal("the current claim must record its outcome")
	}
}

func TestCompletedAndRevive(t *testing.T) {
	pool := dbtest.New(t)
	clk := clock.NewFixed(t0)
	ctx := context.Background()
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.noop", Completed: true})
	enqueue(t, pool, t0, jobs.Spec{Kind: "test.must", DedupeKey: "k", MaxAttempts: 1})
	r := jobs.NewRunner(pool, clk, quiet(), jobs.Config{})
	ran := 0
	r.Handle("test.noop", func(context.Context, jobs.Job) error { ran++; return nil })
	fail := true
	r.Handle("test.must", func(context.Context, jobs.Job) error {
		if fail {
			return errors.New("down")
		}
		return nil
	})
	_, _ = r.RunDue(ctx)
	if ran != 0 || jobRow(t, pool, "test.noop").status != "succeeded" {
		t.Fatal("a completed job is never claimed")
	}
	if jobRow(t, pool, "test.must").status != "dead" {
		t.Fatal("expected dead")
	}
	ok, err := jobs.Revive(ctx, dbq.New(pool), clk.Now(), "test.must", "k")
	if err != nil || !ok {
		t.Fatalf("revive: %v %v", ok, err)
	}
	fail = false
	_, _ = r.RunDue(ctx)
	if j := jobRow(t, pool, "test.must"); j.status != "succeeded" {
		t.Fatalf("revived job: %+v", j)
	}
}
