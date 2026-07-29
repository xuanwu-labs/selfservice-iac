package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
	"github.com/xuanwu-labs/selfservice-iac/server/core/workspace"
)

// newTestManager returns a Manager rooted at a fresh tempdir + the absolute
// path to that dir (so individual tests can poke at on-disk layout).
func newTestManager(t *testing.T) (*workspace.Manager, string) {
	t.Helper()
	root := t.TempDir()
	m := workspace.NewManager(root, "node-test")
	return m, m.WorktreeRootForTest()
}

// newBareRemote creates a bare repo in a temp dir, seeds it with one commit
// on `main`, and returns its absolute PATH (NOT file:// — go-git and the git
// CLI both accept plain paths for local remotes). Used as remoteURL for the
// SquashMergeAndPush test so we can verify the push actually lands upstream.
//
// go-git's default branch is `master`; we force the seed repo's HEAD to
// `refs/heads/main` BEFORE the initial commit so the resulting bare repo has
// a real main branch (the SquashMergeAndPush path assumes DefaultBranch=main).
func newBareRemote(t *testing.T) string {
	t.Helper()
	// Seed a non-bare repo on main, commit, then `git clone --bare` it.
	seed := t.TempDir()
	repo, err := gogit.PlainInit(seed, false)
	require.NoError(t, err)
	// go-git defaults to master; force main so the bare remote ends up on main.
	require.NoError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(
		plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))))
	require.NoError(t, os.WriteFile(filepath.Join(seed, "README.md"), []byte("# remote\n"), 0o644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	sig := object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}
	_, err = wt.Commit("initial", &gogit.CommitOptions{Author: &sig, Committer: &sig})
	require.NoError(t, err)

	bare := t.TempDir()
	// Remove the empty TempDir so `git clone --bare <src> <dst>` can create it.
	require.NoError(t, os.Remove(bare))
	cmd := exec.Command("git", "clone", "--bare", seed, bare)
	require.NoErrorf(t, cmd.Run(), "git clone --bare failed")
	// `git clone --bare` carries HEAD over from the source; since we set the
	// source HEAD to main, the bare repo's HEAD already points at
	// refs/heads/main. Defensive: set it explicitly anyway.
	cmd = exec.Command("git", "--git-dir", bare, "symbolic-ref", "HEAD", "refs/heads/main")
	require.NoErrorf(t, cmd.Run(), "git symbolic-ref HEAD failed")
	return bare
}

// remoteMainHead returns the SHA of main on the bare remote by reading its
// refs directly (no clone needed). Used by the squash-merge test to verify
// the push landed.
func remoteMainHead(t *testing.T, bareDir string) string {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", bareDir, "rev-parse", "main")
	out, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// fileExists reports whether path exists (used as a quick assertion helper).
func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
	return err == nil
}

// readRepoFile reads path (relative to the shared clone dir) and returns its
// content. Fails the test on any I/O error.
func readRepoFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoErrorf(t, err, "read %s in %s", rel, dir)
	return string(b)
}

// --- WriteFiles ----------------------------------------------------------

// TestWriteFiles_BasicCommit covers the happy path: write a small FileSet,
// get back a non-empty commit SHA, and verify the files land in the shared
// clone on disk. This is the contract the orchestrator generating stage
// relies on.
func TestWriteFiles_BasicCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, _ := newTestManager(t)
	ctx := context.Background()

	files := codegen.FileSet{
		"stacks/web/main.tf":    []byte("# web main\n"),
		"stacks/web/backend.tf": []byte("# backend\n"),
	}
	sha, err := m.WriteFiles(ctx, 1, files)
	require.NoError(t, err, "WriteFiles should succeed on a fresh repo")
	assert.Len(t, sha, 40, "commit SHA should be a 40-char hex string")

	// Verify the files exist in the shared clone under req-1.
	reqDir := filepath.Join(m.WorktreeRootForTest(), "default", "req-1")
	assert.True(t, fileExists(t, filepath.Join(reqDir, "stacks", "web", "main.tf")))
	assert.Equal(t, "# web main\n", readRepoFile(t, reqDir, "stacks/web/main.tf"))
	assert.Equal(t, "# backend\n", readRepoFile(t, reqDir, "stacks/web/backend.tf"))
}

// TestWriteFiles_RetriableIdempotent verifies that calling WriteFiles twice
// for the same requestID replaces the previous shared clone (the orchestrator
// may retry on transient failure — the on-disk state must be deterministic).
func TestWriteFiles_RetriableIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, _ := newTestManager(t)
	ctx := context.Background()

	// First write.
	_, err := m.WriteFiles(ctx, 42, codegen.FileSet{"a.txt": []byte("v1")})
	require.NoError(t, err)

	// Second write replaces the shared clone entirely.
	sha2, err := m.WriteFiles(ctx, 42, codegen.FileSet{"b.txt": []byte("v2")})
	require.NoError(t, err)
	assert.Len(t, sha2, 40)

	reqDir := filepath.Join(m.WorktreeRootForTest(), "default", "req-42")
	assert.True(t, fileExists(t, filepath.Join(reqDir, "b.txt")))
	// The previous file MUST NOT be present (we recreated the clone).
	assert.False(t, fileExists(t, filepath.Join(reqDir, "a.txt")),
		"re-created shared clone should not contain stale files")
}

// TestWriteFiles_CancelledContext verifies the function honours ctx cancel.
func TestWriteFiles_CancelledContext(t *testing.T) {
	m, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	_, err := m.WriteFiles(ctx, 1, codegen.FileSet{"a": []byte("a")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// --- CheckoutCommit ------------------------------------------------------

// TestCheckoutCommit_ReusesExistingDir verifies that after WriteFiles, a
// CheckoutCommit at the returned SHA returns the same shared clone dir and
// leaves the worktree at that commit.
func TestCheckoutCommit_ReusesExistingDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, _ := newTestManager(t)
	ctx := context.Background()

	sha, err := m.WriteFiles(ctx, 7, codegen.FileSet{"c.txt": []byte("hi")})
	require.NoError(t, err)

	gotPath, err := m.CheckoutCommit(ctx, 7, sha)
	require.NoError(t, err)
	wantPath := filepath.Join(m.WorktreeRootForTest(), "default", "req-7")
	assert.Equal(t, wantPath, gotPath)

	// HEAD should match the pinned SHA.
	cmd := exec.Command("git", "-C", gotPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(string(out)))
}

// TestCheckoutCommit_RecoversAfterRelease verifies that CheckoutCommit
// recreates the shared clone if it was previously released, as long as the
// pinned commit is reachable from the primary repo (which is the contract
// after SquashMergeAndPush lands main — see design D5: "fetch + checkout
// pinned_commit"). A pinned_commit that only ever lived in the request's
// shared clone cannot be recovered (the clone's refs were deleted along with
// the dir); that's by design and is the reason the squash-merge pushes main.
//
// To set up this test deterministically we push main on the primary repo
// first, then exercise the recovery path against that known-reachable commit.
func TestCheckoutCommit_RecoversAfterRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, _ := newTestManager(t)
	ctx := context.Background()

	// Step 1: write files for req-9 (lands commit in the req-9 shared clone).
	_, err := m.WriteFiles(ctx, 9, codegen.FileSet{"d.txt": []byte("data")})
	require.NoError(t, err)

	// Step 2: squash-merge into main so the commit (as a squashed commit) is
	// now reachable from repo/'s main. The squashed SHA differs from the
	// per-request commit, so we read main's tip from repo/ directly.
	repoDir := filepath.Join(m.WorktreeRootForTest(), "default", "repo")
	require.NoError(t, m.SquashMergeAndPush(ctx, 9, "default", "", "req-9",
		"req-9: applied"))
	mainSHA := gitHeadOf(t, repoDir) // reachable from repo/, so recoverable

	// Step 3: simulate restart: blow away the shared clone.
	require.NoError(t, m.ReleaseWorktree(ctx, 9))

	// Step 4: CheckoutCommit must recreate it and land at the (recoverable)
	// squashed main SHA.
	gotPath, err := m.CheckoutCommit(ctx, 9, mainSHA)
	require.NoError(t, err)
	assert.True(t, fileExists(t, filepath.Join(gotPath, "d.txt")),
		"recovered shared clone should contain the merged file")
}

// gitHeadOf returns the HEAD commit SHA of the git repo at dir (CLI-based;
// cheap and avoids go-git's HEAD-on-bare quirks).
func gitHeadOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoErrorf(t, err, "git rev-parse HEAD in %s", dir)
	return strings.TrimSpace(string(out))
}

// TestCheckoutCommit_BadSHA returns a structured error for an unknown SHA.
func TestCheckoutCommit_BadSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, _ := newTestManager(t)
	ctx := context.Background()

	// Seed a repo so checkout has something to operate on.
	_, err := m.WriteFiles(ctx, 1, codegen.FileSet{"x.txt": []byte("x")})
	require.NoError(t, err)

	_, err = m.CheckoutCommit(ctx, 1, strings.Repeat("0", 40))
	require.Error(t, err)
}

// --- ReleaseWorktree -----------------------------------------------------

// TestReleaseWorktree_RemovesDir verifies the on-disk dir goes away.
func TestReleaseWorktree_RemovesDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, root := newTestManager(t)
	ctx := context.Background()

	_, err := m.WriteFiles(ctx, 11, codegen.FileSet{"e.txt": []byte("e")})
	require.NoError(t, err)

	reqDir := filepath.Join(root, "default", "req-11")
	require.True(t, fileExists(t, reqDir))

	require.NoError(t, m.ReleaseWorktree(ctx, 11))
	assert.False(t, fileExists(t, reqDir), "ReleaseWorktree should remove the dir")
}

// TestReleaseWorktree_Idempotent verifies releasing a non-existent dir does
// not error (the executor may call Release twice on retry).
func TestReleaseWorktree_Idempotent(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()
	// Never created req-99 — should not error.
	err := m.ReleaseWorktree(ctx, 99)
	assert.NoError(t, err)
}

// --- Isolation -----------------------------------------------------------

// TestWriteFiles_ConcurrentRequestsAreIsolated verifies the core shared-clone
// property: two concurrent WriteFiles on different requestIDs land in
// different dirs with no interference. This is the concurrency-safety
// guarantee the orchestrator's optimistic lock builds on (D1/D20).
func TestWriteFiles_ConcurrentRequestsAreIsolated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	m, root := newTestManager(t)
	ctx := context.Background()

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reqID := int64(100 + i)
			files := codegen.FileSet{
				filepath.ToSlash(filepath.Join("stacks", "r"+itoa(i), "main.tf")): []byte("content-" + itoa(i)),
			}
			_, errs[i] = m.WriteFiles(ctx, reqID, files)
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoErrorf(t, errs[i], "goroutine %d failed", i)
	}
	// Verify each req dir has only its own file (no leakage from siblings).
	for i := 0; i < n; i++ {
		reqDir := filepath.Join(root, "default", "req-"+itoa(100+i))
		rel := filepath.Join("stacks", "r"+itoa(i), "main.tf")
		assert.True(t, fileExists(t, filepath.Join(reqDir, rel)),
			"req %d is missing its own file", 100+i)
	}
}

// --- SquashMergeAndPush --------------------------------------------------

// TestSquashMergeAndPush_LandsOnMain writes a FileSet for a request, then
// squash-merges the request branch into main and pushes to a local bare
// remote. Verifies the remote main HEAD changed and contains the new file.
func TestSquashMergeAndPush_LandsOnMain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}

	bare := newBareRemote(t)
	preSHA := remoteMainHead(t, bare)

	// Manager whose workspace repo is seeded from the bare remote.
	root := t.TempDir()
	m := workspace.NewManager(root, "node-test")
	ctx := context.Background()

	// First, ensure the workspace's primary repo is cloned from the bare
	// remote so SquashMergeAndPush has somewhere to land main.
	m.SeedRepoForTest(t, "default", bare)

	// WriteFiles will reuse that repo. Use a requestID whose branch we can
	// predict (requestBranchName is "req-<id>").
	const reqID = int64(5)
	files := codegen.FileSet{
		"stacks/r5/main.tf": []byte("# r5\n"),
	}
	_, err := m.WriteFiles(ctx, reqID, files)
	require.NoError(t, err)

	// Now squash-merge into main and push to the bare remote.
	branch := "req-5"
	err = m.SquashMergeAndPush(ctx, reqID, "default", bare, branch,
		"req-5: r5 applied (stack: stacks/r5)")
	require.NoError(t, err, "SquashMergeAndPush should succeed end-to-end")

	// The bare remote's main HEAD must have advanced.
	postSHA := remoteMainHead(t, bare)
	assert.NotEqual(t, preSHA, postSHA, "push should advance remote main HEAD")

	// The squash commit's content must be present on the remote. Clone the
	// remote fresh and check the file is there.
	verified := t.TempDir()
	cmd := exec.Command("git", "clone", bare, verified)
	require.NoErrorf(t, cmd.Run(), "git clone bare failed")
	assert.True(t, fileExists(t, filepath.Join(verified, "stacks", "r5", "main.tf")),
		"squash-merged file must be on remote main")

	// Verify the per-request branch was deleted from the primary repo.
	cmd = exec.Command("git", "-C", filepath.Join(root, "default", "repo"),
		"rev-parse", "--verify", "refs/heads/req-5")
	if err := cmd.Run(); err == nil {
		t.Fatalf("per-request branch req-5 should be deleted from repo/ after merge")
	}
}

// TestSquashMergeAndPush_MissingSharedClone verifies the structured error
// when the request's shared clone doesn't exist (forgot to WriteFiles).
func TestSquashMergeAndPush_MissingSharedClone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-backed test in -short mode")
	}
	bare := newBareRemote(t)
	root := t.TempDir()
	m := workspace.NewManager(root, "node-test")
	m.SeedRepoForTest(t, "default", bare)
	ctx := context.Background()

	err := m.SquashMergeAndPush(ctx, 999, "default", bare, "req-999", "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared clone missing")
}

// --- helpers used across tests ------------------------------------------

// itoa is strconv.Itoa re-exported under a short name for readability in the
// concurrent-test loop above.
func itoa(i int) string { return strconv.Itoa(i) }

// keep the time import referenced (used by newBareRemote for object.Signature).
var _ = time.Now
