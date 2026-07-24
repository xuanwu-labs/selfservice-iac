package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ModuleDependencyRepo wraps *generated.Queries for module_dependency operations.
// module_dependencies has no deleted_at; cascade-deleted with the parent
// module_version (FK ON DELETE CASCADE).
type ModuleDependencyRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewModuleDependencyRepo creates a ModuleDependencyRepo bound to the given pool.
func NewModuleDependencyRepo(pool *pgxpool.Pool) *ModuleDependencyRepo {
	return &ModuleDependencyRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a module dependency by ID.
func (r *ModuleDependencyRepo) GetByID(ctx context.Context, id int64) (generated.ModuleDependency, error) {
	return r.queries.GetModuleDependency(ctx, id)
}

// ListByVersion returns all dependencies declared by a module version,
// ordered by variable_name.
func (r *ModuleDependencyRepo) ListByVersion(ctx context.Context, moduleVersionID int64) ([]generated.ModuleDependency, error) {
	return r.queries.ListDependenciesByVersion(ctx, moduleVersionID)
}

// Create creates a new module dependency edge.
func (r *ModuleDependencyRepo) Create(ctx context.Context, arg generated.CreateModuleDependencyParams) (generated.ModuleDependency, error) {
	return r.queries.CreateModuleDependency(ctx, arg)
}
