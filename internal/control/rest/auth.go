package rest

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

// Auth is stubbed open in API-001. SEC-001 adds bearer + basic.
func (s *Server) authenticate(r *http.Request) app.Actor {
	actor := app.Actor{ID: "anonymous", Class: "administrator", Transport: "rest"}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return actor
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		actor.ID = "bearer"
		return actor
	}
	if strings.HasPrefix(strings.ToLower(h), "basic ") {
		actor.ID = "basic"
		return actor
	}
	return actor
}

type limiter struct {
	disabled bool
	rate     float64
	burst    float64
	mu       sync.Mutex
	buckets  map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rate, burst float64) *limiter {
	if rate < 0 {
		return &limiter{disabled: true}
	}
	if rate == 0 {
		rate = float64(config.DefaultRequestsPerSecond)
	}
	if burst == 0 {
		burst = float64(config.DefaultBurst)
	}
	return &limiter{rate: rate, burst: burst, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(remote string) error {
	if l == nil || l.disabled {
		return nil
	}
	key := remote
	if host, _, err := net.SplitHostPort(remote); err == nil {
		key = host
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evictIdleLocked(now)
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return domainerr.RateLimited("too many management requests")
	}
	b.tokens--
	return nil
}

func (l *limiter) evictIdleLocked(now time.Time) {
	if l == nil || len(l.buckets) == 0 {
		return
	}
	idleFor := 30 * time.Second
	if l.rate > 0 {
		refill := time.Duration(float64(time.Second) * (l.burst / l.rate) * 4)
		if refill > idleFor {
			idleFor = refill
		}
	}
	for k, b := range l.buckets {
		if now.Sub(b.last) > idleFor {
			delete(l.buckets, k)
		}
	}
}
