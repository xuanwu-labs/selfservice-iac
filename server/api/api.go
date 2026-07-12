// Package api aggregates transport-layer providers for wire.
//
// The transport layer's actual dependencies (gin Deps) are defined in
// api/http/http.go and assembled by internal/server/http.go. This package
// exists only as a wire aggregation point.
package api

import (
	"context"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderSet aggregates transport-layer dependencies.
var ProviderSet = wire.NewSet(
	NewPingFunc,
)

// NewPingFunc returns a DB health-check function for the HTTP /healthz handler.
// It captures the pgxpool so the handler can ping without holding a direct ref.
func NewPingFunc(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error { return pool.Ping(ctx) }
}
