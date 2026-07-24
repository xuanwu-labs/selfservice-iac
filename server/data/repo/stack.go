package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// StackRepo wraps *generated.Queries for stack operations.
// stacks has no deleted_at; stacks are immutable once applied and migration_status
// tracks lifecycle.
type StackRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewStackRepo creates a StackRepo bound to the given pool.
func NewStackRepo(pool *pgxpool.Pool) *StackRepo {
	return &StackRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a stack by ID.
func (r *StackRepo) GetByID(ctx context.Context, id int64) (generated.Stack, error) {
	return r.queries.GetStack(ctx, id)
}

// GetByRepoPath returns a stack by its repo_path (unique).
func (r *StackRepo) GetByRepoPath(ctx context.Context, repoPath string) (generated.Stack, error) {
	return r.queries.GetStackByRepoPath(ctx, repoPath)
}

// List returns all stacks, newest first.
func (r *StackRepo) List(ctx context.Context) ([]generated.Stack, error) {
	return r.queries.ListStacks(ctx)
}

// ListBySpace returns stacks attached to a space.
func (r *StackRepo) ListBySpace(ctx context.Context, spaceID *int64) ([]generated.Stack, error) {
	return r.queries.ListStacksBySpace(ctx, spaceID)
}

// ListByLayer returns stacks in a given layer.
func (r *StackRepo) ListByLayer(ctx context.Context, layer string) ([]generated.Stack, error) {
	return r.queries.ListStacksByLayer(ctx, layer)
}

// ListByEnv returns stacks in a given environment.
func (r *StackRepo) ListByEnv(ctx context.Context, env string) ([]generated.Stack, error) {
	return r.queries.ListStacksByEnv(ctx, env)
}

// Create creates a new stack.
func (r *StackRepo) Create(ctx context.Context, arg generated.CreateStackParams) (generated.Stack, error) {
	return r.queries.CreateStack(ctx, arg)
}

// Update updates a stack's mutable fields (incl. optimistic version bump).
func (r *StackRepo) Update(ctx context.Context, arg generated.UpdateStackParams) (generated.Stack, error) {
	return r.queries.UpdateStack(ctx, arg)
}

// ListByDynamicFilter runs a dynamic query built by QueryWrapper.
// Used for ad-hoc filtering that sqlc can't express well (IN-lists, multi-filter,
// pagination). Note: stacks has no deleted_at.
func (r *StackRepo) ListByDynamicFilter(ctx context.Context, w *QueryWrapper) ([]generated.Stack, error) {
	base := "SELECT id, space_id, catalog_item_id, layer_logical_id, layer_rule_set_version_id, owner_team_id, layer, component, env, tenant_id, stack_id, repo_path, state_key, terramate_tags_json, state_backend_id, pinned_commit, migration_status, sunset_deadline, version, created_at, updated_at FROM stacks"
	sql, args, err := w.BuildSQL(base)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.Stack])
}
