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
- **THEN** 15 个核心实体每个都有 Get（by id）/ Get（by 业务唯一键）/ List（含过滤变体）/ Create / Update / SoftDelete 查询

#### Scenario: 软删除一致性

- **WHEN** 调用任意核心实体的 Get/List 查询
- **THEN** 查询自动过滤 `WHERE deleted_at IS NULL`（软删除记录不返回）
- **AND** SoftDelete 操作设置 `deleted_at = now()` 而非物理删除

### Requirement: data/repo/ 层采用混合范式（ferret Repo struct × DIP 可演进 × sqlc SQL-as-truth）

平台 MUST 在 `server/data/repo/` 为 15 个核心实体建立 Repo struct（参考 ferret `data/email.go` 的 `EmailRepo{log, *GormDB}` 范式，适配 sqlc）。每个 Repo struct MUST 薄包装 `*generated.Queries`，提供三类方法：①薄包装（GetByID/Create/List 直接调 generated）②跨表事务（用 `pool.Begin` + `queries.WithTx(tx)` + Commit）③动态查询（用 `query_wrapper`，处理 sqlc 不擅长的 IN-list/ad-hoc 过滤）。wire MUST 在 `data.ProviderSet` 注册所有 `NewXxxRepo`。core 层 MUST 直接注入 Repo struct（ferret 风，最简）；需要测试/mock 时在 core 提取小 interface（Go 隐式 interface，无需改 data 层）——DIP 可演进。

#### Scenario: Repo struct 薄包装 generated.Queries

- **WHEN** core 业务层调用 `moduleRepo.GetByID(ctx, id)`
- **THEN** Repo 方法委托给 `r.queries.GetModule(ctx, id)`（sqlc 生成）
- **AND** 返回 `generated.Module` struct（类型安全）

#### Scenario: 跨表事务用 sqlc WithTx

- **WHEN** 调用 `moduleRepo.CreateWithVersion(ctx, module, version)`（创建 module + 关联 version）
- **THEN** Repo 方法用 `pool.Begin(ctx)` 开启事务 + `queries.WithTx(tx)` 绑定 + 两个 Create 调用 + `Commit`
- **AND** 任一 Create 失败时 `Rollback` 保证原子性

#### Scenario: 动态查询用 query_wrapper

- **WHEN** 调用 `catalogRepo.ListByDynamicFilter(ctx, wrapper)`（多维可选过滤：layer/owner/status/tag）
- **THEN** query_wrapper 构造 SQL + args（用 squirrel 底层），`pool.Query` 执行
- **AND** 支持运行时确定 IN-list 数量、嵌套 OR 条件、分页

#### Scenario: core 直接注入 Repo struct

- **WHEN** wire 生成依赖图
- **THEN** `server/core/<domain>/` 包通过 wire 接收 `*repo.XxxRepo`（具体类型）
- **AND** core 不直接 import `pkg/db/generated`（通过 Repo 间接访问，依赖方向正确）

#### Scenario: 需要测试时在 core 提取 interface

- **WHEN** 某 core 包需要 mock 数据访问（如 RegistryService 单元测试）
- **THEN** 在 core 包定义小 interface（如 `type ModuleStore interface { GetModule(ctx, id) (generated.Module, error) }`）
- **AND** `*repo.ModuleRepo` 自动满足该 interface（Go 隐式 interface，无需改 data 层）

### Requirement: data/query_wrapper.go 动态查询构造器

平台 MUST 在 `server/data/query_wrapper.go` 实现动态查询构造器（参考 ferret `data/query_wrapper.go` 的 MyBatis-Plus 风格，适配 pgx 而非 GORM，用 `Masterminds/squirrel` 底层）。MUST 提供链式 API：`New() / Eq / Ne / In / Like / Between / IsNull / Or(嵌套) / OrderBy / Page / BuildSQL(base) → (sql, args)`。MUST 仅用于 sqlc 不擅长的动态场景（IN-list、ad-hoc 过滤组合、分页）；固定查询 MUST 走 sqlc `.sql` 文件（SQL-as-truth）。

#### Scenario: query_wrapper 构造正确的 SQL + args

- **WHEN** 构造 `New().Eq("status", "active").In("layer", "db", "mw").OrderBy("created_at", true).Page(1, 20).BuildSQL("SELECT * FROM modules WHERE deleted_at IS NULL")`
- **THEN** 生成 SQL `SELECT * FROM modules WHERE deleted_at IS NULL AND status = $1 AND layer IN ($2, $3) ORDER BY created_at DESC LIMIT $4 OFFSET $5`
- **AND** args `["active", "db", "mw", 20, 0]`

#### Scenario: 嵌套 OR 条件正确

- **WHEN** 构造 `New().Eq("a", 1).Or(func(w){ w.Eq("b", 2).Eq("c", 3) })`
- **THEN** 生成 `a = $1 OR (b = $2 AND c = $3)`

### Requirement: data/dbset/ 多表组合（opt-in）

平台 MUST 在 `server/data/dbset/` 提供多表聚合 struct，**仅在 core 需要聚合视图时建**（不强制每实体）。W1 范围 MUST 至少建 `StackWithSpace`（stack + space + layer 跨表聚合，W1-04 stackmodel 列表视图用）。dbset 的数据获取 MUST 通过 Repo 方法（sqlc.embed() JOIN 或 query_wrapper）。

#### Scenario: StackWithSpace 聚合 stack + space + layer

- **WHEN** W1-04 stackmodel 需要展示 stack 列表（含 space 名 + layer 名）
- **THEN** 调用 `stackRepo.ListStacksWithSpace(ctx, filter)` 返回 `[]dbset.StackWithSpace`
- **AND** 每个 StackWithSpace 含 Stack + Space + LayerLogicalRef 三个 struct 的字段

### Requirement: server/AGENTS.md 矛盾修正

平台 MUST 修正 `server/AGENTS.md` 的三处内部矛盾（line 68 "core/store(薄包装,调用方)" / line 127 "dbset 等 core/store 落地" / line 132 数据流图）。MUST 统一为混合范式：`core/<domain>/`（注入 Repo struct）→ `data/repo/`（Repo 实现）→ `pkg/db/generated`（sqlc）。MUST 删除 `core/store/` 薄包装表述（违反 DIP，core 不应 import pkg/db）。MUST 修正 `data.go` 的 `NewQueries` 误导性注释。

#### Scenario: AGENTS.md 三处统一为混合范式

- **WHEN** 阅读 server/AGENTS.md 的数据层章节
- **THEN** line 68/127/132 三处表述一致：core 注入 data/repo Repo struct，data/repo 实现，pkg/db/generated 是 sqlc 输出
- **AND** 无 "core/store 薄包装" 或 "等 core/store 落地" 的矛盾表述

### Requirement: testdb 测试覆盖 Repo 层

平台 MUST 扩展现有 `server/data/teams_test.go` 的 testdb + snowflake + generated.Queries 测试模式到 Repo 层。MUST 覆盖：①核心 Repo CRUD（module/catalog/stack/space/environment）②跨表事务（CreateWithVersion 成功 commit / 失败 rollback）③动态查询（query_wrapper SQL + args 正确）④FK 关系（stack→space、catalog→module_version、env_tenant_binding→env+tenant）⑤migration up/down 幂等。MUST NOT 引入 mock interface（Repo struct 直接测，testcontainers 真实 PG 是事实源）。

#### Scenario: Repo CRUD 测试通过

- **WHEN** 运行 `go test ./server/data/... -short`
- **THEN** module/catalog/stack/space/environment 各自 Repo 的 CRUD 测试通过
- **AND** 业务唯一约束被验证（重复 slug/source_url 报错）

#### Scenario: 跨表事务原子性

- **WHEN** 测试 `moduleRepo.CreateWithVersion` 第二步失败
- **THEN** 第一步的 CreateModule 被 rollback（数据库无残留）
- **AND** 返回错误包含失败原因

#### Scenario: migration up/down 幂等

- **WHEN** testdb 跑 013 migration up → down → up
- **THEN** 无错误，无残留对象，幂等可重复
