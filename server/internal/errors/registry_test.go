package errors

import (
	"context"
	stderrors "errors"
	"testing"

	"connectrpc.com/connect"

	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

func TestNew_StructuredConnectError(t *testing.T) {
	err := New(commonv1.ErrorCode_ERROR_CODE_PLATFORM_UNAVAILABLE, connect.CodeUnavailable, "connection refused: %s", "db")
	if err == nil {
		t.Fatal("New: want non-nil error")
	}
	ce := new(connect.Error)
	if !stderrors.As(err, &ce) {
		t.Fatalf("New: want *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeUnavailable {
		t.Errorf("Code: want CodeUnavailable, got %v", ce.Code())
	}
	details := ce.Details()
	if len(details) != 1 {
		t.Fatalf("Details: want 1, got %d", len(details))
	}
	msg, derr := details[0].Value()
	if derr != nil {
		t.Fatalf("detail Value: %v", derr)
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
	if err := NewFromError(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal, nil); err != nil {
		t.Errorf("NewFromError(nil): want nil, got %v", err)
	}
}

func TestNewFromError_Wraps(t *testing.T) {
	err := NewFromError(commonv1.ErrorCode_ERROR_CODE_GIT_OPERATION_FAILED, connect.CodeUnavailable, errOops)
	if !IsConnectError(err) {
		t.Fatal("NewFromError: want *connect.Error")
	}
}

func TestIsConnectError(t *testing.T) {
	structured := New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, connect.CodeFailedPrecondition, "bad")
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
	structured := New(commonv1.ErrorCode_ERROR_CODE_POLICY_VIOLATION, connect.CodeFailedPrecondition, "bad")
	ic := WrapInterceptor()
	next := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, structured
	})
	if _, err := next(nil, nil); err != structured {
		t.Error("WrapInterceptor should pass structured errors through unchanged")
	}
}

func TestWrapInterceptor_WrapsRawError(t *testing.T) {
	ic := WrapInterceptor()
	next := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, errOops // raw error
	})
	_, err := next(nil, nil)
	if !IsConnectError(err) {
		t.Fatal("WrapInterceptor(raw): want *connect.Error")
	}
	ce := new(connect.Error)
	if !stderrors.As(err, &ce) {
		t.Fatal("WrapInterceptor(raw): not a *connect.Error")
	}
	if ce.Code() != connect.CodeInternal {
		t.Errorf("Code: want CodeInternal, got %v", ce.Code())
	}
}

func TestIsRetryable(t *testing.T) {
	retryable := []connect.Code{
		connect.CodeUnavailable,
		connect.CodeResourceExhausted,
		connect.CodeAborted,
		connect.CodeDeadlineExceeded,
	}
	for _, c := range retryable {
		if !IsRetryable(c) {
			t.Errorf("IsRetryable(%v): want true", c)
		}
	}
	notRetryable := []connect.Code{
		connect.CodeInvalidArgument,
		connect.CodeNotFound,
		connect.CodePermissionDenied,
		connect.CodeInternal,
		connect.CodeUnauthenticated,
	}
	for _, c := range notRetryable {
		if IsRetryable(c) {
			t.Errorf("IsRetryable(%v): want false", c)
		}
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		code connect.Code
		want int
	}{
		{connect.CodeNotFound, 404},
		{connect.CodeInvalidArgument, 400},
		{connect.CodePermissionDenied, 403},
		{connect.CodeUnauthenticated, 401},
		{connect.CodeAlreadyExists, 409},
		{connect.CodeAborted, 409},
		{connect.CodeResourceExhausted, 429},
		{connect.CodeInternal, 500},
		{connect.CodeUnavailable, 503},
		{connect.CodeDeadlineExceeded, 504},
		{connect.CodeUnimplemented, 501},
	}
	for _, c := range cases {
		if got := HTTPStatus(c.code); got != c.want {
			t.Errorf("HTTPStatus(%v): want %d, got %d", c.code, c.want, got)
		}
	}
}

func TestEnumToKey(t *testing.T) {
	cases := []struct {
		code commonv1.ErrorCode
		want string
	}{
		{commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, "STATE_CONFLICT"},
		{commonv1.ErrorCode_ERROR_CODE_CATALOG_ITEM_NOT_FOUND, "CATALOG_ITEM_NOT_FOUND"},
		{commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		if got := enumToKey(c.code); got != c.want {
			t.Errorf("enumToKey(%v): want %q, got %q", c.code, c.want, got)
		}
	}
}

// --- helpers ---

type oopsErr struct{}

func (oopsErr) Error() string { return "oops" }

var errOops error = oopsErr{}
