package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// CatalogRepo wraps *generated.Queries for catalog_items operations.
// The generated model is generated.CatalogItem; catalog_items is soft-delete
// aware (deleted_at IS NULL filters in SQL).
type CatalogRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewCatalogRepo creates a CatalogRepo bound to the given pool.
func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo {
	return &CatalogRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active catalog item by ID.
func (r *CatalogRepo) GetByID(ctx context.Context, id int64) (generated.CatalogItem, error) {
	return r.queries.GetCatalogItem(ctx, id)
}

// List returns all active catalog items ordered by display_name.
func (r *CatalogRepo) List(ctx context.Context) ([]generated.CatalogItem, error) {
	return r.queries.ListCatalogItems(ctx)
}

// ListByLayer returns active catalog items in a given logical layer.
func (r *CatalogRepo) ListByLayer(ctx context.Context, layerLogicalID *string) ([]generated.CatalogItem, error) {
	return r.queries.ListCatalogItemsByLayer(ctx, layerLogicalID)
}

// ListByOwner returns active catalog items owned by a team.
func (r *CatalogRepo) ListByOwner(ctx context.Context, ownerTeamID int64) ([]generated.CatalogItem, error) {
	return r.queries.ListCatalogItemsByOwner(ctx, ownerTeamID)
}

// ListVisible returns active catalog items whose visibility_json contains the
// given JSONB document (Postgres @> containment). Pass the caller identity as
// a JSONB blob, e.g. []byte(`{"teams":[123]}`).
func (r *CatalogRepo) ListVisible(ctx context.Context, visibilityFilter []byte) ([]generated.CatalogItem, error) {
	return r.queries.ListVisibleCatalogItems(ctx, visibilityFilter)
}

// Publish creates (publishes) a new catalog item.
func (r *CatalogRepo) Publish(ctx context.Context, arg generated.PublishCatalogItemParams) (generated.CatalogItem, error) {
	return r.queries.PublishCatalogItem(ctx, arg)
}

// Update updates a catalog item's mutable fields.
func (r *CatalogRepo) Update(ctx context.Context, arg generated.UpdateCatalogItemParams) (generated.CatalogItem, error) {
	return r.queries.UpdateCatalogItem(ctx, arg)
}

// SoftDelete soft-deletes a catalog item (sets deleted_at).
func (r *CatalogRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteCatalogItem(ctx, id)
}

// ListByDynamicFilter is the canonical example of a dynamic multi-filter query
// (W1-02 D4). It is used for ad-hoc filtering that sqlc can't express well —
// e.g. "items in layers [a,b] AND owned by team 5 AND status='published'",
// possibly with pagination:
//
//	w := data.New().
//	    In("layer_logical_id", "db", "mw").
//	    Eq("owner_team_id", int64(5)).
//	    Eq("status", "published").
//	    OrderByDesc("display_name").
//	    Page(1, 20)
//	items, err := catalogRepo.ListByDynamicFilter(ctx, w)
//
// The base query carries the soft-delete filter (deleted_at IS NULL); the
// wrapper appends its conditions as AND.
func (r *CatalogRepo) ListByDynamicFilter(ctx context.Context, w *QueryWrapper) ([]generated.CatalogItem, error) {
	base := "SELECT id, module_version_id, display_name, description, category, status, form_schema_json, defaults_json, cardinality, instance_key, per_instance_fields_json, shared_fields_json, layer_logical_id, stack_grouping, owner_team_id, default_tags_json, user_allowed_tag_keys_json, visibility_json, created_at, updated_at, deleted_at FROM catalog_items WHERE deleted_at IS NULL"
	sql, args, err := w.BuildSQL(base)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.CatalogItem])
}
