// Package connect implements Connect-RPC handlers for Aether's business APIs.
//
// catalog.go implements CatalogServiceHandler (user-facing, read-only):
// ListItems + GetCatalogItem backed by CatalogRepo. Mutating operations
// (Publish/Update/Deprecate) live on CatalogAdminService — see catalog_admin.go.

package connect

import (
	"context"
	"encoding/json"
	"strconv"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	catalogv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// CatalogHandler implements the user-facing CatalogService Connect RPC
// (read-only: ListItems + GetCatalogItem).
type CatalogHandler struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	repo *repo.CatalogRepo
}

// Compile-time check.
var _ catalogv1connect.CatalogServiceHandler = (*CatalogHandler)(nil)

// NewCatalogHandler returns a CatalogHandler ready to register on a mux.
func NewCatalogHandler(repo *repo.CatalogRepo) *CatalogHandler {
	return &CatalogHandler{repo: repo}
}

// ListItems returns all active catalog items from the DB.
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

// dbCatalogItemToProto converts a sqlc-generated CatalogItem row to the proto
// message. Field mappings cover the MVP surface.
func dbCatalogItemToProto(ci *generated.CatalogItem) *catalogv1.CatalogItem {
	out := &catalogv1.CatalogItem{
		Id:          strconv.FormatInt(ci.ID, 10),
		Name:        ci.DisplayName,
		Description: ci.Description,
		Category:    ci.Category,
		OwnerTeamId: strconv.FormatInt(ci.OwnerTeamID, 10),
		Status:      statusStringToProto(ci.Status),
		// form_schema_json drives the RJSF dynamic request form. The column is
		// JSONB ([]byte); the proto field is a string. We decode then re-encode
		// to compact text so the frontend can JSON.parse it directly.
		FormSchemaJson: compactJSONString(ci.FormSchemaJson),
	}
	if ci.LayerLogicalID != nil {
		out.LayerLogicalId = *ci.LayerLogicalID
	}
	return out
}

// compactJSONString returns the compact JSON text for a JSONB byte slice. Empty
// or null input returns "". Errors are swallowed (the frontend treats invalid
// JSON as "no dynamic schema" and falls back to a generic form).
func compactJSONString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return ""
	}
	if v == nil {
		return ""
	}
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}

// statusStringToProto maps the DB status string to the proto enum.
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
