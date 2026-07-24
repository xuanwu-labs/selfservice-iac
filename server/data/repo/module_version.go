package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ModuleVersionRepo wraps *generated.Queries for module_version operations.
// module_versions has no deleted_at; is_current marks the active version per module.
type ModuleVersionRepo struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewModuleVersionRepo creates a ModuleVersionRepo bound to the given pool.
func NewModuleVersionRepo(pool *pgxpool.Pool) *ModuleVersionRepo {
	return &ModuleVersionRepo{pool: pool, queries: generated.New(pool)}
}

// GetByID returns a module version by ID.
func (r *ModuleVersionRepo) GetByID(ctx context.Context, id int64) (generated.ModuleVersion, error) {
	return r.queries.GetModuleVersion(ctx, id)
}

// GetByRef returns a version by (module_id, version) ref.
func (r *ModuleVersionRepo) GetByRef(ctx context.Context, arg generated.GetModuleVersionByRefParams) (generated.ModuleVersion, error) {
	return r.queries.GetModuleVersionByRef(ctx, arg)
}

// GetCurrent returns the is_current version of a module.
func (r *ModuleVersionRepo) GetCurrent(ctx context.Context, moduleID int64) (generated.ModuleVersion, error) {
	return r.queries.GetCurrentModuleVersion(ctx, moduleID)
}

// List returns all versions of a module, newest first.
func (r *ModuleVersionRepo) List(ctx context.Context, moduleID int64) ([]generated.ModuleVersion, error) {
	return r.queries.ListModuleVersions(ctx, moduleID)
}

// Create registers a new module version.
func (r *ModuleVersionRepo) Create(ctx context.Context, arg generated.CreateModuleVersionParams) (generated.ModuleVersion, error) {
	return r.queries.CreateModuleVersion(ctx, arg)
}

// SetCurrent atomically makes id the is_current version for its module.
// It runs UnsetOtherCurrentVersions (clears the prior current for the module)
// and SetCurrentModuleVersion (flags id) in a single transaction so that at
// most one version is current at any time (W1-02 D3 transaction pattern).
// The module_id is resolved from the target version row.
func (r *ModuleVersionRepo) SetCurrent(ctx context.Context, id int64) (generated.ModuleVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return generated.ModuleVersion{}, err
	}
	// Defer rollback: no-op after Commit succeeds.
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := r.queries.WithTx(tx)

	// Resolve module_id from the target version so we know which module's
	// other current flags to clear.
	target, err := txQueries.GetModuleVersion(ctx, id)
	if err != nil {
		return generated.ModuleVersion{}, err
	}
	if err := txQueries.UnsetOtherCurrentVersions(ctx, target.ModuleID); err != nil {
		return generated.ModuleVersion{}, err
	}
	updated, err := txQueries.SetCurrentModuleVersion(ctx, id)
	if err != nil {
		return generated.ModuleVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return generated.ModuleVersion{}, err
	}
	return updated, nil
}
