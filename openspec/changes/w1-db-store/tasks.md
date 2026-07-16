## 1. 补齐 4 张 W1 必需表 migration

- [ ] 1.1 编写 `server/cmd/migrate/migrations/013_env_tenant_tagpolicy.sql`：`environments`（env_logical_id 稳定身份 + display_name + status + 网络上下文 json，参考 doc 04 §2.11 + doc 07）+ `tenants`（tenant_logical_id + name + status）+ `environment_tenant_bindings`（env_id + tenant_id + binding 元数据，FK ON DELETE RESTRICT）+ `tag_policies`（scope_type 多态 + scope_id + policy_json，参考 doc 04 §2.12）。含 CREATE TABLE + COMMENT + UNIQUE INDEX + set_updated_at trigger
- [ ] 1.2 在 013 migration 末尾追加 seed：默认 tenant `platform-default` + 默认 envs `dev`/`staging`/`prod`/`dr`；若 layer_rule_set_versions v1 未 seed 则补 seed（doc 04 §2.9 "Phase 1 ships v1 active+default"）
- [ ] 1.3 同步追加 4 张表 schema 到 `server/pkg/db/queries/schema.sql`（sqlc 解析输入）

## 2. 15 个核心实体的 sqlc query

> 遵循 `teams.sql` 范例：snowflake ID 调用方传入、软删除 `WHERE deleted_at IS NULL`、一表一 .sql 文件。

### 2.1 组织归属（teams 已有，补 projects + spaces）

- [ ] 2.1 编写 `server/pkg/db/queries/projects.sql`：GetProject / GetProjectBySlug / ListProjects / ListProjectsByTeam / CreateProject / UpdateProject / SoftDeleteProject
- [ ] 2.2 编写 `server/pkg/db/queries/spaces.sql`：GetSpace / ListSpaces / ListSpacesByProject / ListSpacesByLayer / CreateSpace / UpdateSpace / SoftDeleteSpace

### 2.2 模块与目录（W1-03 前置）

- [ ] 2.3 编写 `server/pkg/db/queries/modules.sql`：GetModule / GetModuleBySource / ListModules / ListModulesByOwner / ListModulesByLayer / CreateModule / UpdateModule / SoftDeleteModule
- [ ] 2.4 编写 `server/pkg/db/queries/module_versions.sql`：GetModuleVersion / GetModuleVersionByRef / GetCurrentModuleVersion / ListModuleVersions / CreateModuleVersion / SetCurrentModuleVersion
- [ ] 2.5 编写 `server/pkg/db/queries/module_dependencies.sql`：ListDependencies / ListDependents / CreateModuleDependency
- [ ] 2.6 编写 `server/pkg/db/queries/catalog_items.sql`：GetCatalogItem / ListCatalogItems / ListCatalogItemsByLayer / ListCatalogItemsByOwner / ListVisibleCatalogItems（visibility_json GIN 过滤）/ PublishCatalogItem / UpdateCatalogItem / SoftDeleteCatalogItem

### 2.3 stack 注册表（W1-04 前置）

- [ ] 2.7 编写 `server/pkg/db/queries/stacks.sql`：GetStack / GetStackByRepoPath / ListStacks / ListStacksBySpace / ListStacksByLayer / ListStacksByEnv / CreateStack / UpdateStack / SoftDeleteStack
- [ ] 2.8 编写 `server/pkg/db/queries/stack_dependencies.sql`：ListDependencies / ListDependents / CreateStackDependency / DeleteStackDependency

### 2.4 分层模型（W1-04 前置）

- [ ] 2.9 编写 `server/pkg/db/queries/layer_logical_refs.sql`：GetLayerLogicalRef / ListLayerLogicalRefs / CreateLayerLogicalRef / UpdateLayerLogicalRefDisplayName
- [ ] 2.10 编写 `server/pkg/db/queries/layer_rule_set_versions.sql`：GetRuleSetVersion / GetActiveRuleSetVersion / ListRuleSetVersions / CreateRuleSetVersion / SupersedeRuleSetVersion

### 2.5 env/tenant/tag_policy（新表 + W1-04 前置）

- [ ] 2.11 编写 `server/pkg/db/queries/environments.sql`：GetEnvironment / GetEnvironmentByLogicalId / ListEnvironments / CreateEnvironment / UpdateEnvironment / SoftDeleteEnvironment
- [ ] 2.12 编写 `server/pkg/db/queries/tenants.sql`：GetTenant / GetTenantByLogicalId / ListTenants / CreateTenant / UpdateTenant / SoftDeleteTenant
- [ ] 2.13 编写 `server/pkg/db/queries/environment_tenant_bindings.sql`：GetBinding / ListBindingsByEnv / ListBindingsByTenant / CreateBinding / DeleteBinding
- [ ] 2.14 编写 `server/pkg/db/queries/tag_policies.sql`：GetTagPolicy / ListTagPoliciesByScope / CreateTagPolicy / UpdateTagPolicy / SoftDeleteTagPolicy（scope_type 多态过滤）

## 3. sqlc 重生成

- [ ] 3.1 运行 `sqlc generate` 重生成 `server/pkg/db/generated/`（从 4 个文件扩展到覆盖 15 个实体）

## 4. ★ 建立 data/repo/ 层（混合范式核心）

> 参考 ferret `data/email.go` 的 `EmailRepo{log, *GormDB}` 范式，适配 sqlc（薄包装 `*generated.Queries` + 跨表事务 WithTx + 动态查询 query_wrapper）。

### 4.1 基础设施

- [ ] 4.1 新建 `server/data/repo/` 目录
- [ ] 4.2 编写 `server/data/query_wrapper.go`：动态查询构造器（适配 pgx，用 `Masterminds/squirrel` 底层）。接口：`New() / Eq / Ne / In / Like / Between / IsNull / Or(嵌套) / OrderBy / Page / BuildSQL(base) → (sql, args)`。参考 ferret `data/query_wrapper.go` 的 MyBatis-Plus 风格
- [ ] 4.3 在 `go.mod` 添加 `github.com/Masterminds/squirrel` 依赖

### 4.2 15 个 Repo struct（每实体一个文件）

- [ ] 4.4 编写 `server/data/repo/team.go`：`TeamRepo{pool, queries}` + GetByID/GetBySlug/List/Create/Update/SoftDelete（薄包装）+ ListByDynamicFilter（query_wrapper 示范）
- [ ] 4.5 编写 `server/data/repo/project.go`：ProjectRepo + 基础 CRUD
- [ ] 4.6 编写 `server/data/repo/space.go`：SpaceRepo + 基础 CRUD
- [ ] 4.7 编写 `server/data/repo/module.go`：ModuleRepo + CRUD + **CreateWithVersion**（跨表事务示范，pool.Begin + WithTx + CreateModule + CreateModuleVersion + Commit）
- [ ] 4.8 编写 `server/data/repo/module_version.go`：ModuleVersionRepo + 基础 CRUD + SetCurrent
- [ ] 4.9 编写 `server/data/repo/module_dependency.go`：ModuleDependencyRepo
- [ ] 4.10 编写 `server/data/repo/catalog.go`：CatalogRepo + CRUD + **ListVisible**（visibility_json GIN）+ **ListByDynamicFilter**（多维可选过滤：layer/owner/status/tag，query_wrapper 示范）
- [ ] 4.11 编写 `server/data/repo/stack.go`：StackRepo + CRUD + GetByRepoPath
- [ ] 4.12 编写 `server/data/repo/stack_dependency.go`：StackDependencyRepo
- [ ] 4.13 编写 `server/data/repo/layer_logical_ref.go`：LayerLogicalRefRepo
- [ ] 4.14 编写 `server/data/repo/layer_rule_set_version.go`：LayerRuleSetVersionRepo + GetActive + Supersede（事务）
- [ ] 4.15 编写 `server/data/repo/environment.go`：EnvironmentRepo
- [ ] 4.16 编写 `server/data/repo/tenant.go`：TenantRepo
- [ ] 4.17 编写 `server/data/repo/environment_tenant_binding.go`：EnvironmentTenantBindingRepo
- [ ] 4.18 编写 `server/data/repo/tag_policy.go`：TagPolicyRepo + ListByScope（多态）

### 4.3 dbset 多表组合（opt-in）

- [ ] 4.19 编写 `server/data/dbset/stack_with_space.go`：`StackWithSpace{Stack, Space, LayerLogicalRef}` 跨表聚合 struct（W1-04 stackmodel 列表视图用）+ 在 Repo 提供 `ListStacksWithSpace(ctx, filter)` 方法（sqlc.embed() 或 query_wrapper JOIN）

### 4.4 wire 装配 + 注释修正

- [ ] 4.20 扩展 `server/data/data.go` 的 `ProviderSet`：注册全部 15 个 `NewXxxRepo`
- [ ] 4.21 修正 `server/data/data.go` 的 `NewQueries` 注释：当前 "the first consumer will be core/store (薄包装) when it lands in Wave 1" → 改为 "core/<domain>/ 通过 wire 注入 data/repo Repo struct（混合范式：ferret Repo struct + DIP 可演进 + sqlc SQL-as-truth）；需要 mock 时在 core 提取小 interface（Go 隐式）"
- [ ] 4.22 修正 `server/AGENTS.md` 三处矛盾（line 68 "core/store 薄包装" / line 127 "等 core/store 落地" / line 132 数据流），统一为：`core/<domain>/`（注入 Repo struct）→ `data/repo/`（实现）→ `pkg/db/generated`

## 5. 测试

- [ ] 5.1 扩展 `server/data/teams_test.go` 模式：新建 `server/data/repo/{module,catalog,stack,space,environment}_test.go`（每个含 Repo CRUD + 业务方法断言）
- [ ] 5.2 新建 `server/data/repo/transaction_test.go`：断言跨表事务（ModuleRepo.CreateWithVersion 成功 commit / 失败 rollback）
- [ ] 5.3 新建 `server/data/query_wrapper_test.go`：断言动态查询构造（Eq/In/Like/Or 嵌套/OrderBy/Page → SQL + args 正确）
- [ ] 5.4 新建 `server/data/repo/fk_relations_test.go`：断言 FK（stack→space、catalog→module_version、env_tenant_binding→env+tenant、tag_policy scope 多态）
- [ ] 5.5 新建 `server/data/migration_idempotent_test.go`：testdb 跑 013 migration up→down→up 幂等
- [ ] 5.6 验证：`go build ./... && go vet ./... && go test ./server/data/... -short` 全部通过

## 6. 验证与提交

- [ ] 6.1 `gofmt -l server/` 无输出
- [ ] 6.2 migration 测试通过（testdb up/down 幂等）
- [ ] 6.3 提交到 `feat/w1-db-store` 分支，commit message 英文，遵循 conventional commits
