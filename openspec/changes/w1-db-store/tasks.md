## 1. 补齐 4 张 W1 必需表 migration

- [ ] 1.1 编写 `server/cmd/migrate/migrations/013_env_tenant_tagpolicy.sql`：`environments`（env_logical_id 稳定身份 + display_name + status + 网络上下文 json）+ `tenants`（tenant_logical_id + name + status）+ `environment_tenant_bindings`（env_id + tenant_id + binding 元数据，参考 doc 04 §2.11 + doc 07）+ `tag_policies`（scope_type 多态 + scope_id + policy_json，参考 doc 04 §2.12）。含 CREATE TABLE + COMMENT + UNIQUE INDEX + set_updated_at trigger
- [ ] 1.2 在 013 migration 末尾追加 seed：默认 tenant `platform-default` + 默认 envs `dev`/`staging`/`prod`/`dr`（参考 doc 04 §2.11）；若 layer_rule_set_versions v1 未 seed 则补 seed（doc 04 §2.9 "Phase 1 ships v1 active+default"）
- [ ] 1.3 同步追加 4 张表 schema 到 `server/pkg/db/queries/schema.sql`（sqlc 解析输入）

## 2. 15 个核心实体的 sqlc query

> 遵循 `teams.sql` 范例：snowflake ID 由调用方传入、软删除 `WHERE deleted_at IS NULL`、每表一个 .sql 文件。

### 2.1 组织归属（teams 已有，补 projects + spaces）

- [ ] 2.1 编写 `server/pkg/db/queries/projects.sql`：GetProject / GetProjectBySlug / ListProjects / ListProjectsByTeam / CreateProject / UpdateProject / SoftDeleteProject
- [ ] 2.2 编写 `server/pkg/db/queries/spaces.sql`：GetSpace / ListSpaces / ListSpacesByProject / ListSpacesByLayer / CreateSpace / UpdateSpace / SoftDeleteSpace

### 2.2 模块与目录（W1-03 前置）

- [ ] 2.3 编写 `server/pkg/db/queries/modules.sql`：GetModule / GetModuleBySource / ListModules / ListModulesByOwner / ListModulesByLayer / CreateModule / UpdateModule / SoftDeleteModule
- [ ] 2.4 编写 `server/pkg/db/queries/module_versions.sql`：GetModuleVersion / GetModuleVersionByRef / GetCurrentModuleVersion / ListModuleVersions / CreateModuleVersion / SetCurrentModuleVersion
- [ ] 2.5 编写 `server/pkg/db/queries/module_dependencies.sql`：ListDependencies / ListDependents / CreateModuleDependency
- [ ] 2.6 编写 `server/pkg/db/queries/catalog_items.sql`：GetCatalogItem / ListCatalogItems / ListCatalogItemsByLayer / ListCatalogItemsByOwner / ListVisibleCatalogItems（visibility_json GIN 过滤）/ PublishCatalogItem / UpdateCatalogItem / SoftDeleteCatalogItem

### 2.3 stack 注册表（W1-04 前置）

- [ ] 2.7 编写 `server/pkg/db/queries/stacks.sql`：GetStack / GetStackByRepoPath / ListStacks / ListStacksBySpace / ListStacksByLayer / ListStacksByEnv / CreateStack / UpdateStack / SoftDeleteStack（+ 可选 ListStacksWithSpaceAndLayer 用 sqlc.embed() 组合查询）
- [ ] 2.8 编写 `server/pkg/db/queries/stack_dependencies.sql`：ListDependencies / ListDependents / CreateStackDependency / DeleteStackDependency

### 2.4 分层模型（W1-04 前置）

- [ ] 2.9 编写 `server/pkg/db/queries/layer_logical_refs.sql`：GetLayerLogicalRef / ListLayerLogicalRefs / CreateLayerLogicalRef / UpdateLayerLogicalRefDisplayName
- [ ] 2.10 编写 `server/pkg/db/queries/layer_rule_set_versions.sql`：GetRuleSetVersion / GetActiveRuleSetVersion / ListRuleSetVersions / CreateRuleSetVersion / SupersedeRuleSetVersion

### 2.5 env/tenant/tag_policy（新表 + W1-04 前置）

- [ ] 2.11 编写 `server/pkg/db/queries/environments.sql`：GetEnvironment / GetEnvironmentByLogicalId / ListEnvironments / CreateEnvironment / UpdateEnvironment / SoftDeleteEnvironment
- [ ] 2.12 编写 `server/pkg/db/queries/tenants.sql`：GetTenant / GetTenantByLogicalId / ListTenants / CreateTenant / UpdateTenant / SoftDeleteTenant
- [ ] 2.13 编写 `server/pkg/db/queries/environment_tenant_bindings.sql`：GetBinding / ListBindingsByEnv / ListBindingsByTenant / CreateBinding / DeleteBinding
- [ ] 2.14 编写 `server/pkg/db/queries/tag_policies.sql`：GetTagPolicy / ListTagPoliciesByScope / CreateTagPolicy / UpdateTagPolicy / SoftDeleteTagPolicy（scope_type 多态过滤）

## 3. sqlc 重生成 + 架构修正

- [ ] 3.1 运行 `sqlc generate` 重生成 `server/pkg/db/generated/`（从 4 个文件扩展到覆盖 15 个实体）
- [ ] 3.2 修正 `server/data/data.go` 注释：当前 "the first consumer will be core/store (薄包装) when it lands in Wave 1" → 改为反映 "core/<domain>/ 直接注入 *generated.Queries，无 store 层（业界 sqlc 最佳实践，Brandur/rednafi/官方共识）"

## 4. 测试

- [ ] 4.1 扩展 `server/data/teams_test.go` 模式到关键实体：新建 `server/data/{spaces,modules,catalog_items,stacks,environments}_test.go`（每个含 CRUD + 业务唯一约束断言）
- [ ] 4.2 新建 `server/data/fk_relations_test.go`：断言关键 FK 关系——stack→space、catalog_item→module_version、environment_tenant_binding→environment+tenant、tag_policy scope 多态
- [ ] 4.3 新建 `server/data/migration_idempotent_test.go`：testdb 跑 013 migration up→down→up，断言幂等（无残留、无错误）
- [ ] 4.4 验证：`go build ./... && go vet ./... && go test ./server/data/... -short` 全部通过

## 5. 验证与主提案同步

- [ ] 5.1 `gofmt -l server/` 无输出（代码格式干净）
- [ ] 5.2 `buf lint`（如有 proto 改动，本次预期无）+ migration 测试通过
- [ ] 5.3 提交到 `feat/w1-db-store` 分支，commit message 英文，遵循 conventional commits
