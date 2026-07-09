package data_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// setupTestDB starts a fresh test database (via testdb.New, which spins up a
// dedicated PG container + migrates + clones) and returns sqlc Queries on it.
// Each test gets an isolated DB, so no truncation is needed.
func setupTestDB(t *testing.T) *generated.Queries {
	t.Helper()
	pool := testdb.New(t)
	return generated.New(pool)
}

func TestCreateTeam(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	team, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "DBA Team",
		Slug: "dba",
	})
	require.NoError(t, err)
	assert.Equal(t, "DBA Team", team.Name)
	assert.Equal(t, "dba", team.Slug)
	assert.True(t, team.ID > 0)
}

func TestGetTeamBySlug(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	created, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "Middleware Team",
		Slug: "middleware",
	})
	require.NoError(t, err)

	found, err := queries.GetTeamBySlug(ctx, "middleware")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Middleware Team", found.Name)
}

func TestListTeams(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	for _, slug := range []string{"alpha", "beta", "gamma"} {
		_, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
			Name: fmt.Sprintf("Team %s", slug),
			Slug: slug,
		})
		require.NoError(t, err)
	}

	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Len(t, teams, 3)
}

func TestDeleteTeam(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	team, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		Name: "Temp Team",
		Slug: "temp",
	})
	require.NoError(t, err)

	err = queries.DeleteTeam(ctx, team.ID)
	require.NoError(t, err)

	_, err = queries.GetTeam(ctx, team.ID)
	assert.Error(t, err)
}

func TestListTeamsEmptySlice(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.NotNil(t, teams)
	assert.Len(t, teams, 0)
}
