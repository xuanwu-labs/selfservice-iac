// Package connect implements Connect-RPC handlers for Aether's business APIs.
//
// registry.go implements RegistryAdminServiceHandler, wiring proto requests to
// core/registry.RegistryService. This is the Admin-layer surface (requires
// admin role, enforced by Connect interceptor — see api/connect/middleware).

package connect

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
	registryv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/registry"
	registryv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/registry/registryv1connect"
)

// RegistryHandler implements the RegistryAdminService Connect RPC.
type RegistryHandler struct {
	registryv1connect.UnimplementedRegistryAdminServiceHandler
	svc *registry.RegistryService
}

// Compile-time check.
var _ registryv1connect.RegistryAdminServiceHandler = (*RegistryHandler)(nil)

// NewRegistryHandler returns a RegistryHandler ready to register on a mux.
func NewRegistryHandler(svc *registry.RegistryService) *RegistryHandler {
	return &RegistryHandler{svc: svc}
}

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
			Status:      statusVal,
		},
	})
	return res, nil
}
