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

### 1B-实现任务（apply 阶段）

- [x] 1.1 实现 `server/internal/utils/snowflake.go`：雪花 ID 生成器（参考 ferret internal/utils/snowflake.go + bwmarrin/snowflake 算法）。提供 `Init(machineID, datacenterID)` 启动初始化 + `GenerateID() int64` 生成 ID。含单测（唯一性/时间有序/并发安全/时钟回拨）。
- [x] 1.2 `server/internal/config/config.go` 加 `Snowflake{MachineID, DatacenterID}` 配置项；`server/config.yaml` 加默认值（0/0）；wire 启动时调 `utils.Init()`。
- [x] 1.3 创建 `server/cmd/migrate/migrations/000_utils.sql`：`set_updated_at()` trigger 函数（通用，所有业务表挂）。
- [x] 1.4 重写 `001_init.sql`：teams 表对齐新规范（`id BIGINT PK`——应用层雪花生成、补 `kind`/`status`/`tags_json`/`policy_json`/`deleted_at`、updated_at trigger、约束命名 `pk_/uq_` 前缀、slug partial unique index `WHERE deleted_at IS NULL`）。

## 02-组织归属表

- [x] 2.1 `migrations/002_organizations.sql`：projects + bundles（layer_logical_id FK 由 010 backfill）
- [ ] 2.2 测试：迁移 up/down 幂等 + FK 约束（删 team 有 project 引用时 RESTRICT）+ slug UNIQUE（**需 Docker 跑 testcontainers**）

## 03-registry 域表（模块注册）

- [x] 3.1 `migrations/003_registry.sql`：modules + module_versions + module_dependencies
- [x] 3.2 修复 docs/04 断裂：modules 补 name/provider/description；module_versions 补 version/providers_json + 接收 variables_contract_json（从 modules 移入）；新增 module_dependencies 表
- [ ] 3.3 测试：FK + status CHECK（**需 Docker 跑 testcontainers**）

## 04-catalog 域表（服务目录）

- [x] 4.1 `migrations/004_catalog.sql`：catalog_items（layer_logical_id FK 由 010 backfill）
- [x] 4.2 修复断裂：补 category 列；visibility_json + user_allowed_tag_keys_json 建 GIN 索引；status 5 值 CHECK
- [ ] 4.3 测试：FK + status CHECK + cardinality CHECK（**需 Docker 跑 testcontainers**）

## 05-lifecycle 域表（工单+审批+artifact）

- [x] 5.1 `migrations/005_lifecycle_requests.sql`：requests（19 值 status CHECK + kind/source/retry_count/version/resolved_params_json/idempotency_key）+ request_events（append-only）
- [x] 5.2 `migrations/006_lifecycle_plan.sql`：plan_artifacts（8 版本校验字段）+ gate_results + backfill requests.plan_artifact_id FK
- [x] 5.3 `migrations/007_lifecycle_approval.sql`：approval_flows + approval_runs（gate 3 值 + req/gate partial unique）+ approval_node_runs + approval_decisions
- [x] 5.4 修复断裂：requests status 19 值；approval gate/mode/decision/status 全 CHECK；plan_artifacts status 4 值
- [ ] 5.5 测试：FK 链 + status CHECK + 乐观锁（**需 Docker 跑 testcontainers**）

## 06-cloud 域表

- [x] 6.1 `migrations/008_cloud.sql`：cloud_accounts（doc 04 §2.12 权威全字段 + status 4 值 cascade）
- [ ] 6.2 测试：provider CHECK + status CHECK（**需 Docker 跑 testcontainers**）

## 07-审计/事件表

- [x] 7.1 `migrations/009_audit.sql`：audit_logs（ai_metadata_json）+ outbox_events（5 值 + event_id UNIQUE）
- [ ] 7.2 测试：audit_logs append-only + outbox status CHECK（**需 Docker 跑 testcontainers**）

## 08-分层表（Phase 1 seed）

- [x] 8.1 `migrations/010_layers.sql`：layer_logical_refs + layer_rule_set_versions + backfill FK（bundles/catalog_items/requests）
- [x] 8.2 出厂 seed：3 条 layer_logical_refs（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active is_default，ON CONFLICT 幂等）
- [ ] 8.3 测试：seed 幂等（重复迁移不重复插入）+ status CHECK（**需 Docker 跑 testcontainers**）

## 09-sqlc 查询 + 重新生成

- [x] 9.1 `queries/schema.sql` 同步全 MVP 表 DDL（topological 排序 + 循环 FK 用 ALTER 解决；注释说明为何用镜像而非直指 migrations——sqlc 解析 goose Up+Down 注释会冲突）
- [x] 9.2 queries/teams.sql 适配新规范（snowflake ID + 全字段 + 软删过滤）；其余 MVP 表 queries 随对应 Wave 实现写（本 change 只落 teams 作为模式范例）
- [x] 9.3 `sqlc generate` 成功；生成 20 表 model + teams 查询全字段 + emit_pointers_for_null_types

## 10-验收

### 基础验收
- [x] 10.1 `goose up && goose down && goose up` 全部幂等（✅ embedded-postgres 验证通过：10 迁移 Up→Down→Up，20 表建完，3 层 seed，trigger 函数）
- [x] 10.2 所有业务表有 created_at/updated_at + updated_at trigger；append-only 表只有 occurred_at/created_at（grep 迁移确认）
- [x] 10.3 所有 FK 显式 REFERENCES；所有枚举有 CHECK；JSONB 字段标类型（grep 确认 23 FK / 全枚举有 CHECK）
- [x] 10.4 proto 实体字段在表里有对应列（零映射）；ID 字段 proto string ↔ DB BIGINT（雪花 int64 ↔ string 传输）
- [x] 10.5 所有表的 INSERT 在应用层调 `utils.GenerateID()` 生成 ID（DB 列无 DEFAULT/serial 自增）
- [x] 10.6 `go build ./... && go vet ./...` 全绿；snowflake 单测全绿（config/utils/errors 通过）
- [ ] 10.7 docs/04 标注：MVP 表指向本 change 的实际 schema，断裂点标记已修复（**待 docs 标注**）

### skill 对账（postgresql-table-design）
- [x] 10.8 全表零 `VARCHAR(n)`/`CHAR(n)`（迁移 SQL grep 确认，仅注释提及）；字符串一律 TEXT
- [x] 10.9 全表零 `serial`/`bigserial`/`money`/`TIMESTAMP`(无 tz)/`timestamptz(n)`（grep 确认）
- [x] 10.10 每个 FK 列都有对应 `ix_<table>_<fk_col>` 索引（23 FK / 65 ix_ 索引覆盖）
- [x] 10.11 软删除表的唯一约束是 partial unique index（`WHERE deleted_at IS NULL`），无 `UNIQUE(..., deleted_at)` 旧写法
- [x] 10.12 JSONB 高频过滤列（visibility_json/user_allowed_tag_keys_json）有 GIN 索引（MVP 范围）

### docs 全量对账（audit-docs-sweep.md）
- [x] 10.13 **requests.status CHECK 覆盖 19 值**（迁移 005 确认，含 blocked-policy/blocked-state-health/paused-drift/reconcile-pending）
- [x] 10.14 **plan_artifacts 含版本校验全字段**（迁移 006 确认 8 字段全在）
- [x] 10.15 **approval_flows 表已建**（迁移 007）；approval_runs.gate CHECK 3 值
- [x] 10.16 **outbox_events.status CHECK 5 值**（迁移 009 确认）+ event_id UNIQUE
- [x] 10.17 **requests 含 kind/resolved_params_json/retry_count/version**（迁移 005 确认）
- [x] 10.18 **cloud_accounts 对齐 doc 04 §2.12 权威**（迁移 008 确认全字段）
- [x] 10.19 **catalog_items.status CHECK 5 值**（迁移 004 确认，含 blocked）
- [x] 10.20 resources 含 tenant_id + status + resource_relations（design.md B9 定稿，非 MVP 不落迁移）
- [x] 10.21 finops_recommendations 含 confidence + 4 支撑字段（design.md B9 定稿）
- [x] 10.22 executor_runs 含 failure_category（design.md B1 定稿）
- [x] 10.23 11 张丢失表已纳入 design.md（B1/B4/B7/B12/B15 段）
- [x] 10.24 drift_records.resolution 含 mark-failed-terminal（design.md B3 定稿）
- [x] 10.25 import_jobs 含 exemption 3 列 + lifecycle/status CHECK（design.md B14 定稿）
- [x] 10.26 gate_events.status 不反向扩展 requests.status（design.md B13 独立枚举）
- [x] 10.27 两套幂等公式独立（requests.idempotency_key vs cicd_triggers.idempotency_key，迁移 005/design B13）

### 链路完整性对账（audit-link-completeness.md）
- [x] 10.28 **approval_runs 有 (request_id, gate) partial unique**（迁移 007 确认 uq_approval_runs_req_gate_pending）
- [x] 10.29 drift_records.remediation_request_id（design.md B3 定稿）
- [x] 10.30 import_jobs.created_stack_id（design.md B14 定稿）
- [x] 10.31 MVP requester_id/env_id/tenant_id 标注为悬挂字符串（迁移 005 + design.md A4 注释）

> **未勾选项**（10.1 + 各域测试 2.2/3.3/4.3/5.5/6.2/7.2/8.3 + 10.7）需 Docker（testcontainers）跑迁移幂等 + CRUD 测试，或 docs/04 标注。代码侧已就绪，待 CI/Docker 环境执行。

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
