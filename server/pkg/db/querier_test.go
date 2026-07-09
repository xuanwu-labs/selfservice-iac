package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TestQueriesCRUD verifies the testdb + sqlc chain works end-to-end:
// the pool from testdb.New satisfies generated.Queries, and basic
// CreateTeam/ListTeams/GetTeamBySlug round-trip correctly.
//
// Requires Docker (DOCKER_HOST). Skipped in -short mode.
func TestQueriesCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}

	pool := db.New(t)
	queries := generated.New(pool)
	ctx := context.Background()

	// Initially empty.
	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Empty(t, teams, "fresh test db should have no teams")

	// Create two teams.
	created1, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "Platform",
		Slug: "platform",
	})
	require.NoError(t, err)
	require.NotZero(t, created1.ID)

	created2, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "Data",
		Slug: "data",
	})
	require.NoError(t, err)
	require.NotZero(t, created2.ID)

	// List now has both.
	teams, err = queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Len(t, teams, 2)

	// Lookup by slug.
	got, err := queries.GetTeamBySlug(ctx, "platform")
	require.NoError(t, err)
	assert.Equal(t, "Platform", got.Name)
	assert.Equal(t, "platform", got.Slug)

	// Slug uniqueness is enforced (DB constraint teams_slug_uk).
	_, err = queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "Dup",
		Slug: "platform", // duplicate slug
	})
	assert.Error(t, err, "duplicate slug must violate unique constraint")
}
