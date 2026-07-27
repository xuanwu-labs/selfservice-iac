// Package catalog: formgen.go — generates a user-facing form JSON Schema
// (catalog_items.form_schema_json) from a registry.Contract (W1-03 D3).
//
// The contract captures every scalar variable of a module. The form schema
// crops that down to ONLY what an end user (requester) should see: secrets and
// platform-inferable identifiers are hidden, required fields are surfaced (and
// enforced via the `required` array), and optional fields with a scalar default
// are exposed as pre-filled defaults the user can override.
//
// The output is a Draft 2020-12 JSON Schema document; it is re-checked at
// publish time by the D40 Validator (validator.go) before being persisted.

package catalog

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
)

// platformInferredVars is the MVP heuristic (W1-03 D3) for variables that the
// platform injects at request resolution time (doc 08 param pipeline S1-S5):
// they are NOT user-facing, so they are hidden from the form regardless of
// required/default. Matching is case-insensitive on the bare variable name
// (terraform identifiers are lowercase by convention).
//
// region / vpc_id / subnet_id  — injected from envtenant bindings (S3/S5)
// tenant                       — injected from request.tenant_id
// env                          — injected from request.env_id
var platformInferredVars = map[string]struct{}{
	"region":    {},
	"vpc_id":    {},
	"subnet_id": {},
	"tenant":    {},
	"env":       {},
}

// GenerateFormSchema crops a module contract down to the user-visible form
// schema (W1-03 D3). It is a free function: no state is needed and the result
// is a pure function of the input contract, which makes it trivially testable.
//
// Cropping rules (applied in order, first match wins):
//  1. HIDE if Sensitive=true   — runtime injection (secrets), never user-facing.
//  2. HIDE if platform-inferred (region, vpc_id, subnet_id, tenant, env).
//  3. EXPOSE if Required=true  — added to `properties` AND `required`.
//  4. EXPOSE if Required=false with a scalar Default — added to `properties`
//     with that `default` (user may override).
//  5. SKIP otherwise (complex/optional with no scalar default).
//
// Returns a Draft 2020-12 schema document of shape:
//
//	{ "type": "object", "properties": { ... }, "required": [ ... ] }
//
// `properties` and `required` are always present (possibly empty) so the
// document is a legal object schema even when nothing is user-visible.
func GenerateFormSchema(contract *registry.Contract) (json.RawMessage, error) {
	if contract == nil {
		return nil, fmt.Errorf("formgen: contract is nil")
	}

	properties := make(map[string]map[string]any)
	var required []string

	for _, v := range contract.Variables {
		// Rule 1: secrets are never user-facing.
		if v.Sensitive {
			continue
		}
		// Rule 2: platform-injected identifiers.
		if isPlatformInferred(v.Name) {
			continue
		}

		jsonType, ok := tfTypeToJSONSchema(v.Type)
		// Rule 5 (partial): complex types we can't represent scalars for are
		// skipped, UNLESS the variable is required — a required complex var is
		// still surfaced as an opaque object so the user must supply it.
		if !ok && !v.Required {
			continue
		}

		prop := map[string]any{"type": jsonType}
		if v.Description != "" {
			prop["description"] = v.Description
		}

		// Rule 3 vs Rule 4.
		if v.Required {
			prop["type"] = jsonType // may be "object" fallback for required-complex
			properties[v.Name] = prop
			required = append(required, v.Name)
			continue
		}

		// Optional: only expose when a scalar default exists.
		if !isScalar(v.Default) {
			continue
		}
		prop["default"] = v.Default
		properties[v.Name] = prop
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}

	buf, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("formgen: marshal form schema: %w", err)
	}
	return buf, nil
}

// isPlatformInferred reports whether the variable name is one the platform
// injects at resolution time. Case-insensitive on the bare name.
func isPlatformInferred(name string) bool {
	_, ok := platformInferredVars[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// tfTypeToJSONSchema maps a Terraform type expression to a JSON Schema 2020-12
// type keyword. Returns (type, true) for scalar types we can surface; for
// anything complex (list/map/object/tuple/set/any) it returns ("object", false)
// to let the caller decide whether to still include the variable.
//
// Terraform types come from tfconfig as Go strings (after the cty → string
// normalization in terraform-config-inspect). They may be:
//   - "string", "number", "bool"           (simple)
//   - "list(...)", "set(...)", "tuple(...)" (collections)
//   - "map(...)", "object(...)"             (collections)
//   - "any"                                 (dynamic)
//
// number maps to "integer" when the default is an integer-shaped value and to
// "number" otherwise; here we default to "number" (the more permissive type)
// and rely on the caller's default value to carry the actual shape. This keeps
// the mapping a pure function of the type string.
func tfTypeToJSONSchema(tfType string) (string, bool) {
	switch strings.TrimSpace(tfType) {
	case "string":
		return "string", true
	case "number":
		return "number", true
	case "bool":
		return "boolean", true
	default:
		// list/set/map/object/tuple/any/empty — complex or unknown.
		return "object", false
	}
}

// isScalar reports whether v is a scalar the contract/JSON round-trips cleanly:
// string, bool, or a JSON number (int/float). Mirrors the scalarDefault rule
// in the registry extractor (D25): complex defaults are not encoded.
func isScalar(v any) bool {
	switch v.(type) {
	case string, bool, int, int32, int64, float32, float64:
		return true
	default:
		return false
	}
}
