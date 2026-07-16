package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// SpaceRepo wraps *generated.Queries for space-specific operations.
// spaces is soft-delete aware (deleted_at IS NULL filters in SQL).
type SpaceRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewSpaceRepo creates a SpaceRepo bound to the given pool.
func NewSpaceRepo(pool *pgxpool.Pool) *SpaceRepo {
	return &SpaceRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active space by ID.
func (r *SpaceRepo) GetByID(ctx context.Context, id int64) (generated.Space, error) {
	return r.queries.GetSpace(ctx, id)
}

// List returns all active spaces ordered by name.
func (r *SpaceRepo) List(ctx context.Context) ([]generated.Space, error) {
	return r.queries.ListSpaces(ctx)
}

// ListByProject returns active spaces belonging to a project.
func (r *SpaceRepo) ListByProject(ctx context.Context, projectID int64) ([]generated.Space, error) {
	return r.queries.ListSpacesByProject(ctx, projectID)
}

// ListByLayer returns active spaces assigned to a logical layer.
func (r *SpaceRepo) ListByLayer(ctx context.Context, layerLogicalID *string) ([]generated.Space, error) {
	return r.queries.ListSpacesByLayer(ctx, layerLogicalID)
}

// Create creates a new space.
func (r *SpaceRepo) Create(ctx context.Context, arg generated.CreateSpaceParams) (generated.Space, error) {
	return r.queries.CreateSpace(ctx, arg)
}

// Update updates a space's mutable fields.
func (r *SpaceRepo) Update(ctx context.Context, arg generated.UpdateSpaceParams) (generated.Space, error) {
	return r.queries.UpdateSpace(ctx, arg)
}

// SoftDelete soft-deletes a space (sets deleted_at).
func (r *SpaceRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteSpace(ctx, id)
}

// ListByDynamicFilter runs a dynamic query built by QueryWrapper.
// Used for ad-hoc filtering that sqlc can't express well (IN-lists, multi-filter,
// pagination) on top of the soft-delete-aware base query.
func (r *SpaceRepo) ListByDynamicFilter(ctx context.Context, w *QueryWrapper) ([]generated.Space, error) {
	sql, args, err := w.BuildSQL("SELECT id, name, project_id, layer_logical_id, repo_path, tags_json, created_at, updated_at, deleted_at FROM spaces WHERE deleted_at IS NULL")
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.Space])
}
