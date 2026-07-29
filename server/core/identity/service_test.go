package identity_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/identity"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
)

// setupService returns an IdentityService bound to a fresh, fully-migrated
// test database plus the underlying pool (for ad-hoc verification queries).
// Skipped in -short mode (needs Docker via DOCKER_HOST).
func setupService(t *testing.T) (*identity.IdentityService, *pgxpool.Pool) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	return identity.NewIdentityService(pool), pool
}

// uniqueExternalID returns a unique external id per test to avoid collisions
// on the (external_id, provider_name) unique index across tests sharing the
// same template DB.
func uniqueExternalID(t *testing.T) string {
	t.Helper()
	return "admin-" + strconv.FormatInt(utils.GenerateID(), 10) + "@example.com"
}

// TestBootstrapAdmin_Idempotent verifies the bootstrap is a no-op the second
// time: same identity, same role_binding row, no duplicate.
func TestBootstrapAdmin_Idempotent(t *testing.T) {
	svc, pool := setupService(t)
	ctx := context.Background()

	ext := uniqueExternalID(t)

	// First call: creates identity + binding.
	ident1, binding1, err := svc.BootstrapAdmin(ctx, ext, "Admin One", "admin1@example.com")
	require.NoError(t, err)
	require.NotZero(t, ident1.ID, "identity id should be populated")
	require.NotZero(t, binding1, "binding id should be populated")
	assert.Equal(t, "Admin One", ident1.DisplayName)

	// Second call: must be a no-op — same identity, same binding.
	ident2, binding2, err := svc.BootstrapAdmin(ctx, ext, "Admin One", "admin1@example.com")
	require.NoError(t, err)
	assert.Equal(t, ident1.ID, ident2.ID, "identity should not be re-created")
	assert.Equal(t, binding1, binding2, "role_binding should not be re-created")

	// Sanity: exactly one platform-admin binding for this subject.
	var n int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM role_bindings WHERE subject_id = $1 AND scope_type = 'platform' AND role = 'admin'`,
		ext,
	).Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "expected exactly one platform-admin binding after two bootstraps")
}

// TestCreateAndGetByExternalID covers the happy-path CRUD surface.
func TestCreateAndGetByExternalID(t *testing.T) {
	svc, _ := setupService(t)
	ctx := context.Background()

	ext := uniqueExternalID(t)
	created, err := svc.Create(ctx, identity.CreateParams{
		ExternalID:   ext,
		DisplayName:  "Alice",
		Email:        "alice@example.com",
		ProviderName: identity.ProviderOIDC,
	})
	require.NoError(t, err)
	assert.Equal(t, ext, created.ExternalID)
	assert.Equal(t, identity.ProviderOIDC, created.ProviderName)

	got, err := svc.GetByExternalID(ctx, ext, identity.ProviderOIDC)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	byID, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ExternalID, byID.ExternalID)
}

// TestGetByExternalID_NotFound verifies the not-found path returns a pgx
// ErrNoRows-wrapped error that BootstrapAdmin can detect.
func TestGetByExternalID_NotFound(t *testing.T) {
	svc, _ := setupService(t)
	ctx := context.Background()

	_, err := svc.GetByExternalID(ctx, "nonexistent@example.com", identity.ProviderLocal)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// TestList verifies List returns the created identity.
func TestList(t *testing.T) {
	svc, _ := setupService(t)
	ctx := context.Background()

	ext := uniqueExternalID(t)
	_, err := svc.Create(ctx, identity.CreateParams{
		ExternalID:   ext,
		DisplayName:  "Bob",
		ProviderName: identity.ProviderLocal,
	})
	require.NoError(t, err)

	all, err := svc.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)

	var found bool
	for _, id := range all {
		if id.ExternalID == ext {
			found = true
			break
		}
	}
	assert.True(t, found, "created identity should appear in List")
}
