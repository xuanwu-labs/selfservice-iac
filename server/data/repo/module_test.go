package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// setupRepoTestDB starts a fresh test DB (testcontainers + migrate) and returns
// a pool + ModuleRepo + a pre-created owner team (modules.owner_team_id FK).
// Each test gets an isolated DB; tests requiring Docker are skipped in -short.
func setupRepoTestDB(t *testing.T) (*repo.ModuleRepo, *generated.Team) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	// Seed an owner team so modules.owner_team_id has a valid FK target.
	queries := generated.New(pool)
	team, err := queries.CreateTeam(ctx(t), generated.CreateTeamParams{
		ID:         utils.GenerateID(),
		Name:       "DBA Team",
		Slug:       "dba",
		Kind:       "dba",
		Status:     "active",
		TagsJson:   []byte(`{}`),
		PolicyJson: []byte(`{}`),
	})
	require.NoError(t, err)
	return repo.NewModuleRepo(pool), &team
}

// ctx returns a background context (helper to keep test bodies short).
func ctx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

func newModuleParams(ownerTeamID int64) generated.CreateModuleParams {
	return generated.CreateModuleParams{
		ID:          utils.GenerateID(),
		Name:        "alicloud-rds-mysql",
		GitSource:   "git@github.com:xuanwu-labs/tf-modules.git//atomic/rds-mysql",
		Provider:    "alicloud",
		Layer:       "middleware",
		OwnerTeamID: ownerTeamID,
		Status:      "validated",
		Description: "Atomic RDS MySQL module",
	}
}

func newModuleVersionParams(moduleID int64) generated.CreateModuleVersionParams {
	return generated.CreateModuleVersionParams{
		ID:                    utils.GenerateID(),
		ModuleID:              moduleID,
		Version:               "v1.0.0",
		CommitSha:             "abc123def456",
		ProvidersJson:         []byte(`{"alicloud":">=1.200"}`),
		VariablesContractJson: []byte(`{"instance_type":"string"}`),
		IsCurrent:             true,
	}
}

// TestModuleRepo_CRUD verifies the basic CRUD wrapper methods round-trip.
func TestModuleRepo_CRUD(t *testing.T) {
	r, team := setupRepoTestDB(t)
	c := ctx(t)

	// Create
	created, err := r.Create(c, newModuleParams(team.ID))
	require.NoError(t, err)
	assert.Equal(t, "alicloud-rds-mysql", created.Name)
	assert.Equal(t, "middleware", created.Layer)

	// GetByID
	got, err := r.GetByID(c, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// GetByGitSource
	got2, err := r.GetByGitSource(c, created.GitSource)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got2.ID)

	// List
	list, err := r.List(c)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// ListByOwner
	byOwner, err := r.ListByOwner(c, team.ID)
	require.NoError(t, err)
	assert.Len(t, byOwner, 1)

	// ListByLayer
	byLayer, err := r.ListByLayer(c, "middleware")
	require.NoError(t, err)
	assert.Len(t, byLayer, 1)
}

// TestModuleRepo_CreateWithVersion_Success verifies the cross-table transaction
// commits both module and version atomically (W1-02 D3 pattern).
func TestModuleRepo_CreateWithVersion_Success(t *testing.T) {
	r, team := setupRepoTestDB(t)
	c := ctx(t)

	mod, ver, err := r.CreateWithVersion(c, newModuleParams(team.ID), newModuleVersionParams(0))
	require.NoError(t, err)
	assert.NotZero(t, mod.ID)
	assert.NotZero(t, ver.ID)
	// Version must point to the freshly created module.
	assert.Equal(t, mod.ID, ver.ModuleID)
	assert.True(t, ver.IsCurrent)

	// Verify both rows persisted (not just returned).
	got, err := r.GetByID(c, mod.ID)
	require.NoError(t, err)
	assert.Equal(t, mod.ID, got.ID)
}

// TestModuleRepo_CreateWithVersion_RollbackOnFKError verifies the transaction
// rolls back the module insertion if the version creation fails (e.g. FK error).
// We force failure by pointing the version at a non-existent module_id AFTER the
// real module is created — the wrapper overrides verArg.ModuleID = mod.ID, so to
// actually trigger a rollback we need a different failure vector: we pass a
// version with an invalid module_id seed, but CreateWithVersion overwrites it.
// Instead we test rollback by closing the pool mid-tx is too complex here;
// the atomic-commit success test above already exercises the happy path.
// Kept as a placeholder documenting intent; a true rollback test needs a
// fault-injection seam (TODO).
func TestModuleRepo_CreateWithVersion_Documentation(t *testing.T) {
	t.Skip("rollback test needs fault-injection seam; see TestModuleRepo_CreateWithVersion_Success for happy path")
}
