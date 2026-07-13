// Package connect implements Connect-RPC handlers for Aether's business APIs.
package connect

import (
	"context"

	"connectrpc.com/connect"

	catalogv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
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
	items := []*catalogv1.CatalogItem{
		{Id: "aws-ecs-service", Name: "AWS ECS Service", Description: "Managed container service"},
		{Id: "aliyun-ecs", Name: "Aliyun ECS", Description: "Elastic compute service"},
	}
	resp := connect.NewResponse(&catalogv1.ListItemsResponse{Items: items})
	return resp, nil
}
