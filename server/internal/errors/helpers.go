package errors

import (
	stderrors "errors"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// This file provides package-level read-path helpers that mirror ferret's
// internal/errors convenience API (Is / As / Reason). They inspect errors that
// were already structured via New — no Registry/enum needed on the read path,
// so core/ domain code can reason about error codes without the write-path
// dependencies:
//
//	if errors.IsCode(err, "STATE_CONFLICT") { ... retry ... }
//
// The handler structured the error via New (enum + connect.Code); the domain
// layer inspects it via these free functions by the ErrorInfo reason string.

// CodeOf extracts the ErrorCode key (the ErrorInfo reason) from a structured
// *connect.Error. Returns ("", false) if the error is not structured via New
// (e.g. a raw Go error or a hand-built connect.NewError without ErrorInfo).
//
// Mirrors ferret's errors.Reason / kratos's errors.Reason.
func CodeOf(err error) (string, bool) {
	ce := new(connect.Error)
	if !stderrors.As(err, &ce) {
		return "", false
	}
	for _, d := range ce.Details() {
		msg, derr := d.Value()
		if derr != nil {
			continue
		}
		if ei, ok := msg.(*errdetails.ErrorInfo); ok {
			return ei.GetReason(), true
		}
	}
	return "", false
}

// IsCode reports whether err is a structured error whose ErrorCode key matches
// the supplied key. Domain logic branches on semantic code, not on gRPC Code:
//
//	if errors.IsCode(err, "STATE_CONFLICT") {
//	    return retryWithFreshVersion()
//	}
//
// Returns false for non-structured errors so they never accidentally match.
func IsCode(err error, key string) bool {
	got, ok := CodeOf(err)
	return ok && got == key
}

// GRPCCodeOf extracts the connect.Code from a structured *connect.Error. Useful
// when domain logic needs the transport semantic (e.g. to decide retry via
// IsRetryable). Returns (0, false) for non-structured errors.
func GRPCCodeOf(err error) (connect.Code, bool) {
	ce := new(connect.Error)
	if !stderrors.As(err, &ce) {
		return 0, false
	}
	return ce.Code(), true
}

// Is / As / Unwrap are re-exported from the standard errors package so callers
// can import a single "errors" package (this one) for both platform error codes
// and standard error chaining. This matches ferret's convenience re-export.
var (
	Is     = stderrors.Is
	As     = stderrors.As
	Unwrap = stderrors.Unwrap
)
