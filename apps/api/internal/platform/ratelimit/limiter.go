// Package ratelimit provides a process-local fixed-window limiter. It is the
// fallback for a single instance; a Redis-backed limiter replaces it when the
// API runs with more than one replica.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most limit events per key within one window.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	start time.Time
	count int
}

// New returns a limiter with the given limit per window.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, now: time.Now, buckets: map[string]*bucket{}}
}

// Allow reports whether the event is permitted. When it is not, it also
// returns how long the caller should wait before retrying.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &bucket{start: now, count: 1}
		l.cleanupLocked(now)
		return true, 0
	}
	if b.count >= l.limit {
		return false, l.window - now.Sub(b.start)
	}
	b.count++
	return true, 0
}

// cleanupLocked drops expired buckets so the map does not grow forever.
func (l *Limiter) cleanupLocked(now time.Time) {
	if len(l.buckets) < 10_000 {
		return
	}
	for k, b := range l.buckets {
		if now.Sub(b.start) >= l.window {
			delete(l.buckets, k)
		}
	}
}

// Allower is the limiter contract used by the HTTP layer.
type Allower interface {
	Allow(key string) (bool, time.Duration)
}
