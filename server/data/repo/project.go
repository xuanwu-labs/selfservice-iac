package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ProjectRepo wraps *generated.Queries for project-specific operations.
// projects is soft-delete aware and has only id/name/team_id/timestamps.
type ProjectRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewProjectRepo creates a ProjectRepo bound to the given pool.
func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the active project by ID.
func (r *ProjectRepo) GetByID(ctx context.Context, id int64) (generated.Project, error) {
	return r.queries.GetProject(ctx, id)
}

// GetByName returns the active project by name.
func (r *ProjectRepo) GetByName(ctx context.Context, name string) (generated.Project, error) {
	return r.queries.GetProjectByName(ctx, name)
}

// List returns all active projects ordered by name.
func (r *ProjectRepo) List(ctx context.Context) ([]generated.Project, error) {
	return r.queries.ListProjects(ctx)
}

// ListByTeam returns active projects owned by a team.
func (r *ProjectRepo) ListByTeam(ctx context.Context, teamID int64) ([]generated.Project, error) {
	return r.queries.ListProjectsByTeam(ctx, teamID)
}

// Create creates a new project.
func (r *ProjectRepo) Create(ctx context.Context, arg generated.CreateProjectParams) (generated.Project, error) {
	return r.queries.CreateProject(ctx, arg)
}

// Update updates a project's name.
func (r *ProjectRepo) Update(ctx context.Context, arg generated.UpdateProjectParams) (generated.Project, error) {
	return r.queries.UpdateProject(ctx, arg)
}

// SoftDelete soft-deletes a project (sets deleted_at).
func (r *ProjectRepo) SoftDelete(ctx context.Context, id int64) error {
	return r.queries.SoftDeleteProject(ctx, id)
}
