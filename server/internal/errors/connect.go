package errors

import (
	"context"

	"connectrpc.com/connect"

	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// WrapInterceptor returns a Connect interceptor that catches raw Go errors
// (not already a *connect.Error) and wraps them as a structured INTERNAL_ERROR.
// This is the safety net: if a handler forgets to use errors.New and returns a
// bare error, clients never see an unstructured 500 with a leaked internal
// message — they get a typed INTERNAL_ERROR.
//
// Errors already structured via errors.New (IsConnectError == true) pass
// through unchanged so their original code/details are preserved.
//
// Known limitation: the check is IsConnectError, which is true for ANY
// *connect.Error — including ones a handler built directly via connect.NewError
// without going through this package. Such errors pass through. Enforcing
// "every structured error must come from errors.New" requires a lint rule or a
// custom error type; deferred. Place this AFTER business interceptors.
func WrapInterceptor() connect.Interceptor {
	return &wrapInterceptor{}
}

type wrapInterceptor struct{}

func (i *wrapInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err == nil {
			return resp, nil
		}
		if IsConnectError(err) {
			// Already a structured *connect.Error — preserve it.
			return resp, err
		}
		// Raw error — wrap as INTERNAL_ERROR so the client gets a structured
		// error instead of a leaked internal message.
		return resp, NewFromError(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal, err)
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
		return NewFromError(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal, err)
	}
}

// Compile-time check: wrapInterceptor implements connect.Interceptor.
var _ connect.Interceptor = (*wrapInterceptor)(nil)
