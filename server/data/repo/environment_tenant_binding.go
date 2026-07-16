package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// EnvironmentTenantBindingRepo wraps *generated.Queries for
// environment_tenant_bindings operations. The table has no deleted_at; hard-delete
// only. The (env_id, tenant_id, layer_logical_id) triple is the lookup key.
type EnvironmentTenantBindingRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewEnvironmentTenantBindingRepo creates an EnvironmentTenantBindingRepo bound
// to the given pool.
func NewEnvironmentTenantBindingRepo(pool *pgxpool.Pool) *EnvironmentTenantBindingRepo {
	return &EnvironmentTenantBindingRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a binding by ID.
func (r *EnvironmentTenantBindingRepo) GetByID(ctx context.Context, id int64) (generated.EnvironmentTenantBinding, error) {
	return r.queries.GetBinding(ctx, id)
}

// GetByTriple returns the binding for a (env_id, tenant_id, layer_logical_id) triple.
func (r *EnvironmentTenantBindingRepo) GetByTriple(ctx context.Context, arg generated.GetBindingByTripleParams) (generated.EnvironmentTenantBinding, error) {
	return r.queries.GetBindingByTriple(ctx, arg)
}

// ListByEnv returns all bindings for an environment, ordered by created_at.
func (r *EnvironmentTenantBindingRepo) ListByEnv(ctx context.Context, envID int64) ([]generated.EnvironmentTenantBinding, error) {
	return r.queries.ListBindingsByEnv(ctx, envID)
}

// ListByTenant returns all bindings for a tenant, ordered by created_at.
func (r *EnvironmentTenantBindingRepo) ListByTenant(ctx context.Context, tenantID int64) ([]generated.EnvironmentTenantBinding, error) {
	return r.queries.ListBindingsByTenant(ctx, tenantID)
}

// Create creates a new environment-tenant binding.
func (r *EnvironmentTenantBindingRepo) Create(ctx context.Context, arg generated.CreateBindingParams) (generated.EnvironmentTenantBinding, error) {
	return r.queries.CreateBinding(ctx, arg)
}

// Delete hard-deletes a binding by ID.
func (r *EnvironmentTenantBindingRepo) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteBinding(ctx, id)
}
