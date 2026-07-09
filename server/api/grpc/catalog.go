// Package grpc implements Connect-RPC handlers for Aether's business APIs.
// catalog.go implements CatalogService (specs/02) — Phase 1 returns a static
// placeholder; full catalog logic lands in iac-self-service-platform.
package grpc

import (
	"context"

	"connectrpc.com/connect"

	platformv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1"
	platformv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/platformv1connect"
)

// CatalogHandler implements the CatalogService Connect RPC.
type CatalogHandler struct {
	platformv1connect.UnimplementedCatalogServiceHandler
}

// Compile-time check: CatalogHandler satisfies the Connect handler interface.
var _ platformv1connect.CatalogServiceHandler = (*CatalogHandler)(nil)

// NewCatalogHandler returns a CatalogHandler ready to register on a mux.
func NewCatalogHandler() *CatalogHandler { return &CatalogHandler{} }

// ListItems returns the catalog items. Phase 1: static placeholder list so the
// Connect pipeline can be exercised end to end. Real data comes from the
// catalog module once specs/02 is implemented.
func (h *CatalogHandler) ListItems(ctx context.Context, req *connect.Request[platformv1.ListItemsRequest]) (*connect.Response[platformv1.ListItemsResponse], error) {
	items := []*platformv1.CatalogItem{
		{Id: "aws-ecs-service", Name: "AWS ECS Service", Description: "Managed container service"},
		{Id: "aliyun-ecs", Name: "Aliyun ECS", Description: "Elastic compute service"},
	}
	resp := connect.NewResponse(&platformv1.ListItemsResponse{Items: items})
	return resp, nil
}
