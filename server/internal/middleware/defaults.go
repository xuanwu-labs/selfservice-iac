// Package middleware: defaults.go — the standard middleware/interceptor chains
// used unless overridden by caller-provided Options.
//
// These are wire-provided (server.DefaultOptions) so the "out of the box"
// server has all cross-cutting concerns wired without manual Option plumbing.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// DefaultGinMiddlewares returns the standard gin middleware chain:
// otelgin (trace) → RequestID → Recovery → Logger (trace-aware).
func DefaultGinMiddlewares(logger *otelzap.Logger) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		otelgin.Middleware("aether-server"),
		RequestID(),
		gin.Recovery(),
		Logger(logger),
	}
}

// DefaultConnectInterceptors returns the Connect interceptors that don't need
// external dependencies (auth/RBAC/ratelimit are pass-through skeletons).
// The otelconnect interceptor and audit interceptor are added by the server
// layer (they need the otel SDK / logger).
func DefaultConnectInterceptors() []Option {
	return []Option{
		WithConnectInterceptor(
			ConnectAuth(),
			ConnectRBAC(),
			ConnectRateLimit(),
		),
	}
}
