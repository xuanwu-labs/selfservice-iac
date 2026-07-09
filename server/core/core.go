// Package core is the domain heart of Aether (package-by-feature layering).
// core.go aggregates the wire ProviderSets of every domain package so that
// cmd/server/wire.go can compose them with a single core.ProviderSet.
//
// Status: scaffold stage — the 25 domain packages (registry, catalog, codegen,
// orchestrator, drift, ...) are empty placeholders. As each business package
// lands (per iac-self-service-platform waves), append its ProviderSet here:
//
//	var ProviderSet = wire.NewSet(
//	    registry.ProviderSet,
//	    catalog.ProviderSet,
//	    codegen.ProviderSet,
//	    // ... etc
//	)
//
// Until then, an empty set keeps the wire graph valid for the scaffold's
// /healthz-only server (core has no providers the health path depends on).
package core

import "github.com/google/wire"

// ProviderSet aggregates domain-layer dependencies.
// Empty during scaffold stage; populated as business packages land.
var ProviderSet = wire.NewSet()
