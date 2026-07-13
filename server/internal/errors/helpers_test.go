package errors

import (
	"testing"

	"connectrpc.com/connect"

	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

func TestCodeOf_StructuredError(t *testing.T) {
	err := New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, connect.CodeUnavailable, "connection refused")
	code, ok := CodeOf(err)
	if !ok {
		t.Fatal("CodeOf(structured): want ok=true")
	}
	if code != "PLATFORM_UNAVAILABLE" {
		t.Errorf("code: want PLATFORM_UNAVAILABLE, got %s", code)
	}
}

func TestCodeOf_RawGoError(t *testing.T) {
	code, ok := CodeOf(errOops)
	if ok {
		t.Fatal("CodeOf(raw error): want ok=false")
	}
	if code != "" {
		t.Errorf("code: want empty, got %s", code)
	}
}

func TestCodeOf_HandBuiltConnectError(t *testing.T) {
	// A connect.NewError built WITHOUT New has no ErrorInfo detail, so CodeOf
	// correctly returns false — the documented limitation.
	raw := connect.NewError(connect.CodeInternal, errOops)
	if _, ok := CodeOf(raw); ok {
		t.Fatal("CodeOf(hand-built connect error): want ok=false (no ErrorInfo)")
	}
}

func TestIsCode_Match(t *testing.T) {
	err := New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, connect.CodeFailedPrecondition, "bad input")
	if !IsCode(err, "POLICY_VIOLATION") {
		t.Error("IsCode(matching): want true")
	}
}

func TestIsCode_NoMatch(t *testing.T) {
	err := New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, connect.CodeFailedPrecondition, "bad input")
	if IsCode(err, "STATE_CONFLICT") {
		t.Error("IsCode(different code): want false")
	}
}

func TestIsCode_RawError(t *testing.T) {
	if IsCode(errOops, "POLICY_VIOLATION") {
		t.Error("IsCode(raw error): want false")
	}
}

func TestGRPCCodeOf(t *testing.T) {
	err := New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, connect.CodeAborted, "version mismatch")
	code, ok := GRPCCodeOf(err)
	if !ok {
		t.Fatal("GRPCCodeOf(structured): want ok=true")
	}
	if code != connect.CodeAborted {
		t.Errorf("code: want CodeAborted, got %v", code)
	}
	if _, ok := GRPCCodeOf(errOops); ok {
		t.Error("GRPCCodeOf(raw): want ok=false")
	}
}

func TestRoundTrip_NewThenCodeOf(t *testing.T) {
	// Critical invariant: what New writes, CodeOf reads back.
	code := commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	err := New(code, connect.CodeAborted, "detail %d", 42)
	got, ok := CodeOf(err)
	want := enumToKey(code)
	if !ok || got != want {
		t.Errorf("round-trip: CodeOf got (%q, %v), want %q", got, ok, want)
	}
}

func TestReExportedStdErrors(t *testing.T) {
	// Is/As/Unwrap are re-exported std errors; verify they work on plain errors.
	wrapped := wrapOps("context")
	if !Is(wrapped, errOops) {
		t.Error("re-exported Is: want true for wrapped error")
	}
}

// wrapOps mimics fmt.Errorf("...: %w", err) for the Is re-export test.
type wrapped struct{ inner error }

func (w wrapped) Error() string { return "context: " + w.inner.Error() }
func (w wrapped) Unwrap() error { return w.inner }

func wrapOps(msg string) error { return wrapped{inner: errOops} }
