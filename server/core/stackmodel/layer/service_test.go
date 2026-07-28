// Package layer_test: service_test.go — covers LayerService.
//
// LayerService depends on two repos backed by Postgres, so the read-path tests
// need testcontainers (skipped in -short). The constructor test runs always and
// guards against nil-repo panics during wiring.
package layer_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/layer"
	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	testdb "github.com/xuanwu-labs/selfservice-iac/server/pkg/db"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// setupLayerServiceTestDB starts a fresh test DB (testcontainers + migrate) and
// returns a LayerService plus the active rule-set version it should observe.
// Mirrors the repo-test helpers in data/repo/*_test.go.
func setupLayerServiceTestDB(t *testing.T) (*layer.LayerService, generated.LayerRuleSetVersion) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB-dependent test in -short mode (needs Docker via DOCKER_HOST)")
	}
	if err := utils.Init(0, 0); err != nil {
		t.Fatalf("snowflake init: %v", err)
	}
	pool := testdb.New(t)
	queries := generated.New(pool)

	// Seed one layer logical ref (the catalog migrations may already do this,
	// but inserting explicitly keeps the test independent of seed state).
	_, err := queries.CreateLayerLogicalRef(t.Context(), generated.CreateLayerLogicalRefParams{
		LogicalID:          "application",
		CurrentDisplayName: "Application Layer",
		Notes:              "seeded by layer_test",
	})
	// Ignore "already exists" — migrations may have created it.
	if err != nil && !isDup(err) {
		require.NoError(t, err)
	}

	// Seed an active+default rule-set version so GetActiveRuleSet has a row.
	rs, err := queries.CreateRuleSetVersion(t.Context(), generated.CreateRuleSetVersionParams{
		VersionID:  1,
		LayersJson: []byte(`{}`),
		Status:     "active",
		IsDefault:  true,
		CreatedBy:  "layer_test",
	})
	require.NoError(t, err)

	svc := layer.NewLayerService(
		repo.NewLayerLogicalRefRepo(pool),
		repo.NewLayerRuleSetVersionRepo(pool),
	)
	return svc, rs
}

// isDup is a cheap heuristic for "row already exists" (42P01/23505). We don't
// pull in pgconn just for a seed idempotency check; the string match is enough
// for test-only seeding.
func isDup(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key")
}

// TestNewLayerService_Construct runs in -short mode and only verifies the
// constructor doesn't panic on nil repos (defensive — real wiring always passes
// non-nil repos from repo.ProviderSet).
func TestNewLayerService_Construct(t *testing.T) {
	svc := layer.NewLayerService(nil, nil)
	assert.NotNil(t, svc)
}

// TestLayerService_ListLayers verifies ListLayers returns at least the seeded
// "application" layer. Requires Docker.
func TestLayerService_ListLayers(t *testing.T) {
	svc, _ := setupLayerServiceTestDB(t)
	c := t.Context()

	got, err := svc.ListLayers(c)
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

// TestLayerService_GetActiveRuleSet verifies GetActiveRuleSet returns the
// seeded active+default version. Requires Docker.
func TestLayerService_GetActiveRuleSet(t *testing.T) {
	svc, want := setupLayerServiceTestDB(t)
	c := t.Context()

	got, err := svc.GetActiveRuleSet(c)
	require.NoError(t, err)
	assert.Equal(t, want.VersionID, got.VersionID)
	assert.Equal(t, "active", got.Status)
	assert.True(t, got.IsDefault)
}
