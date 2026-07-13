package errors

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
)

// minimalYAML is a tiny registry for unit tests (independent of the real
// error-codes.yaml so tests don't break when codes are added/removed).
const minimalYAML = `
errors:
  - code: TEST_RETRYABLE
    http_status: 503
    grpc_code: UNAVAILABLE
    retryable: true
    manual_required: false
    remediation: "Retry with backoff."
    owner: platform
  - code: TEST_FATAL
    http_status: 400
    grpc_code: INVALID_ARGUMENT
    retryable: false
    manual_required: false
    remediation: "Fix the input."
    owner: core/test
  - code: INTERNAL_ERROR
    http_status: 500
    grpc_code: INTERNAL
    retryable: false
    manual_required: false
    remediation: "Contact platform team with correlation_id."
    owner: platform
`

func TestLoad_ParsesEntries(t *testing.T) {
	reg, err := Load(minimalYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(reg.entries); got != 3 {
		t.Fatalf("entries: want 3, got %d", got)
	}
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
	reg := mustLoad(t, minimalYAML)
	e, err := reg.Lookup("TEST_RETRYABLE")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !e.Retryable {
		t.Error("Retryable: want true")
	}
	if e.HTTPStatus != 503 {
		t.Errorf("HTTPStatus: want 503, got %d", e.HTTPStatus)
	}
}

func TestLookup_NotFound(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	if _, err := reg.Lookup("NOPE"); err == nil {
		t.Fatal("Lookup(unknown): want error, got nil")
	}
}

func TestMustLookup_PanicsOnUnknown(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustLookup(unknown): want panic")
		}
	}()
	reg.MustLookup("NOPE")
}

func TestNew_StructuredConnectError(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_RETRYABLE", "connection refused: %s", "db")
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
	// Verify the detail is an ErrorInfo carrying reason + metadata.
	msg, err := details[0].Value()
	if err != nil {
		t.Fatalf("detail Value: %v", err)
	}
	ei, ok := msg.(interface{ GetReason() string })
	if !ok {
		t.Fatalf("detail: want ErrorInfo, got %T", msg)
	}
	if got := ei.GetReason(); got != "TEST_RETRYABLE" {
		t.Errorf("reason: want TEST_RETRYABLE, got %s", got)
	}
}

func TestNew_UnknownCodePanics(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New(unknown): want panic")
		}
	}()
	_ = reg.New("NOPE", "x")
}

func TestIsConnectError(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	structured := reg.New("TEST_FATAL", "bad input")
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
	reg := mustLoad(t, minimalYAML)
	structured := reg.New("TEST_FATAL", "bad")
	ic := WrapInterceptor(reg)
	next := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, structured
	})
	if _, err := next(nil, nil); err != structured {
		t.Error("WrapInterceptor should pass structured errors through unchanged")
	}
}

func TestWrapInterceptor_WrapsRawError(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
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

func mustLoad(t *testing.T, yaml string) *Registry {
	t.Helper()
	reg, err := Load(yaml)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

// connectError mirrors connect's IsWireError pattern (errors.As into *Error).
func connectError(err error, target **connect.Error) bool {
	return errors.As(err, target)
}
