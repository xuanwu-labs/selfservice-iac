// Package middleware: ratelimit.go — per-IP token-bucket rate limiting.
//
// MVP: single-instance (in-memory). Each client IP gets a golang.org/x/time/rate
// limiter; over-budget requests get HTTP 429. For multi-instance deployment,
// swap this for a shared store (Redis) — the gin.HandlerFunc signature stays.
package middleware

import (
	"net"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig controls the per-IP token bucket.
type RateLimiterConfig struct {
	// RequestsPerSecond is the steady-state refill rate (tokens/sec).
	RequestsPerSecond rate.Limit
	// Burst is the bucket capacity (max instant requests).
	Burst int
}

// DefaultRateLimiterConfig targets ~50 req/s per IP with a 100-request burst.
var DefaultRateLimiterConfig = RateLimiterConfig{
	RequestsPerSecond: 50,
	Burst:             100,
}

// ipLimiter holds a limiter per IP, created lazily and guarded by a mutex.
type ipLimiter struct {
	mu   sync.Mutex
	lmts map[string]*rate.Limiter
	cfg  RateLimiterConfig
}

func newIPLimiter(cfg RateLimiterConfig) *ipLimiter {
	return &ipLimiter{lmts: map[string]*rate.Limiter{}, cfg: cfg}
}

func (i *ipLimiter) get(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()
	l, ok := i.lmts[ip]
	if !ok {
		l = rate.NewLimiter(i.cfg.RequestsPerSecond, i.cfg.Burst)
		i.lmts[ip] = l
	}
	return l
}

// RateLimit returns a gin middleware that enforces per-IP rate limiting.
// Requests exceeding the budget receive HTTP 429 Too Many Requests.
func RateLimit(cfg RateLimiterConfig) gin.HandlerFunc {
	il := newIPLimiter(cfg)
	return func(c *gin.Context) {
		ip := clientIP(c)
		if !il.get(ip).Allow() {
			c.AbortWithStatus(429)
			return
		}
		c.Next()
	}
}

// clientIP extracts the client address, honoring X-Forwarded-For when present
// (behind a proxy/load balancer).
func clientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// First entry in the list is the original client.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}
