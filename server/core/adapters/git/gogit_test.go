package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/git"
)

// makeLocalRepo creates a bare repo in a temp dir, commits one file, and
// returns its file:// URL + the expected HEAD commit SHA. Used to test
// GoGitProvider without network access.
func makeLocalRepo(t *testing.T) (fileURL string, wantSHA string) {
	t.Helper()
	work := t.TempDir()
	repo, err := gogit.PlainInit(work, false)
	require.NoError(t, err)

	// Write a file + commit so HEAD is non-empty.
	err = os.WriteFile(filepath.Join(work, "README.md"), []byte("# test module"), 0o644)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)

	sig := object.Signature{Name: "test", Email: "test@example.com"}
	_, err = wt.Commit("initial", &gogit.CommitOptions{Author: &sig, Committer: &sig})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	// Use the work dir directly as a file:// source (go-git PlainClone can
	// read from a non-bare working repo for test purposes).
	return "file:///" + filepath.ToSlash(work), head.Hash().String()
}

// TestGoGitProvider_CloneAndSHA verifies Clone fetches the repo and CommitSHA
// resolves the HEAD commit. Uses a local file:// bare repo (no network).
func TestGoGitProvider_CloneAndSHA(t *testing.T) {
	srcURL, wantSHA := makeLocalRepo(t)
	p := git.NewGoGitProvider()
	ctx := context.Background()

	dest := t.TempDir()
	err := p.Clone(ctx, srcURL, "HEAD", dest)
	require.NoError(t, err, "Clone should succeed against local bare repo")

	// The cloned repo must be a normal (non-bare) worktree.
	_, err = os.Stat(filepath.Join(dest, "README.md"))
	assert.NoError(t, err, "cloned worktree should contain the committed file")

	gotSHA, err := p.CommitSHA(ctx, dest)
	require.NoError(t, err)
	assert.Equal(t, wantSHA, gotSHA, "CommitSHA should match the source HEAD")
}

// TestGoGitProvider_CloneEmptyURL verifies structured error on empty input.
func TestGoGitProvider_CloneEmptyURL(t *testing.T) {
	p := git.NewGoGitProvider()
	err := p.Clone(context.Background(), "", "HEAD", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is empty")
}

// TestGoGitProvider_CloneEmptyDest verifies structured error on empty dest.
func TestGoGitProvider_CloneEmptyDest(t *testing.T) {
	p := git.NewGoGitProvider()
	err := p.Clone(context.Background(), "file:///nonexistent", "HEAD", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dest is empty")
}

// TestGoGitProvider_CommitSHA_NotARepo verifies error when dir isn't a git repo.
func TestGoGitProvider_CommitSHA_NotARepo(t *testing.T) {
	p := git.NewGoGitProvider()
	_, err := p.CommitSHA(context.Background(), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git open")
}

// TestGoGitProvider_Fetch verifies Fetch updates a clone (no-op for fresh clone).
func TestGoGitProvider_Fetch(t *testing.T) {
	srcURL, _ := makeLocalRepo(t)
	p := git.NewGoGitProvider()
	ctx := context.Background()

	dest := t.TempDir()
	require.NoError(t, p.Clone(ctx, srcURL, "HEAD", dest))
	// Fetch on a freshly-cloned repo should succeed (already up-to-date is OK).
	err := p.Fetch(ctx, dest)
	assert.NoError(t, err)
}

// Compile-time check: GoGitProvider implements GitProvider interface.
var _ git.GitProvider = (*git.GoGitProvider)(nil)
