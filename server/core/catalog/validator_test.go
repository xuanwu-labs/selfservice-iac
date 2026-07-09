package catalog_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/catalog"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture %s", path)
	return b
}

// TestValidateSchemaAcceptsValid verifies a well-formed Draft 2020-12 schema
// passes level-1 validation (the schema itself is legal).
func TestValidateSchemaAcceptsValid(t *testing.T) {
	v := catalog.NewValidator()
	schema := loadFixture(t, "testdata/form_schema_valid.json")
	assert.NoError(t, v.ValidateSchema(schema))
}

// TestValidateSchemaRejectsInvalid verifies a malformed schema (bad "type",
// typo'd keyword) fails level-1 validation.
func TestValidateSchemaRejectsInvalid(t *testing.T) {
	v := catalog.NewValidator()
	schema := loadFixture(t, "testdata/form_schema_invalid.json")
	err := v.ValidateSchema(schema)
	require.Error(t, err, "malformed schema must be rejected")
}

// TestValidateInstanceAcceptsConforming verifies level-2 validation passes
// when instance data conforms to the schema.
func TestValidateInstanceAcceptsConforming(t *testing.T) {
	v := catalog.NewValidator()
	schema := loadFixture(t, "testdata/form_schema_valid.json")

	instance := map[string]any{
		"name":     "my-stack",
		"replicas": 3,
		"enabled":  true,
	}
	assert.NoError(t, v.ValidateInstance(schema, instance))
}

// TestValidateInstanceRejectsViolating covers multiple violation kinds.
func TestValidateInstanceRejectsViolating(t *testing.T) {
	v := catalog.NewValidator()
	schema := loadFixture(t, "testdata/form_schema_valid.json")

	t.Run("missing required field", func(t *testing.T) {
		err := v.ValidateInstance(schema, map[string]any{"replicas": 3})
		require.Error(t, err)
	})

	t.Run("wrong type", func(t *testing.T) {
		err := v.ValidateInstance(schema, map[string]any{"name": 123})
		require.Error(t, err)
	})

	t.Run("out of range", func(t *testing.T) {
		err := v.ValidateInstance(schema, map[string]any{"name": "x", "replicas": 9999})
		require.Error(t, err)
	})

	t.Run("additional property rejected", func(t *testing.T) {
		err := v.ValidateInstance(schema, map[string]any{"name": "x", "extra": "no"})
		require.Error(t, err)
	})
}

// TestDraft2020Features verifies Draft 2020-12-specific features work:
// prefixItems (array tuple validation) and the 2020-12 $schema declaration.
func TestDraft2020Features(t *testing.T) {
	v := catalog.NewValidator()
	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "array",
		"prefixItems": []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "integer"},
		},
		"items": false,
	}
	require.NoError(t, v.ValidateSchema(schema))

	assert.NoError(t, v.ValidateInstance(schema, []any{"a", 1}))
	assert.Error(t, v.ValidateInstance(schema, []any{"a", "b"}))  // 2nd must be int
	assert.Error(t, v.ValidateInstance(schema, []any{"a", 1, 2})) // items:false, no extra
}

// TestGoStructInstance verifies ValidateInstance accepts a Go struct (not just maps).
func TestGoStructInstance(t *testing.T) {
	v := catalog.NewValidator()
	schema := loadFixture(t, "testdata/form_schema_valid.json")

	type form struct {
		Name     string `json:"name"`
		Replicas int    `json:"replicas"`
		Enabled  bool   `json:"enabled"`
	}
	assert.NoError(t, v.ValidateInstance(schema, form{Name: "x", Replicas: 2, Enabled: true}))
}
