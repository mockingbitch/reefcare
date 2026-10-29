// Package ratelimit provides in-process keyed token-bucket limiters.
//
// State lives in memory, so limits apply per API instance. This is sufficient for the
// single-instance deployment; a shared store is required before scaling out
// (docs/api-guidelines.md §6, decision D7).
package ratelimit

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"reefcare/internal/platform/clock"
)

// DefaultMaxKeys bounds memory per limiter (about 150 bytes per key).
const DefaultMaxKeys = 100_000

// evictionSample is how many random buckets are inspected to pick one to evict when the
// table is full (approximate LRU, as in Redis).
const evictionSample = 8

// Limiter allows up to Burst events per key, refilled evenly over Window.
type Limiter struct {
	limit   rate.Limit
	burst   int
	clock   clock.Clock
	maxKeys int

	mu      sync.Mutex
	buckets map[[sha256.Size]byte]*bucket
	idleTTL time.Duration
	gcEvery time.Duration
	lastGC  time.Time
}

type bucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// New returns a limiter permitting n events per window for each key,
// e.g. New(10, 15*time.Minute, clk) for "10 login attempts per 15 minutes".
// It panics on non-positive arguments, which are programming errors.
func New(n int, window time.Duration, clk clock.Clock) *Limiter {
	if n <= 0 || window <= 0 || window/time.Duration(n) <= 0 {
		panic(fmt.Sprintf("ratelimit: invalid limit %d per %v", n, window))
	}
	return &Limiter{
		limit:   rate.Every(window / time.Duration(n)),
		burst:   n,
		clock:   clk,
		maxKeys: DefaultMaxKeys,
		buckets: make(map[[sha256.Size]byte]*bucket),
		idleTTL: window,
		gcEvery: max(window/10, time.Second),
		lastGC:  clk.Now(),
	}
}

// WithMaxKeys overrides the key cap (mainly for tests).
func (l *Limiter) WithMaxKeys(n int) *Limiter {
	l.maxKeys = n
	return l
}

// Allow consumes one token for key. When denied it returns how long until a token
// becomes available. Keys are stored as SHA-256 digests, so memory per key is fixed
// no matter how long the (possibly attacker-supplied) key is. When the table is full,
// the least recently seen of a random sample is evicted: a flood of new keys degrades
// limiting for idle keys instead of locking out every new caller.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := l.clock.Now()
	k := sha256.Sum256([]byte(key))
	l.mu.Lock()
	defer l.mu.Unlock()

	l.gc(now)
	b, ok := l.buckets[k]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.evictOne()
		}
		b = &bucket{lim: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[k] = b
	}
	b.seen = now

	r := b.lim.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// evictOne removes the least recently seen bucket among a random sample (map iteration
// order is randomized). Called with mu held.
func (l *Limiter) evictOne() {
	var (
		victim [sha256.Size]byte
		oldest time.Time
		n      int
	)
	for k, b := range l.buckets {
		if n == 0 || b.seen.Before(oldest) {
			victim, oldest = k, b.seen
		}
		if n++; n == evictionSample {
			break
		}
	}
	delete(l.buckets, victim)
}

// gc drops buckets idle for longer than a full window; such a bucket is full again,
// so forgetting it does not change behavior. Sweeping every window/10 keeps each pass
// small. Called with mu held.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < l.gcEvery {
		return
	}
	for k, b := range l.buckets {
		if now.Sub(b.seen) >= l.idleTTL {
			delete(l.buckets, k)
		}
	}
	l.lastGC = now
}

// Len returns the number of tracked keys (for tests and metrics).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
