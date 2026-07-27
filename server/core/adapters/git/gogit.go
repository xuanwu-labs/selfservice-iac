// Package git defines the GitProvider adapter interface (D7).
//
// gogit.go implements GitProvider using go-git/v5 (pure Go, no external git
// binary required — fits the W1-02 process Executor mode which is zero-dep).
//
// Credentials (MVP): SSH via GIT_SSH_COMMAND env var, HTTPS via GIT_TOKEN env
// var (injected as Bearer header). Full D23 credential injection (Vault/KMS)
// is deferred to W2; this MVP implementation reads env vars directly so local
// dev and CI work without extra plumbing.

package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// GoGitProvider implements GitProvider using go-git/v5.
//
// Zero-config default: uses environment for credentials (GIT_SSH_COMMAND /
// GIT_TOKEN). Clone depth is full (not shallow) so CommitSHA resolution and
// subsequent checkout at any pinned_commit works without re-fetching.
type GoGitProvider struct{}

// NewGoGitProvider returns a GoGitProvider. Credentials are read from env at
// Clone time, so no constructor params needed for MVP.
func NewGoGitProvider() *GoGitProvider { return &GoGitProvider{} }

// Compile-time check.
var _ GitProvider = (*GoGitProvider)(nil)

// Clone fetches a remote repo at the given ref into dest.
//
// ref semantics:
//   - "" or "HEAD" → default branch (remote HEAD)
//   - "main", "master", "feature/x" → branch name
//   - "v1.0.0", "abc123" → tag or commit-ish (resolved via checkout after clone)
//
// Credential resolution:
//   - SSH URLs (git@host:org/repo.git) → GIT_SSH_COMMAND env var (go-git honors it)
//   - HTTPS URLs with GIT_TOKEN set → CloneOptions.Auth = http.BasicAuth (go-git
//     does NOT read GIT_USERNAME/GIT_PASSWORD env vars; the Auth field is the
//     only way to inject HTTPS credentials)
//   - HTTPS URLs without GIT_TOKEN → anonymous (public repos)
func (g *GoGitProvider) Clone(ctx context.Context, url, ref, dest string) error {
	if url == "" {
		return fmt.Errorf("git Clone: url is empty")
	}
	if dest == "" {
		return fmt.Errorf("git Clone: dest is empty")
	}

	opts := &git.CloneOptions{URL: url}
	g.applyCredentials(opts, url)

	// Full clone (no ReferenceName/SingleBranch) so any branch/tag/commit-ish
	// can be checked out afterwards. The isBranch/isTag heuristic was unreliable
	// for short refs (v2, v1) — full clone + checkout is the robust alternative.
	repo, err := git.PlainClone(dest, false, opts)
	if err != nil {
		return fmt.Errorf("git clone %s into %s: %w", url, dest, err)
	}

	// Checkout the requested ref if not HEAD.
	if ref != "" && ref != "HEAD" {
		wt, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("git worktree after clone: %w", err)
		}
		// Try branch, then tag, then commit-ish (checkout handles all three).
		if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(ref)}); err != nil {
			if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewTagReferenceName(ref)}); err != nil {
				// Fall back to commit-ish hash.
				h, hashErr := repo.ResolveRevision(plumbing.Revision(ref))
				if hashErr != nil {
					return fmt.Errorf("git checkout ref %q (tried branch/tag/commit): %w", ref, err)
				}
				if err := wt.Checkout(&git.CheckoutOptions{Hash: *h}); err != nil {
					return fmt.Errorf("git checkout commit %q: %w", ref, err)
				}
			}
		}
	}
	return nil
}

// applyCredentials sets CloneOptions.Auth for HTTPS URLs when GIT_TOKEN is
// present. go-git's HTTP transport reads ONLY CloneOptions.Auth (a
// transport.AuthMethod), not GIT_USERNAME/GIT_PASSWORD env vars. SSH uses
// GIT_SSH_COMMAND which go-git honors via its SSH transport automatically.
func (g *GoGitProvider) applyCredentials(opts *git.CloneOptions, url string) {
	if token := os.Getenv("GIT_TOKEN"); token != "" && strings.HasPrefix(url, "https://") {
		opts.Auth = &githttp.BasicAuth{Username: "oauth2", Password: token}
	}
}

// Fetch updates the local clone at dir with the latest remote refs.
func (g *GoGitProvider) Fetch(ctx context.Context, dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("git open %s: %w", dir, err)
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return fmt.Errorf("git list remotes: %w", err)
	}
	if len(remotes) == 0 {
		return fmt.Errorf("git fetch: no remotes configured for %s", dir)
	}
	// Fetch the first remote (origin) with all branches + tags.
	if err := remotes[0].FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/remotes/origin/*"),
			config.RefSpec("+refs/tags/*:refs/tags/*"),
		},
		Tags: git.AllTags,
	}); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git fetch %s: %w", dir, err)
	}
	return nil
}

// CommitSHA returns the resolved commit SHA for the current HEAD of dir.
func (g *GoGitProvider) CommitSHA(ctx context.Context, dir string) (string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("git open %s: %w", dir, err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("git head %s: %w", dir, err)
	}
	return head.Hash().String(), nil
}

// buildCloneOptions removed (P0-4): the isBranch/isTag heuristic was
// unreliable for short refs (v2, v1). Clone now does a full clone + explicit
// checkout with branch→tag→commit fallback. See Clone().
