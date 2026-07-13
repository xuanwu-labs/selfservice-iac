// Package errors provides structured Connect errors keyed by a proto ErrorCode
// enum. The design mirrors kratos/ferret: an error carries (1) a typed code
// identity (the ErrorCode enum, buf-generated — the single source of truth),
// (2) a transport semantic (connect.Code, like kratos's HTTP-status-style code),
// and (3) a human-readable message supplied at the call site.
//
// There is no separate registry/YAML of behavior fields. Retryability is
// derived from connect.Code per the gRPC standard (Unavailable/ResourceExhausted/
// Aborted/DeadlineExceeded are retryable); HTTP status is derived from connect.Code.
// This keeps error-code identity to a single source (the proto enum) and avoids
// the dual-source drift a YAML behavior table would introduce.
package errors

import (
	stderrors "errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// New wraps a typed ErrorCode + connect.Code + message into a structured
// Connect error carrying an ErrorInfo detail. Handlers call this instead of
// connect.NewError directly:
//
//	return nil, errors.New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, connect.CodeAborted, "version mismatch: %d", got)
//
// The returned *connect.Error has:
//   - Code: the supplied connect.Code (transport semantic)
//   - underlying error: the formatted message
//   - ErrorInfo detail: reason = the ErrorCode's YAML-style key (enum value
//     name minus the ERROR_CODE_ prefix), so clients identify the business code
//
// Clients (connect-es / connect-go) read the gRPC Code for retry decisions and
// the ErrorInfo reason for business-code branching.
func New(code commonv1.ErrorCode, grpcCode connect.Code, format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	connectErr := connect.NewError(grpcCode, stderrors.New(msg))
	detail, err := connect.NewErrorDetail(&errdetails.ErrorInfo{
		Reason: enumToKey(code),
		Domain: "aether.platform",
	})
	if err != nil {
		// NewErrorDetail only fails on nil message; errdetails.ErrorInfo is never nil.
		return connectErr
	}
	connectErr.AddDetail(detail)
	return connectErr
}

// NewFromError wraps a raw Go error with a typed code + connect.Code. Use when
// an underlying error (e.g. from the data layer) must be surfaced as a typed
// platform error:
//
//	return nil, errors.NewFromError(commonv1.ErrorCode_ERROR_CODE_GIT_OPERATION_FAILED, connect.CodeUnavailable, err)
func NewFromError(code commonv1.ErrorCode, grpcCode connect.Code, err error) error {
	if err == nil {
		return nil
	}
	return New(code, grpcCode, "%s", err.Error())
}

// enumToKey converts a proto ErrorCode enum value to its ErrorInfo reason key
// by stripping the ERROR_CODE_ prefix: ErrorCode_ERROR_CODE_STATE_CONFLICT →
// "STATE_CONFLICT". This is the value clients match against via CodeOf/IsCode.
func enumToKey(code commonv1.ErrorCode) string {
	const prefix = "ERROR_CODE_"
	name := code.String()
	if len(name) > len(prefix) && name[:len(prefix)] == prefix {
		return name[len(prefix):]
	}
	return name
}

// IsRetryable reports whether a connect.Code is retryable per the gRPC
// standard. Clients should back off and retry on these; the rest indicate a
// client-side problem (bad request, not found, auth) that retrying won't fix.
// See https://grpc.github.io/grpc/core/md_doc_statuscodes.html.
func IsRetryable(c connect.Code) bool {
	switch c {
	case connect.CodeUnavailable,
		connect.CodeResourceExhausted,
		connect.CodeAborted,
		connect.CodeDeadlineExceeded:
		return true
	default:
		return false
	}
}

// HTTPStatus maps a connect.Code to its conventional HTTP status code. This is
// the standard gRPC-HTTP mapping (connect-openapi / grpc-gateway use the same
// table). Useful for gateways/proxies and for logging.
func HTTPStatus(c connect.Code) int {
	switch c {
	case connect.CodeCanceled:
		return 499
	case connect.CodeUnknown:
		return 500
	case connect.CodeInvalidArgument:
		return 400
	case connect.CodeDeadlineExceeded:
		return 504
	case connect.CodeNotFound:
		return 404
	case connect.CodeAlreadyExists:
		return 409
	case connect.CodePermissionDenied:
		return 403
	case connect.CodeResourceExhausted:
		return 429
	case connect.CodeFailedPrecondition:
		return 400
	case connect.CodeAborted:
		return 409
	case connect.CodeOutOfRange:
		return 400
	case connect.CodeUnimplemented:
		return 501
	case connect.CodeInternal:
		return 500
	case connect.CodeUnavailable:
		return 503
	case connect.CodeDataLoss:
		return 500
	case connect.CodeUnauthenticated:
		return 401
	default:
		return 500
	}
}

// IsConnectError reports whether err is a *connect.Error (i.e. already
// structured). Used by the fallback interceptor to decide whether a raw error
// needs wrapping. Mirrors connect's own IsWireError pattern (errors.As).
func IsConnectError(err error) bool {
	ce := new(connect.Error)
	return stderrors.As(err, &ce)
}
