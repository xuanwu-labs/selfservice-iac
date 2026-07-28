// Package layer: service.go — LayerService (read-only) for the layer-first
// stack model (W1-04 tasks 5.1-5.2).
//
// A "layer" in the stack model is an abstraction over two related tables:
//
//	layer_logical_refs        — the stable identity of a layer (logical_id TEXT,
//	                             e.g. "application", "middleware", "global").
//	                             Hard-lifecycle (no deleted_at).
//	layer_rule_set_versions   — versioned policy bundle for layers: the
//	                             layers_json path templates (consumed by
//	                             pathgenerator), status, default flag, etc.
//	                             Versioned-lifecycle (status/superseded_at
//	                             instead of deleted_at).
//
// Phase 1 is read-only: codegen/pathgenerator only need to read the active rule
// set and enumerate layers. Write operations (Create/Supersede) land in Phase 2
// with the admin API.
package layer

import (
	"context"

	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// LayerService exposes read-only access to layer logical refs and rule-set
// versions. The repos are concrete struct pointers (ferret style).
type LayerService struct {
	layerRefRepo *repo.LayerLogicalRefRepo
	ruleSetRepo  *repo.LayerRuleSetVersionRepo
}

// NewLayerService constructs a LayerService from its repo dependencies.
// The repos themselves are provided by repo.ProviderSet (not re-declared here).
func NewLayerService(
	layerRefRepo *repo.LayerLogicalRefRepo,
	ruleSetRepo *repo.LayerRuleSetVersionRepo,
) *LayerService {
	return &LayerService{
		layerRefRepo: layerRefRepo,
		ruleSetRepo:  ruleSetRepo,
	}
}

// ListLayers returns all layer logical refs ordered by created_at.
func (s *LayerService) ListLayers(ctx context.Context) ([]generated.LayerLogicalRef, error) {
	return s.layerRefRepo.List(ctx)
}

// GetLayer returns a layer logical ref by its logical_id.
func (s *LayerService) GetLayer(ctx context.Context, logicalID string) (generated.LayerLogicalRef, error) {
	return s.layerRefRepo.GetByID(ctx, logicalID)
}

// GetActiveRuleSet returns the single active rule-set version (status='active'
// AND is_default=true). This is the version codegen/pathgenerator consult.
func (s *LayerService) GetActiveRuleSet(ctx context.Context) (generated.LayerRuleSetVersion, error) {
	return s.ruleSetRepo.GetActive(ctx)
}

// GetRuleSet returns a rule-set version by version_id (for history/audit).
func (s *LayerService) GetRuleSet(ctx context.Context, versionID int32) (generated.LayerRuleSetVersion, error) {
	return s.ruleSetRepo.GetByID(ctx, versionID)
}

// ListRuleSets returns all rule-set versions, newest version_id first.
func (s *LayerService) ListRuleSets(ctx context.Context) ([]generated.LayerRuleSetVersion, error) {
	return s.ruleSetRepo.List(ctx)
}

// ProviderSet wires LayerService for dependency injection. The repo
// dependencies come from repo.ProviderSet; we only register the constructor.
var ProviderSet = wire.NewSet(
	NewLayerService,
)
