# code-generation Specification

## Purpose
TBD - created by archiving change w2-codegen. Update Purpose after archive.
## Requirements
### Requirement: Codegen Generator 主入口（表单→stack 目录树）

平台 MUST 在 `server/core/codegen/` 实现 Generator 主入口。输入 CodegenInput（StackMeta + 模块契约 + catalog defaults + 用户表单值 + 依赖图 + state backend 配置）→ 输出 FileSet（map[string][]byte，路径→文件内容）。Generator MUST 调用 PathGenerator（W1-04）渲染路径（repo_path/state_key/stack_id/tags），MUST NOT 字符串拼接路径。Generator MUST 调用 text/template 模板渲染各文件（main.tf/backend.tf/stack.tm.hcl/cross-layer.tf/outputs.tf）。

#### Scenario: 生成 RDS middleware stack（single cardinality）

- **WHEN** Generator.Generate(CodegenInput{layer:"middleware", component:"rds", env:"prod", cardinality:"single", ...})
- **THEN** 返回 FileSet 含以下文件：
  - `middleware/platform-default/rds-prod/main.tf`（module "rds" { source = "git::..." ... }）
  - `middleware/platform-default/rds-prod/backend.tf`（terraform backend s3）
  - `middleware/platform-default/rds-prod/stack.tm.hcl`（Terramate stack 定义）
  - `middleware/platform-default/rds-prod/outputs.tf`（outputs 聚合）
- AND FileSet 不含 cross-layer.tf（middleware 无上游 global VPC 依赖时）

#### Scenario: 生成 ECS application stack（map cardinality，有跨层依赖）

- **WHEN** Generator.Generate(CodegenInput{layer:"application", component:"ecs", cardinality:"map", dependencies:[vpc,rds], ...})
- **THEN** FileSet 含 cross-layer.tf（terraform_remote_state data 块引用 vpc + rds 的 state_key）
- AND main.tf 含 `for_each = tomap({...})`（map 多实例注入）

#### Scenario: 确定性（D19）

- **WHEN** 同一 CodegenInput 两次调用 Generate
- **THEN** 返回完全相同的 FileSet（路径 + 内容字节级一致）

### Requirement: module source 构造（git 源 vs registry 源）

平台 MUST 按 modules.source_type 构造 main.tf 的 source URL。git 源（MVP 默认）= `git::{{git_source}}//{{module_path}}?ref={{commit_sha}}`。registry 源（Phase 2）= `{{registry_source}}` + `version = "{{version}}"`。

#### Scenario: git 源 source 构造

- **WHEN** module.source_type = "git", git_source = "git@github.com:org/repo.git", module_path = "atomic/ecs", commit_sha = "abc123"
- **THEN** main.tf 的 source = "git::ssh://git@github.com/org/repo.git//atomic/ecs?ref=abc123"

### Requirement: Phase 1 五阶段参数管道

平台 MUST 实现 5 阶段简化参数管道（Phase 1，可映射到 D28 的 S1-S9）。阶段：contract（兜底）→ defaults（catalog）→ governance（平台强制 tag/state_key）→ user（表单）→ dependency（跨层 remote_state）。优先级：governance > user > dependency > defaults > contract。

#### Scenario: 治理覆盖用户

- **WHEN** user 传 instance_type="small"，但 catalog defaults 传 instance_type="large"
- **THEN** 最终值 = "small"（user rank 4 > defaults rank 2）

#### Scenario: 平台强制 tag 注入

- **WHEN** governance 阶段注入 platform-managed=true
- **THEN** 最终参数含 tags.platform-managed = true（用户不能覆盖）

### Requirement: CardinalityInjector（single/list/map）

平台 MUST 按 catalog_items.cardinality 在 main.tf 注入多实例。single = 直接 module 调用；map = for_each = tomap({...})（MVP 支持）；list = count = N（Phase 2）。模块 variables.tf 全 scalar，零侵入（D25）。

#### Scenario: single cardinality

- **WHEN** cardinality = "single"
- **THEN** main.tf 生成 `module "rds" { source = "..." instance_type = "..." }`（无 for_each/count）

#### Scenario: map cardinality

- **WHEN** cardinality = "map", instances = [{name:"web", type:"large"}, {name:"api", type:"medium"}]
- **THEN** main.tf 生成 `module "ecs" { for_each = tomap({web={type="large"}, api={type="medium"}}) ... instance_type = each.value.type }`

### Requirement: backend.tf 从 state_backends 表读

平台 MUST 从 state_backends 表读 bucket/region（doc 09 §6 修订），生成 backend.tf。key 用 PathGenerator.StateKey。

#### Scenario: backend.tf 生成

- **WHEN** state_backend.bucket = "tm-state", region = "cn-hangzhou", state_key = "middleware/platform-default/rds-prod"
- **THEN** backend.tf 含 `terraform { backend "s3" { bucket = "tm-state" region = "cn-hangzhou" key = "middleware/platform-default/rds-prod" } }`

### Requirement: stack.tm.hcl 生成（Terramate stack 定义）

平台 MUST 生成 stack.tm.hcl（Terramate stack 元数据）。含 stack_id（PathGenerator.StackID）+ tags（PathGenerator.TerramateTags）+ after（跨层依赖上游 stack 路径，可选）。

#### Scenario: stack.tm.hcl 生成

- **WHEN** stack_id = "middleware-platform-default-rds-prod", tags = ["layer:middleware","env:prod"]
- **THEN** stack.tm.hcl 含 `stack { id = "middleware-platform-default-rds-prod" tags = ["layer:middleware","env:prod"] }`

### Requirement: cross-layer.tf 生成（依赖图驱动）

平台 MUST 按 DependencyGraph 生成交叉层 remote_state data 块。每个依赖生成一个 data "terraform_remote_state" 块，key 引用上游 stack 的 state_key。

#### Scenario: cross-layer.tf 生成（有依赖）

- **WHEN** dependencies = [{alias:"vpc", state_key:"global/vpc-platform-default-prod"}]
- **THEN** cross-layer.tf 含 `data "terraform_remote_state" "vpc" { backend = "s3" config = { bucket="..." key = "global/vpc-platform-default-prod" } }`

### Requirement: golden file 测试（确定性验证）

平台 MUST 有 golden file 测试：固定 CodegenInput → 固定 FileSet 输出。覆盖 single/map cardinality + 三层（global/middleware/application）+ 有/无 space + 有/无跨层依赖。

#### Scenario: golden file 对比

- **WHEN** 运行 codegen golden file 测试
- **THEN** 生成的 FileSet 与 testdata/golden/ 下的预期文件字节级一致

