package repo_test

import (
	"strings"
	"testing"
)

// envSuffix generates a unique suffix for test logical IDs to avoid collisions
// across tests in the same DB (GetByLogicalId uniqueness). Based on t.Name() so
// it's stable under parallel subtests and doesn't depend on global mutable state.
func envSuffix(t *testing.T) string {
	t.Helper()
	// Sanitize t.Name(): replace "/" (subtest separator) and spaces, lowercase.
	name := strings.ToLower(t.Name())
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	// Keep it short: take last 40 chars to stay within typical slug limits.
	if len(name) > 40 {
		name = name[len(name)-40:]
	}
	return name
}
