## Why

W1-02 是 `iac-self-service-platform` 的第二个实现模块，为 W1-03（模块注册）/ W1-04（分层与租户）准备元数据底座。当前状态：**12 个 migration 文件已落地 25 张表 + sqlc 三件套已配好 + `teams` 1 张表有 query 范例**，但其余 24 张表没有 sqlc query → 没有 CRUD 方法；同时 `docs/04` 定义的 `environments / tenants / environment_tenant_bindings / tag_policies` 4 张 W1 必需表尚未建 migration。

**影响层级**：数据访问层（`server/pkg/db/` + `server/data/`），不改 API/proto 契约。

**为什么现在做**：W1-03 的 registry/catalog 和 W1-04 的 tenancy/envtenant 都需要 query 层 CRUD 方法才能开始；缺少 env/tenant 表会让 stacks.env_id / tenant_id FK 悬空。

## What Changes

- **补齐 4 张 W1 必需表的 migration**（`013_env_tenant_tagpolicy.sql`）：`environments`、`tenants`、`environment_tenant_bindings`、`tag_policies`（doc 04 §2.11/§2.12 + doc 07 env+tenant 模型）。**身份/编排/漂移/审批表（identities/oidc_providers/sessions/role_bindings/executor_runs/drift_runs/adapters_config/manual_intervention_tasks）推迟到对应 W2 模块**（YAGNI，W1 不需要）。
- **为 W1 范围内的核心表写 sqlc query**（`server/pkg/db/queries/<table>.sql`）：覆盖 `teams / projects / spaces / modules / module_versions / module_dependencies / catalog_items / stacks / stack_dependencies / environments / tenants / environment_tenant_bindings / tag_policies / layer_logical_refs / layer_rule_set_versions` 共 15 个核心实体。每个 query 文件含 Get/List/Create/Update/SoftDelete（或对应的业务语义查询），遵循 `teams.sql` 已建立的模式（snowflake ID 由调用方传入、软删除用 `WHERE deleted_at IS NULL`）。
- **`sqlc generate` 重新生成**：`server/pkg/db/generated/` 从当前 4 个文件扩展到覆盖全部 15 个实体的类型安全 CRUD。
- **修正 `server/data/data.go` 误导性注释**：当前注释写 "the first consumer will be core/store (薄包装) when it lands in Wave 1"，但按业界 sqlc 最佳实践（Brandur/rednafi/官方共识），**不建 store 层**——core 包直接注入 `*generated.Queries`。注释改为反映正确架构。
- **`schema.sql` 同步投影**：把新增 4 张表的 schema 追加到 `server/pkg/db/queries/schema.sql`（sqlc 解析输入）。
- **测试扩展**：把 `server/data/teams_test.go` 的 testdb + snowflake + generated.Queries 测试模式扩展到关键实体（spaces / modules / catalog_items / stacks / environments + tenant binding），断言 CRUD + 业务唯一约束 + FK 关系。

### 不做（本次范围外）

- 身份相关表（identities/oidc_providers/sessions/role_bindings）→ W2 身份模块
- 编排相关表（executor_runs/plan_artifacts 关联）→ W2 编排模块（plan_artifacts 已建表，但 query 层 W2 补）
- 漂移表（drift_runs/drift_records）→ W2 漂移模块
- 审批表 query 层（approval_flows/approval_runs 等已建表）→ W2 审批模块（表已落地，query 层 W2 补）
- adapters_config / manual_intervention_tasks / outbox_events query 层 → W2
- 跨聚合事务 UnitOfWork → 仅当真实跨域事务需求出现时才加（YAGNI）
- core/<domain>/ 业务逻辑 → W1-03/04

## Capabilities

### New Capabilities

- `metadata-store`: W1 元数据访问层——15 个核心实体的 sqlc query + 类型安全 CRUD + env/tenant/tag_policy 表落地 + testdb 测试覆盖。遵循 sqlc "SQL-as-truth" 原则（复杂查询写 .sql 不写 Go），core 层直接注入 `*generated.Queries`（不建 store/repo 包装层）。

### Modified Capabilities

（无——本次不修改已有 spec 的 requirements。`adapter-interfaces` capability 已归档，不涉及。）

## Impact

- **代码**：新增 1 个 migration（4 表）+ 14 个 query .sql 文件（teams 已有）+ 扩展 generated/ + 扩展 schema.sql + 修正 data.go 注释 + 扩展测试。纯增量 + 注释修正，不改已有逻辑。
- **API**：不改 proto 契约（数据访问层是内部 Go 接口，不是 RPC）。
- **依赖**：无新外部依赖（sqlc + pgx/v5 + testcontainers/cockroach 已在 go.mod）。
- **DB**：新增 4 张表（environments/tenants/environment_tenant_bindings/tag_policies），其余 25 张表已落地。
- **配置**：不涉及（adapters_config 推迟 W2）。
- **测试**：扩展 testdb 模式到关键实体，断言 CRUD + 唯一约束 + FK 完整性。
