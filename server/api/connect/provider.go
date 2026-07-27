// Package connect provides Connect-RPC handlers for Aether's business APIs.
// provider.go aggregates the wire ProviderSet for all Connect handlers.
//
// As new services land (RequestService, ApprovalService, ...), add their
// constructors here so wire picks them up without touching cmd/server/wire.go.
package connect

import "github.com/google/wire"

// ProviderSet aggregates all Connect-RPC handler providers.
// W1-03: CatalogHandler (List/Get) + CatalogAdminHandler (Publish/Update/Deprecate)
// + RegistryHandler (RegisterModule).
var ProviderSet = wire.NewSet(
	NewCatalogHandler,
	NewCatalogAdminHandler,
	NewRegistryHandler,
)
