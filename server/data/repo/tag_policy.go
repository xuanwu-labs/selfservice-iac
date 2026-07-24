package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TagPolicyRepo wraps *generated.Queries for tag_policies operations.
// tag_policies is soft-delete aware (deleted_at IS NULL filters in SQL). A
// policy is scoped by (scope_type, scope_id) — e.g. ("team", "123").
type TagPolicyRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewTagPolicyRepo creates a TagPolicyRepo bound to the given pool.
func NewTagPolicyRepo(pool *pgxpool.Pool) *TagPolicyRepo {
	return &TagPolicyRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active tag policy by ID.
func (r *TagPolicyRepo) GetByID(ctx context.Context, id int64) (generated.TagPolicy, error) {
	return r.queries.GetTagPolicy(ctx, id)
}

// GetByScope returns the active tag policy for a (scope_type, scope_id) scope.
func (r *TagPolicyRepo) GetByScope(ctx context.Context, arg generated.GetTagPolicyByScopeParams) (generated.TagPolicy, error) {
	return r.queries.GetTagPolicyByScope(ctx, arg)
}

// ListByScopeType returns active tag policies of a given scope type, ordered by
// created_at.
func (r *TagPolicyRepo) ListByScopeType(ctx context.Context, scopeType string) ([]generated.TagPolicy, error) {
	return r.queries.ListTagPoliciesByScopeType(ctx, scopeType)
}

// Create creates a new tag policy.
func (r *TagPolicyRepo) Create(ctx context.Context, arg generated.CreateTagPolicyParams) (generated.TagPolicy, error) {
	return r.queries.CreateTagPolicy(ctx, arg)
}

// Update updates a tag policy's mutable fields (incl. optimistic version bump).
func (r *TagPolicyRepo) Update(ctx context.Context, arg generated.UpdateTagPolicyParams) (generated.TagPolicy, error) {
	return r.queries.UpdateTagPolicy(ctx, arg)
}

// SoftDelete soft-deletes a tag policy (sets deleted_at).
func (r *TagPolicyRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteTagPolicy(ctx, id)
}
