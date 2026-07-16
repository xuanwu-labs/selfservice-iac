## ADDED Requirements

### Requirement: W1 必需表完整落地

平台 MUST 在 `server/cmd/migrate/migrations/` 落地 W1-03（registry/catalog）和 W1-04（tenancy/envtenant/stackmodel）所需的全部表。当前 25 张表已存在；本能力 MUST 补齐 `environments`、`tenants`、`environment_tenant_bindings`、`tag_policies` 4 张表的 migration（doc 04 §2.11/§2.12 + doc 07 env+tenant 模型）。身份/编排/漂移/审批/adapters_config/manual_intervention_tasks 表推迟到 W2 对应模块。

#### Scenario: environments 表支持稳定身份 + 网络上下文

- **WHEN** migration 013 执行
- **THEN** `environments` 表创建，含 `env_logical_id`（稳定身份如 dev/staging/prod/dr）+ `display_name` + `status` + 网络上下文 JSON 字段
- **AND** 默认 seed 包含 dev/staging/prod/dr 4 个环境

#### Scenario: tenants 表支持逻辑租户身份

- **WHEN** migration 013 执行
- **THEN** `tenants` 表创建，含 `tenant_logical_id`（稳定身份）+ `name` + `status`
- **AND** 默认 seed 包含 `platform-default` 租户

#### Scenario: environment_tenant_bindings 表连接 env 和 tenant

- **WHEN** migration 013 执行
- **THEN** `environment_tenant_bindings` 表创建，含 `env_id` + `tenant_id` FK + binding 元数据
- **AND** FK ON DELETE RESTRICT 保护被引用的 env/tenant 不被误删

#### Scenario: tag_policies 表支持多态 scope

- **WHEN** migration 013 执行
- **THEN** `tag_policies` 表创建，含 `scope_type`（platform/env/tenant/team/space/catalog_item 多态）+ `scope_id` + `policy_json`
- **AND** 完整性由应用层校验（多态引用非 DB FK）

### Requirement: 15 个核心实体的 sqlc query

平台 MUST 在 `server/pkg/db/queries/` 为 15 个核心实体编写 sqlc query（遵循 `teams.sql` 范例：snowflake ID 调用方传入、软删除 `WHERE deleted_at IS NULL`、一表一 .sql 文件）。覆盖 teams/projects/spaces/modules/module_versions/module_dependencies/catalog_items/stacks/stack_dependencies/layer_logical_refs/layer_rule_set_versions/environments/tenants/environment_tenant_bindings/tag_policies。每个 query 文件 MUST 提供基础 CRUD（Get/List/Create/Update/SoftDelete）+ 业务语义查询（如 ListVisibleCatalogItems 按 visibility_json GIN 过滤、GetStackByRepoPath 按 repo_path 唯一定位）。

#### Scenario: 每个核心实体有完整 CRUD query

- **WHEN** sqlc 解析 `server/pkg/db/queries/*.sql`
- **THEN** 15 个核心实体每个都有 Get（by id）/ Get（by 业务唯一键）/ List（含分页或过滤变体）/ Create / Update / SoftDelete 查询

#### Scenario: 复杂组合查询用 sqlc.embed() 不退回 pgx

- **WHEN** 需要多表 JOIN 组合查询（如 ListStacksWithSpaceAndLayer 返回 stack + space + layer 信息）
- **THEN** 查询写在 `queries/<主导实体>.sql`，用 sqlc 1.7+ 的 `sqlc.embed()` 处理嵌套 struct
- **AND** 不为"灵活"退回 pgx 手写 SQL（"SQL-as-truth" 原则）

#### Scenario: 软删除一致性

- **WHEN** 调用任意核心实体的 Get/List 查询
- **THEN** 查询自动过滤 `WHERE deleted_at IS NULL`（软删除记录不返回）
- **AND** SoftDelete 操作设置 `deleted_at = now()` 而非物理删除

### Requirement: core 直接注入 *generated.Queries 不建 store 层

平台 MUST NOT 创建 `server/core/store/` 或任何 store/repository 包装层。`server/core/<domain>/` 包 MUST 通过 wire 直接注入 `*generated.Queries`，调用 sqlc 生成的方法。这是业界 sqlc 最佳实践共识（Brandur 生产实践 / rednafi 2026 专文 / sqlc 官方 DBTX 接口设计）。`server/data/data.go` MUST 移除当前误导性注释 "the first consumer will be core/store (薄包装) when it lands in Wave 1"。

#### Scenario: core 包注入 *generated.Queries

- **WHEN** wire 生成依赖图
- **THEN** `server/core/<domain>/` 包接收 `*generated.Queries`（来自 `data.NewQueries`）
- **AND** 直接调用生成的方法（如 `queries.CreateModule(ctx, params)`），无中间包装

#### Scenario: 跨聚合事务用 sqlc WithTx 显式管理

- **WHEN** 出现需要原子写多表的操作（如 create module + create version）
- **THEN** core 层用 `pool.BeginTx()` + `generated.New(tx)` 显式管理事务边界
- **AND** 不引入 UnitOfWork 抽象层（YAGNI，真实跨域事务需求出现再加）

### Requirement: testdb 测试覆盖关键实体

平台 MUST 扩展现有 `server/data/teams_test.go` 的 testdb + snowflake + generated.Queries 测试模式到关键实体。MUST 覆盖：①核心实体 CRUD（spaces/modules/catalog_items/stacks/environments）②业务唯一约束（team slug、module source_url+ref、catalog display_name）③FK 关系（stack→space、catalog→module_version、env_tenant_binding→env+tenant）④migration up/down 幂等。MUST NOT 引入 mock interface（sqlc 生成的方法不需 mock，testcontainers 真实 PG 是事实源）。

#### Scenario: 核心实体 CRUD 测试通过

- **WHEN** 运行 `go test ./server/data/... -short`
- **THEN** spaces/modules/catalog_items/stacks/environments 各自的 CRUD 测试通过
- **AND** 业务唯一约束被验证（重复 slug/source_url 报错）

#### Scenario: FK 关系完整性测试

- **WHEN** 测试创建带 FK 关系的实体（如 stack 引用 space）
- **THEN** 引用不存在的 space_id 时 FK 约束报错
- **AND** 正常 FK 关系查询能 JOIN 返回关联数据

#### Scenario: migration up/down 幂等

- **WHEN** testdb 跑 013 migration up → down → up
- **THEN** 无错误，无残留对象，幂等可重复
