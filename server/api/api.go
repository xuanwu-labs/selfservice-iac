// Package api aggregates transport-layer providers for wire.
package api

import (
	"context"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
)

// ProviderSet aggregates transport-layer dependencies.
var ProviderSet = wire.NewSet(
	NewHTTPDeps,
)

// Deps holds transport-layer dependencies for the HTTP router.
type Deps struct {
	Logger   *otelzap.Logger
	PingFunc func(ctx context.Context) error
}

// NewHTTPDeps builds the HTTP Deps from wire-provided components.
func NewHTTPDeps(logger *otelzap.Logger, pool *pgxpool.Pool) *Deps {
	return &Deps{
		Logger:   logger,
		PingFunc: func(ctx context.Context) error { return pool.Ping(ctx) },
	}
}
