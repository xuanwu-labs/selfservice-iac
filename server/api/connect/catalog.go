// Package connect implements Connect-RPC handlers for Aether's business APIs.
package connect

import (
	"context"

	"connectrpc.com/connect"

	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	catalogv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// CatalogHandler implements the CatalogService Connect RPC.
//
// reg is the error registry (wire-injected): handlers return structured
// errors via reg.New(code, ...) instead of hardcoding connect.NewError.
// See internal/errors/registry.go and specs/03-平台契约.md.
type CatalogHandler struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	reg *platformerrors.Registry
}

// Compile-time check.
var _ catalogv1connect.CatalogServiceHandler = (*CatalogHandler)(nil)

// NewCatalogHandler returns a CatalogHandler ready to register on a mux.
// reg is injected by wire (loaded from embedded error-codes.yaml at startup).
func NewCatalogHandler(reg *platformerrors.Registry) *CatalogHandler {
	return &CatalogHandler{reg: reg}
}

// ListItems returns the catalog items. Phase 1: static placeholder.
func (h *CatalogHandler) ListItems(ctx context.Context, req *connect.Request[catalogv1.ListItemsRequest]) (*connect.Response[catalogv1.ListItemsResponse], error) {
	items := []*catalogv1.CatalogItem{
		{Id: "aws-ecs-service", Name: "AWS ECS Service", Description: "Managed container service"},
		{Id: "aliyun-ecs", Name: "Aliyun ECS", Description: "Elastic compute service"},
	}
	resp := connect.NewResponse(&catalogv1.ListItemsResponse{Items: items})
	return resp, nil
}

// GetCatalogItem returns a single catalog item. Phase 1: demonstrates
// structured error return via the registry — an unknown id yields a
// registered CATALOG_ITEM_NOT_FOUND error carrying ErrorInfo detail
// (retryable=false, remediation, owner), not a bare connect.NewError.
func (h *CatalogHandler) GetCatalogItem(ctx context.Context, req *connect.Request[catalogv1.GetCatalogItemRequest]) (*connect.Response[catalogv1.GetCatalogItemResponse], error) {
	for _, item := range catalogItems() {
		if item.Id == req.Msg.GetCatalogItemId() {
			return connect.NewResponse(&catalogv1.GetCatalogItemResponse{Item: item}), nil
		}
	}
	return nil, h.reg.New(commonv1.ErrorCode_ERROR_CODE_CATALOG_ITEM_NOT_FOUND, "catalog item %q not found", req.Msg.GetCatalogItemId())
}

// catalogItems is the Phase 1 static seed shared by ListItems/GetCatalogItem.
func catalogItems() []*catalogv1.CatalogItem {
	return []*catalogv1.CatalogItem{
		{Id: "aws-ecs-service", Name: "AWS ECS Service", Description: "Managed container service"},
		{Id: "aliyun-ecs", Name: "Aliyun ECS", Description: "Elastic compute service"},
	}
}
