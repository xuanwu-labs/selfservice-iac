package main_test

import (
	"embed"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// embedMigrations mirrors the //go:embed in main.go so this test can parse
// the same migration files goose would run.
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

// TestMigrationFilesParse verifies all migration files can be parsed by goose
// (SQL structure: -- +goose Up/Down markers, version numbering) WITHOUT a
// running database. This is the structural prerequisite for the up/down/up
// idempotency test — if parsing fails, the DB test would fail too.
//
// It validates:
// 1. All 10 migration files (001-010) are discoverable and parseable.
// 2. Version numbers are sequential (1-10, no gaps).
// 3. Each migration has both Up and Down blocks.
// 4. No version 0 (goose skips it — we merged 000_utils into 001).
func TestMigrationFilesParse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	// Point goose at the embedded FS (same as main.go does at runtime).
	goose.SetBaseFS(embedMigrations)
	defer goose.SetBaseFS(nil)

	// CollectMigrations parses all .sql files in the migrations/ directory.
	// current=-1, target=999999 → collect everything.
	migrations, err := goose.CollectMigrations("migrations", -1, 999999)
	require.NoError(t, err, "goose should parse all migration files without error")
	require.NotEmpty(t, migrations, "should find at least one migration")

	// Verify sequential versions 1..N with no gaps.
	for i, m := range migrations {
		expectedVersion := int64(i + 1)
		assert.Equal(t, expectedVersion, m.Version,
			"migration %d should have version %d (sequential, no gaps)", i, expectedVersion)

		// Each migration must have parsed Up and Down statements.
		assert.NotNil(t, m.Up, "migration v%d must have Up block", m.Version)
		assert.NotNil(t, m.Down, "migration v%d must have Down block", m.Version)

		// No version 0 (we merged 000_utils.sql into 001_init.sql because
		// goose skips version 0).
		assert.Greater(t, m.Version, int64(0), "no version 0 allowed (merged into 001)")
	}

	t.Logf("parsed %d migrations (versions 1-%d), all have Up+Down blocks",
		len(migrations), migrations[len(migrations)-1].Version)
}

// TestMigrationFilesCount confirms the exact migration count matches
// expectedMigrationCount in main_test.go. If they drift, one test is stale.
func TestMigrationFilesCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	goose.SetBaseFS(embedMigrations)
	defer goose.SetBaseFS(nil)

	migrations, err := goose.CollectMigrations("migrations", -1, 999999)
	require.NoError(t, err)
	assert.Equal(t, expectedMigrationCount, len(migrations),
		"migration count must match expectedMigrationCount (update both if adding migrations)")
}
