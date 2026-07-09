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

// cleanSlate drops teams + goose_db_version so goose sees a fresh DB.
func cleanSlate(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS teams CASCADE")
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS goose_db_version CASCADE")
}

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

	// Step 1: Up
	p := newProvider()
	results, err := p.Up(ctx)
	require.NoError(t, err, "Up should succeed")
	assert.Len(t, results, 1, "should apply 1 migration")

	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "teams table should exist after Up")

	// Step 2: Down
	_, err = p.Down(ctx)
	require.NoError(t, err, "Down should succeed")

	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'teams')").Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "teams table should be gone after Down")

	// Step 3: Up again (idempotent)
	results, err = p.Up(ctx)
	require.NoError(t, err, "Up after Down should succeed")
	assert.Len(t, results, 1, "should re-apply 1 migration")

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
	assert.Len(t, statuses, 1, "should have 1 migration in history")
	assert.Equal(t, int64(1), statuses[0].Source.Version)

	// Leave DB in "applied" state
}
