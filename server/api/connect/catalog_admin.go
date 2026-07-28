// Package connect implements Connect-RPC handlers for Aether's business APIs.
//
// catalog_admin.go implements CatalogAdminServiceHandler (operator-facing,
// requires admin role): PublishCatalogItem + UpdateCatalogItem +
// DeprecateCatalogItem. Per catalog/srv.proto these mutating RPCs are on
// CatalogAdminService (NOT CatalogService) so they get a separate handler.

package connect

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/core/catalog"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	catalogv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// CatalogAdminHandler implements the operator-facing CatalogAdminService
// Connect RPC (Publish/Update/Deprecate). Admin role enforced by Connect
// interceptor (api/connect/middleware) — this handler only does business logic.
type CatalogAdminHandler struct {
	catalogv1connect.UnimplementedCatalogAdminServiceHandler
	svc *catalog.CatalogService
}

// Compile-time check.
var _ catalogv1connect.CatalogAdminServiceHandler = (*CatalogAdminHandler)(nil)

// NewCatalogAdminHandler returns a CatalogAdminHandler ready to register on a mux.
func NewCatalogAdminHandler(svc *catalog.CatalogService) *CatalogAdminHandler {
	return &CatalogAdminHandler{svc: svc}
}

// PublishCatalogItem publishes a new catalog item: formgen + defaults + D40
// validation + DB write, all orchestrated by CatalogService.Publish.
func (h *CatalogAdminHandler) PublishCatalogItem(
	ctx context.Context,
	req *connect.Request[catalogv1.PublishCatalogItemRequest],
) (*connect.Response[catalogv1.PublishCatalogItemResponse], error) {
	in := req.Msg

	moduleVersionID, err := strconv.ParseInt(in.ModuleVersion, 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_MODULE_VERSION_NOT_FOUND, connect.CodeInvalidArgument,
			"module_version must be numeric, got %q", in.ModuleVersion)
	}
	ownerTeamID, err := strconv.ParseInt(in.OwnerTeamId, 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInvalidArgument,
			"owner_team_id must be numeric, got %q", in.OwnerTeamId)
	}

	ci, err := h.svc.Publish(ctx, catalog.PublishInput{
		ModuleVersionID: moduleVersionID,
		DisplayName:     in.Name,
		Description:     in.Description,
		Category:        in.Category,
		LayerLogicalID:  in.LayerLogicalId,
		OwnerTeamID:     ownerTeamID,
		Visibility:      in.VisibleToTeams,
	})
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"publish catalog item: %v", err)
	}

	return connect.NewResponse(&catalogv1.PublishCatalogItemResponse{
		Item: dbCatalogItemToProto(&ci),
	}), nil
}
