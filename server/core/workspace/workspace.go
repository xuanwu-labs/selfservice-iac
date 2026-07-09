// Package workspace manages the Terramate working git repository (clone/fetch/
// commit/push). Per D4, the platform uses go-git to own the infra repo and
// persists remote+branch+commit metadata so it can restore a consistent
// working directory after a restart.
//
// This is a scaffold placeholder — the full implementation (clone, branch
// checkout, metadata persistence, recovery) is delivered by the
// iac-self-service-platform change (D4 / docs/11).
package workspace

import (
	// go-git is locked here so go.mod retains the dependency (D4 / task 14.3).
	// The concrete clone/commit logic lands with the orchestrator module.
	_ "github.com/go-git/go-git/v5"
)
