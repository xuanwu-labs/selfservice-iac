// Package codegen implements the W2 MVP code generation engine.
//
// provider.go wires the codegen Generator for wire DI. The Generator depends
// only on PathGenerator (a stateless type supplied by the stackmodel
// ProviderSet), so the codegen ProviderSet is a single constructor binding.
package codegen

import "github.com/google/wire"

// ProviderSet exposes Generator to the wire dependency graph.
//
// codegen does not own its own PathGenerator binding — it reuses the one
// already provided by stackmodel.ProviderSet (pathgenerator.NewPathGenerator),
// so there is exactly one PathGenerator instance in the graph. cmd/server/
// wire.go pulls this set into the aggregate core.ProviderSet when the W2
// orchestrator lands.
var ProviderSet = wire.NewSet(
	NewGenerator,
)
