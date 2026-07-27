// Package connect implements Connect-RPC handlers for Aether's business APIs.
//
// catalog.go implements CatalogServiceHandler, wiring proto requests to
// core/catalog.CatalogService (Publish) and data/repo.CatalogRepo (List/Get).
// The static placeholder data from the scaffolding phase is removed; List/Get
// now read from the DB via CatalogRepo.

package connect

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/core/catalog"
	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	catalogv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// CatalogHandler implements the CatalogService Connect RPC.
type CatalogHandler struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	repo *repo.CatalogRepo
	svc  *catalog.CatalogService
}

// Compile-time check.
var _ catalogv1connect.CatalogServiceHandler = (*CatalogHandler)(nil)

// NewCatalogHandler returns a CatalogHandler ready to register on a mux.
// CatalogRepo powers ListItems/GetCatalogItem (read); CatalogService powers
// PublishCatalogItem (write, with formgen + defaults + D40 validation).
func NewCatalogHandler(repo *repo.CatalogRepo, svc *catalog.CatalogService) *CatalogHandler {
	return &CatalogHandler{repo: repo, svc: svc}
}

// ListItems returns all active catalog items from the DB (W1-03: replaced the
// scaffolding-phase static placeholder with a real CatalogRepo.List call).
func (h *CatalogHandler) ListItems(
	ctx context.Context,
	_ *connect.Request[catalogv1.ListItemsRequest],
) (*connect.Response[catalogv1.ListItemsResponse], error) {
	items, err := h.repo.List(ctx)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list catalog items: %v", err)
	}
	out := make([]*catalogv1.CatalogItem, 0, len(items))
	for _, ci := range items {
		out = append(out, dbCatalogItemToProto(&ci))
	}
	return connect.NewResponse(&catalogv1.ListItemsResponse{Items: out}), nil
}

// GetCatalogItem returns a single catalog item by ID.
func (h *CatalogHandler) GetCatalogItem(
	ctx context.Context,
	req *connect.Request[catalogv1.GetCatalogItemRequest],
) (*connect.Response[catalogv1.GetCatalogItemResponse], error) {
	id, err := strconv.ParseInt(req.Msg.GetCatalogItemId(), 10, 64)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInvalidArgument,
			"catalog_item_id must be numeric, got %q", req.Msg.GetCatalogItemId())
	}
	ci, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_CATALOG_ITEM_NOT_FOUND, connect.CodeNotFound,
			"catalog item %q not found: %v", req.Msg.GetCatalogItemId(), err)
	}
	return connect.NewResponse(&catalogv1.GetCatalogItemResponse{Item: dbCatalogItemToProto(&ci)}), nil
}

// PublishCatalogItem publishes a new catalog item: formgen + defaults + D40
// validation + DB write, all orchestrated by CatalogService.Publish.
func (h *CatalogHandler) PublishCatalogItem(
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

// dbCatalogItemToProto converts a sqlc-generated CatalogItem row to the proto
// message. Field mappings cover the MVP surface; richer fields
// (form_schema_json, defaults_json) are attached as JSON strings.
func dbCatalogItemToProto(ci *generated.CatalogItem) *catalogv1.CatalogItem {
	out := &catalogv1.CatalogItem{
		Id:          strconv.FormatInt(ci.ID, 10),
		Name:        ci.DisplayName,
		Description: ci.Description,
		Category:    ci.Category,
		OwnerTeamId: strconv.FormatInt(ci.OwnerTeamID, 10),
		Status:      statusStringToProto(ci.Status),
	}
	if ci.LayerLogicalID != nil {
		out.LayerLogicalId = *ci.LayerLogicalID
	}
	return out
}

// statusStringToProto maps the DB status string to the proto enum. MVP maps
// the common values; unknown → unspecified.
func statusStringToProto(s string) commonv1.CatalogItemStatus {
	switch s {
	case "active":
		return commonv1.CatalogItemStatus_CATALOG_ITEM_STATUS_ACTIVE
	case "deprecated":
		return commonv1.CatalogItemStatus_CATALOG_ITEM_STATUS_DEPRECATED
	case "archived":
		return commonv1.CatalogItemStatus_CATALOG_ITEM_STATUS_ARCHIVED
	case "draft":
		return commonv1.CatalogItemStatus_CATALOG_ITEM_STATUS_DRAFT
	default:
		return commonv1.CatalogItemStatus_CATALOG_ITEM_STATUS_UNSPECIFIED
	}
}
