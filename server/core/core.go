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
	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
)

// ProviderSet aggregates domain-layer dependencies.
// W1-03: catalog (Validator + CatalogService) + registry (RegistryService +
// ContractExtractor) + git GoGitProvider bound to GitProvider interface.
// queue.ProviderSet will be added when the worker lifecycle wrapper lands
// (NewClient needs Start/Stop lifecycle management, not a plain provider).
var ProviderSet = wire.NewSet(
	catalog.ProviderSet,
	clock.ProviderSet,
	registry.ProviderSet,
	git.NewGoGitProvider,
	// Bind the concrete GoGitProvider to the GitProvider interface so consumers
	// (RegistryService) get the interface injected.
	wire.Bind(new(git.GitProvider), new(*git.GoGitProvider)),
	// Future: queue.ProviderSet, codegen.ProviderSet, ...
)
