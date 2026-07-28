## Why

W1-04 是 `iac-self-service-platform` 的第四个实现模块（W1 最后一个），落地**团队归属治理 + 分层路径模型**。这是 codegen（W2 task 5.1）的前置依赖——PathGenerator 输出的 `repo_path + state_key + stack_id + terramate_tags` 是 stack 身份契约，所有代码生成、state 隔离、Terramate 编排都依赖它。

**影响层级**：业务核心层（`server/core/{tenancy,stackmodel}/`），不改 DB schema / proto 契约。

**为什么现在做**：W1-02 的 TeamRepo/ProjectRepo/SpaceRepo/StackRepo + W1-03 的 ModuleRepo/CatalogRepo 已就位；layer_logical_refs + layer_rule_set_versions 表已 seed（migration 010）。前置依赖满足。

## What Changes

### 1. core/tenancy/（task 4.1）—— 团队归属治理

新建 `server/core/tenancy/`：
- **TenancyService**：注入 TeamRepo/ProjectRepo/SpaceRepo，实现 team/project/space CRUD
- **资源归属规则**：按 module.layer → 默认 owner_team（如 RDS→DBA team）。MVP 用 layer→team 映射表（硬编码或 config），Phase 2 走 team_cloud_grants 驱动

### 2. core/stackmodel/（task 4.2 + 4.3 + 4.4 + 4.5）—— 分层模型 + PathGenerator

- **LayerService**（task 4.2）：读 layer_logical_refs + layer_rule_set_versions（Phase 1 只读，不开放管理员改）
- **PathGenerator**（task 4.3）：按 D29 layer-first Path Contract，输入 (layer, tenant, team, space, component, env) → 输出 (repo_path, state_key, stack_id, terramate_tags_json)。**确定性 + 一次性输出四元组**
- **StackGranularity**（task 4.4）：读 catalog_items.stack_grouping → 决定 stack 粒度（per-component 默认 / per-space / per-team / custom）
- **DependencyGraph**（task 4.5）：stack_dependencies 依赖图 + 拓扑排序（layer.depends_on 校验无环）

### 3. 测试（task 4.12 简化版）

路径模板渲染 + 粒度评估 + 依赖图拓扑排序 + 归属判定。

### 不做（Phase 2/3 推迟）

- task 4.6 MigrationPlanner（Phase 2，layer 版本化迁移 dry-run）
- task 4.7 StateMover（Phase 3，state mv 半自动）
- task 4.8 QuiesceBatch（Phase 2，迁移批次冻结）
- task 4.9 StateBackup（Phase 2，state 快照）
- task 4.10 RollbackEngine（Phase 3，逆向 state mv）
- task 4.11 SunsetTracker（Phase 3，deprecated stack 下线追踪）
- bundle → space 术语已在 W1-02 重命名完成（task 4.1 的 "bundle" 引用是旧的，实际是 space）

## Capabilities

### New Capabilities

- `tenancy-management`: 团队/项目组/space CRUD + 资源归属规则（layer→team 映射）
- `stack-path-model`: PathGenerator（D29 layer-first 路径契约）+ StackGranularity 评估 + DependencyGraph 拓扑排序 + Layer 只读服务

### Modified Capabilities

（无）

## Impact

- **代码**：新建 `core/tenancy/`（service + 归属规则）+ `core/stackmodel/`（layer + pathgenerator + granularity + dependency）。不改现有代码。
- **API**：不新增 proto RPC（tenancy/stackmodel 是 codegen 的内部依赖，不是用户直接 API；Phase 2 可加 admin RPC）
- **依赖**：无新外部依赖（text/template 标准库 + W1-02 Repo）
- **DB**：不改 schema（用 W1-02 的 teams/projects/spaces/layer_* 表）
- **测试**：路径模板渲染 + 粒度评估 + 拓扑排序 + 归属判定
