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

## 03-全量表清单（初版设计全部 ~52 张，分优先级实现）

> **原则**：所有表都在本 change **推导初版**（字段/类型/约束/FK 全规范化），不是只设计 MVP 子集。区别只在实现顺序——MVP 表先落迁移 SQL + sqlc 查询，非 MVP 表设计已就绪、迁移 SQL 随对应 Wave 实现时再落。这样后续 Wave 不用回头重新设计表结构，只需按已定稿的 schema 落迁移。

### 优先级 A：MVP 主链路表（本 change 落迁移 + sqlc）

#### A1. 组织归属（3 张）
```
teams(id, name, slug, kind[platform|dba|middleware|business], tags_json, policy_json,
      created_at, updated_at, deleted_at)
projects(id, name, team_id FK→teams, created_at, updated_at)
bundles(id, name, project_id FK→projects, layer_logical_id FK→layer_logical_refs,
        repo_path, tags_json, created_at, updated_at)
```

#### A2. 模块注册（3 张）
```
modules(id, name, git_source, provider, layer, owner_team_id FK→teams, status, description,
        created_at, updated_at)
module_versions(id, module_id FK→modules, version, commit_sha, providers_json,
                variables_contract_json, registered_at)
module_dependencies(id, module_version_id FK→module_versions, variable_name,
                    depends_on_layer, depends_on_module, output_key, required, description)
```
**修复断裂**：modules 补 name/provider/description（proto 有）；module_versions 补 version/providers_json + 接收 variables_contract_json（从 modules 移入，proto 归属版本）；新增 module_dependencies 表（proto 有 ModuleDependency，docs/04 无表）。

#### A3. 服务目录（1 张）
```
catalog_items(id, module_version_id FK→module_versions, display_name, description, category,
              status, form_schema_json, defaults_json, cardinality[single|list|map],
              instance_key, per_instance_fields_json, shared_fields_json,
              layer_logical_id FK→layer_logical_refs, stack_grouping, owner_team_id FK→teams,
              default_tags_json, user_allowed_tag_keys_json, visibility_json,
              created_at, updated_at, deleted_at)
```
**修复断裂**：补 category（proto CatalogItem 有）。visibility_json 建 GIN 索引（按团队过滤目录高频查询）。

#### A4. 工单生命周期（4 张）
```
requests(id, catalog_item_id FK→catalog_items, bundle_id FK→bundles, env_id, tenant_id,
         team_id FK→teams, requester_id, status, current_stage, source,
         form_values_json, form_hash, idempotency_key, pinned_commit,
         plan_artifact_id FK→plan_artifacts, cost_estimate_cents, cost_currency,
         correlation_id, version, layer_rule_set_version_id FK→layer_rule_set_versions,
         created_at, updated_at)
request_events(id, request_id FK→requests, event_type, stage, from_status, to_status,
               actor_id, actor_type, message, correlation_id, occurred_at)  -- append-only
plan_artifacts(id, request_id FK→requests, status, plan_hash, storage_uri,
               resources_to_add, resources_to_change, resources_to_destroy,
               cost_estimate_cents, expires_at, created_at)
gate_results(id, request_id FK→requests, gate_id, passed, policy, message, severity,
             evaluated_at)
```
**修复 A 级断裂**：requests 补 team_id/source/cost_estimate_cents/cost_currency/correlation_id/plan_artifact_id（全 proto 有而表缺）；plan_artifacts 独立成表（解决"并入 executor_runs"三处矛盾）；gate_results 独立成表（proto GateResult）。

#### A5. 审批（3 张）
```
approval_runs(id, request_id FK→requests, flow_id, gate, current_node, status,
               decided_by, decided_at, started_at, finished_at, expires_at)
approval_node_runs(id, run_id FK→approval_runs, node_id, mode, decided_count,
                   required_count, status, timeout_at)
approval_decisions(id, node_run_id FK→approval_node_runs, approver_id, decision,
                   comment, decided_at)  -- append-only
```
**修复 B 级断裂**：approval_runs 补 decided_by/decided_at/expires_at（proto 有）；status 取值域对齐 proto（expired 统一不用 timeout）。

#### A6. 云账号（1 张）
```
cloud_accounts(id, provider, account_id, name, status[active|suspended], regions_json,
               default_region, credentials_ref, bootstrap_status, oidc_trust_configured,
               created_at, updated_at)
```
**修复断裂**：补 status（proto CloudAccountStatus）+ regions_json（proto repeated regions）。

#### A7. 分层（2 张，Phase 1 只 seed）
```
layer_logical_refs(logical_id UUID PK, current_display_name, notes, created_at)
layer_rule_set_versions(version_id INT PK, layers_json, status[active|superseded|deprecated|archived],
                        is_default, created_at, created_by, superseded_at, superseded_by)
```
Phase 1 出厂 seed：3 条 layer_logical_refs（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active）。

#### A8. 审计/事件（2 张）
```
audit_logs(id, actor_id, actor_type[human|ai|system], action, target_type, target_id,
           before_json, after_json, correlation_id, occurred_at)  -- append-only
outbox_events(id, aggregate_type, aggregate_id, event_type, payload_json,
              status[pending|processed|failed], retry_count, created_at, processed_at)
```

### 优先级 B：非 MVP 表（本 change 设计定稿，迁移随 Wave 落）

> 这些表**字段/类型/约束/FK 本 change 全部推导定稿**，但不写迁移 SQL——等对应 Wave 实现功能时直接按定稿 schema 落迁移。设计依据来自 docs/04 §2.1-2.16 全部读完 + proto 契约对齐。

#### B1. 执行与工作仓库（Wave 2-3 实现）
```
executor_runs(id, request_id FK→requests, phase[plan|apply|drift|import], pinned_commit,
              artifact_id FK→plan_artifacts nullable, toolchain_profile_hash, provider_lock_hash,
              credential_session_id nullable, exit_code, started_at, finished_at,
              status[running|succeeded|failed|interrupted])
workspaces(id, name, remote_url, default_branch, created_at)
workspace_checkouts(id, workspace_id FK→workspaces, node_id, worktree_path, branch,
                    pinned_commit, purpose, leased_by_request_id FK→requests nullable,
                    leased_until, status[active|released|stale], updated_at)
```
注意：executor_runs.artifact_id → plan_artifacts（非"并入"——plan_artifacts 独立，executor_runs 引用它）。这解决了 docs/04 A1 三处矛盾。

#### B2. stack 注册表（Wave 2 codegen 后实现）
```
stacks(id, bundle_id FK→bundles nullable, layer, component, env, tenant_id,
       stack_id, repo_path, state_key, terramate_tags_json, owner_team_id FK→teams,
       catalog_item_id FK→catalog_items,
       layer_logical_id FK→layer_logical_refs,          -- ← 修复 A6：补此列闭合动态分层链路
       layer_rule_set_version_id FK→layer_rule_set_versions,
       migration_status[stable|migration_pending|migrating|deprecated], sunset_deadline,
       version, created_at)
stack_dependencies(id, from_stack_id FK→stacks, to_stack_id FK→stacks,
                   kind[remote_state|data_source|watch_only], output_key, inject_as,
                   status[active|deprecated], created_at)
```
**修复 A6**：stacks 补 `layer_logical_id`（docs/04 §2.3 缺此列，动态分层链路在 stack 端断开）。现在 stacks 同时有 layer_logical_id（稳定身份）+ layer_rule_set_version_id（版本 pin），链路闭合。

#### B3. 漂移检测（Wave 4 实现）
```
drift_runs(id, stack_id FK→stacks, started_at, finished_at, exit_code,
           has_drift, diff_summary_json)
drift_records(id, stack_id FK→stacks, drift_run_id FK→drift_runs,
              status[open|adopted|reverted], detected_at, resolved_at, resolver_id,
              resolution[adopt-cloud|restore-desired])
```

#### B4. 鉴权与身份（Wave 2-3 实现，docs/04 §2.7）
```
oidc_providers(name PK, issuer_url, client_id, client_secret_encrypted, claim_mapping_json,
               scopes_json, bridge_mode, is_default, status[active|disabled],
               created_at, updated_at)
identities(id, external_id, display_name, email, provider_name FK→oidc_providers,
           primary_source, status[active|disabled|merged], merged_into_id FK→identities nullable,
           last_synced_at, created_at)
sessions(id, identity_id FK→identities, idp_session_id, issued_at, expires_at, revoked_at)
role_bindings(id, subject_id, role, scope_type[team|project|bundle|stack|layer], scope_id, actions_json)
emergency_runs(id, request_id FK→requests, break_glass_operator_ids_json, reason, recording_url,
               retroactive_approval_id, retroactive_deadline,
               status[active|retroactively_approved|overdue], created_at)
sensitive_field_blacklist(pattern, added_at)
```

#### B5. 适配器配置（Wave 1 实现，docs/04 §2.8）
```
adapters_config(id, adapter_type[git|state|policy|cost|notify|cloud|identity|credentials],
                name, provider, config_json, status[active|disabled], created_at, updated_at)
```

#### B6. Saga 补偿（Wave 3 实现，docs/04 §2.8a）
```
reconcile_jobs(id, aggregate_type, aggregate_id, job_type, payload_json,
               status[pending|running|succeeded|failed], retry_count, last_error,
               created_at, updated_at)
manual_intervention_tasks(id, request_id FK→requests nullable, task_type, description,
                          status[open|in_progress|resolved|cancelled], assigned_to,
                          resolution_notes, created_at, updated_at, resolved_at)
```

#### B7. 商业级 Run Hooks / 运营（Phase 2+ 实现，docs/04 §2.8b）
```
run_hooks(id, name, phase[pre-plan|post-plan|pre-apply|post-apply], target_url, auth_secret_ref,
          timeout_seconds, failure_policy[fail-open|fail-closed|warn-only], requires_credentials,
          status[active|disabled], created_at, updated_at)
run_hook_results(id, hook_id FK→run_hooks, request_id FK→requests, phase, status[running|succeeded|failed|timeout],
                 decision[allow|deny|warn], summary, details_ref, started_at, finished_at)
incidents(id, severity[p0|p1|p2|p3], title, status[open|mitigated|resolved|closed], ...)
runbook_executions(id, runbook_id, incident_id FK→incidents nullable, triggered_by, ...)
drill_results(id, drill_type, status, started_at, finished_at, ...)
platform_scorecards(id, period, category, score, metadata_json, ...)
catalog_health_checks(id, catalog_item_id FK→catalog_items, check_type, status, ...)
```

#### B8. 分层迁移（Phase 2-3 实现，docs/04 §2.9）
```
stack_grouping_rules(id, scope_type[catalog_item|layer|global], scope_id,
                     granularity[per-component|per-bundle|per-team|custom], custom_rule_json, priority)
layer_migrations(id, from_version_id FK→layer_rule_set_versions, to_version_id FK→layer_rule_set_versions,
                 batch_id, stack_id FK→stacks, tier[1|2|3], old_path, new_path,
                 state_mv_script_path, backup_token, status[planned|running|succeeded|rolled_back|failed],
                 operator, approved_by, silence_until, created_at, completed_at)
```

#### B9. CMDB 与 FinOps（Wave 4 实现，docs/04 §2.11）
```
resources(id, stack_id FK→stacks nullable, bundle_id FK→bundles nullable, team_id FK→teams nullable,
          layer, address, type, cloud_provider, region, cloud_resource_id, name,
          tags_json, attributes_json, monthly_cost_estimate_cents, currency, managed,
          first_seen_at, last_synced_at)
cost_records(id, period_month, team_id FK→teams, bundle_id FK→bundles, stack_id FK→stacks,
             resource_id FK→resources nullable, cloud_provider, service_code, amount_cents, currency,
             cost_source[bill|estimate], tags_json, recorded_at)
cost_budgets(id, scope_type[team|bundle|stack|layer], scope_id, period_month, budget_cents,
             alert_thresholds_json, alert_status[ok|warning|exceeded])
finops_recommendations(id, kind[rightsize|release_orphan|reserved_instance|tag_missing],
                       resource_id FK→resources, detail_json, estimated_saving_cents,
                       status[open|dismissed|applied], created_at)
```

#### B10. 云凭据管理（Wave 5 实现，docs/04 §2.12）
```
team_cloud_grants(id, team_id FK→teams, bundle_id FK→bundles nullable, cloud_account_id FK→cloud_accounts,
                  allowed_layers_json, iam_role_template, budget_quota_cents, expires_at,
                  env_scope_json, granted_by, granted_at)
cloud_credentials(id, cloud_account_id FK→cloud_accounts, team_id FK→teams nullable, bundle_id FK→bundles nullable,
                  credential_type[bootstrap|execution_long_lived|execution_oidc], name, secret_ref,
                  iam_role_assumed, status[active|rotating|revoked|expired], rotated_at, rotate_after,
                  guardians_json, last_used_at, last_used_by_request, created_at)
catalog_items_required_permissions(catalog_item_id FK→catalog_items, provider, permission)
iam_role_templates(id, name, team_id FK→teams, cloud_account_id FK→cloud_accounts, policy_json, ...)
```

#### B11. 环境与租户（Wave 6 实现，docs/04 §2.13）
```
environments(id, env_logical_id, stage[dev|staging|prod|dr], cloud_account_id FK→cloud_accounts,
             region, network_topology[shared|distributed], tag_namespace_json,
             status[active|frozen|deprecated], created_at)
tenants(id, tenant_logical_id, name, isolation_level[vpc-per-env|account-per-env],
        kind[internal|external], owner_team_id FK→teams, tag_namespace_json,
        status[active|frozen|deprecated], created_at)
environment_tenant_bindings(id, env_id FK→environments, tenant_id FK→tenants,
                            layer_logical_id FK→layer_logical_refs,
                            vpc_stack_id FK→stacks, subnet_blocks_json, security_group_base_id,
                            override_cloud_account_id FK→cloud_accounts nullable,
                            status[active|pending-cleanup], created_at)
```

#### B12. 标签策略（Wave 7 实现，docs/04 §2.14）
```
tag_policies(id, scope_type[platform|env|tenant|team|bundle|catalog_item], scope_id,
             tag_namespace_json, mandatory_keys_json, user_allowed_tag_keys_json, version, updated_at)
```

#### B13. CICD 触发（Wave 5 实现，docs/04 §2.15）
```
cicd_triggers(id, request_id FK→requests nullable, cicd_kind[pipeline|webhook|manual],
              pipeline, commit, artifact, catalog_item_id FK→catalog_items, form_hash,
              triggered_by, idempotency_key UNIQUE, created_at)
gate_subscriptions(id, request_id FK→requests, mode[poll|webhook], webhook_url, secret_ref, expires_at)
gate_events(id, request_id FK→requests, status, occurred_at, actor_id, detail_json)
```

#### B14. 存量导入（Wave 5 实现，docs/04 §2.16）
```
import_jobs(id, request_id FK→requests nullable, module_version_id FK→module_versions,
            catalog_item_id FK→catalog_items, bundle_id FK→bundles nullable, requester_id,
            lifecycle, status, review_required, created_at, updated_at)
import_resources(id, import_job_id FK→import_jobs, cloud_id, tf_address,
                 import_status[pending|imported|limited|sensitive|failed], limited,
                 sensitive_fields_json, last_plan_summary_json, created_at)
```

### 全量表统计

| 优先级 | 域 | 表数 | 本 change 动作 |
|---|---|---|---|
| A（MVP）| 组织/registry/catalog/lifecycle/approval/cloud/layer/审计 | 19 | 落迁移 SQL + sqlc |
| B（非 MVP）| exec/workspace/stacks/drift/auth/adapters/saga/hooks/layer-migration/cmdb/finops/cloud-creds/env-tenant/tag/cicd/import | ~33 | **设计定稿**，迁移随 Wave 落 |
| **合计** | | **~52** | 全部初版设计完成 |

## 04-proto 断裂修复方案（A 级优先）

按审计报告 A 级断裂逐项修复：

| 断裂 | 修复 |
|---|---|
| A1 PlanArtifact 三处矛盾 | 独立 `plan_artifacts` 表（不并入 executor_runs），MVP 只建这张 |
| A2 requests 缺 cost 字段 | 补 `cost_estimate_cents` BIGINT + `cost_currency` VARCHAR(8) |
| A3 requests 缺 team_id | 补 `team_id` UUID FK |
| A4 requests 缺 correlation_id | 补 `correlation_id` VARCHAR(64) |
| A5 requests 缺 source | 补 `source` VARCHAR(32) CHECK |
| A6 stacks 无 layer_logical_id | 本 change 在 B2 stacks 表设计中补 `layer_logical_id` 列（定稿），Wave 2 落迁移时直接含此列 |
| A7 CatalogItem proto 暴露少 | 本 change 只管表结构（表有全部列）；proto 暴露字段调整是 proto change 的事 |
| A8 module_versions 缺 dependencies | 新增 `module_dependencies` 表 |
| A9 modules 缺 provider | 补 `provider` 列 |
| A10 variables_contract_json 归属 | 移到 `module_versions`（proto 归属版本） |
| A11 layer 表不在 Phase 1 清单 | 本 change 纳入 `layer_logical_refs` + `layer_rule_set_versions`（Phase 1 seed 用） |

## 05-Phase 推进策略

**本 change = 全量初版设计 + MVP 落地**。

所有 ~52 张表的字段/类型/约束/FK/索引在本 change **全部推导定稿**（§03 A+B）。区别只在实现动作：

| 动作 | 范围 | 本 change 做 |
|---|---|---|
| **设计定稿** | 全部 ~52 张表 | ✅ §03 已定稿（字段/类型/FK/约束/索引规范） |
| **落迁移 SQL + sqlc** | MVP 19 张（优先级 A） | ✅ 本 change 落 |
| **落迁移 SQL（不带 sqlc）** | 非 MVP 33 张（优先级 B） | ❌ 随对应 Wave 实现时落 |

**后续打磨**（本 change 之后，随 Wave 实现逐步完善）：
- 各 Wave 落 B 级表迁移时，按 §03 定稿 schema 直接建表，不重新设计。
- 实现功能过程中发现字段需求变化，回头改 §03 定稿 + 迁移（正常演进，不算返工）。
- docs/04 C 级规范瑕疵随各表落地时修复（docs/04 本身标注指向本 change 的定稿为权威）。

**实现顺序**（本 change 内 MVP 落地）：
1. 规范基线（000_utils.sql trigger 函数）
2. 重写 teams（对齐 UUID + 审计统一）
3. MVP 表按域分批迁移（组织→registry→catalog→layer seed→lifecycle→approval→cloud→审计）
4. sqlc 查询 + 重新生成
5. 测试（迁移幂等 + CRUD + 约束验证）

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
