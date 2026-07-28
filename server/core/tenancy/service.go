// Package tenancy: service.go — TenancyService orchestrates team/project/space
// lifecycle (W1-04 tasks 1.1-1.3).
//
// TenancyService is a thin façade over the repo layer: it forwards each call to
// the corresponding TeamRepo / ProjectRepo / SpaceRepo method without adding
// business logic. Phase 1 deliberately keeps the service anemic so that
// ownership/authorization rules (see ownership.go) can be layered in at the
// handler boundary. Phase 2 will add cross-table transaction orchestration
// (e.g. creating a team + default tenant atomically) here.
package tenancy

import (
	"context"

	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// TenancyService exposes team/project/space operations. Repos are concrete struct
// pointers (ferret style, same as CatalogService); tests inject fakes by defining
// a small interface at the call site — no change needed here.
type TenancyService struct {
	teamRepo    *repo.TeamRepo
	projectRepo *repo.ProjectRepo
	spaceRepo   *repo.SpaceRepo
}

// NewTenancyService constructs a TenancyService from its repo dependencies.
// The repos themselves are provided by repo.ProviderSet (not re-declared here).
func NewTenancyService(
	teamRepo *repo.TeamRepo,
	projectRepo *repo.ProjectRepo,
	spaceRepo *repo.SpaceRepo,
) *TenancyService {
	return &TenancyService{
		teamRepo:    teamRepo,
		projectRepo: projectRepo,
		spaceRepo:   spaceRepo,
	}
}

// -----------------------------------------------------------------------------
// Team operations
// -----------------------------------------------------------------------------

// CreateTeam creates a new team (status, tags_json, policy_json set by caller).
func (s *TenancyService) CreateTeam(ctx context.Context, arg generated.CreateTeamParams) (generated.Team, error) {
	return s.teamRepo.Create(ctx, arg)
}

// GetTeam returns the active team by ID.
func (s *TenancyService) GetTeam(ctx context.Context, id int64) (generated.Team, error) {
	return s.teamRepo.GetByID(ctx, id)
}

// GetTeamBySlug returns the active team by slug.
func (s *TenancyService) GetTeamBySlug(ctx context.Context, slug string) (generated.Team, error) {
	return s.teamRepo.GetBySlug(ctx, slug)
}

// ListTeams returns all active teams.
func (s *TenancyService) ListTeams(ctx context.Context) ([]generated.Team, error) {
	return s.teamRepo.List(ctx)
}

// ListTeamsByKind returns active teams of the given kind (e.g. "dba", "platform").
func (s *TenancyService) ListTeamsByKind(ctx context.Context, kind string) ([]generated.Team, error) {
	return s.teamRepo.ListByKind(ctx, kind)
}

// UpdateTeam updates a team's mutable fields (name, tags_json, policy_json).
func (s *TenancyService) UpdateTeam(ctx context.Context, arg generated.UpdateTeamParams) (generated.Team, error) {
	return s.teamRepo.Update(ctx, arg)
}

// SoftDeleteTeam soft-deletes a team (sets deleted_at, status='deprecated').
func (s *TenancyService) SoftDeleteTeam(ctx context.Context, id int64) error {
	return s.teamRepo.SoftDelete(ctx, id)
}

// -----------------------------------------------------------------------------
// Project operations
// -----------------------------------------------------------------------------

// CreateProject creates a new project owned by a team.
func (s *TenancyService) CreateProject(ctx context.Context, arg generated.CreateProjectParams) (generated.Project, error) {
	return s.projectRepo.Create(ctx, arg)
}

// GetProject returns the active project by ID.
func (s *TenancyService) GetProject(ctx context.Context, id int64) (generated.Project, error) {
	return s.projectRepo.GetByID(ctx, id)
}

// ListProjects returns all active projects.
func (s *TenancyService) ListProjects(ctx context.Context) ([]generated.Project, error) {
	return s.projectRepo.List(ctx)
}

// ListProjectsByTeam returns active projects owned by a team.
func (s *TenancyService) ListProjectsByTeam(ctx context.Context, teamID int64) ([]generated.Project, error) {
	return s.projectRepo.ListByTeam(ctx, teamID)
}

// UpdateProject updates a project's name.
func (s *TenancyService) UpdateProject(ctx context.Context, arg generated.UpdateProjectParams) (generated.Project, error) {
	return s.projectRepo.Update(ctx, arg)
}

// -----------------------------------------------------------------------------
// Space operations
// -----------------------------------------------------------------------------

// CreateSpace creates a new space belonging to a project.
func (s *TenancyService) CreateSpace(ctx context.Context, arg generated.CreateSpaceParams) (generated.Space, error) {
	return s.spaceRepo.Create(ctx, arg)
}

// GetSpace returns the active space by ID.
func (s *TenancyService) GetSpace(ctx context.Context, id int64) (generated.Space, error) {
	return s.spaceRepo.GetByID(ctx, id)
}

// ListSpaces returns all active spaces.
func (s *TenancyService) ListSpaces(ctx context.Context) ([]generated.Space, error) {
	return s.spaceRepo.List(ctx)
}

// ListSpacesByProject returns active spaces belonging to a project.
func (s *TenancyService) ListSpacesByProject(ctx context.Context, projectID int64) ([]generated.Space, error) {
	return s.spaceRepo.ListByProject(ctx, projectID)
}

// UpdateSpace updates a space's mutable fields.
func (s *TenancyService) UpdateSpace(ctx context.Context, arg generated.UpdateSpaceParams) (generated.Space, error) {
	return s.spaceRepo.Update(ctx, arg)
}

// SoftDeleteSpace soft-deletes a space (sets deleted_at).
func (s *TenancyService) SoftDeleteSpace(ctx context.Context, id int64) error {
	return s.spaceRepo.SoftDelete(ctx, id)
}

// ProviderSet wires TenancyService for dependency injection. The repo
// dependencies come from repo.ProviderSet; we only register the constructor.
var ProviderSet = wire.NewSet(
	NewTenancyService,
)
