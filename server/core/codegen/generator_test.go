package codegen_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/pathgenerator"
)

// seed v1 path templates (from migration 010).
const (
	tmplGlobal      = "global/{{.component}}-{{.tenant}}-{{.env}}"
	tmplMiddleware  = "middleware/{{.tenant}}/{{.component}}-{{.env}}"
	tmplApplication = "application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}"
)

func newGenerator() *codegen.Generator {
	return codegen.NewGenerator(pathgenerator.NewPathGenerator())
}

// TestGenerate_RDS_Middleware_Single verifies a single-cardinality RDS stack.
func TestGenerate_RDS_Middleware_Single(t *testing.T) {
	g := newGenerator()
	fs, err := g.Generate(context.Background(), codegen.CodegenInput{
		Meta: pathgenerator.StackMeta{
			Layer: "middleware", Tenant: "platform-default", Component: "rds", Env: "prod",
		},
		PathTemplate: tmplMiddleware,
		Contract: &registry.Contract{
			Variables: []registry.ContractVariable{
				{Name: "instance_type", Type: "string", Required: true},
				{Name: "engine_version", Type: "string", Default: "8.0"},
			},
			Outputs: []registry.ContractOutput{
				{Name: "rds_id", Description: "RDS instance ID"},
			},
		},
		Defaults:      map[string]any{"instance_type": "mysql.n2.large.1c"},
		FormValues:    map[string]any{"instance_type": "mysql.n2.large.1c"},
		Cardinality:   "single",
		ModuleSource:  "git::ssh://git@github.com/org/modules.git//atomic/rds?ref=abc123",
		ComponentName: "rds",
		Backend:       codegen.BackendConfig{Kind: "s3", Bucket: "tm-state", Region: "cn-hangzhou"},
	})
	require.NoError(t, err)

	// FileSet must contain main.tf, backend.tf, stack.tm.hcl, outputs.tf.
	// No cross-layer.tf (no dependencies).
	assert.Contains(t, fs, "middleware/platform-default/rds-prod/main.tf")
	assert.Contains(t, fs, "middleware/platform-default/rds-prod/backend.tf")
	assert.Contains(t, fs, "middleware/platform-default/rds-prod/stack.tm.hcl")
	assert.Contains(t, fs, "middleware/platform-default/rds-prod/outputs.tf")
	assert.NotContains(t, fs, "middleware/platform-default/rds-prod/cross-layer.tf")

	// main.tf must contain the module source + resolved params.
	mainTF := string(fs["middleware/platform-default/rds-prod/main.tf"])
	assert.Contains(t, mainTF, `source = "git::ssh://git@github.com/org/modules.git//atomic/rds?ref=abc123"`)
	assert.Contains(t, mainTF, "instance_type")
	assert.Contains(t, mainTF, "engine_version")
	// P0-1: state_key must NOT appear as a module argument.
	assert.NotContains(t, mainTF, "state_key")

	// backend.tf must contain bucket + key.
	backendTF := string(fs["middleware/platform-default/rds-prod/backend.tf"])
	assert.Contains(t, backendTF, `bucket = "tm-state"`)
	assert.Contains(t, backendTF, `"middleware/platform-default/rds-prod"`)

	// stack.tm.hcl must contain stack id + tags.
	stackHCL := string(fs["middleware/platform-default/rds-prod/stack.tm.hcl"])
	assert.Contains(t, stackHCL, `"middleware-platform-default-rds-prod"`)
	assert.Contains(t, stackHCL, "layer:middleware")

	// outputs.tf must reference the module output.
	outputsTF := string(fs["middleware/platform-default/rds-prod/outputs.tf"])
	assert.Contains(t, outputsTF, "rds_id")
	assert.Contains(t, outputsTF, "module.rds.rds_id")
}

// TestGenerate_VPC_Global_Single verifies a global-layer stack (no dependencies, no team/space).
func TestGenerate_VPC_Global_Single(t *testing.T) {
	g := newGenerator()
	fs, err := g.Generate(context.Background(), codegen.CodegenInput{
		Meta: pathgenerator.StackMeta{
			Layer: "global", Tenant: "platform-default", Component: "vpc", Env: "prod",
		},
		PathTemplate: tmplGlobal,
		Contract: &registry.Contract{
			Variables: []registry.ContractVariable{
				{Name: "vpc_cidr", Type: "string", Default: "172.31.0.0/16"},
			},
			Outputs: []registry.ContractOutput{
				{Name: "vpc_id", Description: "VPC ID"},
			},
		},
		FormValues:    map[string]any{},
		Cardinality:   "single",
		ModuleSource:  "git::ssh://git@github.com/org/modules.git//atomic/vpc?ref=abc123",
		ComponentName: "vpc",
		Backend:       codegen.BackendConfig{Kind: "s3", Bucket: "tm-state", Region: "cn-hangzhou"},
	})
	require.NoError(t, err)

	assert.Contains(t, fs, "global/vpc-platform-default-prod/main.tf")
	assert.Contains(t, fs, "global/vpc-platform-default-prod/stack.tm.hcl")
	stackHCL := string(fs["global/vpc-platform-default-prod/stack.tm.hcl"])
	assert.Contains(t, stackHCL, `"global-vpc-platform-default-prod"`)
}

// TestGenerate_ECS_Application_Map verifies map cardinality with for_each + cross-layer deps.
func TestGenerate_ECS_Application_Map(t *testing.T) {
	g := newGenerator()
	fs, err := g.Generate(context.Background(), codegen.CodegenInput{
		Meta: pathgenerator.StackMeta{
			Layer: "application", Tenant: "platform-default", Team: "team-a",
			Space: "orders", Component: "ecs", Env: "prod",
		},
		PathTemplate: tmplApplication,
		Contract: &registry.Contract{
			Variables: []registry.ContractVariable{
				{Name: "instance_type", Type: "string", Required: true},
				{Name: "image_id", Type: "string"},
			},
			Outputs: []registry.ContractOutput{
				{Name: "instance_id", Description: "ECS instance ID"},
			},
		},
		FormValues:  map[string]any{},
		Cardinality: "map",
		Instances: []map[string]any{
			{"name": "web", "instance_type": "ecs.g7.large", "image_id": "m-aaa"},
			{"name": "api", "instance_type": "ecs.g7.xlarge", "image_id": "m-bbb"},
		},
		InstanceKey:   "name",
		ModuleSource:  "git::ssh://git@github.com/org/modules.git//atomic/ecs?ref=abc123",
		ComponentName: "ecs",
		Backend:       codegen.BackendConfig{Kind: "s3", Bucket: "tm-state", Region: "cn-hangzhou"},
		Dependencies: []codegen.DependencyRef{
			{Alias: "vpc", StateKey: "global/vpc-platform-default-prod",
				Variables: map[string]string{"vswitch_id": "data.terraform_remote_state.vpc.outputs.vswitch_ids[0]"}},
		},
	})
	require.NoError(t, err)

	// Path with space.
	basePath := "application/platform-default/team-a/orders/ecs-prod"
	assert.Contains(t, fs, basePath+"/main.tf")
	assert.Contains(t, fs, basePath+"/cross-layer.tf")

	// main.tf must have for_each.
	mainTF := string(fs[basePath+"/main.tf"])
	assert.Contains(t, mainTF, "for_each")

	// P0-1: state_key must NOT appear as a module argument.
	assert.NotContains(t, mainTF, "state_key",
		"P0-1: state_key must not leak into main.tf module args")

	// P0-2: dependency expression must NOT be string-quoted (raw HCL reference).
	assert.Contains(t, mainTF, "data.terraform_remote_state.vpc.outputs.vswitch_ids[0]",
		"P0-2: dependency expression must be raw (unquoted)")
	assert.NotContains(t, mainTF, `"data.terraform_remote_state`,
		"P0-2: dependency expression must not be string-quoted")

	// P0-3: per-instance fields must be bound via each.value.
	assert.Contains(t, mainTF, "instance_type = each.value.instance_type",
		"P0-3: per-instance field must use each.value binding")
	assert.Contains(t, mainTF, "image_id = each.value.image_id",
		"P0-3: per-instance field must use each.value binding")

	// P2-2: instance key field must NOT leak into tomap body.
	assert.NotContains(t, mainTF, `name = "web"`,
		"P2-2: instance key 'name' must not appear in tomap body")

	// cross-layer.tf must have remote_state for vpc.
	crossTF := string(fs[basePath+"/cross-layer.tf"])
	assert.Contains(t, crossTF, `data "terraform_remote_state" "vpc"`)
	assert.Contains(t, crossTF, "global/vpc-platform-default-prod")

	// outputs.tf must use for-comprehension (map cardinality).
	outputsTF := string(fs[basePath+"/outputs.tf"])
	assert.Contains(t, outputsTF, "for k, m in module.ecs")
}

// TestGenerate_Deterministic verifies D19 (same input → byte-identical output).
func TestGenerate_Deterministic(t *testing.T) {
	g := newGenerator()
	in := codegen.CodegenInput{
		Meta: pathgenerator.StackMeta{
			Layer: "middleware", Tenant: "platform-default", Component: "rds", Env: "prod",
		},
		PathTemplate: tmplMiddleware,
		Contract: &registry.Contract{
			Variables: []registry.ContractVariable{{Name: "x", Type: "string", Default: "y"}},
		},
		Cardinality:   "single",
		ModuleSource:  "git::ssh://x//y?ref=z",
		ComponentName: "rds",
		Backend:       codegen.BackendConfig{Kind: "s3", Bucket: "b", Region: "r"},
	}

	fs1, err := g.Generate(context.Background(), in)
	require.NoError(t, err)
	fs2, err := g.Generate(context.Background(), in)
	require.NoError(t, err)

	assert.Equal(t, fs1, fs2, "D19: same input must produce byte-identical FileSet")
}

// TestGenerate_PipelinePriority verifies governance > user > defaults > contract.
func TestGenerate_PipelinePriority(t *testing.T) {
	g := newGenerator()
	fs, err := g.Generate(context.Background(), codegen.CodegenInput{
		Meta: pathgenerator.StackMeta{
			Layer: "global", Tenant: "x", Component: "vpc", Env: "prod",
		},
		PathTemplate: tmplGlobal,
		Contract: &registry.Contract{
			Variables: []registry.ContractVariable{
				{Name: "instance_type", Type: "string", Default: "small"}, // contract stage
			},
		},
		Defaults:      map[string]any{"instance_type": "medium"}, // defaults stage
		FormValues:    map[string]any{"instance_type": "large"},  // user stage
		Governance:    map[string]any{},                          // no governance override
		Cardinality:   "single",
		ModuleSource:  "x",
		ComponentName: "vpc",
		Backend:       codegen.BackendConfig{Kind: "s3", Bucket: "b", Region: "r"},
	})
	require.NoError(t, err)
	mainTF := string(fs["global/vpc-x-prod/main.tf"])
	assert.Contains(t, mainTF, "large", "user (rank 4) should win over defaults (rank 2) + contract (rank 1)")
}
