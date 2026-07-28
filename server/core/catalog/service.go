// Package catalog: service.go — CatalogService orchestrates the
// PublishCatalogItem flow (W1-03 task 4.3).
//
// Publish turns a registered module version into a user-requestable catalog
// item. The pipeline is:
//
//	1. fetch module_version (+ its variables_contract_json) + parent module
//	2. unmarshal the contract JSON
//	3. GenerateFormSchema(contract) → form_schema_json   (W1-03 D3 cropping)
//	4. ApplyDefaults(module.Name, contract) → defaults_json  (W1-03 4.2)
//	5. validator.ValidateSchema(form_schema_json)  (D40 gate — never persist
//	   an untrusted/malformed form schema)
//	6. catalogRepo.Publish(...) → row in catalog_items
//
// The service depends on the existing Validator (D40, validator.go) — it does
// NOT reimplement schema validation.

package catalog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// CatalogService orchestrates catalog item publishing. Repos are concrete
// struct pointers (ferret style, same as RegistryService); tests inject fakes
// by having the consumer define a small interface — no change needed here.
type CatalogService struct {
	catalogRepo       *repo.CatalogRepo
	moduleVersionRepo *repo.ModuleVersionRepo
	moduleRepo        *repo.ModuleRepo
	validator         *Validator
}

// NewCatalogService constructs a CatalogService. The Validator is the existing
// D40 validator (shared via the catalog ProviderSet).
//
// moduleRepo is needed to resolve the parent module's name (for ApplyDefaults
// substring matching) — ModuleVersion carries only module_id, not the name.
func NewCatalogService(
	catalogRepo *repo.CatalogRepo,
	moduleVersionRepo *repo.ModuleVersionRepo,
	moduleRepo *repo.ModuleRepo,
	validator *Validator,
) *CatalogService {
	return &CatalogService{
		catalogRepo:       catalogRepo,
		moduleVersionRepo: moduleVersionRepo,
		moduleRepo:        moduleRepo,
		validator:         validator,
	}
}

// PublishInput is the request to CatalogService.Publish. It mirrors the proto
// PublishCatalogItemRequest but uses Go-native types (team_ids are int64 /
// string at this layer; see Visibility field comment).
type PublishInput struct {
	ModuleVersionID int64    // FK module_versions.id (the pinned version)
	DisplayName     string   // user-facing name (e.g. "RDS MySQL")
	Description     string   // human-readable; "" allowed
	Category        string   // grouping label (e.g. "database")
	LayerLogicalID  string   // layer this item belongs to; "" = unassigned
	OwnerTeamID     int64    // FK teams.id
	Visibility      []string // team_ids that can see/request this item; empty = global
}

// Publish runs the full publish pipeline and returns the persisted CatalogItem.
//
// On any pre-DB failure (contract unmarshal, form gen, defaults gen, or D40
// validation) nothing is written and the error is returned. The D40
// ValidateSchema gate is the security boundary: a malformed form schema never
// reaches catalog_items.form_schema_json.
func (s *CatalogService) Publish(ctx context.Context, in PublishInput) (generated.CatalogItem, error) {
	if in.ModuleVersionID == 0 {
		return generated.CatalogItem{}, fmt.Errorf("catalog: module_version_id is required")
	}
	if in.DisplayName == "" {
		return generated.CatalogItem{}, fmt.Errorf("catalog: display_name is required")
	}
	if in.OwnerTeamID == 0 {
		return generated.CatalogItem{}, fmt.Errorf("catalog: owner_team_id is required")
	}

	// 1. Fetch the pinned module version + its parent module (for the name).
	version, err := s.moduleVersionRepo.GetByID(ctx, in.ModuleVersionID)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: load module version %d: %w", in.ModuleVersionID, err)
	}
	module, err := s.moduleRepo.GetByID(ctx, version.ModuleID)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: load module %d: %w", version.ModuleID, err)
	}

	// 2. Unmarshal the contract JSON. An empty/missing contract is treated as
	//    the empty contract (no variables) so the flow still produces a valid
	//    (empty) form schema — useful for bootstrapping modules without a
	//    variables.tf yet.
	contract := &registry.Contract{Variables: []registry.ContractVariable{}, Outputs: []registry.ContractOutput{}}
	if len(version.VariablesContractJson) > 0 {
		if err := json.Unmarshal(version.VariablesContractJson, contract); err != nil {
			return generated.CatalogItem{}, fmt.Errorf("catalog: parse contract json: %w", err)
		}
	}

	// 3. Form schema (S1 of the doc-08 param pipeline).
	formSchema, err := GenerateFormSchema(contract)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: generate form schema: %w", err)
	}

	// 4. Catalog defaults (S2).
	defaultsJSON, err := ApplyDefaults(module.Name, contract)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: apply defaults: %w", err)
	}

	// 5. D40 gate — validate the generated form schema as a legal Draft
	//    2020-12 document before persisting it. GenerateFormSchema is
	//    deterministic, but the gate is defense-in-depth: a future bug or a
	//    hand-edited contract must not be able to land a broken schema.
	if err := s.validator.ValidateSchema(formSchema); err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: form schema failed D40 validation: %w", err)
	}

	// 6. Persist. Defaults follow the catalog_items schema constraints:
	//      status         = 'active'         (publish = active)
	//      cardinality    = 'single'         (D25 MVP; multi-instance is W2)
	//      stack_grouping = 'per-component'  (D24 default)
	//    Optional JSON blobs that have no caller-supplied value are seeded
	//    with their schema defaults (empty object/array) so the row is never
	//    NULL on the JSONB columns.
	visibilityJSON, err := marshalVisibility(in.Visibility)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: marshal visibility: %w", err)
	}

	var layerLogicalID *string
	if in.LayerLogicalID != "" {
		lid := in.LayerLogicalID
		layerLogicalID = &lid
	}

	arg := generated.PublishCatalogItemParams{
		ID:                     utils.GenerateID(),
		ModuleVersionID:        in.ModuleVersionID,
		DisplayName:            in.DisplayName,
		Description:            in.Description,
		Category:               in.Category,
		Status:                 "active",
		FormSchemaJson:         formSchema,
		DefaultsJson:           defaultsJSON,
		Cardinality:            "single",
		InstanceKey:            "",
		PerInstanceFieldsJson:  []byte(`{}`),
		SharedFieldsJson:       []byte(`{}`),
		LayerLogicalID:         layerLogicalID,
		StackGrouping:          "per-component",
		OwnerTeamID:            in.OwnerTeamID,
		DefaultTagsJson:        []byte(`{}`),
		UserAllowedTagKeysJson: []byte(`[]`),
		VisibilityJson:         visibilityJSON,
	}

	item, err := s.catalogRepo.Publish(ctx, arg)
	if err != nil {
		return generated.CatalogItem{}, fmt.Errorf("catalog: publish catalog item: %w", err)
	}
	return item, nil
}

// marshalVisibility serializes the Visibility team_id list. An empty list
// means "global" (any team can see/request) and is persisted as `[]`, matching
// the catalog_items.visibility_json default and the GIN-indexed containment
// query in ListVisible.
func marshalVisibility(teamIDs []string) ([]byte, error) {
	if len(teamIDs) == 0 {
		return []byte(`[]`), nil
	}
	return json.Marshal(teamIDs)
}
