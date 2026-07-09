// Package interceptor provides Connect-RPC interceptors for the cross-cutting
// concerns every RPC shares: authentication, authorization (RBAC), audit
// logging, and rate limiting (task 15.5).
//
// Phase 1: skeleton interceptors that pass through (no enforcement yet). They
// establish the chain shape so real logic drops in without rewiring the mux.
// gin middleware shares the same concerns via pure functions where possible.
//
// Audit uses otelzap.Logger (not slog) so audit log lines carry the active
// trace_id (D41/D28 — audit events MUST be trace-correlated).
package interceptor

import (
	"context"

	"connectrpc.com/connect"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// Auth returns a unary interceptor that validates credentials.
// Phase 1: pass-through (records that auth was invoked).
func Auth() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO(task-15): validate JWT/OIDC bearer or AK/SK from req.Header().
			// On failure: return nil, connect.NewError(connect.CodeUnauthenticated, err)
			return next(ctx, req)
		}
	}
}

// RBAC returns a unary interceptor that enforces role-based access control.
// Phase 1: pass-through.
func RBAC() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO(task-15): check the caller's team/role against the RPC method.
			return next(ctx, req)
		}
	}
}

// Audit returns a unary interceptor that records each RPC for the audit trail.
// Uses otelzap so the log line carries the active trace_id (D41/D28).
func Audit(logger *otelzap.Logger) connect.UnaryInterceptorFunc {
	base := zap.NewNop()
	if logger == nil {
		logger = otelzap.New(base)
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

// RateLimit returns a unary interceptor that enforces per-actor rate limits.
// Phase 1: pass-through (gin has a per-IP limiter for HTTP; this is for RPC).
func RateLimit() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// TODO(task-15): per-actor token bucket (share logic with middleware.RateLimit).
			return next(ctx, req)
		}
	}
}

// Chain combines the four interceptors in evaluation order:
// auth → rbac → ratelimit → audit(handler).
// OTel's otelconnect interceptor (task 11.4) is added separately when wiring
// the mux, since it wraps the whole chain.
func Chain(logger *otelzap.Logger) []connect.UnaryInterceptorFunc {
	return []connect.UnaryInterceptorFunc{
		Auth(),
		RBAC(),
		RateLimit(),
		Audit(logger),
	}
}

// OtelZapLogger returns the global otelzap logger (set by otel.WrapLogger at
// startup). Used by wire providers that need a trace-aware logger.
func OtelZapLogger() *otelzap.Logger { return otelzap.L() }
