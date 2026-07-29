// Package orchestrator provider.go exposes the wire ProviderSet so the
// assembled app (cmd/server/wire.go) can construct the Pipeline and
// ApprovalService with their injected collaborators.
//
// The concrete adapters (CodeGenerator / TerramateRunner / WorkspaceManager /
// RequestStore / EventLogger / ApprovalDecisionRecorder) are bound elsewhere —
// in their owning packages' ProviderSets or via wire.Bind in
// core/adapters/provider.go — so this file only declares the two constructors
// the orchestrator package itself owns.
package orchestrator

import "github.com/google/wire"

// ProviderSet wires the orchestrator's own constructors. Interface bindings
// for the collaborators (RequestStore, CodeGenerator, TerramateRunner,
// WorkspaceManager, EventLogger, ApprovalDecisionRecorder) live in the
// packages that own the concrete types; add wire.Bind entries there as each
// real implementation lands (W1-02 repo, W2-05 codegen, W2-07 workspace, ...).
var ProviderSet = wire.NewSet(
	NewPipeline,
	NewApprovalService,
	NewEventLogger,
)
