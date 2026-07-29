// Package workspace manages the Terramate working git repository for the
// orchestrator pipeline (design D1-D6, w2-workspace change).
//
// Per request, the orchestrator needs:
//   - an isolated working directory where codegen output is written + committed
//     (the commit SHA becomes the request's pinned_commit);
//   - the ability to checkout that pinned_commit later for plan / apply;
//   - the ability to release the working dir when the request is done;
//   - on apply success, to squash-merge the request branch into main and push
//     upstream (merge.go);
//   - on platform restart, to reconcile any stale on-disk working dirs
//     (reconcile.go).
//
// # Shared Clone strategy (D1)
//
// We deliberately do NOT use `git worktree`: go-git v5 has no worktree-add API
// (its Repository.Worktree() only returns the main working tree), and go-git
// v6's worktree API is still an unstable x/ package. Instead we mirror the
// object storage with go-git's `CloneOptions.Shared`:
//
//	<worktreeRoot>/<workspaceName>/
//	  ├── repo/        ← primary clone (go-git PlainClone, BARE — see D1)
//	  └── req-<id>/    ← go-git PlainClone(Shared: true) — shared objects,
//	                     INDEPENDENT refs (a branch created here does NOT
//	                     exist in repo/ until explicitly fetched: P0-1).
//
// Shared clones share .git/objects (cheap) but each has its own refs/HEAD, so
// two concurrent requests on the same workspace never collide on refs while
// still benefiting from a single object database.
//
// # git CLI vs go-git (D4)
//
// All operations use go-git except the three that go-git v5 cannot do:
//   - `git pull --ff-only origin main` (no upstream fast-forward in go-git);
//   - `git fetch <dir> <branch>:<branch>` (fetch a specific ref from a shared
//     clone's local refs);
//   - `git merge --squash <branch>` (go-git only supports FastForwardMerge).
//
// Those are shelled out via exec.CommandContext — see merge.go.
package workspace

import (
	// go-git is locked here in addition to manager.go so go.mod retains the
	// dependency even if the rest of the package is in flux. The concrete
	// clone/commit/push logic lives in manager.go / merge.go.
	_ "github.com/go-git/go-git/v5"
)
