package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// SquashMergeAndPush merges the request's per-request branch from its shared
// clone into <workspaceName>'s main branch and pushes the result upstream
// (design D4).
//
// This is the ONLY place in the package that shells out to the git CLI. The
// three operations go-git v5 cannot do are:
//
//  1. `git pull --ff-only origin main` — go-git has no upstream fast-forward
//     against a remote (its Worktree.Pull only handles the configured upstream
//     of the current branch and is finicky with bare repos).
//  2. `git fetch <reqDir> <branch>:<branch>` — fetch a SPECIFIC ref from a
//     SHARED clone's independent refs. Shared clones do not share refs (only
//     objects), so a branch created in req-123/ does NOT exist in repo/ until
//     we explicitly fetch it. This is the P0-1 trap the design calls out.
//  3. `git merge --squash <branch>` — go-git only supports FastForwardMerge.
//
// Everything else (checkout main, commit the squashed index, push, delete the
// ref) uses go-git so we keep a typed handle on the repo object.
//
// On ANY error we leave the per-request branch + shared clone in place so the
// failure is debuggable (D4 "失败不 push — 保留 worktree + 分支供排障"). The
// caller (Executor, W2-08) decides whether to retry, route to waiting_manual,
// or alert.
func (m *Manager) SquashMergeAndPush(
	ctx context.Context,
	requestID int64,
	workspaceName, remoteURL, branchName, commitMsg string,
) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("workspace: SquashMergeAndPush cancelled: %w", ctx.Err())
	default:
	}

	repoDir := m.repoPath(workspaceName)
	reqDir := m.requestPath(workspaceName, requestID)

	// Sanity: shared clone must exist (WriteFiles or a reconcile ran first).
	if !dirExists(reqDir) {
		return fmt.Errorf("workspace: shared clone missing: %s", reqDir)
	}
	// Sanity: primary repo must exist (WriteFiles or SeedRepo created it).
	if !dirExists(repoDir) {
		return fmt.Errorf("workspace: primary repo missing: %s", repoDir)
	}

	// 2. Prepare a scratch non-bare clone of repo/ that we can run
	//    `git merge --squash` inside (a bare repo has no worktree). The clone
	//    shares objects with repo/ (Shared:true) so it is cheap. We also wire
	//    up an "upstream" remote pointing at remoteURL so the push back lands
	//    on the real origin.
	mergeDir, cleanup, err := m.scratchClone(repoDir, remoteURL)
	if err != nil {
		return fmt.Errorf("workspace: scratch clone: %w", err)
	}
	defer cleanup()

	// 3. Pull main fast-forward from upstream so we don't push a stale main
	//    (concurrent stacks land on main in between). If remoteURL is empty
	//    (local-only test), skip — there's nothing to fast-forward from. An
	//    empty-remote failure (first push ever) is tolerated: the subsequent
	//    push will create the branch.
	if remoteURL != "" {
		if err := gitPullFFOnly(ctx, mergeDir, "upstream", DefaultBranch); err != nil {
			// Tolerate "no such ref on remote" — first-ever push to an empty
			// remote. Anything else (auth, network) is still an error.
			if !isNoUpstreamErr(err) {
				return fmt.Errorf("workspace: pull --ff-only upstream main: %w", err)
			}
		}
	}

	// 4. P0-1 CRITICAL: fetch the per-request branch from the shared clone
	//    INTO THE SCRATCH CLONE. Shared clones share .git/objects but each has
	//    INDEPENDENT refs — the branch created in req-<id>/ does NOT exist in
	//    repo/ (or in the scratch clone of repo/) until we explicitly fetch
	//    it. We fetch into the scratch clone (not repo/) because that's where
	//    the subsequent `git merge --squash` runs, and fetching into a local
	//    ref there means the merge can reference it by bare name.
	if err := gitFetch(ctx, mergeDir, reqDir, branchName+":"+branchName); err != nil {
		return fmt.Errorf("workspace: fetch %s from %s: %w", branchName, reqDir, err)
	}

	// 5. Squash-merge the request branch into the index.
	if err := gitMergeSquash(ctx, mergeDir, branchName); err != nil {
		return fmt.Errorf("workspace: merge --squash %s: %w", branchName, err)
	}

	// 6. Commit the squashed index. We use the git CLI here (not go-git)
	//    because go-git would not see the staged squash tree — `git merge
	//    --squash` mutated the index on disk and go-git caches the index in
	//    memory. Committing via CLI keeps the index handoff consistent.
	if err := gitCommit(ctx, mergeDir, commitMsg); err != nil {
		return fmt.Errorf("workspace: squash commit: %w", err)
	}

	// 7. Push main back to the primary repo (so repo/ is updated for the next
	//    request's squash base) AND upstream (if configured). Both via go-git
	//    so we get typed PushOptions.
	mergeRepo, err := git.PlainOpen(mergeDir)
	if err != nil {
		return fmt.Errorf("workspace: reopen %s: %w", mergeDir, err)
	}
	mainSpec := config.RefSpec("refs/heads/main:refs/heads/main")
	if err := mergeRepo.Push(&git.PushOptions{
		RemoteName: "origin", // == repoDir from scratchClone
		RefSpecs:   []config.RefSpec{mainSpec},
	}); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("workspace: push to repo: %w", err)
	}
	if remoteURL != "" {
		if err := mergeRepo.Push(&git.PushOptions{
			RemoteName: "upstream",
			RefSpecs:   []config.RefSpec{mainSpec},
		}); err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("workspace: push upstream: %w", err)
		}
	}

	// 8. Delete the per-request branch ref from the shared clone (D4 step 6).
	//    We delete from reqDir's refs (where it was created by WriteFiles) and
	//    from the scratch clone (where it was fetched). The primary repo/
	//    never had the ref (P0-1: refs are independent), so no cleanup there.
	//    Errors are non-fatal — the squash already landed on main.
	reqRepo, err := git.PlainOpen(reqDir)
	if err == nil {
		_ = reqRepo.DeleteBranch(branchName)
		_ = reqRepo.Storer.RemoveReference(plumbing.NewBranchReferenceName(branchName))
	}
	_ = mergeRepo.DeleteBranch(branchName)
	_ = mergeRepo.Storer.RemoveReference(plumbing.NewBranchReferenceName(branchName))

	return nil
}

// scratchClone creates a fresh non-bare clone of repoDir in a temp subdir
// under the workspace, returning its path + a cleanup func. The clone shares
// objects with repoDir (Shared:true) so it's cheap; the cleanup func removes
// the dir. We keep the dir under the workspace root (not the OS tempdir) so
// it shares the same filesystem as repo/ — this matters on Windows where
// hardlinks across volumes fail.
//
// If upstreamURL is non-empty, an "upstream" remote is configured pointing at
// it, so the caller's Push to "upstream" lands on the real origin.
func (m *Manager) scratchClone(repoDir, upstreamURL string) (string, func(), error) {
	dir := filepath.Join(m.workspacePath(defaultWorkspaceName), ".scratch-"+uniqueSuffix())
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", nil, fmt.Errorf("mkdir parent: %w", err)
	}
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:    repoDir,
		Shared: true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("clone scratch: %w", err)
	}
	if upstreamURL != "" {
		if _, err := repo.CreateRemote(&config.RemoteConfig{
			Name: "upstream",
			URLs: []string{upstreamURL},
		}); err != nil && err != git.ErrRemoteExists {
			_ = os.RemoveAll(dir)
			return "", nil, fmt.Errorf("create upstream remote: %w", err)
		}
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// --- git CLI wrappers ----------------------------------------------------
//
// Each wrapper runs a single git subcommand in dir, captures stdout+stderr,
// and returns a structured error on non-zero exit. The wrappers are tiny on
// purpose: the contract is "this is the one place we shell out" (D1) and the
// command surface is exactly three subcommands.

// gitPullFFOnly runs `git pull --ff-only <remote> <branch>` in dir.
func gitPullFFOnly(ctx context.Context, dir, remote, branch string) error {
	return runGit(ctx, dir, "pull", "--ff-only", remote, branch)
}

// gitFetch runs `git fetch <remote> <refspec>` in dir. remote can be a path
// (the shared clone dir) — git happily fetches from a local path.
func gitFetch(ctx context.Context, dir, remote, refspec string) error {
	return runGit(ctx, dir, "fetch", remote, refspec)
}

// gitMergeSquash runs `git merge --squash <branch>` in dir. Does NOT commit —
// the caller commits next. `--squash` stages the result in the index, ready
// for one commit.
func gitMergeSquash(ctx context.Context, dir, branch string) error {
	return runGit(ctx, dir, "merge", "--squash", branch)
}

// gitCommit runs `git commit -m <msg>` in dir. Used after `git merge --squash`
// because go-git would not see the staged squash tree (the CLI mutated the
// index on disk, but go-git caches the index in memory).
func gitCommit(ctx context.Context, dir, msg string) error {
	// Identity is forced via -c flags so the CLI uses the same author as
	// go-git commits in WriteFiles (audits can correlate).
	return runGitWithConfig(ctx, dir,
		[]string{
			"-c", "user.name=" + CommitIdentity.Name,
			"-c", "user.email=" + CommitIdentity.Email,
			"-c", "committer.name=" + CommitIdentity.Name,
			"-c", "committer.email=" + CommitIdentity.Email,
		},
		"commit", "-m", msg)
}

// runGit is the single exec wrapper. Sets the working dir to dir (so the git
// subcommand operates on the right repo) and forwards stderr to the error
// message.
func runGit(ctx context.Context, dir string, args ...string) error {
	return runGitWithConfig(ctx, dir, nil, args...)
}

// runGitWithConfig is the same as runGit but lets the caller inject top-level
// `git -c <name>=<value>` global overrides (passed BEFORE the subcommand).
func runGitWithConfig(ctx context.Context, dir string, preArgs []string, args ...string) error {
	full := append([]string{}, preArgs...)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	var errBuf bytes.Buffer
	cmd.Stdout = nil // discard stdout; stderr is enough for error context
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s in %s: %w (stderr: %s)",
			strings.Join(args, " "), dir, err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// isNoUpstreamErr returns true if the error from a `git pull --ff-only`
// indicates the remote doesn't have the branch yet (e.g. first-ever push to an
// empty remote). Such a failure is expected on the very first land and is
// tolerated by the caller.
func isNoUpstreamErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such ref") ||
		strings.Contains(msg, "couldn't find remote ref") ||
		strings.Contains(msg, "does not appear to be a git repository") ||
		strings.Contains(msg, "fatal: couldn't find")
}

// uniqueSuffix returns a timestamp+pid suffix used to name scratch dirs
// uniquely. We avoid pulling in another dep (uuid is overkill here) —
// UnixNano + pid is unique enough for a single node.
func uniqueSuffix() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}
