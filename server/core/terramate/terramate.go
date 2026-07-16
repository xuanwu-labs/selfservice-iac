// Package terramate implements the TerramateAdapter (D1 exec boundary).
//
// The platform calls the terramate CLI binary as a subprocess — it MUST NOT
// import github.com/terramate-io/terramate internal packages (D1 boundary,
// enforced by the D1 guard test in server/internal/audit/).
//
// The orchestrator module (06) uses this adapter to run `terramate run`
// in stack directories, leveraging Terramate's DAG ordering, tag filtering,
// and per-stack exec isolation (D29).
package terramate

import (
	"context"
)

// Adapter abstracts terramate CLI operations so the platform can mock
// terramate in tests without depending on the real binary.
type Adapter interface {
	// Run executes a terramate command (e.g. "run --tags env:prod -- terraform plan")
	// in the given working directory (the stack dir or project root).
	Run(ctx context.Context, dir string, args []string) (RunResult, error)
	// Version returns the terramate CLI version string.
	Version(ctx context.Context) (string, error)
}

// RunResult captures the output of a terramate subprocess invocation.
// It is used by the orchestrator for logging, error attribution, and
// determining next-state transitions.
type RunResult struct {
	ExitCode   int    // process exit code (0 = success)
	Stdout     string // captured stdout
	Stderr     string // captured stderr
	DurationMs int64  // wall-clock duration in milliseconds
}
