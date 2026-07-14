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
	"embed"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/api/types/container"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	// Register the pgx driver under "pgx" so database/sql (used internally
	// by pgtestdb) can open connections.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// gooseMigrator runs real goose migrations from the embedded migrations/
// directory. This replaces the old inline schemaSQL (which was a stale copy
// of 001_init.sql). By using the real migrations, the test DB always matches
// production schema — no drift.
//
//go:embed migrations/*.sql
var migrationFS embed.FS

type gooseMigrator struct{}

func (gooseMigrator) Hash() (string, error) { return "goose-migrations-v1", nil }

func (gooseMigrator) Migrate(ctx context.Context, db *sql.DB, _ pgtestdb.Config) error {
	goose.SetBaseFS(migrationFS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// pgContainer holds the single shared test PG container for a test run.
var pgContainer *tcpg.PostgresContainer

// testContainerConfig holds testcontainers PG parameters, overridable via env vars.
// These are TEST infrastructure config — not application runtime config.
type testContainerConfig struct {
	Image    string
	User     string
	Password string
	Database string
}

func loadTestContainerConfig() testContainerConfig {
	return testContainerConfig{
		Image:    envOrDefault("AETHER_TEST_PG_IMAGE", "postgres:16-alpine"),
		User:     envOrDefault("AETHER_TEST_PG_USER", "postgres"),
		Password: envOrDefault("AETHER_TEST_PG_PASSWORD", "password"),
		Database: envOrDefault("AETHER_TEST_PG_DATABASE", "postgres"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// startContainer starts (once per process) a dedicated PG container and
// returns its reachable host + port. Testcontainers honors DOCKER_HOST env var.
func startContainer(t testing.TB) (host, port string) {
	t.Helper()
	ctx := context.Background()

	if pgContainer == nil {
		tc := loadTestContainerConfig()
		c, err := tcpg.Run(ctx, tc.Image,
			tcpg.WithDatabase(tc.Database),
			tcpg.WithUsername(tc.User),
			tcpg.WithPassword(tc.Password),
			// The host's default seccomp profile rejects a syscall PG 16's
			// initdb uses (CentOS 7 Docker). seccomp=unconfined bypasses it.
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
	tc := loadTestContainerConfig()
	conf := pgtestdb.Custom(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       host,
		User:       tc.User,
		Password:   tc.Password,
		Database:   tc.Database,
		Port:       port,
		Options:    "sslmode=disable",
	}, gooseMigrator{})
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
	tc := loadTestContainerConfig()

	conf := pgtestdb.Custom(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       host,
		User:       tc.User,
		Password:   tc.Password,
		Database:   tc.Database,
		Port:       port,
		Options:    "sslmode=disable",
	}, gooseMigrator{})

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
