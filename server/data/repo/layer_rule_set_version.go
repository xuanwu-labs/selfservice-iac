package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// LayerRuleSetVersionRepo wraps *generated.Queries for layer_rule_set_versions
// operations. The PK is version_id INTEGER (NOT snowflake). The table is
// versioned-lifecycle: it uses status/superseded_at instead of deleted_at.
type LayerRuleSetVersionRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewLayerRuleSetVersionRepo creates a LayerRuleSetVersionRepo bound to the given pool.
func NewLayerRuleSetVersionRepo(pool *pgxpool.Pool) *LayerRuleSetVersionRepo {
	return &LayerRuleSetVersionRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a rule-set version by version_id.
func (r *LayerRuleSetVersionRepo) GetByID(ctx context.Context, versionID int32) (generated.LayerRuleSetVersion, error) {
	return r.queries.GetRuleSetVersion(ctx, versionID)
}

// GetActive returns the single active rule-set version (status='active' AND
// is_default=true).
func (r *LayerRuleSetVersionRepo) GetActive(ctx context.Context) (generated.LayerRuleSetVersion, error) {
	return r.queries.GetActiveRuleSetVersion(ctx)
}

// GetDefault returns the is_default rule-set version.
func (r *LayerRuleSetVersionRepo) GetDefault(ctx context.Context) (generated.LayerRuleSetVersion, error) {
	return r.queries.GetDefaultRuleSetVersion(ctx)
}

// List returns all rule-set versions, newest version_id first.
func (r *LayerRuleSetVersionRepo) List(ctx context.Context) ([]generated.LayerRuleSetVersion, error) {
	return r.queries.ListRuleSetVersions(ctx)
}

// Create creates a new rule-set version.
func (r *LayerRuleSetVersionRepo) Create(ctx context.Context, arg generated.CreateRuleSetVersionParams) (generated.LayerRuleSetVersion, error) {
	return r.queries.CreateRuleSetVersion(ctx, arg)
}

// Supersede marks an existing rule-set version as superseded, recording the
// version that replaced it. The generated query is a single :exec UPDATE, so no
// transaction wrapper is needed here. Callers that swap default/active flags as
// part of a larger cutover should wrap this in their own transaction.
func (r *LayerRuleSetVersionRepo) Supersede(ctx context.Context, arg generated.SupersedeRuleSetVersionParams) error {
	return r.queries.SupersedeRuleSetVersion(ctx, arg)
}
