package errors

import (
	"testing"

	"connectrpc.com/connect"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/asset"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// realReg loads the actual embedded error-codes.yaml for read-path tests.
// The helpers inspect ErrorInfo on structured errors, so they need a real
// registry to produce errors whose metadata matches the YAML behavior rows.
func realReg(t *testing.T) *Registry {
	t.Helper()
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

func TestCodeOf_StructuredError(t *testing.T) {
	reg := realReg(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, "connection refused")
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
	// A connect.NewError built WITHOUT the registry has no ErrorInfo detail,
	// so CodeOf correctly returns false — the documented limitation: hand-built
	// structured errors carry no behavior metadata.
	raw := connect.NewError(connect.CodeInternal, errOops)
	if _, ok := CodeOf(raw); ok {
		t.Fatal("CodeOf(hand-built connect error): want ok=false (no ErrorInfo)")
	}
}

func TestIsCode_Match(t *testing.T) {
	reg := realReg(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "bad input")
	if !IsCode(err, "POLICY_VIOLATION") {
		t.Error("IsCode(matching): want true")
	}
}

func TestIsCode_NoMatch(t *testing.T) {
	reg := realReg(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "bad input")
	if IsCode(err, "STATE_CONFLICT") {
		t.Error("IsCode(different code): want false")
	}
}

func TestIsCode_RawError(t *testing.T) {
	if IsCode(errOops, "POLICY_VIOLATION") {
		t.Error("IsCode(raw error): want false")
	}
}

func TestIsRetryable(t *testing.T) {
	reg := realReg(t)
	// PLATFORM_UNAVAILABLE is retryable=true (grpc UNAVAILABLE); POLICY_VIOLATION is not.
	retryable := reg.New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, "x")
	fatal := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "x")
	if !IsRetryable(retryable) {
		t.Error("IsRetryable(retryable code): want true")
	}
	if IsRetryable(fatal) {
		t.Error("IsRetryable(fatal code): want false")
	}
	if IsRetryable(errOops) {
		t.Error("IsRetryable(raw error): want false")
	}
}

func TestManualRequired(t *testing.T) {
	reg := realReg(t)
	// MANUAL_INTERVENTION_REQUIRED has manual_required=true; POLICY_VIOLATION does not.
	manual := reg.New(commonv1.ErrorCode_ERROR_CODE_MANUAL_INTERVENTION_REQUIRED, "x")
	normal := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "x")
	if !ManualRequired(manual) {
		t.Error("ManualRequired(manual code): want true")
	}
	if ManualRequired(normal) {
		t.Error("ManualRequired(normal code): want false")
	}
}

func TestRemediation(t *testing.T) {
	reg := realReg(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, "x")
	if got := Remediation(err); got == "" {
		t.Error("Remediation: want non-empty (YAML has remediation text)")
	}
}

func TestOwner(t *testing.T) {
	reg := realReg(t)
	err := reg.New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, "x")
	if got := Owner(err); got == "" {
		t.Error("Owner: want non-empty (YAML has owner)")
	}
}

func TestRoundTrip_NewThenCodeOf(t *testing.T) {
	// The critical invariant: what reg.New writes, CodeOf reads back.
	// If this breaks, the write/read paths drifted (e.g. ErrorInfo schema
	// changed in registry.go but helpers.go wasn't updated).
	reg := realReg(t)
	code := commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	err := reg.New(code, "detail %d", 42)
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
