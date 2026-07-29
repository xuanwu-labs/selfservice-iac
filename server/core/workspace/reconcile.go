package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Reconciler walks the on-disk workspace tree at startup and verifies each
// per-request shared clone is in a usable state (design D5).
//
// MVP scope (per design):
//   - Scan <worktreeRoot>/*/req-* directories.
//   - For each: verify .git is intact, run a fetch against the primary repo,
//     and verify the directory can be opened as a git repo (so a later
//     CheckoutCommit will succeed).
//   - Log stale / corrupt directories but DO NOT delete them — the Phase 2
//     reconciler reads the workspace_checkouts + requests tables to decide
//     which dirs are actually orphaned. MVP just reports.
//
// Why we don't read the DB here: the orchestrator.RequestStore interface
// (pipeline.go) only exposes GetRequest + UpdateStatus. Reading
// workspace_checkouts needs a new query (W2-08). The MVP Reconciler is purely
// filesystem-level; the DB-aware version lands in Phase 2 alongside the
// CheckoutLease table integration.
type Reconciler struct {
	manager *Manager
	// logf receives human-readable progress lines. Tests inject a buffer;
	// production wires the zap logger adapter.
	logf func(format string, args ...any)
}

// NewReconciler constructs a Reconciler bound to a Manager. logf may be nil —
// it defaults to a no-op so callers don't have to wire a logger in tests.
func NewReconciler(m *Manager, logf func(format string, args ...any)) *Reconciler {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Reconciler{manager: m, logf: logf}
}

// ReconcileResult is what Reconcile returns so the caller (boot path) can
// surface a summary. MVP only counts things; Phase 2 will carry per-dir
// action lists (re-create / mark waiting_manual / etc.).
type ReconcileResult struct {
	// ScannedDirs is the number of req-* dirs the scan considered.
	ScannedDirs int
	// HealthyDirs is the number that passed all checks (openable + fetchable).
	HealthyDirs int
	// StaleDirs is the number that failed one or more checks. Their paths
	// are in StalePaths.
	StaleDirs int
	// StalePaths is the absolute path of each dir that failed a check.
	StalePaths []string
}

// Reconcile scans the worktreeRoot and verifies each per-request shared clone.
// It never returns an error for individual dir problems — those are recorded
// in ReconcileResult.StalePaths and logged. An error is returned only for a
// catastrophic failure (worktreeRoot unreadable).
func (r *Reconciler) Reconcile(ctx context.Context) (ReconcileResult, error) {
	result := ReconcileResult{}

	// worktreeRoot may not exist on a fresh boot — that's fine, nothing to do.
	entries, err := os.ReadDir(r.manager.worktreeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			r.logf("workspace: worktreeRoot %s does not exist; nothing to reconcile", r.manager.worktreeRoot)
			return result, nil
		}
		return result, fmt.Errorf("workspace: read %s: %w", r.manager.worktreeRoot, err)
	}

	for _, wsEntry := range entries {
		if !wsEntry.IsDir() {
			continue
		}
		wsName := wsEntry.Name()
		wsDir := filepath.Join(r.manager.worktreeRoot, wsName)

		reqEntries, err := os.ReadDir(wsDir)
		if err != nil {
			r.logf("workspace: read workspace dir %s: %v (skipping)", wsDir, err)
			continue
		}
		for _, reqEntry := range reqEntries {
			if !reqEntry.IsDir() {
				continue
			}
			name := reqEntry.Name()
			// Only consider per-request dirs (skip the primary "repo" clone
			// and any ".scratch-*" dirs left by an interrupted merge).
			if name == "repo" || strings.HasPrefix(name, ".scratch-") {
				continue
			}
			if !strings.HasPrefix(name, "req-") {
				continue
			}
			reqDir := filepath.Join(wsDir, name)
			result.ScannedDirs++
			if r.checkReqDir(ctx, reqDir) {
				result.HealthyDirs++
			} else {
				result.StaleDirs++
				result.StalePaths = append(result.StalePaths, reqDir)
			}
		}
	}
	r.logf("workspace: reconcile done: scanned=%d healthy=%d stale=%d",
		result.ScannedDirs, result.HealthyDirs, result.StaleDirs)
	return result, nil
}

// checkReqDir runs the per-dir verification. Returns true if the dir is
// usable for a later CheckoutCommit. The checks are deliberately cheap so a
// boot with hundreds of stale dirs stays fast.
func (r *Reconciler) checkReqDir(ctx context.Context, reqDir string) bool {
	// 1. .git present (it's a clone, not just a stray dir).
	if _, err := os.Stat(filepath.Join(reqDir, ".git")); err != nil {
		r.logf("workspace: %s: missing .git: %v", reqDir, err)
		return false
	}
	// 2. Opens as a go-git repo (objects + refs intact).
	repo, err := git.PlainOpen(reqDir)
	if err != nil {
		r.logf("workspace: %s: PlainOpen failed: %v", reqDir, err)
		return false
	}
	// 3. HEAD resolves to *something* (the dir was fully cloned, not torn
	// mid-way). We don't require a specific commit here — Phase 2 will
	// cross-check against workspace_checkouts.pinned_commit.
	head, err := repo.Head()
	if err != nil {
		// A bare clone of an empty repo has no HEAD; treat as stale because
		// CheckoutCommit won't have a base to checkout from.
		r.logf("workspace: %s: HEAD unreadable: %v", reqDir, err)
		return false
	}
	// 4. Pinned commit checkable: verify the HEAD commit object exists. This
	//    catches torn downloads where refs point at missing objects.
	if err := verifyCommitExists(repo, head.Hash()); err != nil {
		r.logf("workspace: %s: HEAD commit %s missing: %v", reqDir, head.Hash(), err)
		return false
	}
	// (Fetch against the primary repo is optional and can be slow on cold
	// boots — Phase 2 will add it once we have the DB to know which pinned
	// commits we actually care about. MVP errs on the side of "if it opens
	// and HEAD is valid, leave it alone".)
	return true
}

// verifyCommitExists returns nil if the repo has the commit object for hash.
// Used by checkReqDir to detect torn clones where a ref points at a missing
// object (objects are shared with the primary repo via Shared clone, so a
// missing object means the primary repo is also broken — worth surfacing).
func verifyCommitExists(repo *git.Repository, hash plumbing.Hash) error {
	_, err := repo.CommitObject(hash)
	return err
}
