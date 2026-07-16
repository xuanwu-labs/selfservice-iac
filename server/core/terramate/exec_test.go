package terramate_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/terramate"
)

// fakeTerramateScript creates a temporary shell script that mimics terramate
// behavior for testing. Returns the absolute path to the script.
func fakeTerramateScript(t *testing.T, behavior string) string {
	t.Helper()
	dir := t.TempDir()
	var scriptPath string
	var content string

	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(dir, "terramate.bat")
		switch behavior {
		case "echo_stdout":
			content = "@echo off\necho stdout-output\nexit /b 0"
		case "echo_stderr":
			content = "@echo off\necho stderr-output 1>&2\nexit /b 0"
		case "exit_1":
			content = "@echo off\necho error-here 1>&2\nexit /b 1"
		case "sleep":
			content = "@echo off\nping -n 5 127.0.0.1 > nul\nexit /b 0"
		default:
			content = "@echo off\nexit /b 0"
		}
	} else {
		scriptPath = filepath.Join(dir, "terramate")
		switch behavior {
		case "echo_stdout":
			content = "#!/bin/sh\necho 'stdout-output'\n"
		case "echo_stderr":
			content = "#!/bin/sh\necho 'stderr-output' 1>&2\n"
		case "exit_1":
			content = "#!/bin/sh\necho 'error-here' 1>&2\nexit 1\n"
		case "sleep":
			content = "#!/bin/sh\nsleep 5\n"
		default:
			content = "#!/bin/sh\nexit 0\n"
		}
	}

	require.NoError(t, os.WriteFile(scriptPath, []byte(content), 0o755))
	return scriptPath
}

func TestExecAdapter_StdoutCapture(t *testing.T) {
	binary := fakeTerramateScript(t, "echo_stdout")
	adapter := terramate.NewExecAdapter(binary)

	result, err := adapter.Run(context.Background(), "", []string{"run"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "stdout-output")
}

func TestExecAdapter_StderrCapture(t *testing.T) {
	binary := fakeTerramateScript(t, "echo_stderr")
	adapter := terramate.NewExecAdapter(binary)

	result, err := adapter.Run(context.Background(), "", []string{"run"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stderr, "stderr-output")
}

func TestExecAdapter_ExitCode1(t *testing.T) {
	binary := fakeTerramateScript(t, "exit_1")
	adapter := terramate.NewExecAdapter(binary)

	result, err := adapter.Run(context.Background(), "", []string{"run"})
	require.Error(t, err)
	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "error-here")
}

func TestExecAdapter_WorkingDir(t *testing.T) {
	binary := fakeTerramateScript(t, "echo_stdout")
	workDir := t.TempDir()
	adapter := terramate.NewExecAdapter(binary)

	// The script echoes stdout; we verify the adapter runs without error
	// in the given directory. On non-Windows, we can check pwd via a custom script.
	if runtime.GOOS != "windows" {
		// Create a script that prints its working directory
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "terramate")
		content := "#!/bin/sh\npwd\n"
		require.NoError(t, os.WriteFile(scriptPath, []byte(content), 0o755))

		adapter := terramate.NewExecAdapter(scriptPath)
		result, err := adapter.Run(context.Background(), workDir, nil)
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(result.Stdout), workDir)
		return
	}

	// Windows fallback: just verify it runs
	_, err := adapter.Run(context.Background(), workDir, []string{"run"})
	require.NoError(t, err)
}

func TestExecAdapter_ContextCancellation(t *testing.T) {
	binary := fakeTerramateScript(t, "sleep")
	adapter := terramate.NewExecAdapter(binary)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := adapter.Run(ctx, "", []string{"run"})
	// Context cancellation may manifest as a killed-process error (exit code -1
	// on Windows, signal killed on Unix) rather than a bare context error.
	// The key assertion: the call returns an error and does not hang.
	require.Error(t, err)
	// On Unix, context.DeadlineExceeded is wrapped in the error chain.
	// On Windows, the process is killed and the error is "exit status 1" or
	// similar. We accept either — the important thing is it didn't hang.
	_ = result
}

func TestExecAdapter_BinaryNotFound(t *testing.T) {
	adapter := terramate.NewExecAdapter("/nonexistent/path/terramate")

	_, err := adapter.Run(context.Background(), "", []string{"version"})
	require.Error(t, err)

	// exec.Error or exec.ExitError — either way, it should mention the binary.
	// Use errors.As for safe type assertion.
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		assert.Contains(t, execErr.Name, "terramate")
		return
	}
	// On some platforms the error is wrapped differently; just verify non-nil.
	// (already asserted by require.Error above)
}
