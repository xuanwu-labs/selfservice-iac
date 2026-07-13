//go:build wireinject

// Package main is the Aether platform server entry point.
// wire.go holds the wire injector definition — pure aggregation, no business
// logic. All transport assembly lives in internal/server/.
package main

import (
	"context"

	"github.com/google/wire"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"

	"github.com/xuanwu-labs/selfservice-iac/server/api"
	connectapi "github.com/xuanwu-labs/selfservice-iac/server/api/connect"
	"github.com/xuanwu-labs/selfservice-iac/server/core"
	"github.com/xuanwu-labs/selfservice-iac/server/data"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/asset"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/server"
)

// App is the assembled platform server — the output of wire.
type App struct {
	Config *config.Config
	Logger *otelzap.Logger
	Server *server.Server
}

// --- Wire providers ---

// provideLogger returns a trace-aware otelzap logger. OTel SDK is initialized
// in main.go BEFORE wire runs, so the global TracerProvider is ready when this
// executes — the returned logger carries trace_id in every Ctx() log call (D41).
func provideLogger(cfg *config.Config) *otelzap.Logger {
	var base *zap.Logger
	if cfg.LogLevel == "debug" {
		base, _ = zap.NewDevelopment()
	} else {
		base, _ = zap.NewProduction()
	}
	return otelzap.New(base)
}

// provideAppContext supplies the background context used by data-layer
// providers (e.g. pgxpool construction).
func provideAppContext() context.Context { return context.Background() }

// provideRegistry loads the error-code registry from the embedded
// error-codes.yaml (internal/asset) into an in-memory *errors.Registry.
// This is the runtime source of truth for error behavior (retryable,
// manual_required, remediation, owner); handlers and the fallback
// interceptor consume it. See specs/03-平台契约.md "错误码注册表".
func provideRegistry() (*platformerrors.Registry, error) {
	return platformerrors.Load(asset.ErrorCodes)
}

// allProviders aggregates every layer's ProviderSet. Adding a new package
// means adding its ProviderSet here — nothing else changes in wire.go.
var allProviders = wire.NewSet(
	data.ProviderSet,
	core.ProviderSet,
	api.ProviderSet,
	connectapi.ProviderSet,
	server.ProviderSet,
	provideLogger,
	provideAppContext,
	provideRegistry,
	wire.Struct(new(App), "*"),
)

// InitializeApp builds the server app via wire. cfg is loaded by main.go
// (before wire) from flags + env + yaml — wire receives it as a bound value.
func InitializeApp(cfg *config.Config) (*App, func(), error) {
	wire.Build(allProviders)
	return nil, nil, nil
}
