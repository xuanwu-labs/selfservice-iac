// Package git defines the GitProvider adapter interface (D7).
//
// The registry module (03) uses this to clone module source repos for
// contract extraction. The default implementation is a noop stub; a
// real go-git implementation will be added when registry lands.
package git

import (
	"context"
	"fmt"
)

// GitProvider abstracts git operations so the platform can swap
// implementations (go-git, shell git, mocked for tests).
type GitProvider interface {
	// Clone fetches a remote repo at the given ref into dest.
	Clone(ctx context.Context, url, ref, dest string) error
	// Fetch updates the local clone at dir with the latest remote refs.
	Fetch(ctx context.Context, dir string) error
	// CommitSHA returns the resolved commit SHA for the current HEAD of dir.
	CommitSHA(ctx context.Context, dir string) (string, error)
}

// NoopGit is the default stub. It fails loud to surface unconfigured adapters
// at runtime rather than silently degrading.
type NoopGit struct{}

// Clone returns a structured error indicating the adapter is not configured.
func (NoopGit) Clone(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("git adapter not configured: set adapters.git.impl in config")
}

// Fetch returns a structured error indicating the adapter is not configured.
func (NoopGit) Fetch(_ context.Context, _ string) error {
	return fmt.Errorf("git adapter not configured: set adapters.git.impl in config")
}

// CommitSHA returns a structured error indicating the adapter is not configured.
func (NoopGit) CommitSHA(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("git adapter not configured: set adapters.git.impl in config")
}
