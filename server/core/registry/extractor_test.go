package registry_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
)

// TestContractExtractor_RDS_MySQL verifies scalar/complex/sensitive extraction
// against the rds-mysql fixture (W1-03 task 2.3).
func TestContractExtractor_RDS_MySQL(t *testing.T) {
	e := registry.NewContractExtractor()
	dir := filepath.Join("testdata", "rds-mysql")

	c, err := e.Extract(dir)
	require.NoError(t, err)

	// Build a name→variable map for assertion convenience.
	vars := map[string]registry.ContractVariable{}
	for _, v := range c.Variables {
		vars[v.Name] = v
	}

	// instance_type: required string, no default.
	it := vars["instance_type"]
	assert.Equal(t, "string", it.Type)
	assert.True(t, it.Required, "instance_type has no default → required")
	assert.Nil(t, it.Default)
	assert.False(t, it.Sensitive)

	// engine_version: string with scalar default "8.0".
	ev := vars["engine_version"]
	assert.Equal(t, "string", ev.Type)
	assert.False(t, ev.Required)
	assert.Equal(t, "8.0", ev.Default, "scalar default MUST be preserved")

	// storage_size: number with scalar default 100.
	ss := vars["storage_size"]
	assert.False(t, ss.Required)
	// tfconfig decodes numbers as float64 via cty; accept float64 or int.
	switch d := ss.Default.(type) {
	case float64:
		assert.Equal(t, float64(100), d)
	case int:
		assert.Equal(t, 100, d)
	default:
		t.Errorf("storage_size default should be scalar number, got %T: %v", d, d)
	}

	// tags: complex map(string) default → MUST be nil per D25.
	tags := vars["tags"]
	assert.Contains(t, tags.Type, "map", "type declaration preserved")
	assert.Nil(t, tags.Default, "complex default MUST be nil per D25 (zero-intrusion)")
	assert.False(t, tags.Required, "tags has a default (even if nil'd) → not required")

	// master_password: sensitive=true.
	mp := vars["master_password"]
	assert.True(t, mp.Sensitive)
	assert.True(t, mp.Required, "sensitive no default → required")
	assert.Nil(t, mp.Default)

	// Outputs: 2 (rds_id + connection_string).
	outNames := map[string]bool{}
	for _, o := range c.Outputs {
		outNames[o.Name] = true
	}
	assert.True(t, outNames["rds_id"])
	assert.True(t, outNames["connection_string"])
}

// TestContractExtractor_Minimal verifies the simplest module (1 required var).
func TestContractExtractor_Minimal(t *testing.T) {
	e := registry.NewContractExtractor()
	dir := filepath.Join("testdata", "minimal")

	c, err := e.Extract(dir)
	require.NoError(t, err)

	require.Len(t, c.Variables, 1)
	assert.Equal(t, "name", c.Variables[0].Name)
	assert.Equal(t, "string", c.Variables[0].Type)
	assert.True(t, c.Variables[0].Required)
	assert.Empty(t, c.Outputs, "minimal fixture has no outputs")
}

// TestContractExtractor_NonExistentDir verifies structured error on bad path.
func TestContractExtractor_NonExistentDir(t *testing.T) {
	e := registry.NewContractExtractor()
	_, err := e.Extract(filepath.Join("testdata", "does-not-exist"))
	require.Error(t, err)
}

// TestContractExtractor_ExtractFromRepo verifies the path-join convenience.
func TestContractExtractor_ExtractFromRepo(t *testing.T) {
	e := registry.NewContractExtractor()
	// repo root = testdata, modulePath = "rds-mysql"
	c, err := e.ExtractFromRepo(filepath.Join("testdata"), "rds-mysql")
	require.NoError(t, err)
	assert.NotEmpty(t, c.Variables, "ExtractFromRepo should find rds-mysql vars")

	// Empty modulePath → repo root itself; testdata/rds-mysql dir used as root.
	c2, err := e.ExtractFromRepo(filepath.Join("testdata", "rds-mysql"), "")
	require.NoError(t, err)
	assert.NotEmpty(t, c2.Variables)
}
