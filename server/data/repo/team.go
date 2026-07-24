package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TeamRepo wraps *generated.Queries for team-specific operations.
// teams is soft-delete aware (deleted_at IS NULL filters in SQL).
type TeamRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewTeamRepo creates a TeamRepo bound to the given pool.
func NewTeamRepo(pool *pgxpool.Pool) *TeamRepo {
	return &TeamRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active team by ID.
func (r *TeamRepo) GetByID(ctx context.Context, id int64) (generated.Team, error) {
	return r.queries.GetTeam(ctx, id)
}

// GetBySlug returns the active team by slug.
func (r *TeamRepo) GetBySlug(ctx context.Context, slug string) (generated.Team, error) {
	return r.queries.GetTeamBySlug(ctx, slug)
}

// List returns all active teams ordered by name.
func (r *TeamRepo) List(ctx context.Context) ([]generated.Team, error) {
	return r.queries.ListTeams(ctx)
}

// ListByKind returns active teams of a given kind (e.g. "dba", "platform").
func (r *TeamRepo) ListByKind(ctx context.Context, kind string) ([]generated.Team, error) {
	return r.queries.ListTeamsByKind(ctx, kind)
}

// Create creates a new team.
func (r *TeamRepo) Create(ctx context.Context, arg generated.CreateTeamParams) (generated.Team, error) {
	return r.queries.CreateTeam(ctx, arg)
}

// Update updates a team's mutable fields (name, tags_json, policy_json).
func (r *TeamRepo) Update(ctx context.Context, arg generated.UpdateTeamParams) (generated.Team, error) {
	return r.queries.UpdateTeam(ctx, arg)
}

// SoftDelete soft-deletes a team (sets deleted_at, status='deprecated').
func (r *TeamRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteTeam(ctx, id)
}

// ListByDynamicFilter runs a dynamic query built by QueryWrapper.
// Used for ad-hoc filtering that sqlc can't express well (IN-lists, multi-filter,
// pagination) on top of the soft-delete-aware base query.
func (r *TeamRepo) ListByDynamicFilter(ctx context.Context, w *QueryWrapper) ([]generated.Team, error) {
	sql, args, err := w.BuildSQL("SELECT id, name, slug, kind, status, tags_json, policy_json, created_at, updated_at, deleted_at FROM teams WHERE deleted_at IS NULL")
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.Team])
}
