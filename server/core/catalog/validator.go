// Package catalog validates user-supplied JSON Schemas for service-catalog
// form fields (D40). It is the gate for catalog_items.form_schema_json: a
// user-defined schema that drives the request form, so it MUST be validated
// for both structural legality (against the Draft 2020-12 meta-schema) and
// then used to validate instance data.
package catalog

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator compiles JSON Schemas once and reuses them (D40 §1: compile once).
// Two-level validation:
//  1. ValidateSchema — is the user-supplied schema itself a legal 2020-12 schema?
//  2. ValidateInstance — does the instance data conform to an already-trusted schema?
//
// Remote $ref loading is disabled (D40 §3: user schemas must be self-contained —
// no arbitrary URL fetching, for security and determinism).
type Validator struct {
	compiler *jsonschema.Compiler
}

// NewValidator returns a Validator backed by a fresh compiler configured for
// Draft 2020-12 with no remote resource loading.
func NewValidator() *Validator {
	c := jsonschema.NewCompiler()
	// D40 §3: no remote $ref loader — user schemas must be self-contained.
	// FileLoader is a no-op URLLoader that errors on any remote fetch.
	c.UseLoader(jsonschema.FileLoader{})
	return &Validator{compiler: c}
}

// schemaSeq seeds a unique resource URL per validator so concurrent validators
// don't collide on the "user-schema" resource name.
var schemaSeq int

// ValidateSchema validates that the supplied schema document is a legal
// Draft 2020-12 schema (D40 §4: level 1). Returns nil if legal.
//
// The schema may be a Go value (map/struct) or raw JSON bytes.
func (v *Validator) ValidateSchema(schema any) error {
	doc, err := toDoc(schema)
	if err != nil {
		return fmt.Errorf("parse schema document: %w", err)
	}

	resource := fmt.Sprintf("user-schema-%d", schemaSeq)
	schemaSeq++

	// AddResource + Compile against the 2020-12 meta-schema. Compile fails if
	// the document is not a well-formed Draft 2020-12 schema (it validates the
	// doc against the meta-schema internally).
	if err := v.compiler.AddResource(resource, doc); err != nil {
		return fmt.Errorf("register schema resource: %w", err)
	}
	if _, err := v.compiler.Compile(resource); err != nil {
		return fmt.Errorf("schema is not a valid Draft 2020-12 schema: %w", err)
	}
	return nil
}

// ValidateInstance validates instance data against a trusted schema (D40 §4:
// level 2). The schema must already be trusted (passed ValidateSchema first).
// Returns nil if the instance conforms.
func (v *Validator) ValidateInstance(schema any, instance any) error {
	doc, err := toDoc(schema)
	if err != nil {
		return fmt.Errorf("parse schema document: %w", err)
	}

	resource := fmt.Sprintf("inst-schema-%d", schemaSeq)
	schemaSeq++

	if err := v.compiler.AddResource(resource, doc); err != nil {
		return fmt.Errorf("register schema resource: %w", err)
	}
	compiled, err := v.compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	instDoc, err := toDoc(instance)
	if err != nil {
		return fmt.Errorf("parse instance: %w", err)
	}
	if err := compiled.Validate(instDoc); err != nil {
		return &ValidationError{err: err}
	}
	return nil
}

// ValidationError wraps a jsonschema validation failure.
type ValidationError struct {
	err error
}

func (e *ValidationError) Error() string {
	if e.err == nil {
		return "instance failed schema validation"
	}
	return fmt.Sprintf("instance failed schema validation: %s", e.err.Error())
}

// Unwrap exposes the underlying jsonschema error for errors.Is/As.
func (e *ValidationError) Unwrap() error { return e.err }

// toDoc normalizes a Go value or raw JSON []byte into the any form the
// jsonschema compiler expects (plain maps/lists/scalars after JSON round-trip).
func toDoc(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case []byte:
		var out any
		if err := json.Unmarshal(x, &out); err != nil {
			return nil, err
		}
		return out, nil
	case json.RawMessage:
		var out any
		if err := json.Unmarshal(x, &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		buf, err := json.Marshal(x)
		if err != nil {
			return nil, err
		}
		var out any
		if err := json.Unmarshal(buf, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}
