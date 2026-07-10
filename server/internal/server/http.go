// Package server: http.go — gin HTTP server initialization.
//
// NewHTTPServer builds the gin engine with injected middleware + operational
// routes (/healthz, /ready, /metrics). Dependencies are wired directly —
// no intermediate translation struct.
package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"

	apihttp "github.com/xuanwu-labs/selfservice-iac/server/api/http"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
)

// NewHTTPServer builds the gin engine from injected deps + middleware config.
func NewHTTPServer(
	logger *otelzap.Logger,
	pingFunc func(ctx context.Context) error,
	metrics http.Handler,
	mwCfg *middleware.ServerConfig,
) *gin.Engine {
	return apihttp.NewRouter(&apihttp.Deps{
		Logger:         logger,
		PingFunc:       pingFunc,
		MetricsHandler: metrics,
		Middlewares:    mwCfg.GinMiddlewares,
	})
}
