package pathgenerator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/pathgenerator"
)

// seed v1 path templates (from migration 010 layer_rule_set_versions).
const (
	tmplGlobal      = "global/{{.component}}-{{.tenant}}-{{.env}}"
	tmplMiddleware  = "middleware/{{.tenant}}/{{.component}}-{{.env}}"
	tmplApplication = "application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}"
)

func TestPathGenerator_Global(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	res, err := g.Generate(pathgenerator.StackMeta{
		Layer: "global", Tenant: "platform-default", Component: "vpc", Env: "prod",
	}, tmplGlobal)
	require.NoError(t, err)
	assert.Equal(t, "global/vpc-platform-default-prod", res.RepoPath)
	assert.Equal(t, res.RepoPath, res.StateKey, "MVP: state_key = repo_path")
	assert.Equal(t, "global-vpc-platform-default-prod", res.StackID)
	assert.Contains(t, res.TerramateTags, "layer:global")
	assert.Contains(t, res.TerramateTags, "tenant:platform-default")
	assert.Contains(t, res.TerramateTags, "component:vpc")
	assert.NotContains(t, res.TerramateTags, "team:") // global has no team
}

func TestPathGenerator_Middleware(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	res, err := g.Generate(pathgenerator.StackMeta{
		Layer: "middleware", Tenant: "platform-default", Component: "rds", Env: "prod",
	}, tmplMiddleware)
	require.NoError(t, err)
	assert.Equal(t, "middleware/platform-default/rds-prod", res.RepoPath)
	assert.Equal(t, "middleware-platform-default-rds-prod", res.StackID)
}

func TestPathGenerator_Application_WithSpace(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	res, err := g.Generate(pathgenerator.StackMeta{
		Layer: "application", Tenant: "platform-default", Team: "team-a",
		Space: "orders", Component: "ecs", Env: "prod",
	}, tmplApplication)
	require.NoError(t, err)
	assert.Equal(t, "application/platform-default/team-a/orders/ecs-prod", res.RepoPath)
	assert.Equal(t, "application-platform-default-team-a-orders-ecs-prod", res.StackID)
	assert.Contains(t, res.TerramateTags, "team:team-a")
	assert.Contains(t, res.TerramateTags, "space:orders")
}

func TestPathGenerator_Application_NoSpace(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	res, err := g.Generate(pathgenerator.StackMeta{
		Layer: "application", Tenant: "platform-default", Team: "team-a",
		Space: "", Component: "ecs", Env: "prod",
	}, tmplApplication)
	require.NoError(t, err)
	assert.Equal(t, "application/platform-default/team-a/ecs-prod", res.RepoPath)
	assert.NotContains(t, res.TerramateTags, "space:") // no space tag when empty
}

func TestPathGenerator_EmptyTemplate(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	_, err := g.Generate(pathgenerator.StackMeta{Layer: "global"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path_template is empty")
}

func TestPathGenerator_Deterministic(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	meta := pathgenerator.StackMeta{
		Layer: "application", Tenant: "platform-default", Team: "team-a",
		Space: "orders", Component: "ecs", Env: "prod",
	}
	r1, _ := g.Generate(meta, tmplApplication)
	r2, _ := g.Generate(meta, tmplApplication)
	assert.Equal(t, r1, r2, "same input must produce same output (D19 determinism)")
}

// P1-3 fix: StackID must be lowercase + validated (D29 contract).
func TestPathGenerator_StackID_Lowercase(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	// Uppercase Team/Tenant must produce lowercase stack_id.
	res, err := g.Generate(pathgenerator.StackMeta{
		Layer: "application", Tenant: "Platform-Default", Team: "Team-A",
		Space: "Orders", Component: "ECS", Env: "Prod",
	}, tmplApplication)
	require.NoError(t, err)
	assert.True(t, res.StackID == strings.ToLower(res.StackID),
		"stack_id must be all lowercase, got %q", res.StackID)
}

func TestPathGenerator_StackID_TooLong(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	// Construct a path exceeding 64 chars.
	longComponent := strings.Repeat("verylongcomponent", 5)
	_, err := g.Generate(pathgenerator.StackMeta{
		Layer: "global", Tenant: "x", Component: longComponent, Env: "p",
	}, tmplGlobal)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 64-char")
}

// P1-4/P2-1 fix: missingkey=error catches typo'd template keys.
func TestPathGenerator_TemplateTypoError(t *testing.T) {
	g := pathgenerator.NewPathGenerator()
	// Typo: {{.teannt}} instead of {{.tenant}} → should error, not silently empty.
	_, err := g.Generate(pathgenerator.StackMeta{
		Layer: "global", Tenant: "x", Component: "vpc", Env: "prod",
	}, "global/{{.component}}-{{.teannt}}-{{.env}}")
	require.Error(t, err)
}
