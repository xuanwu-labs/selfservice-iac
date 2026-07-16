package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ModuleRepo wraps *generated.Queries for module-specific operations.
// modules uses status-based lifecycle (no deleted_at): status tracks state.
type ModuleRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewModuleRepo creates a ModuleRepo bound to the given pool.
func NewModuleRepo(pool *pgxpool.Pool) *ModuleRepo {
	return &ModuleRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns the module by ID.
func (r *ModuleRepo) GetByID(ctx context.Context, id int64) (generated.Module, error) {
	return r.queries.GetModule(ctx, id)
}

// GetByGitSource returns the module matching a git source URL.
func (r *ModuleRepo) GetByGitSource(ctx context.Context, gitSource string) (generated.Module, error) {
	return r.queries.GetModuleByGitSource(ctx, gitSource)
}

// List returns all modules ordered by name.
func (r *ModuleRepo) List(ctx context.Context) ([]generated.Module, error) {
	return r.queries.ListModules(ctx)
}

// ListByLayer returns modules in a given layer.
func (r *ModuleRepo) ListByLayer(ctx context.Context, layer string) ([]generated.Module, error) {
	return r.queries.ListModulesByLayer(ctx, layer)
}

// ListByOwner returns modules owned by a team.
func (r *ModuleRepo) ListByOwner(ctx context.Context, ownerTeamID int64) ([]generated.Module, error) {
	return r.queries.ListModulesByOwner(ctx, ownerTeamID)
}

// Create creates a new module.
func (r *ModuleRepo) Create(ctx context.Context, arg generated.CreateModuleParams) (generated.Module, error) {
	return r.queries.CreateModule(ctx, arg)
}

// Update updates a module's mutable fields.
func (r *ModuleRepo) Update(ctx context.Context, arg generated.UpdateModuleParams) (generated.Module, error) {
	return r.queries.UpdateModule(ctx, arg)
}

// UpdateStatus updates a module's lifecycle status.
func (r *ModuleRepo) UpdateStatus(ctx context.Context, arg generated.UpdateModuleStatusParams) (generated.Module, error) {
	return r.queries.UpdateModuleStatus(ctx, arg)
}

// CreateWithVersion creates a module and its first version in a single
// transaction (cross-table atomic write). On success both rows are committed;
// on any error the transaction is rolled back. This is the canonical example
// of the W1-02 D3 transaction pattern (pool.Begin + WithTx + Commit).
func (r *ModuleRepo) CreateWithVersion(
	ctx context.Context,
	modArg generated.CreateModuleParams,
	verArg generated.CreateModuleVersionParams,
) (generated.Module, generated.ModuleVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return generated.Module{}, generated.ModuleVersion{}, err
	}
	// Defer rollback: no-op after Commit succeeds (returns ErrTxClosed, ignored).
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := r.queries.WithTx(tx)

	mod, err := txQueries.CreateModule(ctx, modArg)
	if err != nil {
		return generated.Module{}, generated.ModuleVersion{}, err
	}
	// Ensure the version points at the freshly created module.
	verArg.ModuleID = mod.ID
	ver, err := txQueries.CreateModuleVersion(ctx, verArg)
	if err != nil {
		return generated.Module{}, generated.ModuleVersion{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return generated.Module{}, generated.ModuleVersion{}, err
	}
	return mod, ver, nil
}

// ListByDynamicFilter runs a dynamic query built by QueryWrapper.
// Used for ad-hoc filtering that sqlc can't express well (IN-lists, multi-filter,
// pagination). Note: modules has no deleted_at; status-based lifecycle only.
func (r *ModuleRepo) ListByDynamicFilter(ctx context.Context, w *QueryWrapper) ([]generated.Module, error) {
	sql, args, err := w.BuildSQL("SELECT id, name, git_source, provider, layer, owner_team_id, status, description, created_at, updated_at FROM modules")
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.Module])
}
