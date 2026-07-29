// Package connect implements Connect-RPC handlers for Aether's business APIs.
//
// registry.go implements RegistryAdminServiceHandler, wiring proto requests to
// core/registry.RegistryService. This is the Admin-layer surface (requires
// admin role, enforced by Connect interceptor — see api/connect/middleware).

package connect

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
	registryv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/registry"
	registryv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/registry/registryv1connect"
)

// RegistryHandler implements the RegistryAdminService Connect RPC.
//
// The service field is used by RegisterModule (which drives clone+extract+
// persist through RegistryService). The pool is used for the read/list paths
// (ListModules/GetModule) and for DeprecateModule, which all touch the modules
// table directly. Raw SQL is used for the reads because the sqlc-generated
// Module struct does not include the module_path column (the live schema added
// module_path in migration 003 after sqlc was generated; regenerating sqlc is
// out of scope for this MVP integration fix).
type RegistryHandler struct {
	registryv1connect.UnimplementedRegistryAdminServiceHandler
	svc  *registry.RegistryService
	pool *pgxpool.Pool
}

// Compile-time check.
var _ registryv1connect.RegistryAdminServiceHandler = (*RegistryHandler)(nil)

// NewRegistryHandler returns a RegistryHandler ready to register on a mux.
// The pool is required for ListModules/GetModule/DeprecateModule; pass nil only
// in unit tests that exercise RegisterModule exclusively.
func NewRegistryHandler(svc *registry.RegistryService, pool *pgxpool.Pool) *RegistryHandler {
	return &RegistryHandler{svc: svc, pool: pool}
}

// moduleRow mirrors the modules table columns we surface in the proto Module
// message. It intentionally includes module_path, which the sqlc-generated
// Module struct omits. created_at / updated_at are NOT NULL in the schema, so
// plain time.Time works with pgx.RowToStructByName.
type moduleRow struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	GitSource   string    `db:"git_source"`
	ModulePath  string    `db:"module_path"`
	Provider    string    `db:"provider"`
	Status      string    `db:"status"`
	Description string    `db:"description"`
	OwnerTeamID int64     `db:"owner_team_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// moduleSelectColumns is the canonical column list for module reads. Ordered
// to match moduleRow field tags so pgx.RowToStructByName works.
const moduleSelectColumns = `id, name, git_source, module_path, provider, status, description,
	owner_team_id, created_at, updated_at`

// RegisterModule handles module registration: clones Git source, extracts the
// scalar contract, and persists module + version atomically.
func (h *RegistryHandler) RegisterModule(
	ctx context.Context,
	req *connect.Request[registryv1.RegisterModuleRequest],
) (*connect.Response[registryv1.RegisterModuleResponse], error) {
	in := req.Msg

	// proto OwnerTeamId is string; DB expects int64 (snowflake). Parse.
	ownerTeamID, err := strconv.ParseInt(in.OwnerTeamId, 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInvalidArgument,
			"owner_team_id must be a numeric snowflake ID, got %q", in.OwnerTeamId)
	}

	// Phase 1 git credential injection: the frontend sends an optional
	// x-git-token Connect header (set by the api.ts transport interceptor when
	// the operator supplies a token in the register form). GoGitProvider reads
	// GIT_TOKEN from env at Clone time, so set it for the duration of this call
	// and restore the previous value (or unset) afterwards. Phase 2 will route
	// this through the credentials table + Vault/KMS.
	if token := req.Header().Get("x-git-token"); token != "" {
		prev, hadPrev := os.LookupEnv("GIT_TOKEN")
		os.Setenv("GIT_TOKEN", token)
		defer func() {
			if hadPrev {
				os.Setenv("GIT_TOKEN", prev)
			} else {
				os.Unsetenv("GIT_TOKEN")
			}
		}()
	}

	result, err := h.svc.RegisterModule(ctx, registry.RegisterModuleInput{
		GitSource:   in.GitSource,
		ModulePath:  in.ModulePath,
		Version:     in.Version,
		Provider:    in.Provider,
		Name:        in.Name,
		Description: in.Description,
		OwnerTeamID: ownerTeamID,
		Layer:       "", // Layer inferred from module_path or set via separate call; MVP leaves empty (catalog_items.layer_logical_id is authoritative).
	})
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"register module: %v", err)
	}

	// Build the Module proto message for the response.
	statusVal := commonv1.ModuleStatus_MODULE_STATUS_PENDING_VALIDATION
	switch result.Status {
	case "validated":
		statusVal = commonv1.ModuleStatus_MODULE_STATUS_VALIDATED
	case "validation_failed":
		statusVal = commonv1.ModuleStatus_MODULE_STATUS_VALIDATION_FAILED
	}
	res := connect.NewResponse(&registryv1.RegisterModuleResponse{
		Module: &registryv1.Module{
			Id:          strconv.FormatInt(result.ModuleID, 10),
			Name:        in.Name,
			Description: in.Description,
			GitSource:   in.GitSource,
			Provider:    in.Provider,
			ModulePath:  in.ModulePath,
			OwnerTeamId: in.OwnerTeamId,
			Status:      statusVal,
			// Surface the freshly-created version so the operator sees what was
			// pinned. The dedicated module_version_id field below is the canonical
			// handle for PublishCatalogItem.
			Versions: []*registryv1.ModuleVersion{
				{
					Version:      in.Version,
					CommitSha:    result.CommitSHA,
					RegisteredAt: time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"),
				},
			},
		},
		ModuleVersionId: strconv.FormatInt(result.ModuleVersionID, 10),
	})
	return res, nil
}

// ListModules returns all non-deleted modules. Phase 1 ignores pagination and
// the provider/status filters (the proto fields are accepted but not applied);
// the result set is small enough that client-side filtering is fine.
func (h *RegistryHandler) ListModules(
	ctx context.Context,
	_ *connect.Request[registryv1.ListModulesRequest],
) (*connect.Response[registryv1.ListModulesResponse], error) {
	if h.pool == nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"registry handler has no db pool configured")
	}

	rows, err := h.pool.Query(ctx,
		`SELECT `+moduleSelectColumns+` FROM modules WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list modules: %v", err)
	}
	defer rows.Close()
	mods, err := pgx.CollectRows(rows, pgx.RowToStructByName[moduleRow])
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"collect modules: %v", err)
	}

	out := make([]*registryv1.Module, 0, len(mods))
	for i := range mods {
		out = append(out, moduleRowToProto(&mods[i]))
	}
	return connect.NewResponse(&registryv1.ListModulesResponse{Modules: out}), nil
}

// GetModule returns a single module by id.
func (h *RegistryHandler) GetModule(
	ctx context.Context,
	req *connect.Request[registryv1.GetModuleRequest],
) (*connect.Response[registryv1.GetModuleResponse], error) {
	if h.pool == nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"registry handler has no db pool configured")
	}
	id, err := strconv.ParseInt(req.Msg.GetModuleId(), 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInvalidArgument,
			"module_id must be numeric, got %q", req.Msg.GetModuleId())
	}

	row, err := h.getModuleByID(ctx, id)
	if err != nil {
		return nil, registryNotFoundOrInternal("module", req.Msg.GetModuleId(), err)
	}
	return connect.NewResponse(&registryv1.GetModuleResponse{Module: moduleRowToProto(&row)}), nil
}

// DeprecateModule soft-deletes (or status-deprecates) a module. Per the live
// schema, modules carries a status lifecycle that includes 'deprecated', so we
// flip the status column. If a version is supplied we leave it untouched (the
// proto supports per-version deprecation, but the current schema has no
// per-version status column — Phase 1 deprecates the whole module row).
func (h *RegistryHandler) DeprecateModule(
	ctx context.Context,
	req *connect.Request[registryv1.DeprecateModuleRequest],
) (*connect.Response[registryv1.DeprecateModuleResponse], error) {
	if h.pool == nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"registry handler has no db pool configured")
	}
	id, err := strconv.ParseInt(req.Msg.GetModuleId(), 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInvalidArgument,
			"module_id must be numeric, got %q", req.Msg.GetModuleId())
	}

	tag, err := h.pool.Exec(ctx,
		`UPDATE modules SET status = 'deprecated', updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"deprecate module: %v", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_MODULE_VERSION_NOT_FOUND, connect.CodeNotFound,
			"module %q not found", req.Msg.GetModuleId())
	}

	row, err := h.getModuleByID(ctx, id)
	if err != nil {
		return nil, registryNotFoundOrInternal("module", req.Msg.GetModuleId(), err)
	}
	return connect.NewResponse(&registryv1.DeprecateModuleResponse{Module: moduleRowToProto(&row)}), nil
}

// getModuleByID loads one module row by snowflake id.
func (h *RegistryHandler) getModuleByID(ctx context.Context, id int64) (moduleRow, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT `+moduleSelectColumns+` FROM modules WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return moduleRow{}, err
	}
	defer rows.Close()
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[moduleRow])
	if err != nil {
		return moduleRow{}, err
	}
	return r, nil
}

// moduleRowToProto maps a modules row to the proto Module message. int64 IDs
// are stringified per the proto contract; timestamps are RFC3339 strings.
func moduleRowToProto(m *moduleRow) *registryv1.Module {
	out := &registryv1.Module{
		Id:          strconv.FormatInt(m.ID, 10),
		Name:        m.Name,
		Description: m.Description,
		GitSource:   m.GitSource,
		Provider:    m.Provider,
		ModulePath:  m.ModulePath,
		OwnerTeamId: strconv.FormatInt(m.OwnerTeamID, 10),
		Status:      moduleStatusToProto(m.Status),
	}
	if !m.CreatedAt.IsZero() {
		out.CreatedAt = m.CreatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	if !m.UpdatedAt.IsZero() {
		out.UpdatedAt = m.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	return out
}

// moduleStatusToProto maps the DB status string to the proto enum.
func moduleStatusToProto(s string) commonv1.ModuleStatus {
	switch s {
	case "pending_validation":
		return commonv1.ModuleStatus_MODULE_STATUS_PENDING_VALIDATION
	case "validated":
		return commonv1.ModuleStatus_MODULE_STATUS_VALIDATED
	case "validation_failed":
		return commonv1.ModuleStatus_MODULE_STATUS_VALIDATION_FAILED
	case "deprecated":
		return commonv1.ModuleStatus_MODULE_STATUS_DEPRECATED
	}
	return commonv1.ModuleStatus_MODULE_STATUS_UNSPECIFIED
}

// registryNotFoundOrInternal maps a pgx.ErrNoRows to NOT_FOUND and anything
// else to INTERNAL. entity is the human label; id is the caller-provided id.
func registryNotFoundOrInternal(entity, id string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return platformerrors.New(commonv1.ErrorCode_ERROR_CODE_MODULE_VERSION_NOT_FOUND, connect.CodeNotFound,
			"%s %q not found", entity, id)
	}
	return platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
		"load %s %q: %v", entity, id, err)
}
