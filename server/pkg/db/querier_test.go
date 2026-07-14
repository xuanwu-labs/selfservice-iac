package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TestQueriesCRUD verifies the testdb + sqlc chain works end-to-end:
// the pool from testdb.New (goose-migrated) satisfies generated.Queries,
// and basic CreateTeam/ListTeams/GetTeamBySlug round-trip correctly.
//
// Requires Docker (DOCKER_HOST). Skipped in -short mode.
func TestQueriesCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}

	// Initialize snowflake ID generator (CreateTeam needs app-generated ID).
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}

	pool := db.New(t)
	queries := generated.New(pool)
	ctx := context.Background()

	// Initially empty (goose seeds layer tables, but not teams).
	teams, err := queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Empty(t, teams, "fresh test db should have no teams")

	// Create a team with full params (snowflake ID + all required fields).
	created, err := queries.CreateTeam(ctx, generated.CreateTeamParams{
		ID:         utils.GenerateID(),
		Name:       "Platform Ops",
		Slug:       "platform",
		Kind:       "platform",
		Status:     "active",
		TagsJson:   []byte(`{}`),
		PolicyJson: []byte(`{}`),
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, "Platform Ops", created.Name)
	assert.Equal(t, "platform", created.Slug)
	assert.Equal(t, "platform", created.Kind)

	// List now has one team.
	teams, err = queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Len(t, teams, 1)

	// Lookup by slug.
	got, err := queries.GetTeamBySlug(ctx, "platform")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// ListTeamsByKind filter.
	platformTeams, err := queries.ListTeamsByKind(ctx, "platform")
	require.NoError(t, err)
	assert.Len(t, platformTeams, 1)

	// Soft-delete and verify it's filtered out.
	require.NoError(t, queries.SoftDeleteTeam(ctx, created.ID))
	teams, err = queries.ListTeams(ctx)
	require.NoError(t, err)
	assert.Empty(t, teams, "soft-deleted team should not appear in ListTeams")
}
