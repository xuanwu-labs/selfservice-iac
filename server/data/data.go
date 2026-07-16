// Package data provides the data access layer for Aether.
// data.go holds the pgxpool provider + wire ProviderSet.
package data

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ProviderSet provides data-layer dependencies for wire: the pgxpool, the
// sqlc *generated.Queries, and all Repo structs registered in repo.ProviderSet
// (ferret style: core/<domain>/ injects *repo.XxxRepo directly).
var ProviderSet = wire.NewSet(
	NewPgxPool,
	NewQueries,
	repo.ProviderSet,
)

// NewPgxPool creates a pgxpool connection pool from config, instrumented with
// otelpgx so every query becomes a child span of the request trace (D41).
// TODO(task-09): split into NewAPIPool + NewWorkerPool for connection isolation.
func NewPgxPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, func(), error) {
	if cfg.Data.Database.Host == "" {
		return nil, nil, fmt.Errorf("database not configured (set data.database.* in config)")
	}

	poolCfg, err := newPoolConfig(cfg.Data.Database)
	if err != nil {
		return nil, nil, err
	}

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

// newPoolConfig assembles a *pgxpool.Config from structured DatabaseConfig fields.
// This is the single place where DB connection parameters are translated into
// pgx's internal config — no DSN string involved.
func newPoolConfig(dbCfg config.DatabaseConfig) (*pgxpool.Config, error) {
	// Use a minimal URL so pgx initializes sensible defaults (TLS, dialer,
	// runtime params), then override with our structured fields.
	// This avoids manual construction of DialFunc/LookupFunc/TLSConfig.
	poolCfg, err := pgxpool.ParseConfig("postgres:///dummy")
	if err != nil {
		return nil, fmt.Errorf("init pgxpool config template: %w", err)
	}

	// Override connection fields from structured config.
	poolCfg.ConnConfig.Host = dbCfg.Host
	poolCfg.ConnConfig.Port = uint16(dbCfg.Port)
	poolCfg.ConnConfig.Database = dbCfg.Database
	poolCfg.ConnConfig.User = dbCfg.User
	poolCfg.ConnConfig.Password = dbCfg.Password

	// TLS: disable unless ssl_mode requires it.
	if dbCfg.SSLMode == "disable" || dbCfg.SSLMode == "" {
		poolCfg.ConnConfig.TLSConfig = nil
	}

	// Pool sizing.
	if dbCfg.MaxConns > 0 {
		poolCfg.MaxConns = dbCfg.MaxConns
	}

	// OTel: wrap each query in a span.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	return poolCfg, nil
}

// NewQueries creates a sqlc Queries from the pgxpool.
// Consumed internally by data/repo Repo structs (each Repo holds its own
// *generated.Queries via repo.NewXxxRepo(pool)). Kept in ProviderSet so the
// dependency is visible in the wire graph; core/<domain>/ packages inject
// *repo.XxxRepo (not *generated.Queries directly) per the W1-02 hybrid paradigm
// (ferret Repo struct × DIP evolvable × sqlc SQL-as-truth).
func NewQueries(pool *pgxpool.Pool) *generated.Queries {
	return generated.New(pool)
}
