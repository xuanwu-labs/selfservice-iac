// Package core is the domain heart of Aether (package-by-feature layering).
// core.go aggregates the wire ProviderSets of every domain package so that
// cmd/server/wire.go can compose them with a single core.ProviderSet.
//
// As each business package lands (per iac-self-service-platform waves),
// append its ProviderSet here. Adding a new domain package = one line here,
// nothing else changes in wire.go.
package core

import (
	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/git"
	"github.com/xuanwu-labs/selfservice-iac/server/core/catalog"
	"github.com/xuanwu-labs/selfservice-iac/server/core/clock"
	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
	"github.com/xuanwu-labs/selfservice-iac/server/core/orchestrator"
	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel"
	"github.com/xuanwu-labs/selfservice-iac/server/core/tenancy"
)

// ProviderSet aggregates domain-layer dependencies.
// W1-03: catalog + registry + git GoGitProvider
// W1-04: tenancy + stackmodel (PathGenerator + LayerService)
// W2: codegen (Generator) + orchestrator (StateMachine + Pipeline + Approval)
// queue.ProviderSet will be added when the worker lifecycle wrapper lands.
var ProviderSet = wire.NewSet(
	catalog.ProviderSet,
	clock.ProviderSet,
	registry.ProviderSet,
	tenancy.ProviderSet,
	stackmodel.ProviderSet,
	codegen.ProviderSet,
	orchestrator.ProviderSet,
	git.NewGoGitProvider,
	wire.Bind(new(git.GitProvider), new(*git.GoGitProvider)),
	// Future: queue.ProviderSet, workspace.ProviderSet, ...
)
