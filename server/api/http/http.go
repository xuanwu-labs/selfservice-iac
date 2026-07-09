// Package http provides the gin-based HTTP transport layer for Aether.
// It serves webhook callbacks, operational endpoints (/healthz, /metrics),
// and non-RPC HTTP interfaces. Business RPC goes through Connect-RPC (api/grpc/).
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// NewRouter creates the gin engine with middleware chain and route registration.
// deps injects the pgxpool for healthz and other handlers.
func NewRouter(deps *Deps) *gin.Engine {
	r := gin.New()

	// Middleware chain:
	//   otelgin    — starts a span per request, propagates via c.Request.Context()
	//   RequestID  — injects X-Request-Id into context + header
	//   Recovery   — panics → 500
	//   Logger     — structured access log
	r.Use(otelgin.Middleware("aether-server"))
	r.Use(RequestIDMiddleware())
	r.Use(gin.Recovery())
	r.Use(LoggerMiddleware(deps.Logger))

	// NOTE: business RPC (CatalogService etc.) is handled by Connect-RPC in
	// api/grpc/, mounted on the root http.ServeMux at /api/ — NOT a gin group.
	// gin only serves operational + webhook endpoints here.

	// Operational endpoints
	r.GET("/healthz", HealthzHandler(deps))
	r.GET("/ready", HealthzHandler(deps)) // alias

	// Metrics endpoint (Prometheus scrape target, D41).
	// MetricsHandler() comes from internal/otel; nil fallback guards骨架 phase.
	if deps.MetricsHandler != nil {
		r.GET("/metrics", gin.WrapH(deps.MetricsHandler))
	}

	return r
}

// Deps holds the dependencies for the HTTP transport layer.
type Deps struct {
	Logger *zap.Logger
	// PgxPool is set by wire; healthz uses it to check DB connectivity.
	PingFunc func(ctx context.Context) error
	// MetricsHandler serves Prometheus metrics at /metrics (from otel.SDK).
	// Nil in骨架 phase disables the /metrics route.
	MetricsHandler http.Handler
}

// RequestIDMiddleware injects a UUID request ID into the context and response header.
func RequestIDMiddleware() gin.HandlerFunc {
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

// LoggerMiddleware logs each request with structured zap fields.
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		if logger != nil {
			logger.Info("http request",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("latency", latency),
				zap.String("request_id", c.GetString("request_id")),
			)
		}
	}
}
