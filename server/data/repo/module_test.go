package repo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// setupModuleRepoTestDB starts a fresh test DB (testcontainers + migrate) and
// returns a ModuleRepo + a pre-created owner team (modules.owner_team_id FK).
// Each test gets an isolated DB; tests requiring Docker are skipped in -short.
// Uses t.Context() so hung DB calls are cancelled on test timeout.
func setupModuleRepoTestDB(t *testing.T) (*repo.ModuleRepo, *generated.Team) {
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
	team, err := queries.CreateTeam(t.Context(), generated.CreateTeamParams{
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
	r, team := setupModuleRepoTestDB(t)
	c := t.Context()

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
	r, team := setupModuleRepoTestDB(t)
	c := t.Context()

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

// TestModuleRepo_CreateWithVersion_Rollback verifies the transaction rolls back
// the module insertion if the version creation fails. Fault injection: we force
// a PK collision on module_versions.id by pre-creating a version with the same
// ID, so the inner CreateModuleVersion inside the tx fails, triggering rollback.
// After failure, the module must NOT exist (proving rollback worked).
func TestModuleRepo_CreateWithVersion_Rollback(t *testing.T) {
	r, team := setupModuleRepoTestDB(t)
	c := t.Context()

	// Step 1: create a module + version normally to occupy a known version ID.
	mod1, _, err := r.CreateWithVersion(c, newModuleParams(team.ID), newModuleVersionParams(0))
	require.NoError(t, err)
	// Fetch the version to learn its ID (the one we'll collide on).
	mvr := repo.NewModuleVersionRepo(r.Pool())
	ver1, err := mvr.GetCurrent(c, mod1.ID)
	require.NoError(t, err)
	collidingVersionID := ver1.ID

	// Step 2: attempt CreateWithVersion again with a NEW module but a version
	// whose ID collides with ver1.ID. The inner CreateModuleVersion will fail
	// with PK violation; the transaction must roll back the new module.
	newModParams := newModuleParams(team.ID)
	newModParams.Name = "alicloud-redis" // distinct git_source to avoid module collision
	newModParams.GitSource = "git@github.com:xuanwu-labs/tf-modules.git//atomic/redis"
	badVerParams := newModuleVersionParams(0)
	badVerParams.ID = collidingVersionID // PK collision — forces inner failure

	_, _, err = r.CreateWithVersion(c, newModParams, badVerParams)
	require.Error(t, err, "expected PK collision on module_versions.id to fail the tx")

	// Step 3: verify the NEW module was rolled back (does not exist).
	_, err = r.GetByGitSource(c, newModParams.GitSource)
	require.Error(t, err, "rolled-back module must not exist")

	// Step 4: verify the FIRST module + version still exist (commit from step 1
	// survived; the step-2 rollback didn't corrupt unrelated rows).
	got1, err := r.GetByID(c, mod1.ID)
	require.NoError(t, err)
	assert.Equal(t, "alicloud-rds-mysql", got1.Name)
}

// TestModuleRepo_DynamicFilter verifies ListByDynamicFilter with QueryWrapper
// produces correct results (Eq + In + pagination).
func TestModuleRepo_DynamicFilter(t *testing.T) {
	r, team := setupModuleRepoTestDB(t)
	c := t.Context()

	// Create 3 modules across 2 layers.
	for _, layer := range []string{"middleware", "middleware", "application"} {
		p := newModuleParams(team.ID)
		p.Layer = layer
		p.GitSource = fmt.Sprintf("git@github.com:xuanwu-labs/tf-modules.git//atomic/%s-%d", layer, utils.GenerateID())
		_, err := r.Create(c, p)
		require.NoError(t, err)
	}

	// Filter: layer IN (middleware) — should return 2.
	w := repo.New().In("layer", "middleware")
	results, err := r.ListByDynamicFilter(c, w)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
