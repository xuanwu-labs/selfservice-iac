package grpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servergrpc "github.com/xuanwu-labs/selfservice-iac/server/api/grpc"
	platformv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1"
	platformv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/platformv1connect"
)

// TestCatalogListItems verifies the Connect-RPC pipeline end to end: register
// the handler on a mux, call it with a generated client, assert the response.
func TestCatalogListItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(platformv1connect.NewCatalogServiceHandler(servergrpc.NewCatalogHandler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := platformv1connect.NewCatalogServiceClient(srv.Client(), srv.URL, connect.WithInterceptors())

	resp, err := client.ListItems(context.Background(), connect.NewRequest(&platformv1.ListItemsRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	items := resp.Msg.GetItems()
	require.Len(t, items, 2, "placeholder catalog has 2 items")
	assert.Equal(t, "aws-ecs-service", items[0].GetId())
	assert.NotEmpty(t, items[0].GetName())
}

// TestCatalogListItemsEmptyRequest verifies page_size=0 is accepted.
func TestCatalogListItemsEmptyRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(platformv1connect.NewCatalogServiceHandler(servergrpc.NewCatalogHandler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := platformv1connect.NewCatalogServiceClient(srv.Client(), srv.URL)

	resp, err := client.ListItems(context.Background(), connect.NewRequest(&platformv1.ListItemsRequest{PageSize: 0}))
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.GetItems())
}
