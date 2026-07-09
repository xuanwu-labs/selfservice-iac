// Package db provides test-database helpers built on testcontainers + pgtestdb.
//
// Two layers compose:
//   - testcontainers-go starts a REAL, dedicated Postgres container for tests
//     (separate from any development PG). It honors DOCKER_HOST, so a remote
//     Docker daemon works — set DOCKER_HOST to point at it.
//   - pgtestdb connects to that container, runs migrations once to build a
//     "template" database, then clones it per test — each test gets an
//     isolated, fully-migrated DB in milliseconds.
//
// Usage:
//
//	pool := testdb.New(t)                  // *pgxpool.Pool, fully migrated
//	queries := generated.New(pool)         // sqlc queries
//
// Docker: set DOCKER_HOST for a remote daemon. For a Docker-over-SSH daemon
// (no TCP listener), run a socat proxy container exposing the socket as TCP,
// then point DOCKER_HOST at it, e.g.:
//
//	# on the remote host, once:
//	docker run -d --name docker-api-proxy --restart unless-stopped \
//	  -p 23750:2375 -v /var/run/docker.sock:/var/run/docker.sock \
//	  alpine/socat -d -d TCP-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock
//	# locally:
//	export DOCKER_HOST=tcp://remote-host:23750
package db

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/api/types/container"
	"github.com/peterldowns/pgtestdb"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	// Register the pgx driver under "pgx" so database/sql (used internally
	// by pgtestdb) can open connections.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// schemaSQL mirrors server/cmd/migrate/migrations/001_init.sql (teams table).
// Kept inline to avoid a cross-package embed dependency at test time. If
// migrations grow, switch to embedding cmd/migrate/migrations and update the
// migrator (or wire a gooseMigrator).
const schemaSQL = `
CREATE TABLE IF NOT EXISTS teams (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    slug       VARCHAR(64)  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT teams_slug_uk UNIQUE (slug)
);
CREATE INDEX IF NOT EXISTS teams_name_idx ON teams (name);
`

// schemaMigrator implements pgtestdb.Migrator by running schemaSQL verbatim.
type schemaMigrator struct{}

func (schemaMigrator) Hash() (string, error) { return "teams-v1", nil }

func (schemaMigrator) Migrate(ctx context.Context, db *sql.DB, _ pgtestdb.Config) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply test schema: %w", err)
	}
	return nil
}

// pgContainer holds the single shared test PG container for a test run.
// testcontainers reuses it across calls in the same process; the first call
// pays startup (~5s), later calls attach instantly.
var pgContainer *tcpg.PostgresContainer

// startContainer starts (once per process) a dedicated PG container and
// returns its reachable host + port from the test process's perspective.
// With DOCKER_HOST=tcp://remote, testcontainers reports the remote host:port.
func startContainer(t testing.TB) (host, port string) {
	t.Helper()
	ctx := context.Background()

	if pgContainer == nil {
		c, err := tcpg.Run(ctx, "postgres:16-alpine",
			tcpg.WithDatabase("postgres"),
			tcpg.WithUsername("postgres"),
			tcpg.WithPassword("password"),
			// The host's default seccomp profile rejects a syscall PG 16's
			// initdb uses, failing with "Operation not permitted". Verified
			// empirically: --security-opt seccomp=unconfined makes it start.
			// (Named volumes / tmpfs / PGDATA moves did NOT help — it's seccomp,
			// not the filesystem.) Safe for an ephemeral test container.
			testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
				hc.SecurityOpt = append(hc.SecurityOpt, "seccomp=unconfined")
			}),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(60*time.Second),
			),
		)
		if err != nil {
			t.Fatalf("failed to start postgres container (is DOCKER_HOST set/reachable?): %v", err)
		}
		pgContainer = c
		// NOTE: container is NOT terminated per-test. It's shared across all
		// tests in the process; ryuk (auto-started by testcontainers) reaps it
		// when the test process exits. Do not add t.Cleanup(Terminate) here —
		// it would destroy the container before sibling tests finish.
	}

	h, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	p, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}
	return h, p.Port()
}

// NewDSN returns the connection string of a fresh, fully-migrated test database.
// Useful for tests that need a *sql.DB (e.g. goose migration tests) rather than
// a pgxpool. Like New, each call gets an isolated DB.
func NewDSN(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB test in -short mode")
	}
	host, port := startContainer(t)
	conf := pgtestdb.Custom(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       host,
		User:       "postgres",
		Password:   "password",
		Database:   "postgres",
		Port:       port,
		Options:    "sslmode=disable",
	}, schemaMigrator{})
	return conf.URL()
}

// New returns a *pgxpool.Pool connected to a fresh, fully-migrated test database.
// Each call gets its own cloned DB (isolated from other tests), torn down via
// t.Cleanup. The underlying PG container is shared across calls in the process.
//
// Requires Docker (set DOCKER_HOST). Skipped in -short mode.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	host, port := startContainer(t)

	// pgtestdb.Custom connects, migrates the template, clones a per-test DB,
	// and returns the Config whose URL() points at the cloned DB.
	conf := pgtestdb.Custom(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       host,
		User:       "postgres",
		Password:   "password",
		Database:   "postgres",
		Port:       port,
		Options:    "sslmode=disable",
	}, schemaMigrator{})

	pool, err := pgxpool.New(ctx, conf.URL())
	if err != nil {
		t.Fatalf("failed to create pgxpool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping test db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}
