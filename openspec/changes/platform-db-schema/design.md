# Design: platform-db-schema

## 01-命名规范基线（对标业界 + proto 零映射）

### 1.1 总则

| 规范项 | 决策 | 依据 |
|---|---|---|
| 大小写 | `snake_case`（PG 折叠小写） | PostgreSQL 共识 |
| 表名 | 复数名词（`teams`, `module_versions`） | 业界共识 |
| 列名 | 单数（`name`, `created_at`） | 业界共识 |
| **proto 字段名 = DB 列名** | 零映射（`LifecycleRequest.team_id` → `requests.team_id`） | D1 精神延伸：proto 是唯一源，DB 同名 |
| 内部字段后缀 | `_json`（JSONB）、`_at`（时间戳）、`_id`（外键） | 明确区分 proto 暴露 vs 内部 |

### 1.2 主键策略

**全库统一 `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`**（PostgreSQL 13+ 内置 `gen_random_uuid()` 产出 UUIDv4）。

理由：
- 控制面 DB 写入量低（元数据，非日志），UUID 的 B-Tree 开销可接受。
- 全局唯一，未来分库/分表无迁移成本（bigserial 跨库要协调 sequence）。
- 不暴露增量信息（bigserial 暴露业务量）。
- 业界 2025 共识（Supabase/pganalyze）推荐 UUIDv7/v4 用于控制面。

**现有 teams 表（BIGSERIAL）需重写为 UUID**——但 teams 尚无业务数据（脚手架阶段），重写无成本。

### 1.3 外键策略

显式 FK 约束，命名 `fk_<table>_<col>`：
```sql
team_id UUID NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
```
- 控制面 DB 强制引用完整性（`ON DELETE RESTRICT` 防误删，不用 CASCADE 防连锁）。
- 逻辑引用但无 FK 是 docs/04 的缺陷，本 change 全部补 FK。

### 1.4 约束命名

| 约束类型 | 前缀 | 示例 |
|---|---|---|
| Primary Key | `pk_<table>` | `pk_teams` |
| Foreign Key | `fk_<table>_<col>` | `fk_requests_team_id` |
| Unique | `uq_<table>_<cols>` | `uq_teams_slug` |
| Check | `ck_<table>_<col>` | `ck_requests_status` |
| Index | `ix_<table>_<col>` | `ix_requests_status` |

### 1.5 枚举值策略

`VARCHAR(32) NOT NULL CHECK (xxx IN (...))`，取值域与 proto enum 对齐：
```sql
status VARCHAR(32) NOT NULL CHECK (status IN (
    'submitted', 'generating', 'pending_admission', ...
)) DEFAULT 'submitted',
```
- 不用 PostgreSQL 原生 ENUM 类型（加值要 `ALTER TYPE`，迁移麻烦）。
- 用 VARCHAR + CHECK（加值改 CHECK 约束，迁移友好）。
- snake_case 取值（proto enum 值剥前缀后 lowercase + snake）。

## 02-审计属性规范（sqlc/pgx 集成）

### 2.1 统一审计字段

**所有业务表**（非 append-only）必须有：
```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
```

**append-only 表**（request_events, audit_logs, approval_decisions）只有：
```sql
occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 或 created_at
```

**软删除**（可选，按表需要）：
```sql
deleted_at TIMESTAMPTZ NULL,
```
UNIQUE 约束含 `deleted_at` 以允许软删后重建：
```sql
CONSTRAINT uq_stacks_path UNIQUE (bundle_id, component, env, deleted_at)
```

### 2.2 updated_at 自动维护（trigger）

sqlc 不自动管 `updated_at`（不像 GORM/sqlboiler 有 hook）。业界做法（Brandur/sqlc 共识）：**PG trigger 自动维护**，不让应用层操心。

```sql
-- 一次性创建通用 trigger 函数（migrations/001_init.sql 或独立 000_utils.sql）
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 每张业务表挂 trigger
CREATE TRIGGER trg_<table>_updated_at
    BEFORE UPDATE ON <table>
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

这样应用层（sqlc query）UPDATE 时不需传 `updated_at`，trigger 自动填。与 sqlc 的"SQL 即真理"哲学一致——DB 侧保证，不靠应用层。

### 2.3 sqlc 集成

现有 sqlc 配置（`server/pkg/db/sqlc.yaml`）已用 pgx/v5 + emit_pointers_for_null_types。新增：
- `schema:` 指向真实迁移 SQL（不再用 `queries/schema.sql` 手维护副本）。
- 查询按表分文件（`queries/teams.sql`, `queries/requests.sql`...）。

sqlc 生成的 struct 自动含 created_at/updated_at（timestamptz → `time.Time` / `*time.Time`），与 proto 的 `string`（RFC3339）在 mapping 层转换（`internal/mapping/`）。

## 03-MVP 表清单（初版，约 18-20 张）

### 组织归属（3 张）
```
teams(id, name, slug, kind, tags_json, policy_json, created_at, updated_at, deleted_at)
projects(id, name, team_id FK, created_at, updated_at)
bundles(id, name, project_id FK, layer_logical_id FK, repo_path, tags_json, created_at, updated_at)
```

### 模块注册（3 张）
```
modules(id, name, git_source, provider, layer, owner_team_id FK, status, description,
        created_at, updated_at)
module_versions(id, module_id FK, version, commit_sha, providers_json, 
                variables_contract_json, registered_at)
module_dependencies(id, module_version_id FK, variable_name, depends_on_layer,
                    depends_on_module, output_key, required, description)
```
**修复 docs/04 断裂**：
- modules 补 `name`（proto）+ `provider`（proto）+ `description`（proto）
- module_versions 补 `version`（proto，语义版本字符串）+ `providers_json`（proto repeated）+ `variables_contract_json`（从 modules 移到 module_versions，proto 归属版本）
- 新增 `module_dependencies` 表（proto 有 ModuleDependency，docs/04 无表）

### 服务目录（1 张）
```
catalog_items(id, module_version_id FK, display_name, description, category, status,
              form_schema_json, defaults_json, cardinality, instance_key,
              per_instance_fields_json, shared_fields_json,
              layer_logical_id FK, stack_grouping, owner_team_id FK,
              default_tags_json, user_allowed_tag_keys_json, visibility_json,
              created_at, updated_at, deleted_at)
```
**修复 docs/04 断裂**：补 `category`（proto CatalogItem 有）

### 工单生命周期（4 张）
```
requests(id, catalog_item_id FK, bundle_id FK, env_id, tenant_id, team_id, requester_id,
         status, current_stage, source, form_values_json, form_hash, idempotency_key,
         pinned_commit, plan_artifact_id FK, cost_estimate_cents, cost_currency,
         correlation_id, version, layer_rule_set_version_id FK,
         created_at, updated_at)
request_events(id, request_id FK, event_type, stage, from_status, to_status,
               actor_id, actor_type, message, correlation_id, occurred_at)
plan_artifacts(id, request_id FK, status, plan_hash, storage_uri,
               resources_to_add, resources_to_change, resources_to_destroy,
               cost_estimate_cents, expires_at, created_at)
gate_results(id, request_id FK, gate_id, passed, policy, message, severity, evaluated_at)
```
**修复 docs/04 A 级断裂**：
- requests 补 `team_id`/`source`/`cost_estimate_cents`/`cost_currency`/`correlation_id`/`plan_artifact_id`（全部 proto 有而表缺）
- `plan_artifacts` 独立成表（解决"并入 executor_runs"的三处矛盾；D21 plan-apply 解耦的存储基础）
- `gate_results` 独立成表（proto GateResult 实体）

### 审批（3 张）
```
approval_runs(id, request_id FK, flow_id, gate, current_node, status,
               decided_by, decided_at, started_at, finished_at, expires_at)
approval_node_runs(id, run_id FK, node_id, mode, decided_count, required_count,
                   status, timeout_at)
approval_decisions(id, node_run_id FK, approver_id, decision, comment, decided_at)
```
**修复 docs/04 B 级断裂**：approval_runs 补 `decided_by`/`decided_at`/`expires_at`（proto 有而表缺）；status 取值域对齐 proto enum（`expired` 统一，不用 `timeout`）。

### 云账号（1 张）
```
cloud_accounts(id, provider, account_id, name, status, regions_json, 
               default_region, credentials_ref, bootstrap_status, oidc_trust_configured,
               created_at, updated_at)
```
**修复 docs/04 断裂**：补 `status`（proto CloudAccountStatus）+ `regions_json`（proto repeated regions，DB 存 JSONB 数组）。

### 分层（2 张，Phase 1 只 seed）
```
layer_logical_refs(logical_id UUID PK, current_display_name, notes, created_at)
layer_rule_set_versions(version_id INT PK, layers_json, status, is_default,
                        created_at, created_by, superseded_at, superseded_by)
```
Phase 1 出厂 seed：3 条 layer_logical_refs（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active）。

### 审计/事件（2 张）
```
audit_logs(id, actor_id, actor_type, action, target_type, target_id,
           before_json, after_json, correlation_id, occurred_at)
outbox_events(id, aggregate_type, aggregate_id, event_type, payload_json, 
              status, retry_count, created_at, processed_at)
```

## 04-proto 断裂修复方案（A 级优先）

按审计报告 A 级断裂逐项修复：

| 断裂 | 修复 |
|---|---|
| A1 PlanArtifact 三处矛盾 | 独立 `plan_artifacts` 表（不并入 executor_runs），MVP 只建这张 |
| A2 requests 缺 cost 字段 | 补 `cost_estimate_cents` BIGINT + `cost_currency` VARCHAR(8) |
| A3 requests 缺 team_id | 补 `team_id` UUID FK |
| A4 requests 缺 correlation_id | 补 `correlation_id` VARCHAR(64) |
| A5 requests 缺 source | 补 `source` VARCHAR(32) CHECK |
| A6 stacks 无 layer_logical_id | MVP 不建 stacks（Wave 2 codegen 后建），届时补；layer_logical_refs 先建供 catalog 引用 |
| A7 CatalogItem proto 暴露少 | 本 change 只管表结构（表有全部列）；proto 暴露字段调整是 proto change 的事 |
| A8 module_versions 缺 dependencies | 新增 `module_dependencies` 表 |
| A9 modules 缺 provider | 补 `provider` 列 |
| A10 variables_contract_json 归属 | 移到 `module_versions`（proto 归属版本） |
| A11 layer 表不在 Phase 1 清单 | 本 change 纳入 `layer_logical_refs` + `layer_rule_set_versions`（Phase 1 seed 用） |

## 05-Phase 推进策略

**本 change = 初版推导**，不追求一步到位。分步：

1. **规范基线**（000_utils.sql）：trigger 函数 + 命名约定文档
2. **重写 teams**：对齐新规范（UUID 主键 + 审计字段统一）
3. **MVP 核心表**：按域分批（组织→registry→catalog→lifecycle→cloud→layer→审计）
4. **sqlc 查询**：MVP 表的 CRUD + 业务查询
5. **测试**：迁移 up/down 幂等 + CRUD + 约束验证

**后续打磨**（本 change 之后，随 Wave 实现）：
- stacks/stack_dependencies（Wave 2）
- drift/CMDB/FinOps/CICD 表（Wave 4-5）
- 动态分层扩展（layer_rule_set CRUD + MigrationPlanner，Phase 2）
- docs/04 全量规范化（C 级规范瑕疵随各表落地时修复）

## 06-验证标准

- [ ] 所有 MVP 表遵循命名规范（snake_case/复数表名/约束前缀）。
- [ ] 所有业务表有 created_at/updated_at + updated_at trigger。
- [ ] 所有 FK 显式 REFERENCES。
- [ ] 所有枚举列有 CHECK 约束，取值域与 proto enum 对齐。
- [ ] JSONB 字段标 jsonb 类型；高频查询建 GIN 索引。
- [ ] proto 实体字段在表里有对应列（零映射）。
- [ ] `goose up && goose down && goose up` 幂等。
- [ ] sqlc generate 成功，生成 struct 含全部列。
- [ ] `make build && make test` 全绿。
