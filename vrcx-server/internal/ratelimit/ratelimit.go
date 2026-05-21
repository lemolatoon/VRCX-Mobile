// Package ratelimit provides in-process rate limiting for auth endpoints.
// For single-replica deployments this is sufficient; add Redis/Postgres
// persistence when running multiple proxy replicas.
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	limiter     *rate.Limiter
	lastSeen    time.Time
	failCount   int
	lockedUntil time.Time
}

// Limiter tracks per-key rate limits and lockouts.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	r       rate.Limit // requests per second
	burst   int
}

// New creates a Limiter. r is the sustained rate (req/s), burst is the burst capacity.
func New(r rate.Limit, burst int) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		r:       r,
		burst:   burst,
	}
	go l.cleanupLoop()
	return l
}

func (l *Limiter) get(key string) *entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.r, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e
}

// Allow returns (allowed bool, retryAfter). retryAfter is non-zero when locked out.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	e := l.get(key)
	l.mu.Lock()
	defer l.mu.Unlock()

	if !e.lockedUntil.IsZero() && time.Now().Before(e.lockedUntil) {
		return false, time.Until(e.lockedUntil)
	}
	if !e.limiter.Allow() {
		return false, 0
	}
	return true, 0
}

// RecordFailure increments the failure counter; after 5 consecutive failures
// the key is locked out for 1 hour.
func (l *Limiter) RecordFailure(key string) {
	e := l.get(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	e.failCount++
	if e.failCount >= 5 {
		e.lockedUntil = time.Now().Add(time.Hour)
		e.failCount = 0
	}
}

// RecordSuccess resets the failure counter.
func (l *Limiter) RecordSuccess(key string) {
	e := l.get(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	e.failCount = 0
	e.lockedUntil = time.Time{}
}

// Middleware returns an Echo/net-http compatible middleware key.
func IPKey(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return "ip:" + ip
}

func (l *Limiter) cleanupLoop() {
	for range time.Tick(10 * time.Minute) {
		l.mu.Lock()
		cutoff := time.Now().Add(-30 * time.Minute)
		for k, e := range l.entries {
			if e.lastSeen.Before(cutoff) {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
}
