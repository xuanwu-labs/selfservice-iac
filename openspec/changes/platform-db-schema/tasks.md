# Tasks: platform-db-schema

> **生命周期**：本 change 长存——MVP 表先落，非 MVP 表随 Wave 落，字段调整回写 design.md，直到 DB 结构稳定才由 maintainer 发起 archive。归档条件 = DB 冻结（不是 MVP 完成）。

## 01-规范基线 + 雪花 ID 工具类

### 1A-设计文档（已完成，design.md 已固化）

- [x] 1.5 design.md §01 固化命名规范（雪花主键/外键/约束/枚举/审计/JSON）
- [x] 1.6 design.md §1.6 数据类型规范（TEXT/TIMESTAMPTZ/BIGINT cents/JSONB + 禁用清单）
- [x] 1.7 design.md §1.3 FK 索引硬规则
- [x] 1.8 design.md §2.1 软删除 partial unique index + 保留 deleted_at 偏离 docs 论证
- [x] 1.9 design.md §1.2 snowflake 偏离 skill IDENTITY 默认的论证
- [x] 1.10 design.md §1.7 索引规范（GIN/partial/复合列序/fillfactor）

### 1B-实现任务（apply 阶段执行，未开始）

- [ ] 1.1 实现 `server/internal/utils/snowflake.go`：雪花 ID 生成器（参考 ferret internal/utils/snowflake.go + bwmarrin/snowflake 算法）。提供 `Init(machineID, datacenterID)` 启动初始化 + `GenerateID() int64` 生成 ID。含单测（唯一性/时间有序/并发安全/时钟回拨）。
- [ ] 1.2 `server/internal/config/config.go` 加 `Snowflake{MachineID, DatacenterID}` 配置项；`server/config.yaml` 加默认值（0/0）；wire 启动时调 `utils.Init()`。
- [ ] 1.3 创建 `server/cmd/migrate/migrations/000_utils.sql`：`set_updated_at()` trigger 函数（通用，所有业务表挂）。
- [ ] 1.4 重写 `001_init.sql`：teams 表对齐新规范（`id BIGINT PK`——应用层雪花生成、补 `kind`/`status`/`tags_json`/`policy_json`/`deleted_at`、updated_at trigger、约束命名 `pk_/uq_` 前缀、slug partial unique index `WHERE deleted_at IS NULL`）。

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

### 基础验收
- [ ] 10.1 `goose up && goose down && goose up` 全部幂等
- [ ] 10.2 所有业务表有 created_at/updated_at + updated_at trigger；append-only 表只有 occurred_at/created_at
- [ ] 10.3 所有 FK 显式 REFERENCES；所有枚举有 CHECK；JSONB 字段标类型
- [ ] 10.4 proto 实体字段在表里有对应列（零映射）；ID 字段 proto string ↔ DB BIGINT（雪花 int64 ↔ string 传输）
- [ ] 10.5 所有表的 INSERT 在应用层调 `utils.GenerateID()` 生成 ID（DB 列无 DEFAULT 自增）
- [ ] 10.6 `make build && make test` 全绿（含 snowflake 单测）
- [ ] 10.7 docs/04 标注：MVP 表指向本 change 的实际 schema，断裂点标记已修复

### skill 对账（postgresql-table-design）
- [ ] 10.8 全表零 `VARCHAR(n)`/`CHAR(n)`（grep 迁移 SQL 应无命中）；字符串一律 TEXT
- [ ] 10.9 全表零 `serial`/`bigserial`/`money`/`TIMESTAMP`(无 tz)/`timestamptz(n)`
- [ ] 10.10 每个 FK 列都有对应 `ix_<table>_<fk_col>` 索引（grep 迁移 SQL 校验 FK 列名 vs 索引名）
- [ ] 10.11 软删除表的唯一约束是 partial unique index（`WHERE deleted_at IS NULL`），无 `UNIQUE(..., deleted_at)` 旧写法
- [ ] 10.12 JSONB 高频过滤列（visibility_json/user_allowed_tag_keys_json/tags_json/env_scope_json）有 GIN 索引

### docs 全量对账（audit-docs-sweep.md）
- [ ] 10.13 **requests.status CHECK 覆盖 19 值**（doc 00 §5 + doc 12a：含 blocked-policy/blocked-state-health/paused-drift/reconcile-pending）
- [ ] 10.14 **plan_artifacts 含版本校验全字段**（pinned_commit/toolchain_profile_hash/provider_lock_hash/tf_version_sha256/stack_id/state_key/sha256/size_bytes）
- [ ] 10.15 **approval_flows 表已建**（doc 12 §6 审批流 DSL 持久化）；approval_runs.gate CHECK 3 值
- [ ] 10.16 **outbox_events.status CHECK 5 值**（pending/processing/succeeded/failed/dead-letter）+ event_id UNIQUE
- [ ] 10.17 **requests 含 kind/resolved_params_json/retry_count/version**（doc 08/12/13/15）
- [ ] 10.18 **cloud_accounts 对齐 doc 04 §2.12 权威**（alias/display_name/billing_enabled/default_team_id/tags_json/bootstrap_status）
- [ ] 10.19 **catalog_items.status CHECK 5 值**（draft/active/deprecated/archived/blocked，doc 19 §1）
- [ ] 10.20 **resources 含 tenant_id + status + resource_relations 表**（doc 07 §7 + doc 14 §2）
- [ ] 10.21 **finops_recommendations 含 confidence + 4 支撑字段**（doc 14 §4.5）
- [ ] 10.22 **executor_runs 含 failure_category**（doc 18 §4 Phase 1 验收门）
- [ ] 10.23 **11 张丢失表已纳入**（identity_sources/org_nodes/org_mappings/sync_runs/toolchain_manifest/tag_policy_versions/ai_prompts/skills/service_accounts/service_account_keys/signing_keys/runbooks）
- [ ] 10.24 **drift_records.resolution 含 mark-failed-terminal**（doc 21 RB-001）
- [ ] 10.25 **import_jobs 含 exemption 3 列 + lifecycle/status CHECK**（doc 15 §3.1）
- [ ] 10.26 **gate_events.status 不反向扩展 requests.status**（doc 16 §3.1.1 投影语义）
- [ ] 10.27 **两套幂等公式独立**：requests.idempotency_key（actor+catalog+form_hash+24h）vs cicd_triggers.idempotency_key（pipeline:commit:catalogItem:form_hash）

## 11-非 MVP 表设计定稿（本 change 内，不落迁移）

> 这些表的**字段清单已推导**（design.md §03 B1-B14），但不写迁移 SQL——等对应 Wave 实现功能时按定稿直接落迁移。
>
> **当前状态：字段清单级定稿，未完成 skill 对账**。skill 整改后 §01-02 规范变了（TEXT/FK 索引/软删 partial index/cents/JSONB GIN 取舍），B1-B14 需重新对账才算「可按规范直接落迁移」的真定稿。对账策略：**跟着对应 Wave 走**（Wave 2 落 stacks 时对账 B2，Wave 4 落 drift 时对账 B3…），符合「随实现逐步打磨」原则。

- [ ] 11.1 B1 执行与工作仓库表定稿（executor_runs/workspaces/workspace_checkouts）——含 executor_runs.artifact_id → plan_artifacts 修复 A1 矛盾。**待 skill 对账**（类型明确/FK 索引/枚举 CHECK）
- [ ] 11.2 B2 stack 注册表定稿（stacks/stack_dependencies）——含 stacks.layer_logical_id 修复 A6 断裂。**待 skill 对账**
- [ ] 11.3 B3 漂移检测表定稿（drift_runs/drift_records）。**待 skill 对账**
- [ ] 11.4 B4 鉴权身份表定稿（oidc_providers/identities/sessions/role_bindings/emergency_runs/sensitive_field_blacklist）。**待 skill 对账**
- [ ] 11.5 B5 适配器配置表定稿（adapters_config）。**待 skill 对账**
- [ ] 11.6 B6 Saga 补偿表定稿（reconcile_jobs/manual_intervention_tasks）。**待 skill 对账**
- [ ] 11.7 B7 Run Hooks/运营表定稿（run_hooks/run_hook_results/incidents/runbook_executions/drill_results/platform_scorecards/catalog_health_checks）。**待 skill 对账**
- [ ] 11.8 B8 分层迁移表定稿（stack_grouping_rules/layer_migrations）。**待 skill 对账**
- [ ] 11.9 B9 CMDB/FinOps 表定稿（resources/cost_records/cost_budgets/finops_recommendations）。**待 skill 对账**
- [ ] 11.10 B10 云凭据表定稿（team_cloud_grants/cloud_credentials/catalog_items_required_permissions/iam_role_templates）。**待 skill 对账**
- [ ] 11.11 B11 环境租户表定稿（environments/tenants/environment_tenant_bindings）。**待 skill 对账**
- [ ] 11.12 B12 标签策略表定稿（tag_policies）。**待 skill 对账**
- [ ] 11.13 B13 CICD 表定稿（cicd_triggers/gate_subscriptions/gate_events）。**待 skill 对账**
- [ ] 11.14 B14 存量导入表定稿（import_jobs/import_resources）。**待 skill 对账**

## 12-后续打磨（本 change 之后，随 Wave 实现逐步完善）

- [ ] 12.1 各 Wave 落 B 级表迁移时，按 design.md §03 定稿直接建表（不重新设计）
- [ ] 12.2 实现功能中发现字段需求变化，回头改 design.md §03 定稿 + 迁移（正常演进）
- [ ] 12.3 docs/04 C 级规范瑕疵随各表落地时修复（docs/04 标注指向本 change 定稿为权威）
- [ ] 12.4 动态分层扩展：layer_rule_set CRUD + MigrationPlanner（Phase 2，属 04 模块范畴）
