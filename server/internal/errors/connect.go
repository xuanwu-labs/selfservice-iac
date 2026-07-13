package errors

import (
	"context"

	"connectrpc.com/connect"
)

// WrapInterceptor returns a Connect interceptor that catches raw Go errors
// (not already a *connect.Error) and wraps them as a structured INTERNAL_ERROR.
// This is the safety net: if a handler forgets to use reg.New and returns a
// bare error, clients never see an unstructured 500 with a leaked internal
// message — they get a registered INTERNAL_ERROR carrying the standard
// remediation/owner metadata.
//
// Known limitation (Phase 1): the check is IsConnectError, which is true for
// ANY *connect.Error — including ones a handler built directly via
// connect.NewError without going through the registry. Such errors pass
// through without retryable/remediation/owner metadata. Enforcing "every
// connect.NewError must go through the registry" requires either a lint rule
// or a custom error type distinguishable from plain *connect.Error; both are
// deferred. The spec (specs/03 "错误码不得硬编码") calls this out as a
// MUST NOT, enforced by review until the tooling exists.
//
// Place this AFTER business interceptors in the chain so it sees the final
// handler error. It only wraps on the response path; it never short-circuits
// the request.
func WrapInterceptor(reg *Registry) connect.Interceptor {
	return &wrapInterceptor{reg: reg}
}

type wrapInterceptor struct {
	reg *Registry
}

func (i *wrapInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err == nil {
			return resp, nil
		}
		if IsConnectError(err) {
			// Already a structured *connect.Error from the registry — preserve it.
			return resp, err
		}
		// Raw error — wrap as INTERNAL_ERROR so the client gets structured info
		// (retryable=false, owner=platform, remediation) instead of a leak.
		return resp, i.reg.NewFromError(CodeInternalError, err)
	}
}

func (i *wrapInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *wrapInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err == nil {
			return nil
		}
		if IsConnectError(err) {
			return err
		}
		return i.reg.NewFromError(CodeInternalError, err)
	}
}

// Compile-time check: wrapInterceptor implements connect.Interceptor.
var _ connect.Interceptor = (*wrapInterceptor)(nil)
