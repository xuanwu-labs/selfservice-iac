// Package http provides the gin-based HTTP transport layer for Aether.
// It serves operational endpoints (/healthz, /ready, /metrics) and webhook
// callbacks. Business RPC goes through Connect-RPC (api/connect/).
//
// Middleware is injected via the Deps.Middlewares field — the server layer
// composes the chain, this package only registers routes.
package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
)

// NewRouter creates the gin engine with injected middleware + route registration.
func NewRouter(deps *Deps) *gin.Engine {
	r := gin.New()

	// Apply injected middleware chain (composed by internal/server via Options).
	for _, mw := range deps.Middlewares {
		r.Use(mw)
	}

	// Operational endpoints
	r.GET("/healthz", HealthzHandler(deps))
	r.GET("/ready", HealthzHandler(deps))

	// Metrics endpoint (Prometheus scrape target, D41).
	if deps.MetricsHandler != nil {
		r.GET("/metrics", gin.WrapH(deps.MetricsHandler))
	}

	// Admin REST endpoints (Phase 1): state_backends + workspaces upserts. These
	// back the AdminPage forms that have no proto RPC yet.
	RegisterAdminRoutes(r, deps.Pool)

	return r
}

// Deps holds the dependencies for the HTTP transport layer.
type Deps struct {
	Logger   *otelzap.Logger
	PingFunc func(ctx context.Context) error
	// MetricsHandler serves Prometheus metrics at /metrics (from otel.SDK).
	MetricsHandler http.Handler
	// Middlewares is the gin middleware chain, injected by the server layer.
	Middlewares []gin.HandlerFunc
	// Pool is the pgx pool used by the admin REST handlers for direct DB writes
	// (state_backends / workspaces upserts). Optional; nil disables admin routes.
	Pool *pgxpool.Pool
}
