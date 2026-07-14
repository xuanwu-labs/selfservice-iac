package main_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationDDLExecutes verifies all migration DDL actually executes on a
// real PG (embedded-postgres, no Docker needed). This is the hard validation
// that CREATE TABLE / CHECK / FK / trigger / DO$$ blocks are syntactically
// and semantically correct.
//
// It runs goose Up → Down → Up on an embedded PG 16 instance, confirming:
// 1. All 10 migrations apply cleanly (no DDL syntax errors).
// 2. All 10 migrations roll back cleanly (Down blocks work).
// 3. Re-applying works (idempotent).
// 4. The layer seed (3 layers + v1 rule set) lands correctly.
func TestMigrationDDLExecutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode")
	}

	// Embedded PG needs a temp dir for data + binaries.
	tmpDir := t.TempDir()
	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Version(embeddedpostgres.V16).
			Port(9876).
			DataPath(filepath.Join(tmpDir, "data")).
			RuntimePath(filepath.Join(tmpDir, "runtime")),
	)
	require.NoError(t, pg.Start(), "embedded PG should start")
	defer func() {
		_ = pg.Stop()
	}()

	dsn := "postgres://postgres:postgres@localhost:9876/postgres?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Point goose at the embedded migration FS (same as main.go).
	goose.SetBaseFS(embedMigrations)
	defer goose.SetBaseFS(nil)
	require.NoError(t, goose.SetDialect("postgres"))

	// Step 1: Up — goose.Up creates goose_db_version + applies all migrations.
	require.NoError(t, goose.Up(db, "migrations"), "goose Up should apply all migrations")

	// Verify teams table exists.
	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "teams table should exist after Up")

	// Verify layer seed landed.
	var layerCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layer_logical_refs").Scan(&layerCount)
	require.NoError(t, err)
	assert.Equal(t, 3, layerCount, "Phase 1 should seed 3 layers")

	// Verify rule set seed.
	var rsCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layer_rule_set_versions WHERE is_default = true").Scan(&rsCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rsCount, "should have 1 default rule set version")

	// Verify all 20 MVP tables exist.
	tableCount := 0
	for _, tbl := range []string{
		"teams", "projects", "bundles", "modules", "module_versions", "module_dependencies",
		"catalog_items", "requests", "request_events", "plan_artifacts", "gate_results",
		"approval_flows", "approval_runs", "approval_node_runs", "approval_decisions",
		"cloud_accounts", "audit_logs", "outbox_events", "layer_logical_refs", "layer_rule_set_versions",
	} {
		var ex bool
		err = db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = '%s')", tbl)).Scan(&ex)
		require.NoError(t, err)
		if ex {
			tableCount++
		} else {
			t.Errorf("table %s missing after Up", tbl)
		}
	}
	assert.Equal(t, 20, tableCount, "all 20 MVP tables should exist")

	// Verify the updated_at trigger function exists.
	var fnExists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM pg_proc WHERE proname = 'set_updated_at')").Scan(&fnExists)
	require.NoError(t, err)
	assert.True(t, fnExists, "set_updated_at() function should exist")

	// Step 2: Down — goose.Redo rolls back all then re-applies (tests Down + Up idempotency).
	// Use goose.DownTo(0) to roll back everything.
	require.NoError(t, goose.DownTo(db, "migrations", 0), "goose DownTo 0 should roll back all migrations")

	// Verify teams table gone.
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "teams table should be gone after full Down")

	// Step 3: Up again (idempotent re-application).
	require.NoError(t, goose.Up(db, "migrations"), "goose re-Up should succeed (idempotent)")

	t.Logf("DDL verification passed: 10 migrations Up→Down→Up, 20 tables, 3 layer seed, trigger function")
}

func TestMain(m *testing.M) {
	// embedded-postgres may need to download PG binary on first run.
	// Set EMBEDDED_POSTGRES_CACHE to a stable dir to avoid re-downloading.
	if cacheDir := os.Getenv("EMBEDDED_POSTGRES_CACHE"); cacheDir == "" {
		os.Setenv("EMBEDDED_POSTGRES_CACHE", filepath.Join(os.TempDir(), "embedded-pg-cache"))
	}
	os.Exit(m.Run())
}
