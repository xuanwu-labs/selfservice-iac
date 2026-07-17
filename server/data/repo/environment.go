package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// EnvironmentRepo wraps *generated.Queries for environments operations.
// environments is soft-delete aware (deleted_at IS NULL filters in SQL).
type EnvironmentRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewEnvironmentRepo creates an EnvironmentRepo bound to the given pool.
func NewEnvironmentRepo(pool *pgxpool.Pool) *EnvironmentRepo {
	return &EnvironmentRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active environment by ID.
func (r *EnvironmentRepo) GetByID(ctx context.Context, id int64) (generated.Environment, error) {
	return r.queries.GetEnvironment(ctx, id)
}

// GetByLogicalID returns the active environment by its logical id.
func (r *EnvironmentRepo) GetByLogicalID(ctx context.Context, envLogicalID string) (generated.Environment, error) {
	return r.queries.GetEnvironmentByLogicalId(ctx, envLogicalID)
}

// List returns all active environments ordered by created_at.
func (r *EnvironmentRepo) List(ctx context.Context) ([]generated.Environment, error) {
	return r.queries.ListEnvironments(ctx)
}

// Create creates a new environment.
func (r *EnvironmentRepo) Create(ctx context.Context, arg generated.CreateEnvironmentParams) (generated.Environment, error) {
	return r.queries.CreateEnvironment(ctx, arg)
}

// Update updates an environment's mutable fields.
func (r *EnvironmentRepo) Update(ctx context.Context, arg generated.UpdateEnvironmentParams) (generated.Environment, error) {
	return r.queries.UpdateEnvironment(ctx, arg)
}

// SoftDelete soft-deletes an environment (sets deleted_at).
func (r *EnvironmentRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteEnvironment(ctx, id)
}
