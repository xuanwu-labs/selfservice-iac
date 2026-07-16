package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// StackDependencyRepo wraps *generated.Queries for stack_dependencies operations.
// stack_dependencies has no deleted_at; hard-delete only.
type StackDependencyRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewStackDependencyRepo creates a StackDependencyRepo bound to the given pool.
func NewStackDependencyRepo(pool *pgxpool.Pool) *StackDependencyRepo {
	return &StackDependencyRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a stack dependency edge by ID.
func (r *StackDependencyRepo) GetByID(ctx context.Context, id int64) (generated.StackDependency, error) {
	return r.queries.GetStackDependency(ctx, id)
}

// ListByStack returns the dependencies declared BY a stack (outgoing edges,
// from_stack_id = stackID), ordered by created_at.
func (r *StackDependencyRepo) ListByStack(ctx context.Context, fromStackID int64) ([]generated.StackDependency, error) {
	return r.queries.ListDependenciesByStack(ctx, fromStackID)
}

// ListDependents returns the stacks that depend ON a stack (incoming edges,
// to_stack_id = stackID), ordered by created_at.
func (r *StackDependencyRepo) ListDependents(ctx context.Context, toStackID int64) ([]generated.StackDependency, error) {
	return r.queries.ListDependentsByStack(ctx, toStackID)
}

// Create creates a new stack dependency edge.
func (r *StackDependencyRepo) Create(ctx context.Context, arg generated.CreateStackDependencyParams) (generated.StackDependency, error) {
	return r.queries.CreateStackDependency(ctx, arg)
}

// Delete hard-deletes a stack dependency edge by ID.
func (r *StackDependencyRepo) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteStackDependency(ctx, id)
}
