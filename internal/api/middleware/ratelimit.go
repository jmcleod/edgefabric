package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter is per-IP token bucket rate limiting middleware.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*visitorLimiter
	rps      rate.Limit
	burst    int
	cleanup  time.Duration
}

type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter that allows rps requests per second
// with the given burst size per client IP.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*visitorLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		cleanup:  3 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// Middleware returns an http.Handler middleware that rate limits by client IP.
// If paths are provided, only those paths are rate limited; all other requests
// pass through. If no paths are provided, all requests are rate limited.
func (rl *RateLimiter) Middleware(paths ...string) func(http.Handler) http.Handler {
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If paths specified, only rate limit matching paths.
			if len(pathSet) > 0 && !pathSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			limiter := rl.getLimiter(ip)

			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate_limit_exceeded","message":"too many requests"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.limiters[ip]
	if !ok {
		limiter := rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[ip] = &visitorLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.limiters {
			if time.Since(v.lastSeen) > rl.cleanup {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// clientIP extracts the client IP from RemoteAddr (the actual TCP peer).
//
// Security: we intentionally ignore X-Forwarded-For because any client can
// spoof it to rotate IPs and bypass rate limits. If the controller is behind
// a trusted reverse proxy, the proxy should overwrite RemoteAddr or use
// PROXY protocol. Never trust user-supplied forwarding headers for security
// decisions like rate limiting.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
