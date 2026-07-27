// Package catalog: provider.go — wire ProviderSet for the catalog module.
//
// Registers the D40 JSON Schema Validator (validator.go) and CatalogService
// (service.go). CatalogService's repo dependencies (*repo.CatalogRepo,
// *repo.ModuleVersionRepo, *repo.ModuleRepo) are provided by repo.ProviderSet;
// they are not re-declared here to avoid duplicate providers.
package catalog

import "github.com/google/wire"

// ProviderSet provides the catalog schema Validator and the CatalogService
// that orchestrates PublishCatalogItem.
var ProviderSet = wire.NewSet(
	NewValidator,
	NewCatalogService,
)
