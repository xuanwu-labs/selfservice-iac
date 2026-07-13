package errors

import (
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"gopkg.in/yaml.v3"

	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// Entry is one registered error code's full behavior. Mirrors a row in
// contracts/error-codes.yaml. proto cannot express these fields, so they
// live in YAML and are loaded here at startup.
type Entry struct {
	Code           string `yaml:"code"`
	HTTPStatus     int    `yaml:"http_status"`
	GRPCCode       string `yaml:"grpc_code"`
	Retryable      bool   `yaml:"retryable"`
	ManualRequired bool   `yaml:"manual_required"`
	Remediation    string `yaml:"remediation"`
	Owner          string `yaml:"owner"`
}

// Registry is the in-memory error-code registry, loaded once at startup from
// the embedded error-codes.yaml. It is the single runtime source of truth for
// error behavior (HTTP/gRPC code, retryability, manual intervention,
// remediation, owner). Handlers receive a *Registry via wire injection and
// call New/Lookup instead of hardcoding status/code/message.
type Registry struct {
	entries map[string]Entry
}

type yamlConfig struct {
	Errors []Entry `yaml:"errors"`
}

// Load parses the error-codes.yaml content into a Registry. Called once at
// startup (wire provider); the yaml string comes from internal/asset embed.
func Load(yamlContent string) (*Registry, error) {
	var cfg yamlConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &cfg); err != nil {
		return nil, fmt.Errorf("parse error-codes.yaml: %w", err)
	}
	if len(cfg.Errors) == 0 {
		return nil, fmt.Errorf("error-codes.yaml: no error entries found")
	}
	entries := make(map[string]Entry, len(cfg.Errors))
	for _, e := range cfg.Errors {
		if e.Code == "" {
			return nil, fmt.Errorf("error-codes.yaml: entry with empty code")
		}
		if _, dup := entries[e.Code]; dup {
			return nil, fmt.Errorf("error-codes.yaml: duplicate code %q", e.Code)
		}
		entries[e.Code] = e
	}
	return &Registry{entries: entries}, nil
}

// enumToKey converts a proto ErrorCode enum value to its YAML registry key by
// stripping the ERROR_CODE_ prefix. ErrorCode_ERROR_CODE_SCHEMA_INVALID →
// "SCHEMA_INVALID". This is the convention binding the proto enum (code
// identity, buf-generated) to the YAML registry (code behavior).
func enumToKey(code commonv1.ErrorCode) string {
	return strings.TrimPrefix(code.String(), "ERROR_CODE_")
}

// Lookup returns the Entry for a proto ErrorCode. Returns an error if the
// code is not registered in error-codes.yaml — this is a programmer error
// (enum value exists but no YAML behavior row), not a runtime condition.
func (r *Registry) Lookup(code commonv1.ErrorCode) (Entry, error) {
	e, ok := r.entries[enumToKey(code)]
	if !ok {
		return Entry{}, fmt.Errorf("error code %s not registered in error-codes.yaml", code.String())
	}
	return e, nil
}

// lookupByKey is the string-key lookup used by helpers that inspect errors
// already carrying a reason string (CodeOf/IsCode read path). The reason in
// ErrorInfo is the YAML key (e.g. "STATE_CONFLICT"), not the enum value name.
func (r *Registry) lookupByKey(key string) (Entry, bool) {
	e, ok := r.entries[key]
	return e, ok
}

// MustLookup is Lookup but panics on unknown code. Use only with proto enum
// values (which are buf-generated and type-safe); never with user input.
func (r *Registry) MustLookup(code commonv1.ErrorCode) Entry {
	e, err := r.Lookup(code)
	if err != nil {
		panic(err)
	}
	return e
}

// New wraps a proto ErrorCode into a structured Connect error carrying an
// ErrorInfo detail. Handlers call this instead of connect.NewError directly:
//
//	return nil, reg.New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, "version mismatch: %d", got)
//
// The returned *connect.Error has:
//   - Code: mapped from Entry.GRPCCode (e.g. "ABORTED" → connect.CodeAborted)
//   - underlying error: the formatted message
//   - ErrorInfo detail: reason = YAML key, metadata = retryable/manual_required/owner
//
// Clients (connect-es / connect-go) read ErrorInfo to decide retry behavior,
// surface remediation, and route to the owning team.
func (r *Registry) New(code commonv1.ErrorCode, format string, a ...any) error {
	e := r.MustLookup(code)
	msg := fmt.Sprintf(format, a...)
	connectErr := connect.NewError(toConnectCode(e.GRPCCode), fmt.Errorf("%s", msg))
	detail, err := connect.NewErrorDetail(&errdetails.ErrorInfo{
		Reason: e.Code,
		Domain: "aether.platform",
		Metadata: map[string]string{
			"http_status":     strconv.Itoa(e.HTTPStatus),
			"retryable":       strconv.FormatBool(e.Retryable),
			"manual_required": strconv.FormatBool(e.ManualRequired),
			"remediation":     e.Remediation,
			"owner":           e.Owner,
		},
	})
	if err != nil {
		// NewErrorDetail only fails on nil message; errdetails.ErrorInfo is never nil.
		// If it somehow fails, return the structured error without the detail —
		// better than dropping the error entirely.
		return connectErr
	}
	connectErr.AddDetail(detail)
	return connectErr
}

// NewFromError wraps a raw Go error with a code's behavior. Use when an
// underlying error (e.g. from data layer) needs to be surfaced as a typed
// platform error:
//
//	return nil, reg.NewFromError(commonv1.ErrorCode_ERROR_CODE_GIT_OPERATION_FAILED, err)
func (r *Registry) NewFromError(code commonv1.ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return r.New(code, "%s", err.Error())
}

// EntryByKey exposes a YAML-key lookup for the contract test that iterates
// the registry. Not for handler use (handlers use the typed enum methods).
func (r *Registry) EntryByKey(key string) (Entry, bool) {
	return r.lookupByKey(key)
}

// NumEntries returns the count of registered error codes.
func (r *Registry) NumEntries() int { return len(r.entries) }

// toConnectCode maps a YAML grpc_code string (e.g. "ABORTED",
// "INVALID_ARGUMENT") to connect.Code. Unknown strings map to CodeUnknown
// so a typo in YAML degrades loudly (wrong code) rather than silently.
func toConnectCode(grpcCode string) connect.Code {
	switch strings.ToUpper(grpcCode) {
	case "CANCELLED", "CANCELED":
		return connect.CodeCanceled
	case "UNKNOWN":
		return connect.CodeUnknown
	case "INVALID_ARGUMENT":
		return connect.CodeInvalidArgument
	case "DEADLINE_EXCEEDED":
		return connect.CodeDeadlineExceeded
	case "NOT_FOUND":
		return connect.CodeNotFound
	case "ALREADY_EXISTS":
		return connect.CodeAlreadyExists
	case "PERMISSION_DENIED":
		return connect.CodePermissionDenied
	case "RESOURCE_EXHAUSTED":
		return connect.CodeResourceExhausted
	case "FAILED_PRECONDITION":
		return connect.CodeFailedPrecondition
	case "ABORTED":
		return connect.CodeAborted
	case "OUT_OF_RANGE":
		return connect.CodeOutOfRange
	case "UNIMPLEMENTED":
		return connect.CodeUnimplemented
	case "INTERNAL":
		return connect.CodeInternal
	case "UNAVAILABLE":
		return connect.CodeUnavailable
	case "DATA_LOSS":
		return connect.CodeDataLoss
	case "UNAUTHENTICATED":
		return connect.CodeUnauthenticated
	case "OK":
		// An OK error is nonsensical; fall through to Unknown.
		fallthrough
	default:
		return connect.CodeUnknown
	}
}

// IsConnectError reports whether err is a *connect.Error (i.e. already
// structured by the registry). Used by the fallback interceptor to decide
// whether a raw error needs wrapping. Mirrors connect's own IsWireError
// pattern (errors.As into *connect.Error).
func IsConnectError(err error) bool {
	ce := new(connect.Error)
	return stderrors.As(err, &ce)
}
