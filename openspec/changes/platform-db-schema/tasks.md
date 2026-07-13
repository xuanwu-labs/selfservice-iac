# Tasks: platform-db-schema

## 01-规范基线

- [ ] 1.1 创建 `server/cmd/migrate/migrations/000_utils.sql`：`set_updated_at()` trigger 函数（通用，所有业务表挂）
- [ ] 1.2 重写 `001_init.sql`：teams 表对齐新规范（`id UUID PK DEFAULT gen_random_uuid()`、补 `kind`/`tags_json`/`policy_json`/`deleted_at`、updated_at trigger、约束命名 `pk_/uq_` 前缀）
- [ ] 1.3 在 `design.md` 固化命名规范（主键/外键/约束/枚举/审计/JSON），作为后续所有表的标准

## 02-组织归属表

- [ ] 2.1 `migrations/002_organizations.sql`：teams（重写对齐规范）+ projects + bundles
- [ ] 2.2 测试：迁移 up/down 幂等 + FK 约束（删 team 有 project 引用时 RESTRICT）+ slug UNIQUE

## 03-registry 域表（模块注册）

- [ ] 3.1 `migrations/003_registry.sql`：modules + module_versions + module_dependencies
- [ ] 3.2 修复 docs/04 断裂：modules 补 name/provider/description；module_versions 补 version/providers_json + 接收 variables_contract_json（从 modules 移入）；新增 module_dependencies 表
- [ ] 3.3 测试：FK（module_versions→modules, module_dependencies→module_versions）+ status CHECK（pending_validation/validated/validation_failed/deprecated）

## 04-catalog 域表（服务目录）

- [ ] 4.1 `migrations/004_catalog.sql`：catalog_items（含 layer_logical_id FK → layer_logical_refs，但 layer 表先建见 §08）
- [ ] 4.2 修复断裂：补 category 列（proto CatalogItem 有）；visibility_json 建 GIN 索引
- [ ] 4.3 测试：FK（catalog_items→module_versions, →layer_logical_refs）+ status CHECK（active/deprecated/archived 对齐 proto+archived）+ cardinality CHECK

## 05-lifecycle 域表（工单+审批+artifact）

- [ ] 5.1 `migrations/005_lifecycle_requests.sql`：requests（补 team_id/source/cost_estimate_cents/cost_currency/correlation_id/plan_artifact_id）+ request_events（append-only）
- [ ] 5.2 `migrations/006_lifecycle_plan.sql`：plan_artifacts（独立表，含 summary 三列 + cost_estimate + expires_at）+ gate_results
- [ ] 5.3 `migrations/007_lifecycle_approval.sql`：approval_runs（补 decided_by/decided_at/expires_at）+ approval_node_runs + approval_decisions（append-only）
- [ ] 5.4 修复断裂：requests status 取值域对齐 RequestStatus proto enum（17 值）；approval status 对齐 proto（expired 统一不用 timeout）；plan_artifacts status 对齐 proto ArtifactStatus
- [ ] 5.5 测试：FK 链（requests→catalog_items/teams/bundles/plan_artifacts；approval_runs→requests；node_runs→runs；decisions→node_runs）+ status CHECK + 乐观锁 version

## 06-cloud 域表

- [ ] 6.1 `migrations/008_cloud.sql`：cloud_accounts（补 status + regions_json）
- [ ] 6.2 测试：provider CHECK（对齐 proto CloudProvider 5 值）+ status CHECK（active/suspended）

## 07-审计/事件表

- [ ] 7.1 `migrations/009_audit.sql`：audit_logs（append-only）+ outbox_events（Saga）
- [ ] 7.2 测试：audit_logs append-only（无 update/delete）+ outbox status CHECK（pending/processed/failed）

## 08-分层表（Phase 1 seed）

- [ ] 8.1 `migrations/010_layers.sql`：layer_logical_refs + layer_rule_set_versions
- [ ] 8.2 出厂 seed：3 条 layer_logical_refs（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active is_default）
- [ ] 8.3 测试：seed 幂等（重复迁移不重复插入）+ layer_rule_set_versions status CHECK

## 09-sqlc 查询 + 重新生成

- [ ] 9.1 修正 `server/pkg/db/sqlc.yaml`：schema 指向真实 migrations（不再用 queries/schema.sql 手维护副本）
- [ ] 9.2 按 MVP 表写 queries（teams/projects/bundles/modules/module_versions/module_dependencies/catalog_items/requests/request_events/plan_artifacts/gate_results/approval_runs/approval_node_runs/approval_decisions/cloud_accounts/audit_logs/outbox_events/layer_logical_refs/layer_rule_set_versions）
- [ ] 9.3 `make sqlc-gen` 重新生成；确认生成 struct 含全列 + emit_pointers_for_null_types

## 10-验收

- [ ] 10.1 `goose up && goose down && goose up` 全部幂等
- [ ] 10.2 所有业务表有 created_at/updated_at + updated_at trigger
- [ ] 10.3 所有 FK 显式 REFERENCES；所有枚举有 CHECK；JSONB 字段标类型
- [ ] 10.4 proto 实体字段在表里有对应列（零映射）
- [ ] 10.5 `make build && make test` 全绿
- [ ] 10.6 docs/04 标注：MVP 表指向本 change 的实际 schema，断裂点标记已修复

## 11-后续打磨（本 change 之后）

> 初版推导完成后，随 Wave 实现逐步打磨。不在本 change 范围。

- [ ] 11.1 stacks/stack_dependencies（Wave 2 codegen 后，补 layer_logical_id 闭合动态分层链路）
- [ ] 11.2 drift/CMDB/FinOps/CICD/import 表（Wave 4-5）
- [ ] 11.3 动态分层扩展：layer_rule_set CRUD + MigrationPlanner（Phase 2）
- [ ] 11.4 docs/04 C 级规范瑕疵全量修复（随各表落地）
