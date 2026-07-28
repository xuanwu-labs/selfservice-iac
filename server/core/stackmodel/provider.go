// Package stackmodel implements the D24/D29 layered stack model: PathGenerator
// (path identity contract), StackGranularity (partition strategy), DependencyGraph
// (cross-layer topo sort), and LayerService (read-only layer config).
//
// provider.go aggregates the wire ProviderSets of sub-packages that need DI.
// pathgenerator, granularity, and dependency are pure functions (no ProviderSet).
package stackmodel

import (
	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/layer"
	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/pathgenerator"
)

// ProviderSet aggregates stackmodel dependencies for wire.
// PathGenerator is stateless (NewPathGenerator takes no args); LayerService
// needs Repo injection. Granularity and Dependency are pure functions.
var ProviderSet = wire.NewSet(
	layer.ProviderSet,
	pathgenerator.NewPathGenerator,
)
