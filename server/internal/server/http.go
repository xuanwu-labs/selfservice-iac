// Package server: http.go — gin HTTP server initialization.
//
// NewHTTPServer builds the gin engine with the provided middleware chain +
// operational routes (/healthz, /ready, /metrics). Middleware is injected,
// not hardcoded — callers compose the chain via middleware Options.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/api"
	apihttp "github.com/xuanwu-labs/selfservice-iac/server/api/http"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
)

// NewHTTPServer builds the gin engine from the API deps + middleware config.
// Operational routes (healthz/ready/metrics) are always registered; middleware
// comes from the ServerConfig (composed via Options).
func NewHTTPServer(deps *api.Deps, metrics http.Handler, mwCfg *middleware.ServerConfig) *gin.Engine {
	return apihttp.NewRouter(&apihttp.Deps{
		Logger:         deps.Logger,
		PingFunc:       deps.PingFunc,
		MetricsHandler: metrics,
		Middlewares:    mwCfg.GinMiddlewares,
	})
}

// Compile-time: ensure wire is referenced (ProviderSet uses it).
var _ = wire.NewSet
