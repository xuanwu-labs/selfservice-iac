## Why

W1-02 是 `iac-self-service-platform` 的第二个实现模块，为 W1-03（模块注册）/ W1-04（分层与租户）准备元数据访问底座。当前状态：**12 个 migration 已落地 25 张表 + sqlc 三件套已配好 + `teams` 1 张表有 query 范例**，但其余 24 张表无 sqlc query → 无 CRUD 方法；`docs/04` 定义的 `environments / tenants / environment_tenant_bindings / tag_policies` 4 张 W1 必需表尚未建 migration；且**数据访问层（Repo）尚未建立**——core 业务层没有可注入的数据访问抽象。

**影响层级**：数据访问层（`server/data/`）+ sqlc 层（`server/pkg/db/`），不改 API/proto 契约。

**为什么现在做**：W1-03 的 registry/catalog 和 W1-04 的 tenancy/envtenant 都需要注入 Repo 才能开始；缺少 env/tenant 表会让 stacks.env_id / tenant_id FK 悬空。

## What Changes

### 1. 补齐 4 张 W1 必需表 migration（`013_env_tenant_tagpolicy.sql`）

`environments`、`tenants`、`environment_tenant_bindings`、`tag_policies`（doc 04 §2.11/§2.12 + doc 07 env+tenant 模型）。**身份/编排/漂移/审批表（identities/oidc_providers/sessions/role_bindings/executor_runs/drift_runs/adapters_config/manual_intervention_tasks）推迟到 W2 对应模块**（YAGNI）。

### 2. 为 15 个核心实体写 sqlc query（`server/pkg/db/queries/<table>.sql`）

覆盖 teams/projects/spaces/modules/module_versions/module_dependencies/catalog_items/stacks/stack_dependencies/layer_logical_refs/layer_rule_set_versions/environments/tenants/environment_tenant_bindings/tag_policies。遵循 `teams.sql` 范例（snowflake ID 调用方传入、软删除 `WHERE deleted_at IS NULL`、一表一 .sql 文件）。`sqlc generate` 重生成。

### 3. ★ 建立 data/repo/ 层（混合范式：ferret 工程范式 × DIP × sqlc）

**核心架构决策**：采用 ferret 工程范式（参考 `E:\GolandProjects\ferret`）+ DDD DIP 可演进 + sqlc SQL-as-truth：

- **`data/repo/<entity>.go`**：每个核心实体一个 Repo struct，薄包装 `*generated.Queries`。提供业务语义方法（GetByID/Create/List 薄包装 + CreateWithVersion 等跨表事务 + 动态查询）。wire 在 `data.ProviderSet` 注册所有 `NewXxxRepo`。
- **`data/query_wrapper.go`**：MyBatis-Plus 风格动态查询构造器（适配 pgx，非 GORM），用于 sqlc 不擅长的动态场景（IN-list、ad-hoc 过滤组合、分页）。参考 ferret `data/query_wrapper.go`。
- **`data/dbset/`**：多表组合 struct（opt-in，仅跨表聚合时建，如 `StackWithSpace{Stack, Space, Layer}`）。
- **core 直接注入 Repo struct**（ferret 风，最简）；需要测试/mock 时在 core 提取小 interface（Go 隐式 interface，无需改 data 层）——DIP 可演进。
- **entity 层 opt-in**：仅在 sqlc model 不够（需要聚合/计算字段）时建 `internal/model/entity/`。

### 4. 修正 server/AGENTS.md 内部矛盾 + data.go 注释

server/AGENTS.md 存在两个矛盾版本：
- line 68 "数据访问三件套：core/store(薄包装,调用方)→ data/→ pkg/db/generated"
- line 132 数据流图 "core 业务逻辑 → dbset → pkg/db generated"

**统一为**：`core/<domain>/`（注入 Repo struct，需要时提取 interface）→ `data/repo/`（Repo 实现）→ `pkg/db/generated`（sqlc）。删除 `core/store/` 薄包装表述（违反 DIP，core 不应 import pkg/db）。修正 `data.go` 的 "first consumer will be core/store" 误导性注释。

### 5. 测试扩展

扩展 `data/teams_test.go` 模式到关键实体（spaces/modules/catalog_items/stacks/environments + Repo 层 CRUD + 事务 + 动态查询）。

### 不做（本次范围外）

- 身份/编排/漂移/审批表的 query + Repo → W2 对应模块
- adapters_config / manual_intervention_tasks / outbox_events query → W2
- core/<domain>/ 业务逻辑 → W1-03/04
- 不建 `core/store/`（违反 DIP，由 `data/repo/` 取代）
- 不强求每表建 entity（opt-in，YAGNI）

## Capabilities

### New Capabilities

- `metadata-store`: W1 元数据访问层——混合范式（ferret Repo struct + DIP 可演进 + sqlc SQL-as-truth）。15 个核心实体的 sqlc query + Repo 实现 + 动态查询构造器 + env/tenant/tag_policy 表落地 + dbset 多表组合（opt-in）+ testdb 测试覆盖。

### Modified Capabilities

（无——本次不修改已归档 spec 的 requirements。`adapter-interfaces` 已归档，不涉及。）

## Impact

- **代码**：新增 1 migration（4 表）+ 14 个 query .sql + `data/repo/` 15 个 Repo 文件 + `data/query_wrapper.go` + 扩展 generated/ + 扩展 schema.sql + 修正 data.go 注释 + 修正 server/AGENTS.md + 扩展测试。
- **API**：不改 proto 契约（数据访问层是内部 Go，不是 RPC）。
- **依赖**：可能新增 `Masterminds/squirrel`（动态查询构造，ferret 用自研 wrapper，我们用 squirrel 更标准）。
- **DB**：新增 4 张表，其余 25 张已落地。
- **配置**：不涉及。
- **测试**：扩展 testdb 模式到 Repo 层（CRUD + 事务 + 动态查询 + FK + migration 幂等）。
