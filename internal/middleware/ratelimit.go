package middleware

import (
	"net/http"
	"sync"
	"time"
)

// bucket implements a simple token bucket rate limiter.
type bucket struct {
	tokens   float64
	max      float64
	rate     float64 // tokens per second
	lastTime time.Time
}

func newBucket(perMinute int, burst int) *bucket {
	return &bucket{
		tokens:   float64(burst),
		max:      float64(burst),
		rate:     float64(perMinute) / 60.0,
		lastTime: time.Now(),
	}
}

func (b *bucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.max {
		b.tokens = b.max
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter provides per-client rate limiting for POST /api/chat.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	perMinute  int
	burst      int
	lastClean  time.Time
	cleanEvery time.Duration
}

// NewRateLimiter creates a rate limiter. perMinute is the sustained rate;
// burst is the maximum tokens available at once.
func NewRateLimiter(perMinute, burst int) *RateLimiter {
	if perMinute <= 0 {
		perMinute = 20
	}
	if burst <= 0 {
		burst = 5
	}
	return &RateLimiter{
		buckets:    make(map[string]*bucket),
		perMinute:  perMinute,
		burst:      burst,
		lastClean:  time.Now(),
		cleanEvery: 5 * time.Minute,
	}
}

func (rl *RateLimiter) clientKey(r *http.Request) string {
	// Prefer Grafana session cookie as client identifier.
	if c, err := r.Cookie("grafana_session"); err == nil && c.Value != "" {
		return "cookie:" + c.Value
	}
	return "ip:" + r.RemoteAddr
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Periodic cleanup of stale buckets.
	if time.Since(rl.lastClean) > rl.cleanEvery {
		cutoff := time.Now().Add(-10 * time.Minute)
		for k, b := range rl.buckets {
			if b.lastTime.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.lastClean = time.Now()
	}

	b, ok := rl.buckets[key]
	if !ok {
		b = newBucket(rl.perMinute, rl.burst)
		rl.buckets[key] = b
	}
	return b.allow()
}

// Middleware returns HTTP middleware that rate-limits POST /api/chat.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/chat" {
			key := rl.clientKey(r)
			if !rl.allow(key) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
