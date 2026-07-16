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
	ctx := context.Background()

	// Use a fresh snowflake ID (don't collide with seed IDs 1-4 / 0).
	env, err := envRepo.Create(ctx, generated.CreateEnvironmentParams{
		ID:               utils.GenerateID(),
		EnvLogicalID:     "test-env-" + envSuffix(),
		DisplayName:      "Test Env",
		Stage:            "dev",
		Region:           "cn-hangzhou",
		TagNamespaceJson: []byte(`{}`),
	})
	require.NoError(t, err)

	tenant, err := tenantRepo.Create(ctx, generated.CreateTenantParams{
		ID:               utils.GenerateID(),
		TenantLogicalID:  "test-tenant-" + envSuffix(),
		Name:             "Test Tenant",
		IsolationLevel:   "vpc-per-env",
		Kind:             "internal",
		TagNamespaceJson: []byte(`{}`),
	})
	require.NoError(t, err)

	return envRepo, tenantRepo, &env, &tenant
}

// TestEnvironmentRepo_CRUD verifies environment CRUD + logical-id lookup.
func TestEnvironmentRepo_CRUD(t *testing.T) {
	r, _, env, _ := setupEnvTenantTestDB(t)
	c := context.Background()

	// GetByID
	got, err := r.GetByID(c, env.ID)
	require.NoError(t, err)
	assert.Equal(t, env.ID, got.ID)

	// GetByLogicalId
	got2, err := r.GetByLogicalId(c, env.EnvLogicalID)
	require.NoError(t, err)
	assert.Equal(t, env.ID, got2.ID)

	// List (includes seeded dev/staging/prod/dr + our test env)
	list, err := r.List(c)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 5)
}

// TestTenantRepo_CRUD verifies tenant CRUD + logical-id lookup.
func TestTenantRepo_CRUD(t *testing.T) {
	_, r, _, tenant := setupEnvTenantTestDB(t)
	c := context.Background()

	got, err := r.GetByID(c, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, got.ID)

	got2, err := r.GetByLogicalId(c, tenant.TenantLogicalID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, got2.ID)

	// List includes seeded platform-default + our test tenant.
	list, err := r.List(c)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
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
	def, err := tenantRepo.GetByLogicalId(c, "platform-default")
	require.NoError(t, err)
	assert.Equal(t, "vpc-per-env", def.IsolationLevel)

	// dev/staging/prod/dr envs seeded
	envRepo := repo.NewEnvironmentRepo(pool)
	for _, logicalID := range []string{"dev", "staging", "prod", "dr"} {
		_, err := envRepo.GetByLogicalId(c, logicalID)
		require.NoError(t, err, "seeded env %s not found", logicalID)
	}
}
