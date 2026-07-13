package main_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
)

// testDSN returns a fresh test database DSN (testcontainers PG). Each call
// gets an isolated DB; cleanSlate resets it so goose tests start from empty.
func testDSN(t *testing.T) string {
	t.Helper()
	return testdb.NewDSN(t)
}

// cleanSlate drops all MVP tables + goose_db_version so goose sees a fresh DB.
// Order matters: child tables before parents (FK RESTRICT). We drop CASCADE
// on each to be safe against any lingering dependency.
func cleanSlate(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	// Drop in reverse-dependency order. CASCADE handles any missed edge.
	tables := []string{
		"approval_decisions", "approval_node_runs", "approval_runs", "approval_flows",
		"gate_results", "plan_artifacts",
		"request_events", "requests",
		"catalog_items",
		"module_dependencies", "module_versions", "modules",
		"bundles", "projects",
		"cloud_accounts",
		"audit_logs", "outbox_events",
		"layer_rule_set_versions", "layer_logical_refs",
		"teams",
	}
	for _, tbl := range tables {
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl+" CASCADE")
	}
	// Goose bookkeeping + the shared trigger function (lives outside any table).
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS goose_db_version CASCADE")
	_, _ = db.ExecContext(ctx, "DROP FUNCTION IF EXISTS set_updated_at()")
}

// expectedMigrationCount is the number of .sql migration files applied by Up.
// Update this when adding a new migration file.
const expectedMigrationCount = 11 // 000_utils through 010_layers

func TestMigrationUpDownUpIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", testDSN(t))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrationFS := os.DirFS("migrations")
	newProvider := func() *goose.Provider {
		p, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS)
		require.NoError(t, err)
		return p
	}

	cleanSlate(t, db)

	// Step 1: Up — all migrations apply cleanly in dependency order.
	p := newProvider()
	results, err := p.Up(ctx)
	require.NoError(t, err, "Up should succeed")
	assert.Len(t, results, expectedMigrationCount, "should apply all migrations")

	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "teams table should exist after Up")

	// Verify the layer seed landed (D24 Phase 1 fixed 3-layer).
	var layerCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layer_logical_refs").Scan(&layerCount)
	require.NoError(t, err)
	assert.Equal(t, 3, layerCount, "Phase 1 should seed 3 layer_logical_refs")

	// Step 2: Down — roll back all migrations in reverse.
	for i := 0; i < expectedMigrationCount; i++ {
		_, err := p.Down(ctx)
		require.NoError(t, err, "Down step %d should succeed", i)
	}

	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "teams table should be gone after full Down")

	// Step 3: Up again (idempotent re-application).
	results, err = p.Up(ctx)
	require.NoError(t, err, "Up after Down should succeed")
	assert.Len(t, results, expectedMigrationCount, "should re-apply all migrations")

	// Leave DB in "applied" state for other test packages
}

func TestMigrationStatusFullHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", testDSN(t))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrationFS := os.DirFS("migrations")
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS)
	require.NoError(t, err)

	cleanSlate(t, db)

	_, err = p.Up(ctx)
	require.NoError(t, err)

	statuses, err := p.Status(ctx)
	require.NoError(t, err)
	assert.Len(t, statuses, expectedMigrationCount, "should have all migrations in history")

	// Leave DB in "applied" state
}
