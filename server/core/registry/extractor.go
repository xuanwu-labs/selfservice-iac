// Package registry implements module registration: clone Git source, extract
// a pure-scalar contract from variables.tf/outputs.tf, and persist via
// ModuleRepo (W1-02).
//
// extractor.go implements ContractExtractor using terraform-config-inspect
// (tfconfig). Per D25 (zero-intrusion), the contract is pure-scalar: complex
// defaults (list/map/object) are set to nil while the type declaration is
// preserved. This lets community modules (terraform-aws-modules, alicloud
// official) be reused without modification.

package registry

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// ContractVariable is one scalar contract entry, serialized to
// module_versions.variables_contract_json. The shape is intentionally flat and
// JSON-friendly so codegen (W1-04) and catalog form_schema generation (W1-03
// Group 4) consume it without re-parsing HCL.
type ContractVariable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     any    `json:"default"` // scalar only; complex → nil
	Description string `json:"description"`
	Sensitive   bool   `json:"sensitive"`
	Required    bool   `json:"required"` // true when default == nil
}

// ContractOutput mirrors an output from outputs.tf. Only scalar outputs are
// relevant for cross-layer remote_state injection (D3 cross-layer dependency).
type ContractOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sensitive   bool   `json:"sensitive"`
}

// Contract is the extracted module contract, serialized as
// variables_contract_json. Codegen + formgen read this single structure.
type Contract struct {
	Variables []ContractVariable `json:"variables"`
	Outputs   []ContractOutput   `json:"outputs"`
	// Providers mirrors required_providers from versions.tf (Gap 1 fix).
	// Stored separately in module_versions.providers_json by RegistryService;
	// included here so callers can inspect the full module surface in one type.
	Providers    []ContractProvider `json:"providers,omitempty"`
	RequiredCore []string           `json:"required_core,omitempty"`
	// ValidationError records why extraction failed (status=validation_failed).
	// Empty when extraction succeeded. Stored inside variables_contract_json so
	// the blob is always valid JSON (no string concatenation).
	ValidationError string `json:"validation_error,omitempty"`
}

// ContractProvider is one required_provider entry from versions.tf, used to
// populate module_versions.providers_json (D22 toolchain compatibility check).
type ContractProvider struct {
	LocalName   string   `json:"local_name"`            // e.g. "alicloud"
	Source      string   `json:"source"`                // e.g. "aliyun/alicloud"
	Constraints []string `json:"constraints,omitempty"` // e.g. ["1.280.0"]
}

// ContractExtractor parses .tf files at a given path and returns a pure-scalar
// Contract. Stateful instances are not needed (tfconfig.LoadDir is the only
// entry point), so Extractor is a zero-field struct — methods are package-level
// style but kept on a type for future extensibility (e.g. caching, custom
// resolvers).
type ContractExtractor struct{}

// NewContractExtractor returns a ContractExtractor.
func NewContractExtractor() *ContractExtractor { return &ContractExtractor{} }

// Extract loads the module at moduleDir (absolute or relative path to the dir
// containing .tf files — typically a subdirectory within a cloned repo) and
// returns the pure-scalar Contract. Returns a structured error if the path is
// not a valid Terraform module (missing .tf files, syntax errors, etc.).
//
// Note: modulePath within a cloned repo must be resolved by the caller — pass
// filepath.Join(cloneDest, modulePath) here.
func (e *ContractExtractor) Extract(moduleDir string) (*Contract, error) {
	// tfconfig.LoadModule reads .tf files directly (no terraform init needed).
	// It populates Module.Variables / Module.Outputs from parsing the HCL.
	mod, diags := tfconfig.LoadModule(moduleDir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("extract contract from %s: %w", moduleDir, diags.Err())
	}

	contract := &Contract{
		Variables: make([]ContractVariable, 0, len(mod.Variables)),
		Outputs:   make([]ContractOutput, 0, len(mod.Outputs)),
	}

	// Variables: keep scalar defaults, nil-out complex defaults (D25).
	for name, v := range mod.Variables {
		cv := ContractVariable{
			Name:        name,
			Type:        v.Type,
			Description: v.Description,
			Sensitive:   v.Sensitive,
			Required:    v.Required,
			Default:     nil,
		}
		if !v.Required {
			cv.Default = scalarDefault(v.Default)
		}
		contract.Variables = append(contract.Variables, cv)
	}

	// Outputs: record name/description/sensitive (values are runtime, not contract).
	for name, o := range mod.Outputs {
		contract.Outputs = append(contract.Outputs, ContractOutput{
			Name:        name,
			Description: o.Description,
			Sensitive:   o.Sensitive,
		})
	}

	// Providers (Gap 1 fix): extract required_providers + required_core from
	// versions.tf → module_versions.providers_json (D22 toolchain check input).
	contract.RequiredCore = mod.RequiredCore
	for localName, req := range mod.RequiredProviders {
		contract.Providers = append(contract.Providers, ContractProvider{
			LocalName:   localName,
			Source:      req.Source,
			Constraints: req.VersionConstraints,
		})
	}

	return contract, nil
}

// ExtractFromRepo is a convenience wrapper: given a cloned repo root and a
// modulePath (subdirectory, e.g. "atomic/rds-mysql"), it joins them and calls
// Extract. Empty modulePath means the repo root is the module itself.
func (e *ContractExtractor) ExtractFromRepo(cloneDest, modulePath string) (*Contract, error) {
	dir := cloneDest
	if modulePath != "" {
		dir = filepath.Join(cloneDest, filepath.FromSlash(modulePath))
	}
	return e.Extract(dir)
}

// scalarDefault returns the value if it is a Go scalar (string/number/bool),
// otherwise nil. This enforces D25 zero-intrusion: atomic modules stay
// single-instance, complex defaults are not encoded in the contract (cardinality
// is a catalog-item concern, per D25).
//
// tfconfig decodes HCL defaults into Go types:
//   - string → string
//   - number → float64 (tfconfig uses cty → json round-trip)
//   - bool → bool
//   - list(...) → []interface{}
//   - map(...) → map[string]interface{}
//   - object(...) → map[string]interface{}
//
// We accept only the first three; everything else is treated as complex.
func scalarDefault(v any) any {
	switch v.(type) {
	case string, float64, int, int64, bool:
		return v
	default:
		// Complex type (list/map/object/nil-of-complex) — D25 says nil.
		// Note: a literal nil default is unusual (variables either have a
		// default or are required), but we treat it as complex → nil too.
		return nil
	}
}
