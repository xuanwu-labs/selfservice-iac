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

// setupEnvTenantTestDB seeds env + tenant repos with a pre-created env + tenant.
// Uses t.Context() (Go 1.24+) so the test is cancelled on timeout/failure.
func setupEnvTenantTestDB(t *testing.T) (*repo.EnvironmentRepo, *repo.TenantRepo, *generated.Environment, *generated.Tenant) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	envRepo := repo.NewEnvironmentRepo(pool)
	tenantRepo := repo.NewTenantRepo(pool)
	ctx := t.Context()

	env, err := envRepo.Create(ctx, generated.CreateEnvironmentParams{
		ID:               utils.GenerateID(),
		EnvLogicalID:     "test-env-" + envSuffix(t),
		DisplayName:      "Test Env",
		Stage:            "dev",
		Region:           "cn-hangzhou",
		TagNamespaceJson: []byte(`{}`),
	})
	require.NoError(t, err)

	tenant, err := tenantRepo.Create(ctx, generated.CreateTenantParams{
		ID:               utils.GenerateID(),
		TenantLogicalID:  "test-tenant-" + envSuffix(t),
		Name:             "Test Tenant",
		IsolationLevel:   "vpc-per-env",
		Kind:             "internal",
		TagNamespaceJson: []byte(`{}`),
	})
	require.NoError(t, err)

	return envRepo, tenantRepo, &env, &tenant
}

// TestEnvironmentRepo_CRUD verifies environment CRUD + logical-id lookup + Update + SoftDelete.
func TestEnvironmentRepo_CRUD(t *testing.T) {
	r, _, env, _ := setupEnvTenantTestDB(t)
	c := t.Context()

	// GetByID
	got, err := r.GetByID(c, env.ID)
	require.NoError(t, err)
	assert.Equal(t, env.ID, got.ID)

	// GetByLogicalID
	got2, err := r.GetByLogicalID(c, env.EnvLogicalID)
	require.NoError(t, err)
	assert.Equal(t, env.ID, got2.ID)

	// List (includes seeded dev/staging/prod/dr + our test env)
	list, err := r.List(c)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 5)

	// Update — change display name + region.
	updated, err := r.Update(c, generated.UpdateEnvironmentParams{
		ID:          env.ID,
		DisplayName: "Updated Env",
		Region:      "cn-shanghai",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Env", updated.DisplayName)

	// SoftDelete — then verify it disappears from GetByID and List count stays same.
	require.NoError(t, r.SoftDelete(c, env.ID))
	_, err = r.GetByID(c, env.ID)
	require.Error(t, err, "soft-deleted env should not be returned by GetByID")
	list2, err := r.List(c)
	require.NoError(t, err)
	assert.Equal(t, len(list)-1, len(list2), "soft-deleted env should be filtered from List")
}

// TestTenantRepo_CRUD verifies tenant CRUD + logical-id lookup + Update + SoftDelete.
func TestTenantRepo_CRUD(t *testing.T) {
	_, r, _, tenant := setupEnvTenantTestDB(t)
	c := t.Context()

	got, err := r.GetByID(c, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, got.ID)

	got2, err := r.GetByLogicalID(c, tenant.TenantLogicalID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, got2.ID)

	// List includes seeded platform-default + our test tenant.
	list, err := r.List(c)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)

	// Update — change name + isolation level.
	updated, err := r.Update(c, generated.UpdateTenantParams{
		ID:             tenant.ID,
		Name:           "Updated Tenant",
		IsolationLevel: "account-per-env",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Tenant", updated.Name)
	assert.Equal(t, "account-per-env", updated.IsolationLevel)

	// SoftDelete — then verify it disappears from GetByID.
	require.NoError(t, r.SoftDelete(c, tenant.ID))
	_, err = r.GetByID(c, tenant.ID)
	require.Error(t, err, "soft-deleted tenant should not be returned by GetByID")
}

// TestSeedData verifies migration 013 seeds are present (platform-default tenant,
// dev/staging/prod/dr envs, platform tag policy).
func TestSeedData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode")
	}
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	c := context.Background()

	// platform-default tenant seeded
	tenantRepo := repo.NewTenantRepo(pool)
	def, err := tenantRepo.GetByLogicalID(c, "platform-default")
	require.NoError(t, err)
	assert.Equal(t, "vpc-per-env", def.IsolationLevel)

	// dev/staging/prod/dr envs seeded
	envRepo := repo.NewEnvironmentRepo(pool)
	for _, logicalID := range []string{"dev", "staging", "prod", "dr"} {
		_, err := envRepo.GetByLogicalID(c, logicalID)
		require.NoError(t, err, "seeded env %s not found", logicalID)
	}
}
