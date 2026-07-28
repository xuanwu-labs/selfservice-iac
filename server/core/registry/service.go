// Package registry implements module registration: clone Git source, extract
// a pure-scalar contract, persist via ModuleRepo (W1-02).
//
// service.go implements RegistryService — the orchestration layer for
// RegisterModule. It wires together GitProvider (clone), ContractExtractor
// (HCL → scalar contract), and ModuleRepo.CreateWithVersion (atomic DB write).
//
// State machine (task 3.2): modules.status is written directly in the single
// CreateWithVersion write as either "validated" (HCL parse succeeded) or
// "validation_failed" (HCL parse failed). There is no separate pending_validation
// row first — the registration flow is atomic: extract → write status in one
// transaction. (The pending_validation status is reserved for future async
// validation with real terraform validate in W2.)
//
// terraform validate (real init+validate) is deferred to W2; MVP treats HCL
// parse success as validation (design D6).

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/git"
	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// ProviderSet wires RegistryService for DI.
var ProviderSet = wire.NewSet(
	NewRegistryService,
	NewContractExtractor,
)

// RegisterModuleInput is the request to RegistryService.RegisterModule.
// Mirrors proto RegisterModuleRequest (registry/dto.proto) but uses Go-native
// types (owner_team_id is int64 here, parsed from string in the handler).
type RegisterModuleInput struct {
	GitSource   string // git repo URL
	ModulePath  string // subdirectory within repo (e.g. "atomic/rds-mysql"); "" = repo root
	Version     string // semver tag (e.g. "v1.0.0")
	Provider    string // cloud provider slug (e.g. "alicloud")
	Name        string // module display name (e.g. "rds-mysql")
	Description string // human-readable description
	OwnerTeamID int64  // owning team (FK teams.id)
	Layer       string // layer slug (informational; authoritative = catalog_items.layer_logical_id)
}

// RegisterModuleResult is the outcome of a successful registration.
type RegisterModuleResult struct {
	ModuleID        int64
	ModuleVersionID int64
	CommitSHA       string
	Status          string // "validated" or "validation_failed"
}

// RegistryService orchestrates module registration: clone → extract → persist.
type RegistryService struct {
	moduleRepo *repo.ModuleRepo
	git        git.GitProvider
	extractor  *ContractExtractor
}

// NewRegistryService constructs a RegistryService. The GitProvider and
// ContractExtractor are injected (wire); for tests, pass fakes.
func NewRegistryService(moduleRepo *repo.ModuleRepo, g git.GitProvider, e *ContractExtractor) *RegistryService {
	return &RegistryService{moduleRepo: moduleRepo, git: g, extractor: e}
}

// RegisterModule runs the full registration flow:
//  1. Clone the Git source at the requested version into a temp dir.
//  2. Resolve the commit SHA (recorded on module_versions.commit_sha).
//  3. Extract the pure-scalar contract from variables.tf/outputs.tf.
//  4. Persist module + module_version atomically (ModuleRepo.CreateWithVersion).
//  5. Transition status: pending_validation → validated (or validation_failed).
//
// On HCL parse failure, the module is still persisted with status
// validation_failed (so the operator can see the attempt in the UI and retry),
// and the error is returned to the caller.
func (s *RegistryService) RegisterModule(ctx context.Context, in RegisterModuleInput) (*RegisterModuleResult, error) {
	if in.GitSource == "" {
		return nil, fmt.Errorf("registry: git_source is required")
	}
	if in.Version == "" {
		return nil, fmt.Errorf("registry: version is required")
	}

	// 1. Clone into a temp dir. Cleaned up on return.
	cloneDest, err := os.MkdirTemp("", "aether-register-*")
	if err != nil {
		return nil, fmt.Errorf("registry: create temp dir: %w", err)
	}
	defer os.RemoveAll(cloneDest)

	if err := s.git.Clone(ctx, in.GitSource, in.Version, cloneDest); err != nil {
		return nil, fmt.Errorf("registry: clone %s@%s: %w", in.GitSource, in.Version, err)
	}

	// 2. Resolve commit SHA.
	commitSHA, err := s.git.CommitSHA(ctx, cloneDest)
	if err != nil {
		return nil, fmt.Errorf("registry: resolve commit sha: %w", err)
	}

	// 3. Extract contract. On parse failure we persist with status
	//    validation_failed (so operator sees the attempt) and return an error.
	//    The contract_json is always valid JSON (no string concatenation of
	//    error messages, which could corrupt the blob — P0-5).
	contract, extractErr := s.extractor.ExtractFromRepo(cloneDest, in.ModulePath)
	status := "validated"
	if extractErr != nil {
		status = "validation_failed"
		// Empty contract on failure; validation_error is stored in the Contract
		// struct's optional field so json.Marshal produces valid JSON always.
		contract = &Contract{
			Variables:       []ContractVariable{},
			Outputs:         []ContractOutput{},
			ValidationError: extractErr.Error(),
		}
	}
	b, err := json.Marshal(contract)
	if err != nil {
		return nil, fmt.Errorf("registry: marshal contract: %w", err)
	}
	contractJSON := b

	// Providers JSON (Gap 1 fix): serialize required_providers + required_core
	// into module_versions.providers_json for D22 toolchain compatibility checks.
	providersJSON, err := json.Marshal(map[string]any{
		"required_core":      contract.RequiredCore,
		"required_providers": contract.Providers,
	})
	if err != nil {
		providersJSON = []byte(`{}`)
	}

	// 4. Persist module + version atomically.
	moduleID := utils.GenerateID()
	versionID := utils.GenerateID()
	modArg := generated.CreateModuleParams{
		ID:          moduleID,
		Name:        in.Name,
		GitSource:   in.GitSource,
		Provider:    in.Provider,
		Layer:       in.Layer,
		OwnerTeamID: in.OwnerTeamID,
		Status:      status,
		Description: in.Description,
	}
	verArg := generated.CreateModuleVersionParams{
		ID:                    versionID,
		ModuleID:              moduleID, // CreateWithVersion overwrites this with the real module ID
		Version:               in.Version,
		CommitSha:             commitSHA,
		ProvidersJson:         providersJSON,
		VariablesContractJson: contractJSON,
		IsCurrent:             true,
	}
	mod, _, err := s.moduleRepo.CreateWithVersion(ctx, modArg, verArg)
	if err != nil {
		return nil, fmt.Errorf("registry: persist module+version: %w", err)
	}

	// 5. Return result. If extraction failed, the module was persisted with
	//    status=validation_failed — return nil, err (Go convention: on error,
	//    non-error returns are zero-valued). The operator can find the persisted
	//    module via ListModules(status=validation_failed) and retry.
	result := &RegisterModuleResult{
		ModuleID:        mod.ID,
		ModuleVersionID: versionID,
		CommitSHA:       commitSHA,
		Status:          status,
	}
	if extractErr != nil {
		return nil, fmt.Errorf("registry: module persisted with status=validation_failed (query ListModules to find it): %w", extractErr)
	}
	return result, nil
}
