package edge

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-freya/freya/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// RateLimit is a token bucket per client IP. Zero means the default (20/s, burst 40).
type RateLimit struct {
	PerSecond float64 `json:"per_second" yaml:"per_second"`
	Burst     int     `json:"burst" yaml:"burst"`
	// Routes overrides the limit for exact paths (e.g. "/api/v1/signin").
	Routes map[string]RateLimit `json:"routes" yaml:"routes"`
}

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	cfg     RateLimit
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

func newLimiter(cfg RateLimit) *limiter {
	if cfg.PerSecond <= 0 {
		cfg.PerSecond = 20
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 40
	}
	return &limiter{cfg: cfg, buckets: map[string]*bucket{}, now: time.Now}
}

func (l *limiter) allow(key string, rl RateLimit) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rl.Burst), last: now}
		l.buckets[key] = b
		if len(l.buckets) > 100000 { // bounded memory: reset on overflow
			l.buckets = map[string]*bucket{key: b}
		}
	}
	b.tokens += now.Sub(b.last).Seconds() * rl.PerSecond
	if b.tokens > float64(rl.Burst) {
		b.tokens = float64(rl.Burst)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *Server) rateLimitFilter(trusted []*net.IPNet) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, trusted)
			r = r.WithContext(WithClientIP(r.Context(), ip))
			rl := s.limiter.cfg
			key := ip
			if route, ok := s.limiter.cfg.Routes[r.URL.Path]; ok {
				if route.PerSecond > 0 {
					rl = route
				}
				if rl.Burst <= 0 {
					rl.Burst = s.limiter.cfg.Burst
				}
				key = r.URL.Path + "|" + ip
			}
			if !s.limiter.allow(key, rl) {
				transport.AuditLimitExceeded(s.rt, r.Context(), r.Method+" "+r.URL.Path, r.RemoteAddr, "rate_limit")
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"reason":"rate_limited"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
