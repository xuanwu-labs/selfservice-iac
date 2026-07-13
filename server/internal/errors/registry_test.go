package errors

import (
	"context"
	stderrors "errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/asset"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// realRegistry loads the actual embedded error-codes.yaml. The write-path tests
// (New/Lookup) now take proto ErrorCode enum values, so they must run against
// the real registry (which has rows matching the enum). Synthetic YAML can no
// longer be used for New/Lookup tests because the enum values are buf-generated
// constants bound to the real YAML keys.
func realRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load(real yaml): %v", err)
	}
	return reg
}

func TestLoad_EmptyYAML(t *testing.T) {
	if _, err := Load("errors: []"); err == nil {
		t.Fatal("Load(empty): want error, got nil")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	if _, err := Load("errors:\n  - code: [unclosed"); err == nil {
		t.Fatal("Load(malformed): want error, got nil")
	}
}

func TestLoad_DuplicateCode(t *testing.T) {
	dup := `
errors:
  - code: DUP
    http_status: 400
    grpc_code: INVALID_ARGUMENT
    retryable: false
    manual_required: false
    remediation: "x"
    owner: x
  - code: DUP
    http_status: 409
    grpc_code: ABORTED
    retryable: true
    manual_required: false
    remediation: "y"
    owner: y
`
	if _, err := Load(dup); err == nil {
		t.Fatal("Load(duplicate code): want error, got nil")
	}
}

func TestLookup_Found(t *testing.T) {
	reg := realRegistry(t)
	e, err := reg.Lookup(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !e.Retryable {
		t.Error("Retryable: want true for STATE_CONFLICT")
	}
}

func TestLookup_UnspecifiedHasNoEntry(t *testing.T) {
	reg := realRegistry(t)
	// UNSPECIFIED is a valid enum value but deliberately has no YAML row —
	// it's the zero/invalid sentinel. Lookup must return an error, not panic.
	if _, err := reg.Lookup(commonv1.ErrorCode_ERROR_CODE_UNSPECIFIED); err == nil {
		t.Fatal("Lookup(UNSPECIFIED): want error (no behavior row for sentinel)")
	}
}

func TestMustLookup_PanicsOnUnspecified(t *testing.T) {
	reg := realRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustLookup(UNSPECIFIED): want panic")
		}
	}()
	reg.MustLookup(commonv1.ErrorCode_ERROR_CODE_UNSPECIFIED)
}

func TestNew_StructuredConnectError(t *testing.T) {
	reg := realRegistry(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, "connection refused: %s", "db")
	if err == nil {
		t.Fatal("New: want non-nil error")
	}
	ce := new(connect.Error)
	if !connectError(err, &ce) {
		t.Fatalf("New: want *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeUnavailable {
		t.Errorf("Code: want CodeUnavailable, got %v", ce.Code())
	}
	details := ce.Details()
	if len(details) != 1 {
		t.Fatalf("Details: want 1, got %d", len(details))
	}
	msg, err := details[0].Value()
	if err != nil {
		t.Fatalf("detail Value: %v", err)
	}
	ei, ok := msg.(interface{ GetReason() string })
	if !ok {
		t.Fatalf("detail: want ErrorInfo, got %T", msg)
	}
	if got := ei.GetReason(); got != "PLATFORM_UNAVAILABLE" {
		t.Errorf("reason: want PLATFORM_UNAVAILABLE, got %s", got)
	}
}

func TestNewFromError_NilReturnsNil(t *testing.T) {
	reg := realRegistry(t)
	if err := reg.NewFromError(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, nil); err != nil {
		t.Errorf("NewFromError(nil): want nil, got %v", err)
	}
}

func TestNewFromError_Wraps(t *testing.T) {
	reg := realRegistry(t)
	err := reg.NewFromError(commonv1.ErrorCode_ERROR_CODE_GIT_OPERATION_FAILED, errOops)
	if !IsConnectError(err) {
		t.Fatal("NewFromError: want *connect.Error")
	}
}

func TestIsConnectError(t *testing.T) {
	reg := realRegistry(t)
	structured := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "bad")
	if !IsConnectError(structured) {
		t.Error("IsConnectError(structured): want true")
	}
	raw := connect.NewError(connect.CodeInternal, errOops)
	if !IsConnectError(raw) {
		t.Error("IsConnectError(raw connect): want true")
	}
	if IsConnectError(errOops) {
		t.Error("IsConnectError(plain): want false")
	}
}

func TestWrapInterceptor_PassesThroughStructured(t *testing.T) {
	reg := realRegistry(t)
	structured := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "bad")
	ic := WrapInterceptor(reg)
	next := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, structured
	})
	if _, err := next(nil, nil); err != structured {
		t.Error("WrapInterceptor should pass structured errors through unchanged")
	}
}

func TestWrapInterceptor_WrapsRawError(t *testing.T) {
	reg := realRegistry(t)
	ic := WrapInterceptor(reg)
	next := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, errOops // raw error
	})
	_, err := next(nil, nil)
	if !IsConnectError(err) {
		t.Fatal("WrapInterceptor(raw): want *connect.Error")
	}
	ce := new(connect.Error)
	if !connectError(err, &ce) {
		t.Fatal("WrapInterceptor(raw): not a *connect.Error")
	}
	if ce.Code() != connect.CodeInternal {
		t.Errorf("Code: want CodeInternal, got %v", ce.Code())
	}
}

func TestToConnectCode(t *testing.T) {
	cases := []struct {
		grpc string
		want connect.Code
	}{
		{"ABORTED", connect.CodeAborted},
		{"aborted", connect.CodeAborted}, // case-insensitive
		{"INVALID_ARGUMENT", connect.CodeInvalidArgument},
		{"NOT_FOUND", connect.CodeNotFound},
		{"UNAVAILABLE", connect.CodeUnavailable},
		{"UNAUTHENTICATED", connect.CodeUnauthenticated},
		{"PERMISSION_DENIED", connect.CodePermissionDenied},
		{"RESOURCE_EXHAUSTED", connect.CodeResourceExhausted},
		{"FAILED_PRECONDITION", connect.CodeFailedPrecondition},
		{"INTERNAL", connect.CodeInternal},
		{"BOGUS", connect.CodeUnknown}, // unknown → CodeUnknown (loud)
	}
	for _, c := range cases {
		if got := toConnectCode(c.grpc); got != c.want {
			t.Errorf("toConnectCode(%q): want %v, got %v", c.grpc, c.want, got)
		}
	}
}

// --- helpers ---

type oopsErr struct{}

func (oopsErr) Error() string { return "oops" }

var errOops error = oopsErr{}

// connectError mirrors connect's IsWireError pattern (errors.As into *Error).
func connectError(err error, target **connect.Error) bool {
	return stderrors.As(err, target)
}
