package errors

import (
	"testing"

	"connectrpc.com/connect"
)

func TestCodeOf_StructuredError(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_RETRYABLE", "connection refused")
	code, ok := CodeOf(err)
	if !ok {
		t.Fatal("CodeOf(structured): want ok=true")
	}
	if code != "TEST_RETRYABLE" {
		t.Errorf("code: want TEST_RETRYABLE, got %s", code)
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
	// so CodeOf correctly returns false — this is the documented limitation:
	// hand-built structured errors are indistinguishable to the safety net.
	raw := connect.NewError(connect.CodeInternal, errOops)
	if _, ok := CodeOf(raw); ok {
		t.Fatal("CodeOf(hand-built connect error): want ok=false (no ErrorInfo)")
	}
}

func TestIsCode_Match(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_FATAL", "bad input")
	if !IsCode(err, "TEST_FATAL") {
		t.Error("IsCode(matching): want true")
	}
}

func TestIsCode_NoMatch(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_FATAL", "bad input")
	if IsCode(err, "TEST_RETRYABLE") {
		t.Error("IsCode(different code): want false")
	}
}

func TestIsCode_RawError(t *testing.T) {
	if IsCode(errOops, "TEST_FATAL") {
		t.Error("IsCode(raw error): want false")
	}
}

func TestIsRetryable(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	retryable := reg.New("TEST_RETRYABLE", "x") // retryable: true
	fatal := reg.New("TEST_FATAL", "x")         // retryable: false
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
	reg := mustLoad(t, realManualRegistry(t))
	manual := reg.New("TEST_MANUAL", "x")
	normal := reg.New("TEST_FATAL", "x")
	if !ManualRequired(manual) {
		t.Error("ManualRequired(manual code): want true")
	}
	if ManualRequired(normal) {
		t.Error("ManualRequired(normal code): want false")
	}
}

func TestRemediation(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_RETRYABLE", "x")
	if got := Remediation(err); got != "Retry with backoff." {
		t.Errorf("Remediation: want 'Retry with backoff.', got %q", got)
	}
}

func TestOwner(t *testing.T) {
	reg := mustLoad(t, minimalYAML)
	err := reg.New("TEST_RETRYABLE", "x")
	if got := Owner(err); got != "platform" {
		t.Errorf("Owner: want 'platform', got %q", got)
	}
}

func TestRoundTrip_NewThenCodeOf(t *testing.T) {
	// The critical invariant: what reg.New writes, CodeOf reads back.
	// If this breaks, the write/read paths drifted (e.g. ErrorInfo schema
	// changed in registry.go but helpers.go wasn't updated).
	reg := mustLoad(t, minimalYAML)
	for _, code := range []string{"TEST_RETRYABLE", "TEST_FATAL", "INTERNAL_ERROR"} {
		err := reg.New(code, "detail %d", 42)
		got, ok := CodeOf(err)
		if !ok || got != code {
			t.Errorf("round-trip %q: CodeOf got (%q, %v)", code, got, ok)
		}
	}
}

func TestReExportedStdErrors(t *testing.T) {
	// Is/As/Unwrap are re-exported std errors; verify they work on plain errors.
	wrapped := wrapOps("context")
	if !Is(wrapped, errOops) {
		t.Error("re-exported Is: want true for wrapped error")
	}
}

// realManualRegistry provides a registry with a manual_required code for the
// ManualRequired test (minimalYAML has none).
func realManualRegistry(t *testing.T) string {
	t.Helper()
	return `
errors:
  - code: TEST_MANUAL
    http_status: 409
    grpc_code: FAILED_PRECONDITION
    retryable: false
    manual_required: true
    remediation: "Needs human action."
    owner: ops
  - code: TEST_FATAL
    http_status: 400
    grpc_code: INVALID_ARGUMENT
    retryable: false
    manual_required: false
    remediation: "Fix input."
    owner: core
  - code: INTERNAL_ERROR
    http_status: 500
    grpc_code: INTERNAL
    retryable: false
    manual_required: false
    remediation: "Contact platform."
    owner: platform
`
}

// wrapOops mimics fmt.Errorf("...: %w", err) for the Is re-export test.
type wrapped struct{ inner error }

func (w wrapped) Error() string { return "context: " + w.inner.Error() }
func (w wrapped) Unwrap() error { return w.inner }

func wrapOps(msg string) error { return wrapped{inner: errOops} }
