// Package codegen implements the W2 MVP code generation engine.
//
// pipeline.go implements the Phase 1 simplified 5-stage parameter resolution
// pipeline (design D3, doc 09 §3). Field names and source names are kept
// compatible with the full 9-stage pipeline (S1-S9) so Phase 2 can upgrade
// in place without changing the rendered output or audit semantics.
//
// Stages (later stages override earlier ones unless noted):
//
//	Stage 1 contract   — module contract defaults (variables_contract_json)
//	Stage 2 defaults   — catalog defaults (catalog_items.defaults_json)
//	Stage 3 governance — platform-forced values (state_key, ownership tags)
//	Stage 4 user       — form values (form_values_json)
//	Stage 5 dependency — cross-layer dependency-injected vars
//
// Priority (highest wins): governance > user > dependency > defaults > contract.
//
// Governance MUST always win: it carries platform-mandatory invariants
// (state_key from PathGenerator, ownership tags) that user input must not be
// able to override. Dependency vars are injected from upstream stacks and
// override catalog defaults but NOT user input (a user cannot accidentally
// shadow a remote_state binding the platform wired up).
package codegen

import (
	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
)

// resolveParams runs the 5-stage Phase 1 parameter pipeline and returns the
// final resolved variable map (name → value) that main.tf renders.
//
// The output is a flat map[string]any. Complex values (lists/maps) are kept as
// their Go decoded shapes; the HCL renderer in hcl.go is responsible for
// emitting them correctly at template time.
//
// Parameters:
//   - contract:   module contract (Stage 1 — only non-required vars seed defaults)
//   - defaults:   catalog defaults_json (Stage 2)
//   - formValues: user form input (Stage 4)
//   - governance: platform-forced values (Stage 3) — highest priority
//   - deps:       cross-layer dependencies (Stage 5) — vars injected per ref
func resolveParams(
	contract *registry.Contract,
	defaults map[string]any,
	formValues map[string]any,
	governance map[string]any,
	deps []DependencyRef,
) map[string]any {
	resolved := make(map[string]any, 32)

	// Stage 1 — contract defaults. Seed every declared variable with its
	// default when one exists (Required vars have nil defaults and are left
	// for later stages to fill or surface as missing at render time).
	if contract != nil {
		for _, v := range contract.Variables {
			if v.Required {
				continue
			}
			resolved[v.Name] = v.Default
		}
	}

	// Stage 2 — catalog defaults. Override contract defaults.
	mergeOverwrite(resolved, defaults)

	// Stage 5 — dependency vars. Applied BEFORE user input so users cannot
	// accidentally shadow a platform-wired remote_state binding, but AFTER
	// defaults so deps win over catalog defaults. Per the priority rule
	// (governance > user > dependency > defaults > contract), dependency sits
	// above defaults but below user.
	mergeDependencyVars(resolved, deps)

	// Stage 4 — user form values. Override defaults + dependencies, but NOT
	// governance (governance is applied last and is non-overridable).
	mergeOverwrite(resolved, formValues)

	// Stage 3 — governance (platform-forced). Applied last so it wins over
	// every other stage, including user input. This is how state_key, ownership
	// tags, and other invariants become tamper-proof.
	mergeOverwrite(resolved, governance)

	return resolved
}

// mergeOverwrite shallow-merges src into dst; existing dst keys are
// overwritten by src. nil src is a no-op. Used by every stage except Stage 5
// (which needs key-scoped merging from DependencyRef.Variables).
func mergeOverwrite(dst, src map[string]any) {
	if src == nil {
		return
	}
	for k, v := range src {
		dst[k] = v
	}
}

// mergeDependencyVars injects variables carried by each DependencyRef. The
// ref's Variables map is {contract_var_name: "data.terraform_remote_state
// .<alias>.outputs.<output>"}. We wrap each expression in RawExpr so
// renderHCLValue emits it WITHOUT quotes (P0-2 fix: dependency expressions
// are HCL references, not string literals).
func mergeDependencyVars(resolved map[string]any, deps []DependencyRef) {
	for _, d := range deps {
		for varName, expr := range d.Variables {
			resolved[varName] = RawExpr(expr)
		}
	}
}
