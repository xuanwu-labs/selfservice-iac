//go:build wireinject

// Package main is the Aether platform server entry point.
// wire.go holds the wire injector definition.
// This file is excluded from normal builds via build tag.
package main

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/api"
	servergrpc "github.com/xuanwu-labs/selfservice-iac/server/api/grpc"
	grpcinterceptor "github.com/xuanwu-labs/selfservice-iac/server/api/grpc/interceptor"
	apihttp "github.com/xuanwu-labs/selfservice-iac/server/api/http"
	"github.com/xuanwu-labs/selfservice-iac/server/core"
	"github.com/xuanwu-labs/selfservice-iac/server/data"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/otel"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/platformv1connect"
	"go.uber.org/zap"
)

// App is the assembled platform server — the output of wire.
type App struct {
	Config *config.Config
	Logger *zap.Logger
	Router *gin.Engine
	// ConnectMux holds all Connect-RPC handlers, mounted at its Path.
	ConnectMux *ConnectMux
}

// ConnectMux wraps a ServeMux plus the mount path for the root router.
type ConnectMux struct {
	Mux  *http.ServeMux
	Path string
}

// provideConnect wires the Catalog Connect handler with the OTel interceptor
// (task 11.4) + the auth/RBAC/audit/ratelimit interceptor chain (task 15.5).
// Phase 1: single service; more are appended by adding to the mux.
func provideConnect(catalog *servergrpc.CatalogHandler) (*ConnectMux, error) {
	otelInterceptors, err := otelconnect.NewInterceptor(otelconnect.WithTrustRemote())
	if err != nil {
		return nil, fmt.Errorf("create otelconnect interceptor: %w", err)
	}

	// Build the full interceptor list: otel (trace) + auth/rbac/ratelimit/audit.
	var allInterceptors []connect.Interceptor
	allInterceptors = append(allInterceptors, otelInterceptors)
	for _, ic := range grpcinterceptor.Chain(grpcinterceptor.OtelZapLogger()) {
		allInterceptors = append(allInterceptors, ic)
	}

	mux := http.NewServeMux()
	path, handler := catalogv1connect.NewCatalogServiceHandler(
		catalog,
		connect.WithInterceptors(allInterceptors...),
	)
	mux.Handle(path, handler)
	return &ConnectMux{Mux: mux, Path: "/api/"}, nil
}

// --- Wire providers ---

func provideLogger(cfg *config.Config) *zap.Logger {
	var logger *zap.Logger
	if cfg.LogLevel == "debug" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	return logger
}

// provideMetricsHandler exposes the OTel Prometheus handler for /metrics.
// OTel SDK is initialized in main.go (global state); this just reads it.
func provideMetricsHandler() http.Handler {
	return otel.MetricsHandler()
}

func provideRouter(deps *api.Deps, metrics http.Handler) *gin.Engine {
	return apihttp.NewRouter(&apihttp.Deps{
		Logger:         deps.Logger,
		PingFunc:       deps.PingFunc,
		MetricsHandler: metrics,
	})
}

// provideAppContext supplies the background context used by data-layer
// providers (e.g. pgxpool construction). Refined in task 08 (lifecycle-aware ctx).
func provideAppContext() context.Context { return context.Background() }

var allProviders = wire.NewSet(
	config.ProviderSet,
	data.ProviderSet,
	core.ProviderSet,
	api.ProviderSet,
	provideLogger,
	provideMetricsHandler,
	provideRouter,
	servergrpc.NewCatalogHandler,
	provideConnect,
	provideAppContext,
	wire.Struct(new(App), "*"),
)

func InitializeApp() (*App, func(), error) {
	wire.Build(allProviders)
	return nil, nil, nil
}
