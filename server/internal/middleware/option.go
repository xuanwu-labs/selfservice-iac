// Package middleware: option.go — functional options for Server construction.
//
// Usage:
//
//	srv := server.NewServer(cfg,
//	    middleware.WithGinMiddleware(mw.Logger(logger), mw.RequestID()),
//	    middleware.WithConnectHandler(path, handler),
//	)
//
// wire provides a DefaultOptions provider that bundles the standard middleware
// + interceptor chain, so most callers don't need to specify options manually.
package middleware

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
)

// ServerConfig holds the assembled middleware/interceptor/handler state.
// It is populated by Option functions and consumed by internal/server.
type ServerConfig struct {
	GinMiddlewares      []gin.HandlerFunc
	ConnectInterceptors []connect.Interceptor
	ConnectHandlers     []ConnectHandler
}

// ConnectHandler is a path + http.Handler pair registered on the Connect mux.
type ConnectHandler struct {
	Path    string
	Handler http.Handler
}

// Option configures the ServerConfig at construction time.
type Option func(*ServerConfig)

// WithGinMiddleware adds gin middleware to the HTTP chain.
func WithGinMiddleware(mw ...gin.HandlerFunc) Option {
	return func(c *ServerConfig) {
		c.GinMiddlewares = append(c.GinMiddlewares, mw...)
	}
}

// WithConnectInterceptor adds a Connect interceptor to the RPC chain.
func WithConnectInterceptor(ic ...connect.Interceptor) Option {
	return func(c *ServerConfig) {
		c.ConnectInterceptors = append(c.ConnectInterceptors, ic...)
	}
}

// WithConnectHandler registers a Connect service handler on the /api/ mux.
func WithConnectHandler(path string, handler http.Handler) Option {
	return func(c *ServerConfig) {
		c.ConnectHandlers = append(c.ConnectHandlers, ConnectHandler{Path: path, Handler: handler})
	}
}

// Apply evaluates all options and returns the resulting ServerConfig.
func Apply(opts ...Option) *ServerConfig {
	c := &ServerConfig{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
