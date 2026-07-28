## ADDED Requirements

### Requirement: PathGenerator（D29 layer-first 路径契约四元组）

平台 MUST 在 `server/core/stackmodel/pathgenerator/` 实现 PathGenerator，按 D29 layer-first Path Contract 一次性输出 stack 身份契约四元组：(repo_path, state_key, stack_id, terramate_tags)。PathGenerator 从 `layer_rule_set_versions.layers_json` 读 path_template（Go text/template 渲染），输入 StackMeta(layer, tenant, team, space, component, env)。**codegen MUST 调用此组件，MUST NOT 字符串拼接路径。**

#### Scenario: global 层路径生成

- **WHEN** PathGenerator.Generate(StackMeta{layer:"global", tenant:"platform-default", component:"vpc", env:"prod"})
- **THEN** RepoPath = "global/vpc-platform-default-prod"
- **AND** StateKey = "global/vpc-platform-default-prod"（默认 = RepoPath）
- **AND** StackID = "global-vpc-platform-default-prod"（`/` → `-`）
- **AND** TerramateTags 含 ["layer:global","tenant:platform-default","env:prod","component:vpc"]

#### Scenario: middleware 层路径生成

- **WHEN** PathGenerator.Generate(StackMeta{layer:"middleware", tenant:"platform-default", component:"rds", env:"prod"})
- **THEN** RepoPath = "middleware/platform-default/rds-prod"
- **AND** StackID = "middleware-platform-default-rds-prod"

#### Scenario: application 层路径生成（有 space）

- **WHEN** PathGenerator.Generate(StackMeta{layer:"application", tenant:"platform-default", team:"team-a", space:"orders", component:"ecs", env:"prod"})
- **THEN** RepoPath = "application/platform-default/team-a/orders/ecs-prod"
- **AND** TerramateTags 含 ["layer:application","tenant:platform-default","team:team-a","space:orders","env:prod","component:ecs"]

#### Scenario: application 层路径生成（无 space）

- **WHEN** PathGenerator.Generate(StackMeta{layer:"application", tenant:"platform-default", team:"team-a", space:"", component:"ecs", env:"prod"})
- **THEN** RepoPath = "application/platform-default/team-a/ecs-prod"（无 space 段）

### Requirement: StackGranularity 评估器（per-component 默认）

平台 MUST 在 `server/core/stackmodel/granularity/` 实现 StackGranularity 评估器。读 catalog_items.stack_grouping 字段，MVP 只返回 per-component（默认）。Phase 2 扩展 per-space/per-team/custom。

#### Scenario: per-component 默认评估

- **WHEN** 调用 StackGranularity.Evaluate(catalog_item.stack_grouping="per-component")
- **THEN** 返回 per-component（一个 component 一个 stack）

### Requirement: DependencyGraph 跨层依赖拓扑排序 + 无环校验

平台 MUST 在 `server/core/stackmodel/dependency/` 实现 DependencyGraph。从 stack_dependencies + layer.depends_on 构建 DAG，拓扑排序输出执行顺序（Kahn 算法 BFS）。校验无环（有环报结构化错误）。跨层依赖单向：global→middleware→application。

#### Scenario: 拓扑排序（三层顺序）

- **WHEN** 构建 DAG：vpc(global) → rds(middleware) → ecs(application)，调用 TopoSort
- **THEN** 输出顺序 [vpc, rds, ecs]（global 先，application 后）

#### Scenario: 环检测报错

- **WHEN** 构建 DAG：A→B→A（循环依赖），调用 TopoSort
- **THEN** 返回结构化错误（"cycle detected: A→B→A"）

### Requirement: LayerService 只读（Phase 1）

平台 MUST 在 `server/core/stackmodel/layer/` 实现 LayerService。Phase 1 只读：ListLayers + GetActiveRuleSet + GetRuleSetByID。不开放管理员改 layer 模板（Phase 2 才开放）。

#### Scenario: 读取 seed 三层

- **WHEN** 调用 LayerService.ListLayers
- **THEN** 返回 [global, middleware, application]（migration 010 seed）

#### Scenario: 读取 active rule set

- **WHEN** 调用 LayerService.GetActiveRuleSet
- **THEN** 返回 version_id=1, status=active, is_default=true（seed v1）
