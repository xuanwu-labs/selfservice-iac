package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TenantRepo wraps *generated.Queries for tenants operations.
// tenants is soft-delete aware (deleted_at IS NULL filters in SQL).
type TenantRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewTenantRepo creates a TenantRepo bound to the given pool.
func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo {
	return &TenantRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active tenant by ID.
func (r *TenantRepo) GetByID(ctx context.Context, id int64) (generated.Tenant, error) {
	return r.queries.GetTenant(ctx, id)
}

// GetByLogicalID returns the active tenant by its logical id.
func (r *TenantRepo) GetByLogicalID(ctx context.Context, tenantLogicalID string) (generated.Tenant, error) {
	return r.queries.GetTenantByLogicalId(ctx, tenantLogicalID)
}

// List returns all active tenants ordered by created_at.
func (r *TenantRepo) List(ctx context.Context) ([]generated.Tenant, error) {
	return r.queries.ListTenants(ctx)
}

// Create creates a new tenant.
func (r *TenantRepo) Create(ctx context.Context, arg generated.CreateTenantParams) (generated.Tenant, error) {
	return r.queries.CreateTenant(ctx, arg)
}

// Update updates a tenant's mutable fields.
func (r *TenantRepo) Update(ctx context.Context, arg generated.UpdateTenantParams) (generated.Tenant, error) {
	return r.queries.UpdateTenant(ctx, arg)
}

// SoftDelete soft-deletes a tenant (sets deleted_at).
func (r *TenantRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteTenant(ctx, id)
}
