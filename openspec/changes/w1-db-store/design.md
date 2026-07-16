## Context

W1-02 为 W1-03（registry/catalog）和 W1-04（tenancy/envtenant/stackmodel）准备元数据访问底座。

**现有架构约束**（脚手架已落地）：
- `server/data/data.go`：pgxpool provider + wire ProviderSet + `NewQueries(pool) → *generated.Queries`（otelpgx instrumented，D41）
- `server/pkg/db/queries/`：sqlc SQL 真相源（`schema.sql` 全表投影 + `teams.sql` 范例）
- `server/pkg/db/generated/`：sqlc 生成（`models.go` 24 个 model + `querier.go` + `teams.sql.go`）
- `server/pkg/db/`：testdb 工具（testcontainers + migrate up/down + clone）
- `server/data/teams_test.go`：测试范例（snowflake Init + testdb.New + generated.Queries CRUD 断言）
- snowflake ID：`internal/utils.GenerateID()` 应用层生成 BIGINT，调用方传入（非 DB autoincrement）
- 软删除：`deleted_at TIMESTAMPTZ NULL`，active 过滤用 `WHERE deleted_at IS NULL`

**已有表**（25 张，migration 001-012）：teams/projects/spaces/modules/module_versions/module_dependencies/catalog_items/stacks/stack_dependencies/requests/request_events/plan_artifacts/gate_results/approval_flows/approval_runs/approval_node_runs/approval_decisions/cloud_accounts/audit_logs/outbox_events/layer_logical_refs/layer_rule_set_versions/state_backends/workspaces/workspace_checkouts。

**缺失表**（doc 04 定义但无 migration）：environments/tenants/environment_tenant_bindings/tag_policies（W1 必需）+ identities/oidc_providers/sessions/role_bindings/executor_runs/drift_runs/drift_records/adapters_config/manual_intervention_tasks（W2 归属）。

## Goals / Non-Goals

**Goals:**
- 落地 4 张 W1 必需表（env/tenant/binding/tag_policy）的 migration
- 为 15 个核心实体写 sqlc query（Get/List/Create/Update/SoftDelete + 业务语义查询）
- `sqlc generate` 重生成，扩展类型安全 CRUD 覆盖
- 修正 data.go 注释，反映"core 直接注入 Queries，无 store 层"的正确架构
- testdb 测试模式扩展到关键实体

**Non-Goals:**
- 不建 `server/core/store/` 或任何 store/repo 包装层（业界 sqlc 最佳实践共识）
- 不做 core/<domain>/ 业务逻辑（W1-03/04 的活）
- 不落 W2 归属表的 query 层（身份/编排/漂移/审批/adapters_config）
- 不做跨聚合事务 UnitOfWork（YAGNI，真实需求出现再加）
- 不改 proto 契约
- 不改 DB schema 已落地表的结构（纯增量 + 注释修正）

## Decisions

### D1：core 直接注入 `*generated.Queries`，不建 store 层

**决策**：`server/core/<domain>/` 包通过 wire 注入 `*generated.Queries`，直接调用生成的方法（如 `queries.CreateModule(ctx, params)`）。**不建** `server/core/store/` 或 `server/data/store/` 包装层。

**理由**（业界 sqlc 最佳实践共识，2025-2026）：
- **Brandur（Stripe/CRDB 血统）**：sqlc 是 "the figurative Correct Answer"，生产代码直接 `dbsqlc.New(tx).CreateAuthor(...)`，无 wrapper
- **rednafi（2026.03 专文）**：sqlc 之上的 Repository 层在多数场景是过度工程——sqlc 已提供类型安全 + 编译期检查，经典"mock DB 解耦"理由在 testcontainers 真实 PG 测试下失效
- **sqlc 官方**：`DBTX` 接口（同时接受 `*pgxpool.Pool` 和 `pgx.Tx`）+ `WithTx(tx)` 已是抽象的接缝，不需要再造 Store 接口
- **"SQL-as-truth" 原则的直接推论**：多表 JOIN / 业务组合查询属于 `queries/<name>.sql`，**不是 Go store 方法**

**备选**：①建 server/core/store/ 薄包装（违反共识，多余抽象）②建 server/data/store/（同上）③per-domain store 包（sqlc .sql 文件已按域分区，Go 包再分区是重复）④混合 sqlc+pgx（破坏一致性）。

**影响**：修正 data.go 当前误导性注释（"the first consumer will be core/store (薄包装) when it lands in Wave 1"→ 改为"core/<domain>/ 直接注入 *generated.Queries"）。

### D2：复杂查询用 sqlc 的 `sqlc.embed()` + 多 `.sql` 文件，不退回 pgx

**决策**：多表 JOIN、嵌套结果、业务组合查询全部写在 `server/pkg/db/queries/<name>.sql`，用 sqlc 1.7+ 的 `sqlc.embed()` 处理嵌套 struct（如 `ListStacksWithSpaceAndLayer` 返回 stack + space + layer 信息）。**不**为了"灵活"退回 pgx 手写 SQL。

**理由**：
- "SQL-as-truth" 要求所有 SQL 可审计、可 grep、可版本化
- pgx 手写 SQL 失去类型安全（返回 `pgx.Rows` 要手动 Scan）
- sqlc.embed() 已能处理 90% 的 JOIN 嵌套场景（sqlc issue #643 的官方解法）

**唯一例外**：运行时动态查询（变量 IN-list、ad-hoc 过滤组合）用 pgx——W1 范围内暂无此需求。

**备选**：pgx 手写复杂查询（失去类型安全 + SQL 散落 Go 代码）；view + 普通 query（view 不能传参，限制大）。

### D3：query 按表分区（一表一 .sql 文件）

**决策**：`server/pkg/db/queries/<table>.sql` 一表一文件（如 `modules.sql`、`stacks.sql`），文件内含该表所有 query。跨表组合查询放在主导实体的文件里（如 `ListStacksInSpace` 放 `stacks.sql`）。

**理由**：
- 与 `teams.sql` 范例一致
- 文件按表名定位，O(1) 查找
- sqlc 编译时合并所有 .sql，物理分区不影响生成

**备选**：按业务域分区（queries/registry.sql 含 modules+catalog）——跨表查询归属模糊；按 query 类型分区（queries/gets.sql / queries/creates.sql）——维护时跨多文件。

### D4：W1 表范围 = 15 个核心实体 + 4 张新表

**决策**：W1-02 落地范围：

| 类别 | 实体（已有 migration） | 新增 query |
|------|----------------------|-----------|
| 组织归属 | teams, projects, spaces | ✓ |
| 模块目录 | modules, module_versions, module_dependencies, catalog_items | ✓ |
| stack 注册表 | stacks, stack_dependencies | ✓ |
| 分层模型 | layer_logical_refs, layer_rule_set_versions | ✓ |
| env/tenant（**新表**）| environments, tenants, environment_tenant_bindings | ✓（含 migration）|
| tag 治理（**新表**）| tag_policies | ✓（含 migration）|

**推迟到 W2 的表**（已有 migration，但 query 层 W2 补）：requests/request_events/plan_artifacts/gate_results/approval_*/cloud_accounts/audit_logs/outbox_events/state_backends/workspaces/workspace_checkouts。

**理由**：W1-03（registry/catalog）需要 modules/catalog query；W1-04（tenancy/envtenant）需要 env/tenant/team/space query。其余表归 W2 对应模块，提前做违反模块边界（YAGNI）。

**备选**：①全部 ~40 张表 query 一次性做（W1-02 膨胀，且 W2 模块边界模糊）②只做 W1-03/04 最小集（缺 layer/tag_policy，W1-04 分层模型无法完整测）。

### D5：测试用 testdb + generated.Queries 直测（不引入 mock）

**决策**：扩展现有 `teams_test.go` 模式——testcontainers 起真实 PG + migrate + clone，直接断言 `generated.Queries` 方法。**不**引入 mock interface。

**理由**：
- sqlc 生成的方法不需要 mock（类型已确定，mock 是反模式）
- 真实 PG 测试能捕获 SQL 语义错误（约束、FK、触发器）
- testcontainers 已配好，测试速度可接受（teams_test.go 已验证）

**测试覆盖优先级**：①核心实体 CRUD（teams/projects/spaces/modules/catalog_items/stacks）②业务唯一约束（team slug、module source_url+ref、catalog display_name）③FK 关系（stack→space、catalog→module_version、env_tenant_binding→env+tenant）④migration up/down 幂等。

**备选**：①mock store interface（违背 sqlc 哲学）②只测 migration 不测 query（query 错误漏检）。

## Risks / Trade-offs

- **[15 个 query 文件工作量集中] → 按 teams.sql 范例模板化**：每表 query 结构相似（Get/List/Create/Update/SoftDelete），可批量生成骨架再补业务查询。预计每表 30-50 行 SQL。
- **[sqlc.embed() 嵌套 struct 学习成本] → 仅 W1-04 ListStacksWithSpaceAndLayer 一处用**：MVP 期间组合查询少，embed 用法局限在分层模型场景。
- **[4 张新表的 FK 与现有表耦合] → migration 顺序 + 验证**：environment_tenant_bindings 引用 environments + tenants，tag_policies 引用 scope（多态），需在 013 migration 内按依赖顺序建表 + 加 COMMENT。migration up/down 用 testdb 验证幂等。
- **[core 直接注入 Queries 的"事务边界"问题] → sqlc WithTx + 业务侧显式 Begin**：当 W1-03/04 出现"create module + create version"原子需求时，core 层用 `pool.BeginTx()` + `generated.New(tx).WithTx(tx)` 显式管理。不引入 UnitOfWork 抽象（YAGNI）。
- **[generated/ 重新生成可能影响 teams_test.go] → 重跑测试验证**：sqlc generate 是幂等的，teams.sql 不改则 teams.sql.go 不变。生成后跑 `go test ./server/data/... -short` 确认无回归。
- **[tag_policies 多态 scope 设计复杂] → MVP 用 scope_type + scope_id TEXT（非 FK）**：tag_policies 引用 platform/env/tenant/team/space/catalog_item 多种实体，用多态引用（scope_type + scope_id）而非 N 个可空 FK。完整性由应用层保证（W1-04 tag 治理逻辑）。

## Migration Plan

1. 写 `013_env_tenant_tagpolicy.sql`（4 表 + COMMENT + 索引 + seed 默认 platform-default tenant + dev/staging/prod envs）
2. 更新 `schema.sql` 追加 4 张表（sqlc 解析输入）
3. 写 14 个 query .sql 文件（teams 已有）
4. 跑 `sqlc generate` 重生成
5. 跑 `go build ./... && go vet ./...`
6. 跑 `go test ./server/data/... -short`（含新表测试）
7. 修正 `data.go` 注释
8. 跑 migration up/down 幂等测试（testdb）

**回滚**：migration 013 的 down 路径 DROP 4 张表；query .sql 文件删除后重 generate 回滚 generated/。无数据迁移（新表无存量数据）。

## Open Questions

- **tag_policies 多态引用是否需要 DEFERRABLE 约束？** 倾向不需要（应用层校验），但 W1-04 实现 tag 治理时验证。MVP 用 `ON DELETE RESTRICT` 保护 platform 级 policy 不被误删。
- **layer_rule_set_versions seed v1 是否在本 change 补？** doc 04 §2.9 提到 "Phase 1 ships v1 active+default"，但当前 migration 010 未 seed。倾向在本 change 补 seed（否则 stacks.layer_rule_set_version_id 无默认值）。待 tasks 阶段确认。
