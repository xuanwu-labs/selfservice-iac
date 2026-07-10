// Package data provides the data access layer for Aether.
// data.go holds the pgxpool provider + wire ProviderSet.
package data

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ProviderSet provides data-layer dependencies for wire.
var ProviderSet = wire.NewSet(
	NewPgxPool,
	NewQueries,
)

// NewPgxPool creates a pgxpool connection pool from config, instrumented with
// otelpgx so every query becomes a child span of the request trace (D41).
// TODO(task-09): split into NewAPIPool + NewWorkerPool for connection isolation.
func NewPgxPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, func(), error) {
	dsn := cfg.Data.Database.DSN()
	if dsn == "" {
		return nil, nil, fmt.Errorf("database not configured (set data.database.* in config)")
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("parse pgxpool config: %w", err)
	}
	// Apply pool size from config.
	if cfg.Data.Database.MaxConns > 0 {
		poolCfg.MaxConns = cfg.Data.Database.MaxConns
	}
	// otelpgx: wrap each query in a span + record pool stats as OTel metrics.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create pgxpool: %w", err)
	}
	_ = otelpgx.RecordStats(pool)

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping PG: %w", err)
	}
	cleanup := func() { pool.Close() }
	return pool, cleanup, nil
}

// NewQueries creates a sqlc Queries from the pgxpool.
// WIP: registered in ProviderSet but currently has no wire consumer — the
// first consumer will be core/store (薄包装) when it lands in Wave 1.
// Until then it stays registered so the dependency is visible in the graph.
func NewQueries(pool *pgxpool.Pool) *generated.Queries {
	return generated.New(pool)
}
