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

// testDSN returns a fresh, goose-migrated test database DSN (testcontainers PG).
// pgtestdb now uses gooseMigrator which runs real migrations, so the DB has
// all 20 MVP tables + layer seed. For up/down/up idempotency tests, the test
// runs goose Down then Up again on this already-migrated DB.
func testDSN(t *testing.T) string {
	t.Helper()
	return testdb.NewDSN(t)
}

// cleanSlate is no longer needed — NewRawDSN hands an empty DB. Kept as a
// no-op stub for any future test that might reuse a non-empty template.
func cleanSlate(_ *testing.T, _ *sql.DB) {}

// expectedMigrationCount is the number of .sql migration files applied by Up.
// Update this when adding a new migration file.
const expectedMigrationCount = 10 // 001_init through 010_layers (000 merged into 001: goose skips v0)

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

	// DB is already goose-migrated by testdb.NewDSN (gooseMigrator runs
	// real migrations). Verify the migrated state, then test Down→Up idempotency.

	// Step 1: Verify migrated state (tables exist + seed landed).
	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "teams table should exist after initial migration")

	var layerCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM layer_logical_refs").Scan(&layerCount)
	require.NoError(t, err)
	assert.Equal(t, 3, layerCount, "Phase 1 should seed 3 layer_logical_refs")

	// Step 2: Down — roll back all migrations in reverse.
	p := newProvider()
	for i := 0; i < expectedMigrationCount; i++ {
		_, err := p.Down(ctx)
		require.NoError(t, err, "Down step %d should succeed", i)
	}

	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "teams table should be gone after full Down")

	// Step 3: Up again (idempotent re-application — the core idempotency test).
	results, err := p.Up(ctx)
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
