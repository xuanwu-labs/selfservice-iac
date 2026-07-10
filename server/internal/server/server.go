// Package server assembles the HTTP + Connect-RPC transport into a single
// http.Server. The root http.ServeMux routes:
//
//	/api/...  → Connect handlers (if enabled)
//	/         → gin engine (healthz, ready, metrics, webhooks)
//
// One process, one port, one http.Server.
//
// Cross-cutting concerns (middleware, interceptors) are injected via Options
// (functional options pattern), composable and overridable at startup.
package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
)

// Server is the assembled HTTP server — gin + Connect on one port.
type Server struct {
	httpServer *http.Server
	logger     *otelzap.Logger
}

// NewServer assembles the root http.ServeMux from gin engine + Connect handlers,
// applying all Options (middleware, interceptors, handler registration).
//
// When cfg.Connect.Enabled is false, Connect routes are not mounted.
func NewServer(cfg *config.Config, ginEngine *gin.Engine, mwCfg *middleware.ServerConfig, logger *otelzap.Logger) *Server {
	mux := http.NewServeMux()

	// Mount Connect-RPC handlers under /api/ prefix (if enabled).
	// Actual path: /api/aether.platform.v1.CatalogService/ListItems
	if cfg.Connect.Enabled {
		for _, h := range mwCfg.ConnectHandlers {
			mux.Handle("/api"+h.Path, h.Handler)
		}
	}
	// Everything else goes to gin (healthz, ready, metrics, webhooks).
	mux.Handle("/", ginEngine)

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: mux,
		},
		logger: logger,
	}
}

// Run starts the HTTP server (blocking).
func (s *Server) Run() error {
	s.logger.Info("server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server...")
	err := s.httpServer.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	s.logger.Info("http server stopped")
	return nil
}

// ProvideMetricsHandler exposes the OTel Prometheus handler for /metrics.
func ProvideMetricsHandler() http.Handler {
	return otel.MetricsHandler()
}

// ProviderSet aggregates the server-layer providers for wire.
var ProviderSet = wire.NewSet(
	NewHTTPServer,
	ProvideServerConfig,
	NewServer,
	ProvideMetricsHandler,
)
