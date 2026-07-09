// Package api aggregates transport-layer providers for wire.
package api

import (
	"context"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ProviderSet aggregates transport-layer dependencies.
// grpc ProviderSet will be added in task 15 (Connect-RPC).
var ProviderSet = wire.NewSet(
	NewHTTPDeps,
)

// Deps holds transport-layer dependencies for the HTTP router.
// This is a bridge struct that wire fills, then http.NewRouter consumes.
type Deps struct {
	Logger   *zap.Logger
	PingFunc func(ctx context.Context) error
}

// NewHTTPDeps builds the HTTP Deps from wire-provided components.
func NewHTTPDeps(logger *zap.Logger, pool *pgxpool.Pool) *Deps {
	return &Deps{
		Logger:   logger,
		PingFunc: func(ctx context.Context) error { return pool.Ping(ctx) },
	}
}
