// Package e2e provides end-to-end tests for the Aether platform.
//
// These tests verify the full pipeline using null_resource + random_id providers
// (Terraform built-in, zero cloud credentials) + local backend (no S3/OSS).
// This matches the industry best practice used by Atlantis and Terramate's own
// test suites.
//
// Tests are skipped in -short mode or when terramate/terraform CLI is not available.
package e2e

import (
	"os/exec"
	"testing"
)

// checkCLIs verifies that terramate and terraform are installed.
// Returns true if available, false to skip.
func checkCLIs(t *testing.T) bool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping e2e test in -short mode")
		return false
	}
	for _, cli := range []string{"terramate", "terraform"} {
		if _, err := exec.LookPath(cli); err != nil {
			t.Skipf("skipping e2e: %s CLI not found in PATH", cli)
			return false
		}
	}
	return true
}
