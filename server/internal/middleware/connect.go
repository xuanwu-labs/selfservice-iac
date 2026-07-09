// Package middleware: connect.go — Connect-RPC interceptors.
//
// Each interceptor is a pure function returning connect.UnaryInterceptorFunc.
// The server layer composes them via Options at startup. Phase 1: pass-through
// skeletons; real enforcement logic lands with the auth/identity modules.
package middleware

import (
	"context"

	"connectrpc.com/connect"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// Auth returns a Connect interceptor that validates credentials.
// Phase 1: pass-through.
func ConnectAuth() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO: validate JWT/OIDC bearer or AK/SK from req.Header().
			return next(ctx, req)
		}
	}
}

// RBAC returns a Connect interceptor that enforces role-based access control.
// Phase 1: pass-through.
func ConnectRBAC() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO: check caller's team/role against the RPC method.
			return next(ctx, req)
		}
	}
}

// Audit returns a Connect interceptor that records each RPC for the audit trail.
// Uses otelzap so the log line carries the active trace_id (D41/D28).
func ConnectAudit(logger *otelzap.Logger) connect.UnaryInterceptorFunc {
	if logger == nil {
		logger = otelzap.New(zap.NewNop())
	}
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			proc := req.Spec().Procedure
			resp, err := next(ctx, req)
			logger.Ctx(ctx).Info("rpc",
				zap.String("procedure", proc),
				zap.Error(err),
			)
			return resp, err
		}
	}
}

// ConnectRateLimit returns a Connect interceptor for per-actor rate limiting.
// Phase 1: pass-through.
func ConnectRateLimit() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO: per-actor token bucket.
			return next(ctx, req)
		}
	}
}
