package errors

import (
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"gopkg.in/yaml.v3"
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

// Lookup returns the Entry for a code. Returns an error if the code is not
// registered — this is a programmer error (using an unregistered code), not
// a runtime condition, so callers MUST use the typed constants from codes.go.
func (r *Registry) Lookup(code string) (Entry, error) {
	e, ok := r.entries[code]
	if !ok {
		return Entry{}, fmt.Errorf("error code %q not registered in error-codes.yaml", code)
	}
	return e, nil
}

// MustLookup is Lookup but panics on unknown code. Use only with the typed
// constants from codes.go (which are guaranteed registered); never with
// user/external input.
func (r *Registry) MustLookup(code string) Entry {
	e, err := r.Lookup(code)
	if err != nil {
		panic(err)
	}
	return e
}

// New wraps an error code into a structured Connect error carrying an
// ErrorInfo detail. Handlers call this instead of connect.NewError directly:
//
//	return nil, reg.New(errors.CodeStateConflict, "version mismatch: %d", got)
//
// The returned *connect.Error has:
//   - Code: mapped from Entry.GRPCCode (e.g. "ABORTED" → connect.CodeAborted)
//   - underlying error: the formatted message
//   - ErrorInfo detail: reason = code, metadata = retryable/manual_required/owner
//
// Clients (connect-es / connect-go) read ErrorInfo to decide retry behavior,
// surface remediation, and route to the owning team.
func (r *Registry) New(code string, format string, a ...any) error {
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
//	return nil, reg.NewFromError(errors.CodeGitOperationFailed, err)
func (r *Registry) NewFromError(code string, err error) error {
	if err == nil {
		return nil
	}
	return r.New(code, "%s", err.Error())
}

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
