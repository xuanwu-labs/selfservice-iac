// Package workspace provider.go wires the Manager for wire DI. The Manager is
// constructed from two config values (worktreeRoot + nodeID) which the cmd/
// server wire.go supplies from viper; this set just declares the constructor
// binding.
//
// The Manager satisfies orchestrator.WorkspaceManager; that interface binding
// lives in core/core.go (added when the orchestrator's ProviderSet is
// composed) so this package does not need to import orchestrator just to
// declare the wire.Bind.
package workspace

import "github.com/google/wire"

// ProviderSet exposes Manager to the wire dependency graph.
//
// worktreeRoot and nodeID are expected to come from config (viper string
// bindings) — they're provided elsewhere in the graph, not in this set. cmd/
// server/wire.go is responsible for adding the value bindings (e.g.
// wire.Value or a provider that reads viper).
var ProviderSet = wire.NewSet(
	NewManager,
)
