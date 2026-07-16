package repo

import "github.com/google/wire"

// ProviderSet registers all Repo constructors for wire injection. Imported by
// the data package's data.ProviderSet (which also provides the pool + queries).
//
// core/<domain>/ packages inject *XxxRepo (concrete struct, ferret style).
// To mock in tests, extract a small interface in the core consumer (Go implicit
// interfaces) — no change needed here.
var ProviderSet = wire.NewSet(
	NewTeamRepo,
	NewProjectRepo,
	NewSpaceRepo,
	NewModuleRepo,
	NewModuleVersionRepo,
	NewModuleDependencyRepo,
	NewCatalogRepo,
	NewStackRepo,
	NewStackDependencyRepo,
	NewLayerLogicalRefRepo,
	NewLayerRuleSetVersionRepo,
	NewEnvironmentRepo,
	NewTenantRepo,
	NewEnvironmentTenantBindingRepo,
	NewTagPolicyRepo,
)
