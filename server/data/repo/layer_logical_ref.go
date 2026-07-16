package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// LayerLogicalRefRepo wraps *generated.Queries for layer_logical_refs operations.
// The PK is logical_id TEXT (NOT snowflake), passed by caller. The table has no
// deleted_at; hard-lifecycle only.
type LayerLogicalRefRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewLayerLogicalRefRepo creates a LayerLogicalRefRepo bound to the given pool.
func NewLayerLogicalRefRepo(pool *pgxpool.Pool) *LayerLogicalRefRepo {
	return &LayerLogicalRefRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a layer logical ref by its logical_id.
func (r *LayerLogicalRefRepo) GetByID(ctx context.Context, logicalID string) (generated.LayerLogicalRef, error) {
	return r.queries.GetLayerLogicalRef(ctx, logicalID)
}

// List returns all layer logical refs ordered by created_at.
func (r *LayerLogicalRefRepo) List(ctx context.Context) ([]generated.LayerLogicalRef, error) {
	return r.queries.ListLayerLogicalRefs(ctx)
}

// Create creates a new layer logical ref.
func (r *LayerLogicalRefRepo) Create(ctx context.Context, arg generated.CreateLayerLogicalRefParams) (generated.LayerLogicalRef, error) {
	return r.queries.CreateLayerLogicalRef(ctx, arg)
}

// UpdateDisplayName updates the current_display_name for a layer logical ref.
func (r *LayerLogicalRefRepo) UpdateDisplayName(ctx context.Context, arg generated.UpdateLayerLogicalRefDisplayNameParams) (generated.LayerLogicalRef, error) {
	return r.queries.UpdateLayerLogicalRefDisplayName(ctx, arg)
}
