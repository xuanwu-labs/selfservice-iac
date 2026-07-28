## Context

W1-04 是 W1 最后一个模块，落地 PathGenerator——codegen（W2）的身份契约基础。

**现有架构约束**（已落地）：
- `server/core/{tenancy,stackmodel}/`（空目录，等实现）
- W1-02 Repo：TeamRepo/ProjectRepo/SpaceRepo/StackRepo/StackDependencyRepo/LayerLogicalRefRepo/LayerRuleSetVersionRepo
- migration 010 seed：layer_logical_refs（global/middleware/application）+ layer_rule_set_versions v1（active+default，含 path_template）
- D29 Path Contract（doc 02 §2）：layer-first 拓扑，PathGenerator 一次性输出 repo_path + state_key + stack_id + terramate_tags

**D24/D26 Phase 策略**（主提案 tasks.md 注明）：
- Phase 1（本 change）：hard-code 三层 + per-component 粒度，只读 layer，不开放管理员改
- Phase 2：D24 path_template 自定义 + StackGranularity 多策略 + MigrationPlanner
- Phase 3：D26 版本化迁移 + StateMover + Rollback + Sunset

## Goals / Non-Goals

**Goals:**
- TenancyService：team/project/space CRUD + 资源归属规则
- PathGenerator：D29 layer-first 路径契约（repo_path + state_key + stack_id + tags 四元组）
- StackGranularity：评估器（per-component 默认）
- DependencyGraph：跨层依赖拓扑排序 + 无环校验
- LayerService：Phase 1 只读（读 seed 数据）

**Non-Goals:**
- 不做 MigrationPlanner/StateMover/QuiesceBatch/StateBackup/Rollback/Sunset（Phase 2/3）
- 不开放管理员改 layer 模板（Phase 1 只读）
- 不改 DB schema / proto 契约
- 不做 D24 可配置 path_template 编辑（Phase 1 用 seed v1 模板）

## Decisions

### D1：PathGenerator 用 layer_rule_set_versions.layers_json 的 path_template（不从代码硬编码）

**决策**：PathGenerator 从 `layer_rule_set_versions.layers_json` 读 path_template（Go text/template 语法），用 stack 元数据渲染。

**理由**：
- D24 可配置化的前提是模板在 DB 里（不在代码里）
- Phase 1 用 seed v1 的模板（global/middleware/application 三层 path_template 已 seed）
- Phase 2 管理员改 layers_json 即可改路径，PathGenerator 代码不变

**seed v1 模板**（migration 010）：
```
global:     "global/{{.component}}-{{.tenant}}-{{.env}}"
middleware: "middleware/{{.tenant}}/{{.component}}-{{.env}}"
application: "application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}"
```

**备选**：硬编码路径模板（Phase 1 简单但 Phase 2 要重写）。

### D2：PathGenerator 输出四元组（repo_path + state_key + stack_id + terramate_tags）

**决策**：一次调用输出全部四个字段，不是分别生成。

```go
type PathResult struct {
    RepoPath      string   // "application/platform-default/team-a/orders/ecs-prod"
    StateKey      string   // 默认 = RepoPath（可独立配置，但 MVP 相同）
    StackID       string   // "application-platform-default-team-a-orders-ecs-prod"（- 分隔）
    TerramateTags []string // ["layer:application","tenant:platform-default","env:prod",...]
}
```

**理由**：D29 要求"四者共同构成 stack 身份契约"。分开生成可能不一致（如 repo_path 用 `/` 但 stack_id 用 `-`）。

**stack_id 构造**：repo_path 的 `/` 替换为 `-`（文件系统安全 + 人类可读）。

**terramate_tags 构造**：从元数据生成 `key:value` 数组：
```
["layer:<layer>", "tenant:<tenant>", "env:<env>", "team:<team>", "space:<space>", "component:<component>"]
```

### D3：StackGranularity MVP 只实现 per-component（默认）

**决策**：Phase 1 只支持 per-component（一个 component 一个 stack）。评估器读 catalog_items.stack_grouping 但 MVP 忽略非 per-component 值（默认 per-component）。

**理由**：per-component 是 D24 默认值，MVP 所有 catalog_item 都用 per-component。Phase 2 才支持 per-space/per-team/custom。

**备选**：实现全部 4 种粒度（Phase 1 不需要，YAGNI）。

### D4：DependencyGraph 用拓扑排序 + 无环校验

**决策**：DependencyGraph 读 stack_dependencies + layer.depends_on，构建 DAG，拓扑排序输出执行顺序。校验无环（有环报错）。

**理由**：
- Terramate 自己也做 DAG（after/watch），但平台需要在 codegen 前校验依赖合法性
- layer.depends_on 校验跨层依赖方向（global→middleware→application 单向）

**算法**：Kahn 算法（BFS 拓扑排序）。

### D5：资源归属规则 MVP 用 layer→team 映射（硬编码）

**决策**：TenancyService 的资源归属判定，MVP 用硬编码 layer→default_owner_team 映射：
```
global     → platform-ops team
middleware → dba | middleware team（按 component 细分：rds/redis→dba，kafka→middleware）
application → 业务 team（从 request.team_id）
```

**理由**：team_cloud_grants 表 W2 才落（D23）。MVP 用硬编码映射够用（doc 02 §1 的三层归属定义）。

**备选**：从 DB 配置（Phase 2 team_cloud_grants 驱动）。

### D6：space 不支持嵌套（W1-02 已确认）

**决策**：space 是单层可选路径分组（doc 02 §1.3 已明确）。PathGenerator 的 application 模板 `{{if .space}}{{.space}}/{{end}}` 处理可选 space。

## Risks / Trade-offs

- **[PathGenerator 模板渲染依赖 layers_json 格式] → seed v1 格式固定 + 测试覆盖**：layers_json 是 JSONB，PathGenerator 解析时需要类型断言。用 testdata 覆盖三层模板渲染。
- **[stack_id 与 repo_path 一致性] → 四元组一次生成**：D2 已解（一次输出，不分开）。
- **[DependencyGraph 环检测] → Kahn 算法 + 报错**：有环依赖是配置错误，报结构化错误。
- **[资源归属硬编码 vs DB 配置] → MVP 硬编码，Phase 2 team_cloud_grants**：硬编码够 MVP，改配置不影响 PathGenerator（归属只影响审批路由 + 成本归集，不影响路径）。

## Migration Plan

1. 实现 core/tenancy/（TenancyService + 归属规则）
2. 实现 core/stackmodel/pathgenerator（D29 四元组）
3. 实现 core/stackmodel/granularity（per-component 评估器）
4. 实现 core/stackmodel/dependency（拓扑排序 + 无环校验）
5. 实现 core/stackmodel/layer（只读 LayerService）
6. 测试（路径渲染 + 粒度 + 拓扑 + 归属）
7. `go build ./... && go vet ./... && go test ./server/core/{tenancy,stackmodel}/...`

**回滚**：core/tenancy + core/stackmodel 删除即回退。无 schema 变更。

## Open Questions

- **PathGenerator 是否需要 custom_kv 支持？** Phase 1 不需要（模板变量只有 env/tenant/team/space/component/layer）。Phase 2 path_template 自定义时可能需要 custom_kv。
- **DependencyGraph 是否需要持久化？** MVP 不需要（运行时从 stack_dependencies 表构建）。Phase 2 可缓存。
