package drift_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/drift"
)

// readTestdata loads a testdata JSON file.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func TestParsePlan_NoDrift(t *testing.T) {
	summary, err := drift.ParsePlan(readTestdata(t, "plan-no-drift.json"))
	require.NoError(t, err)

	assert.Empty(t, summary.ChangedResources, "all no-op plan must yield empty diff")
}

func TestParsePlan_WithDrift(t *testing.T) {
	summary, err := drift.ParsePlan(readTestdata(t, "plan-with-drift.json"))
	require.NoError(t, err)

	require.Len(t, summary.ChangedResources, 2, "no-op resources must be filtered out")

	assert.Equal(t, "alicloud_db_instance.this", summary.ChangedResources[0].Address)
	assert.Equal(t, []string{"update"}, summary.ChangedResources[0].Actions)

	assert.Equal(t, "alicloud_vpc.main", summary.ChangedResources[1].Address)
	assert.Equal(t, []string{"create"}, summary.ChangedResources[1].Actions)
}

func TestParsePlan_ErrorPlanEmpty(t *testing.T) {
	// The error-path plan JSON has no resource_changes. The parser returns an
	// empty summary (no drift); the worker treats the upstream exit code as
	// the source of truth for the error verdict.
	summary, err := drift.ParsePlan(readTestdata(t, "plan-error.json"))
	require.NoError(t, err)
	assert.Empty(t, summary.ChangedResources)
}

func TestParsePlan_EmptyInputYieldsEmptyDiff(t *testing.T) {
	summary, err := drift.ParsePlan(nil)
	require.NoError(t, err)
	assert.Empty(t, summary.ChangedResources)

	summary, err = drift.ParsePlan([]byte("   \n  "))
	require.NoError(t, err)
	assert.Empty(t, summary.ChangedResources)
}

func TestParsePlan_InvalidJSONReturnsError(t *testing.T) {
	_, err := drift.ParsePlan([]byte("{not json"))
	require.Error(t, err)
}

func TestParsePlan_DeleteAndCreateDeleteActions(t *testing.T) {
	// Defensively cover delete and create-then-delete (the rare 3-action form)
	// to confirm they are NOT filtered as no-op.
	input := []byte(`{
	  "resource_changes": [
	    {"address": "r.deleted", "change": {"actions": ["delete"]}},
	    {"address": "r.replaced", "change": {"actions": ["delete","create"]}},
	    {"address": "r.unchanged", "change": {"actions": ["no-op"]}}
	  ]
	}`)
	summary, err := drift.ParsePlan(input)
	require.NoError(t, err)

	require.Len(t, summary.ChangedResources, 2)
	assert.Equal(t, []string{"delete"}, summary.ChangedResources[0].Actions)
	assert.Equal(t, []string{"delete", "create"}, summary.ChangedResources[1].Actions)
}

func TestDiffSummary_String(t *testing.T) {
	assert.Equal(t, "no drift", drift.DiffSummary{}.String())

	summary := drift.DiffSummary{ChangedResources: []drift.ResourceChange{
		{Address: "a", Actions: []string{"create"}},
		{Address: "b", Actions: []string{"delete", "create"}},
	}}
	assert.Equal(t, "a:create, b:delete+create", summary.String())
}
