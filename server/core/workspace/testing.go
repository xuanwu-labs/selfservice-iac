package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

// This file exposes small "for-test" hooks on Manager. They are NOT part of
// the public API; they exist so external _test packages (which can't reach
// unexported fields) can drive the Manager into a known on-disk state. They
// are deliberately tiny and self-documenting so they read as test fixtures.
//
// (We put them here rather than in manager.go so the production source file
// isn't cluttered with test-only methods; the file name `testing.go` is a
// convention flag that these are test scaffolding.)

// WorktreeRootForTest returns the configured worktreeRoot. Tests use it to
// build expected paths without re-deriving the layout.
func (m *Manager) WorktreeRootForTest() string { return m.worktreeRoot }

// NodeIDForTest returns the configured nodeID (used in assertions).
func (m *Manager) NodeIDForTest() string { return m.nodeID }

// SeedRepoForTest clones remoteURL into the workspace's primary repo dir,
// mimicking what a first-call ensureRepo(remoteURL != "") would do. Tests use
// it to set up the SquashMergeAndPush flow's starting point without relying on
// the internal ensureRepo path. Fails the test on any git error.
func (m *Manager) SeedRepoForTest(t *testing.T, workspaceName, remoteURL string) {
	t.Helper()
	repoDir := m.repoPath(workspaceName)
	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		t.Fatalf("mkdir parent of %s: %v", repoDir, err)
	}
	if _, err := git.PlainClone(repoDir, true, &git.CloneOptions{URL: remoteURL}); err != nil {
		t.Fatalf("seed repo clone %s <- %s: %v", repoDir, remoteURL, err)
	}
}

// FetchRepoForTest is a thin re-export of the internal fetchRepo helper so
// tests can drive it directly when needed (kept for symmetry with
// SeedRepoForTest; not all suites use it).
func (m *Manager) FetchRepoForTest(ctx context.Context, repoDir, remoteURL string) error {
	return fetchRepo(ctx, repoDir, remoteURL)
}
