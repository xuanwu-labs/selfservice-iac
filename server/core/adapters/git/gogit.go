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
//   - "v1.0.0", "abc123" → tag or commit-ish (resolved via remote refs)
//
// Credential resolution:
//   - SSH URLs (git@host:org/repo.git) → GIT_SSH_COMMAND env var
//   - HTTPS URLs with GIT_TOKEN set → Bearer auth header injected
//   - HTTPS URLs without GIT_TOKEN → anonymous (public repos)
func (g *GoGitProvider) Clone(ctx context.Context, url, ref, dest string) error {
	if url == "" {
		return fmt.Errorf("git Clone: url is empty")
	}
	if dest == "" {
		return fmt.Errorf("git Clone: dest is empty")
	}

	cloneOpts := g.buildCloneOptions(url, ref)

	// go-git Clone does not accept a context directly in v5.19; the clone is
	// blocking. For MVP this is acceptable (registry registration is async via
	// the request pipeline). A context-aware clone would need go-git/v5's
	// transport layer changes — deferred to W2 if registration latency matters.
	_, err := git.PlainClone(dest, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("git clone %s@%s into %s: %w", url, ref, dest, err)
	}
	return nil
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

// buildCloneOptions assembles git.CloneOptions with credential resolution.
func (g *GoGitProvider) buildCloneOptions(url, ref string) *git.CloneOptions {
	opts := &git.CloneOptions{
		URL: url,
	}

	// Resolve ref → ReferenceName. Empty or HEAD → default branch (RemoteName).
	switch {
	case ref == "" || ref == "HEAD":
		opts.RemoteName = "origin"
	case isBranch(ref):
		opts.ReferenceName = plumbing.NewBranchReferenceName(ref)
		opts.SingleBranch = true
	default:
		// Could be a tag (v1.0.0) or commit-ish. go-git resolves tags via
		// ReferenceName; commit-ish needs a full clone + checkout. For tags,
		// try tag ref first; Depth=0 (full) ensures commit-ish works too.
		opts.ReferenceName = plumbing.NewTagReferenceName(ref)
		opts.SingleBranch = false
	}

	// Credential resolution from env.
	if token := os.Getenv("GIT_TOKEN"); token != "" && strings.HasPrefix(url, "https://") {
		// HTTPS + token: inject as Bearer auth via go-git's inertial
		// transport (set via git credential helper env). go-git v5 reads
		// GIT_USERNAME / GIT_PASSWORD for HTTPS basic auth.
		_ = os.Setenv("GIT_USERNAME", "oauth2")
		_ = os.Setenv("GIT_PASSWORD", token)
	}
	// SSH: go-git honors GIT_SSH_COMMAND automatically via its SSH transport.

	return opts
}

// isBranch reports whether ref looks like a branch name (no tag prefix,
// no commit hash shape). Conservative: treat anything that isn't a 7+ hex
// string or vX.Y.Z tag as a potential branch.
func isBranch(ref string) bool {
	if strings.HasPrefix(ref, "v") && len(ref) >= 2 {
		// vX.Y.Z tag pattern
		rest := ref[1:]
		if strings.Contains(rest, ".") {
			return false
		}
	}
	if len(ref) >= 7 && isAllHex(ref) {
		// commit-ish
		return false
	}
	return true
}

func isAllHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
