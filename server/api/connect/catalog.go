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
type CatalogHandler struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
}

// Compile-time check.
var _ catalogv1connect.CatalogServiceHandler = (*CatalogHandler)(nil)

// NewCatalogHandler returns a CatalogHandler ready to register on a mux.
func NewCatalogHandler() *CatalogHandler { return &CatalogHandler{} }

// ListItems returns the catalog items. Phase 1: static placeholder.
func (h *CatalogHandler) ListItems(ctx context.Context, req *connect.Request[catalogv1.ListItemsRequest]) (*connect.Response[catalogv1.ListItemsResponse], error) {
	resp := connect.NewResponse(&catalogv1.ListItemsResponse{Items: catalogItems()})
	return resp, nil
}

// GetCatalogItem returns a single catalog item. Demonstrates structured error
// return via errors.New (typed ErrorCode + connect.Code) instead of a bare
// connect.NewError.
func (h *CatalogHandler) GetCatalogItem(ctx context.Context, req *connect.Request[catalogv1.GetCatalogItemRequest]) (*connect.Response[catalogv1.GetCatalogItemResponse], error) {
	for _, item := range catalogItems() {
		if item.Id == req.Msg.GetCatalogItemId() {
			return connect.NewResponse(&catalogv1.GetCatalogItemResponse{Item: item}), nil
		}
	}
	return nil, platformerrors.New(commonv1.ErrorCode_ERROR_CODE_CATALOG_ITEM_NOT_FOUND, connect.CodeNotFound, "catalog item %q not found", req.Msg.GetCatalogItemId())
}

// catalogItems is the Phase 1 static seed shared by ListItems/GetCatalogItem.
func catalogItems() []*catalogv1.CatalogItem {
	return []*catalogv1.CatalogItem{
		{Id: "aws-ecs-service", Name: "AWS ECS Service", Description: "Managed container service"},
		{Id: "aliyun-ecs", Name: "Aliyun ECS", Description: "Elastic compute service"},
	}
}
