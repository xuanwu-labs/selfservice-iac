package data_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// setupTestDB starts a fresh test database (via testdb.New, which spins up a
// dedicated PG container + migrates + clones) and initializes the snowflake
// generator so CreateTeam can produce IDs. Each test gets an isolated DB.
func setupTestDB(t *testing.T) *generated.Queries {
	t.Helper()
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	return generated.New(pool)
}

// newTeamParams builds a CreateTeamParams with a fresh snowflake ID and sane
// defaults. Callers override fields as needed.
func newTeamParams(name, slug, kind string) generated.CreateTeamParams {
	return generated.CreateTeamParams{
		ID:         utils.GenerateID(),
		Name:       name,
		Slug:       slug,
		Kind:       kind,
		Status:     "active",
		TagsJson:   []byte(`{}`),
		PolicyJson: []byte(`{}`),
	}
}

func TestCreateTeam(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	team, err := queries.CreateTeam(ctx, newTeamParams("DBA Team", "dba", "dba"))
	require.NoError(t, err)
	assert.Equal(t, "DBA Team", team.Name)
	assert.Equal(t, "dba", team.Slug)
	assert.Equal(t, "dba", team.Kind)
	assert.True(t, team.ID > 0)
}

func TestGetTeamBySlug(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	created, err := queries.CreateTeam(ctx, newTeamParams("Middleware Team", "middleware", "middleware"))
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
		_, err := queries.CreateTeam(ctx, newTeamParams(fmt.Sprintf("Team %s", slug), slug, "business"))
		require.NoError(t, err)
	}

	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Len(t, teams, 3)
}

func TestSoftDeleteTeam(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	team, err := queries.CreateTeam(ctx, newTeamParams("Temp Team", "temp", "business"))
	require.NoError(t, err)

	err = queries.SoftDeleteTeam(ctx, team.ID)
	require.NoError(t, err)

	// GetTeam filters deleted_at IS NULL, so a soft-deleted team is gone.
	_, err = queries.GetTeam(ctx, team.ID)
	assert.Error(t, err)
}

func TestListTeamsEmptySlice(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.NotNil(t, teams)
	assert.Len(t, teams, 0) // emit_empty_slices: non-nil empty slice
}

func TestListTeamsByKind(t *testing.T) {
	queries := setupTestDB(t)
	ctx := context.Background()

	_, err := queries.CreateTeam(ctx, newTeamParams("DBA Team", "dba", "dba"))
	require.NoError(t, err)
	_, err = queries.CreateTeam(ctx, newTeamParams("Platform Team", "platform", "platform"))
	require.NoError(t, err)
	_, err = queries.CreateTeam(ctx, newTeamParams("Another DBA", "dba2", "dba"))
	require.NoError(t, err)

	dbaTeams, err := queries.ListTeamsByKind(ctx, "dba")
	require.NoError(t, err)
	assert.Len(t, dbaTeams, 2)
}
