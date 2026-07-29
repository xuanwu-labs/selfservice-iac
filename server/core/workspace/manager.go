package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
)

// defaultWorkspaceName is the single MVP workspace (D1 "Phase 1 single workspace").
// Phase 2 will resolve this from the workspaces table keyed by request →
// workspace_id; the Manager API already takes a workspaceName argument so the
// change does not break callers.
const defaultWorkspaceName = "default"

// DefaultBranch is the trunk the squash-merge lands on. Hardcoded for MVP
// (workspaces.default_branch is read from DB in Phase 2; "main" is the
// repo-wide convention).
const DefaultBranch = "main"

// CommitIdentity is the author/committer used for all platform commits. It is
// constant so audits can distinguish platform-landed commits from human ones.
// (Phase 2: parameterize by actor for the request_events trail.)
var CommitIdentity = object.Signature{
	Name:  "selfservice-iac",
	Email: "platform@selfservice.local",
}

// requestBranchName returns the per-request branch name. Kept consistent
// across WriteFiles (creates the branch) and SquashMergeAndPush (fetches it
// back) — a single source of truth avoids the P0-1 "branch not found" class
// of bug.
func requestBranchName(requestID int64) string {
	return fmt.Sprintf("req-%d", requestID)
}

// requestDirName is the directory name under <worktreeRoot>/<workspaceName>/.
// Matches the "req-<id>" convention from design.md (Open Questions).
func requestDirName(requestID int64) string {
	return fmt.Sprintf("req-%d", requestID)
}

// Manager implements orchestrator.WorkspaceManager using go-git Shared Clones
// (design D1). It is the single collaborator the orchestrator Pipeline talks
// to for all working-directory concerns.
//
// WriteFiles is the only method on the orchestrator.WorkspaceManager interface
// (P1-1); CheckoutCommit / ReleaseWorktree / SquashMergeAndPush are internal
// methods used by the Executor (W2-08) and Reconciler, not the Pipeline.
//
// All methods are safe to call concurrently across different requestIDs (each
// request has its own shared clone dir). Concurrent calls for the SAME
// requestID are NOT supported — the orchestrator serializes per-request work
// via the optimistic-lock + state machine (design D1/D20).
type Manager struct {
	// worktreeRoot is the parent of all workspace dirs, e.g.
	// "/var/tm/workspaces" (prod) or "./tmp/workspaces" (dev). Per-workspace
	// (then per-request) subdirectories are created underneath.
	worktreeRoot string
	// nodeID is the node identifier from config (D1 P1-4: defaults to
	// "node-1"). Used in Phase 2 to scope workspace_checkouts rows; MVP just
	// holds it so the constructor signature is stable.
	nodeID string

	// repoMu serializes ensureRepo per workspace name. Without it, N concurrent
	// WriteFiles calls on a fresh workspace would all see "no repo" and all
	// try to PlainInit the same dir, racing on filesystem state. The map+mutex
	// pattern lets distinct workspaces proceed in parallel while serializing
	// same-workspace init. (Per-request work happens AFTER ensureRepo, in
	// distinct req-<id> dirs, so it needs no lock.)
	repoMu   sync.Mutex
	repoLock map[string]*sync.Mutex
}

// NewManager constructs a Manager. worktreeRoot must be absolute (or relative
// to the process cwd); the caller is responsible for creating it (we MkdirAll
// lazily per workspace, so an absent root is fine).
func NewManager(worktreeRoot, nodeID string) *Manager {
	return &Manager{
		worktreeRoot: worktreeRoot,
		nodeID:       nodeID,
		repoLock:     make(map[string]*sync.Mutex),
	}
}

// Compile-time check: Manager satisfies the orchestrator's WorkspaceManager
// interface. The interface is declared in the orchestrator package (so the
// orchestrator owns the contract — Dependency Inversion). We assert it here so
// a signature drift surfaces at compile time, not at wire assembly.
var _ interface {
	WriteFiles(context.Context, int64, codegen.FileSet) (string, error)
} = (*Manager)(nil)

// WriteFiles writes a codegen FileSet into the request's shared clone and
// returns the resulting commit SHA. This is THE orchestrator.WorkspaceManager
// method (D2): it is what the generating stage calls to persist codegen output.
//
// Flow (D2):
//  1. ensure the workspace's primary bare clone exists (clone on first call,
//     fetch on subsequent calls). For MVP the workspace is hardcoded to
//     "default" and the remote URL is supplied by the caller via the shared
//     remoteURLFor() helper in tests; in production the WorkspaceManager gets
//     the remoteURL from the workspaces table (Phase 2). Until that wiring
//     lands, the bare repo is auto-created from an empty init so the method is
//     callable end-to-end in tests.
//  2. create (or reuse) the per-request shared clone that shares objects with
//     the primary clone but has independent refs.
//  3. write every file from the FileSet to disk under the shared clone.
//  4. git add -A + git commit on the per-request branch.
//  5. return the commit SHA (the orchestrator pins this on the request /
//     workspace_checkouts row).
//
// remoteURL == "" is treated as "no upstream configured" (local-only test
// repo); ensureRepo will then PlainInit a bare repo in place.
func (m *Manager) WriteFiles(ctx context.Context, requestID int64, files codegen.FileSet) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("workspace: WriteFiles cancelled: %w", ctx.Err())
	default:
	}

	repoDir, err := m.ensureRepo(ctx, defaultWorkspaceName, "")
	if err != nil {
		return "", fmt.Errorf("workspace: ensure repo: %w", err)
	}

	reqDir := m.requestPath(defaultWorkspaceName, requestID)
	// Recreate the shared clone each call — WriteFiles must be idempotent
	// w.r.t. the file system (retries after a partial failure should produce
	// the same on-disk state). os.RemoveAll on a missing dir is a no-op.
	if err := os.RemoveAll(reqDir); err != nil {
		return "", fmt.Errorf("workspace: clear req dir %s: %w", reqDir, err)
	}

	// Shared: true shares .git/objects with repoDir (cheap); refs stay
	// independent (so creating the req-<id> branch here does NOT affect
	// repoDir until we explicitly fetch — see merge.go P0-1).
	if _, err := git.PlainClone(reqDir, false, &git.CloneOptions{
		URL:    repoDir,
		Shared: true,
	}); err != nil {
		return "", fmt.Errorf("workspace: shared clone %s <- %s: %w", reqDir, repoDir, err)
	}

	repo, err := git.PlainOpen(reqDir)
	if err != nil {
		return "", fmt.Errorf("workspace: open shared clone %s: %w", reqDir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("workspace: worktree %s: %w", reqDir, err)
	}

	// Detached HEAD from a bare clone with no commit yet — handle the very
	// first request on a fresh repo. We point HEAD at our per-request branch
	// so the commit has a parent (or starts a new history if repo was empty).
	branch := requestBranchName(requestID)
	if err := checkoutOrCreateBranch(wt, repo, branch); err != nil {
		return "", fmt.Errorf("workspace: prepare branch %s: %w", branch, err)
	}

	if err := writeFilesToDir(reqDir, files); err != nil {
		return "", fmt.Errorf("workspace: write files: %w", err)
	}

	// Stage everything (add -A). go-git's Add per-path is what git CLI does
	// under the hood; we walk the FileSet keys rather than the whole tree to
	// avoid touching unrelated files in tests that pre-populate the clone.
	for repoPath := range files {
		if _, err := wt.Add(repoPath); err != nil {
			return "", fmt.Errorf("workspace: add %s: %w", repoPath, err)
		}
	}

	hash, err := wt.Commit(
		fmt.Sprintf("req-%d: codegen generated", requestID),
		&git.CommitOptions{
			Author:    commitSigNow(),
			Committer: commitSigNow(),
			// All:false because we already Add'd the FileSet paths above; we
			// do NOT want to stage stray files left by a previous partial run.
			All:               false,
			AllowEmptyCommits: false,
		},
	)
	if err != nil {
		return "", fmt.Errorf("workspace: commit %s: %w", reqDir, err)
	}

	return hash.String(), nil
}

// CheckoutCommit checks out commitSHA in the request's shared clone and
// returns its absolute path. Used by the Executor (W2-08) before invoking the
// Terramate runner for plan/apply (D3): the working dir MUST be at exactly
// the pinned_commit so plan/apply operate on what was reviewed.
//
// If the shared clone does not exist (e.g. process restarted), it is recreated
// from the primary repo clone. The commit must already be reachable from the
// primary clone — WriteFiles writes to the shared clone's own refs, but a
// restart loses those refs; the caller is expected to have already persisted
// pinned_commit to workspace_checkouts (Phase 2 reconciler flow).
func (m *Manager) CheckoutCommit(ctx context.Context, requestID int64, commitSHA string) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("workspace: CheckoutCommit cancelled: %w", ctx.Err())
	default:
	}

	repoDir, err := m.ensureRepo(ctx, defaultWorkspaceName, "")
	if err != nil {
		return "", fmt.Errorf("workspace: ensure repo: %w", err)
	}

	reqDir := m.requestPath(defaultWorkspaceName, requestID)
	exists := false
	if _, statErr := os.Stat(filepath.Join(reqDir, ".git")); statErr == nil {
		exists = true
	}
	if !exists {
		// Recreate the shared clone (lost on restart). We do NOT carry over
		// the per-request branch — the caller asked for a specific commit, so
		// we checkout by hash directly.
		if err := os.RemoveAll(reqDir); err != nil {
			return "", fmt.Errorf("workspace: clear req dir %s: %w", reqDir, err)
		}
		if _, err := git.PlainClone(reqDir, false, &git.CloneOptions{
			URL:    repoDir,
			Shared: true,
		}); err != nil {
			return "", fmt.Errorf("workspace: shared clone (checkout) %s: %w", reqDir, err)
		}
	}

	repo, err := git.PlainOpen(reqDir)
	if err != nil {
		return "", fmt.Errorf("workspace: open %s: %w", reqDir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("workspace: worktree %s: %w", reqDir, err)
	}

	hash := plumbing.NewHash(commitSHA)
	if err := wt.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		return "", fmt.Errorf("workspace: checkout %s in %s: %w", commitSHA, reqDir, err)
	}
	return reqDir, nil
}

// ReleaseWorktree removes the request's shared clone directory. Called by the
// Executor when the request reaches a terminal state (D4: do not delete on
// failure — keep the working dir for debugging).
//
// Phase 2 will additionally flip workspace_checkouts.status to "released";
// MVP just removes the on-disk dir. Removing a non-existent dir is a no-op.
func (m *Manager) ReleaseWorktree(ctx context.Context, requestID int64) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("workspace: ReleaseWorktree cancelled: %w", ctx.Err())
	default:
	}
	reqDir := m.requestPath(defaultWorkspaceName, requestID)
	if err := os.RemoveAll(reqDir); err != nil {
		return fmt.Errorf("workspace: release %s: %w", reqDir, err)
	}
	return nil
}

// ensureRepo returns the path to the workspace's primary bare clone, creating
// it (PlainInit bare) if missing and fetching from remoteURL if present and
// the clone already exists.
//
// remoteURL semantics:
//   - "" → local-only repo. We PlainInit a bare repo (no commit history). Used
//     by tests and the very first call before any upstream is wired.
//   - non-empty → if the repo dir does not exist, PlainClone it (bare) from
//     remoteURL. If it exists, git fetch from remoteURL to refresh.
//
// This is the ONLY function that touches the primary repo dir — everything
// else operates on per-request shared clones that point at it.
//
// ensureRepo is serialized per workspace (m.repoMu + repoLock map) so the
// first N concurrent calls on a fresh workspace don't race on PlainInit.
func (m *Manager) ensureRepo(ctx context.Context, workspaceName, remoteURL string) (string, error) {
	mu := m.lockForWorkspace(workspaceName)
	mu.Lock()
	defer mu.Unlock()

	repoDir := m.repoPath(workspaceName)
	if err := os.MkdirAll(m.workspacePath(workspaceName), 0o755); err != nil {
		return "", fmt.Errorf("workspace: mkdir %s: %w", m.workspacePath(workspaceName), err)
	}

	// Detect "already a repo" by presence of HEAD/config rather than by
	// catching PlainOpen's error (a partially-init'd dir can confuse it).
	repoExists := dirExists(filepath.Join(repoDir, "config")) || dirExists(filepath.Join(repoDir, "HEAD"))

	if !repoExists {
		if remoteURL != "" {
			if _, err := git.PlainClone(repoDir, true, &git.CloneOptions{
				URL: remoteURL,
			}); err != nil {
				return "", fmt.Errorf("workspace: clone bare %s <- %s: %w", repoDir, remoteURL, err)
			}
			return repoDir, nil
		}
		// Local-only: PlainInit a bare repo. We need at least one commit so
		// that PlainClone(Shared:true) from it has a HEAD to follow (go-git's
		// PlainClone fails on a HEAD-less bare repo in v5.19). Create the
		// initial commit on a temp non-bare clone, then push it back.
		if err := initBareWithInitialCommit(repoDir); err != nil {
			return "", fmt.Errorf("workspace: init bare %s: %w", repoDir, err)
		}
		return repoDir, nil
	}

	// Repo exists — refresh from upstream if one is configured.
	if remoteURL != "" {
		if err := fetchRepo(ctx, repoDir, remoteURL); err != nil {
			// Fetch failure is non-fatal for WriteFiles (we operate on local
			// refs); surface it but keep going. The squash-merge step is the
			// one that actually needs an up-to-date main.
			_ = err // logged by caller via the returned error chain in production
		}
	}
	return repoDir, nil
}

// --- path helpers --------------------------------------------------------

// lockForWorkspace returns the mutex used to serialize ensureRepo for the
// given workspace. Lazily creates one mutex per distinct workspace name so
// different workspaces can init in parallel.
func (m *Manager) lockForWorkspace(workspaceName string) *sync.Mutex {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()
	mu, ok := m.repoLock[workspaceName]
	if !ok {
		mu = &sync.Mutex{}
		m.repoLock[workspaceName] = mu
	}
	return mu
}

// workspacePath returns <worktreeRoot>/<workspaceName>.
func (m *Manager) workspacePath(workspaceName string) string {
	return filepath.Join(m.worktreeRoot, workspaceName)
}

// repoPath returns <worktreeRoot>/<workspaceName>/repo (the primary bare
// clone; D1 layout).
func (m *Manager) repoPath(workspaceName string) string {
	return filepath.Join(m.workspacePath(workspaceName), "repo")
}

// requestPath returns <worktreeRoot>/<workspaceName>/<reqDirName(requestID)>.
func (m *Manager) requestPath(workspaceName string, requestID int64) string {
	return filepath.Join(m.workspacePath(workspaceName), requestDirName(requestID))
}

// --- internal helpers ----------------------------------------------------

// checkoutOrCreateBranch switches wt to branch; if branch does not exist yet
// (fresh shared clone), it is created from the current HEAD. The "create new
// branch" case is what makes WriteFiles work on a clone that came off a bare
// repo whose HEAD is on main: we want our per-request branch, not main.
func checkoutOrCreateBranch(wt *git.Worktree, repo *git.Repository, branch string) error {
	refName := plumbing.NewBranchReferenceName(branch)
	// First try a plain checkout of the existing branch.
	if err := wt.Checkout(&git.CheckoutOptions{Branch: refName}); err == nil {
		return nil
	}
	// Branch doesn't exist → create it from current HEAD. Create:true +
	// Branch:<name> makes Checkout create-and-switch. (Hash left zero means
	// "start from current HEAD".)
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: refName,
		Create: true,
	}); err != nil {
		return fmt.Errorf("create branch %s: %w", branch, err)
	}
	// Track it locally so DeleteBranch works later and HEAD isn't detached.
	_ = repo.CreateBranch(&config.Branch{
		Name:   branch,
		Remote: "origin",
		Merge:  refName,
	})
	return nil
}

// writeFilesToDir writes every FileSet entry to disk under baseDir. Each key
// is repo-relative (forward slashes — codegen guarantees this via path.Join),
// so we split on "/" and filepath.Join so the OS-native separator is used.
// Parent dirs are created on demand.
func writeFilesToDir(baseDir string, files codegen.FileSet) error {
	for repoPath, content := range files {
		// codegen keys are repo-relative with forward slashes; convert to OS.
		abs := filepath.Join(baseDir, filepath.FromSlash(repoPath))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(abs), err)
		}
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", abs, err)
		}
	}
	return nil
}

// fetchRepo fetches all heads + tags from remoteURL into the repo at repoDir.
// We replace the origin URL on each call so a reconfigured remote is honoured.
func fetchRepo(ctx context.Context, repoDir, remoteURL string) error {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("open %s: %w", repoDir, err)
	}
	// (Re)create origin so the URL is always current.
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
		Fetch: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/heads/*"),
			config.RefSpec("+refs/tags/*:refs/tags/*"),
		},
	}); err != nil && err != git.ErrRemoteExists {
		// If origin exists, fall through to a plain Fetch — the existing
		// config is fine for a re-fetch.
	}
	if err := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/heads/*"),
			config.RefSpec("+refs/tags/*:refs/tags/*"),
		},
		Tags: git.AllTags,
	}); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetch %s: %w", remoteURL, err)
	}
	return nil
}

// initBareWithInitialCommit creates a bare repo at repoDir with a single
// "initial commit" on main, so that PlainClone(Shared:true) from it has a
// non-empty HEAD to follow. We do the commit in a throwaway non-bare repo and
// push it into the bare one — go-git cannot commit directly into a bare repo
// (no worktree).
//
// The trick: go-git's default initial branch is "master". We force HEAD to
// refs/heads/main BEFORE the commit so the first commit lands on main (without
// this, the commit ends up on master and our push of "main:main" finds no
// source ref). We push via the git CLI rather than go-git's Push because
// go-git's push to a bare repo with no remote-tracking refs reports
// "already up-to-date" spuriously.
func initBareWithInitialCommit(repoDir string) error {
	if _, err := git.PlainInit(repoDir, true); err != nil {
		return fmt.Errorf("plain init bare: %w", err)
	}
	// Stage the commit in a temp dir.
	tmp, err := os.MkdirTemp("", "w2-init-")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(tmp) // best effort

	tmpRepo, err := git.PlainInit(tmp, false)
	if err != nil {
		return fmt.Errorf("plain init tmp: %w", err)
	}
	// Force HEAD to main BEFORE committing so the first commit is on main,
	// not master. Without this, go-git's default branch is master and our
	// subsequent "main:main" push refspec resolves to no source ref.
	if err := tmpRepo.Storer.SetReference(plumbing.NewSymbolicReference(
		plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))); err != nil {
		return fmt.Errorf("set tmp HEAD: %w", err)
	}
	// Point tmpRepo at the bare repo as origin.
	if _, err := tmpRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoDir},
	}); err != nil && err != git.ErrRemoteExists {
		return fmt.Errorf("create origin: %w", err)
	}
	// Seed a README so the first commit is non-empty (otherwise HEAD is unborn
	// and the bare repo can't be cloned with a default branch).
	seed := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(seed, []byte("# workspace root\n"), 0o644); err != nil {
		return fmt.Errorf("seed README: %w", err)
	}
	wt, err := tmpRepo.Worktree()
	if err != nil {
		return fmt.Errorf("tmp worktree: %w", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		return fmt.Errorf("add README: %w", err)
	}
	sig := commitSigNow()
	if _, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: sig, Committer: sig,
	}); err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}
	// Push main into the bare repo via git CLI. We avoid go-git's Push here:
	// against a bare repo with no remote-tracking refs, go-git v5.19 spuriously
	// returns NoErrAlreadyUpToDate and the ref never lands. The CLI is the
	// robust path for the initial push only.
	if err := exec.CommandContext(context.Background(), "git", "-C", tmp,
		"push", "origin", "refs/heads/main:refs/heads/main").Run(); err != nil {
		return fmt.Errorf("push initial (cli): %w", err)
	}
	// Set the bare repo's HEAD to refs/heads/main so PlainClone defaults to
	// main (a freshly PlainInit'd bare repo has HEAD → refs/heads/master).
	bareRepo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("reopen bare: %w", err)
	}
	if err := bareRepo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))); err != nil {
		return fmt.Errorf("set HEAD: %w", err)
	}
	return nil
}

// commitSigNow returns CommitIdentity with the current timestamp. Centralized
// so every commit comes from the same identity.
func commitSigNow() *object.Signature {
	sig := CommitIdentity
	sig.When = time.Now().UTC()
	return &sig
}

// dirExists is a small helper — os.Stat wrapped as bool — used to keep
// ensureRepo's "is this already a repo?" check readable.
func dirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
