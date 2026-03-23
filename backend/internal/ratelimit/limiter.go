// Package ratelimit provides a per-key token-bucket rate limiter.
// No external dependencies — uses only the standard library.
package ratelimit

import (
	"sync"
	"time"
)

// bucket is a single token-bucket for one key.
type bucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	rate     float64 // tokens added per second
	capacity float64 // max tokens (burst)
}

// allow returns true if a token is available and consumes it.
func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter is the interface both in-memory and Redis limiters satisfy.
type RateLimiter interface {
	Allow(key string) bool
}

// Limiter manages per-key token buckets and periodically evicts idle ones.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity float64
}

// New creates a Limiter.
// rate = sustained tokens/second, burst = max burst (initial/max tokens).
// Starts a background goroutine that evicts keys idle > 10 minutes.
func New(rate float64, burst int) *Limiter {
	l := &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: float64(burst),
	}
	go l.evict()
	return l
}

// Allow returns true if key is within rate limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{
			tokens:   l.capacity, // start full
			last:     time.Now(),
			rate:     l.rate,
			capacity: l.capacity,
		}
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.allow()
}

// evict removes buckets that have been idle for > 10 minutes.
func (l *Limiter) evict() {
	t := time.NewTicker(5 * time.Minute)
	for range t.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for k, b := range l.buckets {
			b.mu.Lock()
			idle := b.last.Before(cutoff)
			b.mu.Unlock()
			if idle {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}
