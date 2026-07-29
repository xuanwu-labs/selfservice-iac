// Package middleware holds cross-cutting concerns shared by all transports:
// gin middlewares and Connect-RPC interceptors. Each is a pure function that
// the server layer composes via Options at startup.
package middleware

import (
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/cors"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// --- gin middlewares ---

// RequestID injects a UUID request ID into the context and response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-Id", requestID)
		c.Next()
	}
}

// Logger logs each request with structured zap fields. Uses otelzap so the
// log line carries the active trace_id (D41).
func Logger(logger *otelzap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		if logger != nil {
			logger.Ctx(c.Request.Context()).Info("http request",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("latency", latency),
				zap.String("request_id", c.GetString("request_id")),
			)
		}
	}
}

// CORS returns a gin middleware for browser clients (the React frontend).
// AllowedOrigins defaults to common dev origins if nil.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id", "x-git-token"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	return func(ctx *gin.Context) {
		c.HandlerFunc(ctx.Writer, ctx.Request)
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
		}
	}
}

// --- rate limiting (gin, per-IP token bucket) ---

// RateLimiterConfig controls the per-IP token bucket.
type RateLimiterConfig struct {
	RequestsPerSecond rate.Limit
	Burst             int
}

// DefaultRateLimiterConfig targets ~50 req/s per IP with a 100-request burst.
var DefaultRateLimiterConfig = RateLimiterConfig{
	RequestsPerSecond: 50,
	Burst:             100,
}

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
// Requests exceeding the budget receive HTTP 429.
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

func clientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
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
