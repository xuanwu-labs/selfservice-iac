package errors

import (
	stderrors "errors"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// This file provides package-level helpers that mirror ferret's internal/errors
// convenience API (Is / As / Reason). Unlike Registry methods (New/Lookup),
// these do NOT need a *Registry instance — they inspect errors that were
// already structured. This lets core/ domain code reason about error codes
// without depending on the Registry (which lives at the wire/handler layer):
//
//	if errors.IsCode(err, errors.CodeStateConflict) { ... retry ... }
//
// The handler structured the error via reg.New; the domain layer inspects it
// via these free functions. No Registry needed on the read path.

// CodeOf extracts the registered error code (the ErrorInfo reason) from a
// structured *connect.Error. Returns ("", false) if the error is not a
// registry-structured error (e.g. a raw Go error or a hand-built
// connect.NewError without ErrorInfo).
//
// Mirrors ferret's errors.Reason.
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

// IsCode reports whether err is a structured error whose registered code
// matches code. This is the typed equivalent of errors.Is for platform error
// codes — domain logic branches on semantic code, not on gRPC Code or message:
//
//	if errors.IsCode(err, errors.CodeStateConflict) {
//	    return retryWithFreshVersion()
//	}
//
// Returns false for non-structured errors (raw Go errors, hand-built
// connect.NewError) so they never accidentally match a code check.
func IsCode(err error, code string) bool {
	got, ok := CodeOf(err)
	return ok && got == code
}

// IsRetryable reports whether err carries retryable=true in its ErrorInfo
// metadata. Non-structured errors return false (treat as non-retryable by
// default — safer than retrying an unknown error).
func IsRetryable(err error) bool {
	return boolMeta(err, "retryable")
}

// ManualRequired reports whether err carries manual_required=true. Domain
// logic / orchestrator uses this to decide whether to create a
// manual_intervention_task rather than silently failing.
func ManualRequired(err error) bool {
	return boolMeta(err, "manual_required")
}

// Remediation extracts the remediation hint from a structured error, if any.
// Returns "" for non-structured errors.
func Remediation(err error) string {
	meta := errorMeta(err)
	return meta["remediation"]
}

// Owner extracts the owning team from a structured error, if any.
// Returns "" for non-structured errors.
func Owner(err error) string {
	meta := errorMeta(err)
	return meta["owner"]
}

// --- internal helpers ---

// errorMeta pulls the ErrorInfo metadata map out of a structured error.
func errorMeta(err error) map[string]string {
	ce := new(connect.Error)
	if !stderrors.As(err, &ce) {
		return nil
	}
	for _, d := range ce.Details() {
		msg, derr := d.Value()
		if derr != nil {
			continue
		}
		if ei, ok := msg.(*errdetails.ErrorInfo); ok {
			return ei.GetMetadata()
		}
	}
	return nil
}

// boolMeta reads a boolean metadata key; false if absent or non-structured.
func boolMeta(err error, key string) bool {
	meta := errorMeta(err)
	if meta == nil {
		return false
	}
	return meta[key] == "true"
}

// Is / As / Unwrap are re-exported from the standard errors package so callers
// can import a single "errors" package (this one) for both platform error
// codes and standard error chaining. This matches ferret's convenience
// re-export pattern (its internal/errors exposes Is/As too).
//
// For STRUCTURED platform errors use reg.New (Registry method) on the write
// path; these re-exports are for the read/inspect path and plain error needs.
var (
	Is     = stderrors.Is
	As     = stderrors.As
	Unwrap = stderrors.Unwrap
)
