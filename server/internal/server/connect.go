// Package server: connect.go — Connect-RPC handler registration.
//
// ProvideServerConfig is the single place where all Options (middleware,
// interceptors, Connect handlers) are assembled. It reads the catalog handler
// (wire-injected) and wraps it with the standard interceptor chain, producing
// a ServerConfig that NewServer + NewHTTPServer consume.
package server

import (
	"fmt"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"

	connectapi "github.com/xuanwu-labs/selfservice-iac/server/api/connect"
	platformerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/middleware"
	catalogv1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/catalog/catalogv1connect"
)

// ProvideServerConfig assembles the full ServerConfig: gin middleware chain
// + Connect interceptor chain + Connect handler registration.
//
// This is the wire provider that bundles middleware.DefaultGinMiddlewares,
// middleware.DefaultConnectInterceptors, the otelconnect interceptor, the
// audit interceptor (needs logger), the error-wrap fallback interceptor, and
// registers all Connect service handlers.
func ProvideServerConfig(catalog *connectapi.CatalogHandler, logger *otelzap.Logger) (*middleware.ServerConfig, error) {
	// Build the Connect interceptor chain: otelconnect + auth/rbac/ratelimit + audit + error-wrap.
	otelIC, err := otelconnect.NewInterceptor(otelconnect.WithTrustRemote())
	if err != nil {
		return nil, fmt.Errorf("create otelconnect interceptor: %w", err)
	}

	connectOpts := []middleware.Option{
		middleware.WithGinMiddleware(middleware.DefaultGinMiddlewares(logger)...),
		middleware.WithConnectInterceptor(otelIC),
		middleware.WithConnectInterceptor(
			middleware.ConnectAuth(),
			middleware.ConnectRBAC(),
			middleware.ConnectRateLimit(),
			middleware.ConnectAudit(logger),
			// Error-wrap interceptor: last in the chain so it sees the final
			// handler error. Raw (non-structured) errors get wrapped as
			// INTERNAL_ERROR; structured errors pass through unchanged.
			platformerrors.WrapInterceptor(),
		),
	}

	// Register Connect service handlers with the full interceptor chain.
	var allInterceptors []connect.Interceptor
	cfg := middleware.Apply(connectOpts...)
	allInterceptors = cfg.ConnectInterceptors

	path, handler := catalogv1connect.NewCatalogServiceHandler(
		catalog,
		connect.WithInterceptors(allInterceptors...),
	)
	connectOpts = append(connectOpts, middleware.WithConnectHandler(path, handler))

	return middleware.Apply(connectOpts...), nil
}
