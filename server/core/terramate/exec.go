package terramate

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// ExecAdapter is the production implementation of Adapter.
// It shells out to the terramate binary, never importing terramate Go packages.
type ExecAdapter struct {
	binaryPath string // path to terramate binary; default "terramate"
}

// NewExecAdapter creates an ExecAdapter with the given binary path.
// If binaryPath is empty, it defaults to "terramate" (resolved via PATH).
func NewExecAdapter(binaryPath string) *ExecAdapter {
	if binaryPath == "" {
		binaryPath = "terramate"
	}
	return &ExecAdapter{binaryPath: binaryPath}
}

// Run executes the terramate binary with the given args in dir.
func (a *ExecAdapter) Run(ctx context.Context, dir string, args []string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, a.binaryPath, args...)
	cmd.Dir = dir // D29: run in stack directory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	result := RunResult{
		ExitCode:   exitCode(err),
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: duration,
	}
	return result, err
}

// Version runs `terramate version` and returns the trimmed output.
func (a *ExecAdapter) Version(ctx context.Context) (string, error) {
	result, err := a.Run(ctx, "", []string{"version"})
	if err != nil && result.ExitCode == -1 {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// exitCode extracts the process exit code from an exec error.
// Returns -1 if the error is not an ExitError (e.g. binary not found).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
