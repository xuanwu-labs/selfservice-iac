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

// Note: Connect interceptors are assembled inline in server.ProvideServerConfig
// (connect.go) because they need runtime dependencies (otelconnect, logger).
// There is no DefaultConnectInterceptors() — each interceptor factory is called
// directly where the chain is built.
