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

### 1.2 主键策略：雪花 ID（应用层生成，全库统一）

**全库统一 `id BIGINT PRIMARY KEY`**，ID 由应用层雪花 ID 工具类生成（不是 DB 自增、不是 UUID）。

工具类位置：`server/internal/utils/snowflake.go`（参考 ferret `internal/utils/snowflake.go` + 业界 bwmarrin/snowflake 算法）。所有表的 INSERT 在应用层（sqlc query 调用方/data 层）调 `utils.GenerateID()` 生成 ID，DB 列不设 DEFAULT（不依赖 DB 生成）。

```go
// server/internal/utils/snowflake.go
// 全局默认生成器，启动时初始化（machineID/datacenterID 从 config 读）
var defaultNode *Snowflake

func Init(machineID, datacenterID int64) error { ... }  // 启动时 wire 调
func GenerateID() int64 { ... }                          // 所有 INSERT 前调
```

**雪花 ID 结构**（Twitter Snowflake 标准，63 bit）：
```
| 1 bit unused | 41 bit 毫秒时间戳 | 5 bit datacenterID | 5 bit machineID | 12 bit 序列号 |
```
- 时间有序 → B-Tree 插入友好（不像 UUIDv4 随机导致页分裂）
- 全局唯一 → 未来多实例部署只需分配不同 machineID/datacenterID
- int64 → 比 UUID（128 bit）省存储/索引空间
- 不暴露增量信息 → 不像 bigserial 暴露业务量

**proto ID 字段类型**：保持 `string`。雪花 ID 是 int64，但 API/proto 层用 string 传输——这是业界惯例（Twitter/Discord 的雪花 ID API 都用 string），因为 JavaScript/JSON 对 >2^53 的整数会精度丢失。应用层 `int64 → strconv.FormatInt → proto string`，查询时 `string → strconv.ParseInt → int64`。

**现有 teams 表（BIGSERIAL）需重写**：改为 `id BIGINT PRIMARY KEY`（去掉 BIGSERIAL 的自增），INSERT 时应用层传雪花 ID。teams 尚无业务数据，重写无成本。

> **为何偏离 skill 默认（skill 偏好 `BIGINT GENERATED ALWAYS AS IDENTITY`）**：postgresql-table-design skill 把 `IDENTITY` 列为默认首选，snowflake 属于"分布式部署需要"的例外场景。本平台采用 snowflake 的论证：
> 1. **day 1 就为多实例设计**——控制面未来必然水平扩展（多 region 部署），snowflake 提前到位避免后期从 IDENTITY 迁移（IDENTITY 单 sequence 跨实例要协调，迁移成本高；snowflake 用 machineID/datacenterID 天然分布式）。
> 2. **时间有序 → B-Tree 插入友好**——雪花 ID 单调递增，新行追加到 B-Tree 末尾，不像 UUIDv4 随机导致页分裂（skill 在 Insert-Heavy 段也承认这点，故建议 `IDENTITY over UUID`；snowflake 同样具备这个优势）。
> 3. **int64 比 UUID 省空间**——主键 + 所有 FK 列都受益（索引/表存储减半）。
> 4. **不暴露增量信息**——不像 `BIGSERIAL` 能从 ID 推断业务量（这是 P0：现有 `BIGSERIAL` 同时违反 skill 的 `DO NOT use serial`，必须改）。
>
> 此决策记录在 audit-postgresql-skill.md P0-4，属 skill 接受的"分布式例外"。代价：需应用层 `utils.Init(machineID, datacenterID)` 协调（单实例默认 0/0），多实例时配不同值。

**machineID/datacenterID 配置**：从 `server/config.yaml` 读（`snowflake.machine_id` / `snowflake.datacenter_id`），多实例部署时每实例配不同值。单实例开发默认 0/0。

### 1.3 外键策略

显式 FK 约束，命名 `fk_<table>_<col>`：
```sql
team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
```
- 所有 FK 列类型 `BIGINT`（与主键一致）。
- 控制面 DB 强制引用完整性（`ON DELETE RESTRICT` 防误删，不用 CASCADE 防连锁）。
- 逻辑引用但无 FK 是 docs/04 的缺陷，本 change 全部补 FK。

> **P0 硬规则（对账 skill：FK 必须手动索引）**：PostgreSQL **不自动索引 FK 列**（skill Core Rules #4 + Gotchas + Constraints 段三处强调）。每个 FK 列**必须**配显式索引，否则父表删除/更新时子表全表扫 + 锁。规则：
> ```sql
> team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
> -- 紧跟索引（命名 ix_<table>_<fk_col>）
> CREATE INDEX ix_projects_team_id ON projects(team_id);
> ```
> - **无例外**：所有 FK 列都要建索引，单列索引即可。
> - 多 FK 列指向同一父表或常一起查询时，按访问路径考虑复合索引（如 `ix_requests_(team_id, status)`）。
> - 索引在迁移 SQL 中紧跟建表语句，不靠 sqlc 管理。

### 1.4 约束命名

| 约束类型 | 前缀 | 示例 |
|---|---|---|
| Primary Key | `pk_<table>` | `pk_teams` |
| Foreign Key | `fk_<table>_<col>` | `fk_requests_team_id` |
| Unique | `uq_<table>_<cols>` | `uq_teams_slug` |
| Check | `ck_<table>_<col>` | `ck_requests_status` |
| Index | `ix_<table>_<col>` | `ix_requests_status` |

### 1.5 枚举值策略

`TEXT NOT NULL CHECK (xxx IN (...))`，取值域与 proto enum 对齐：
```sql
status TEXT NOT NULL CHECK (status IN (
    'submitted', 'generating', 'pending_admission', ...
)) DEFAULT 'submitted',
```
- 不用 PostgreSQL 原生 ENUM 类型（加值要 `ALTER TYPE`，迁移麻烦）。
- 用 `TEXT + CHECK`（加值改 CHECK 约束，迁移友好）——**对齐 skill**：skill 明确"evolving values（如订单状态）→ TEXT + CHECK"，本平台 status 都属 evolving。
- snake_case 取值（proto enum 值剥前缀后 lowercase + snake）。

### 1.6 数据类型规范（对账 postgresql-table-design skill）

> skill 的硬 DO/DO NOT 作为红线。下表是全库统一的类型选择，落迁移时无例外：

| 类别 | 决策 | 依据（skill 原文） |
|---|---|---|
| **字符串** | `TEXT`（禁 `VARCHAR(n)`/`CHAR(n)`）| "DO NOT use char(n) or varchar(n); DO use text" |
| 字符串长度上限 | 如需，`CHECK (LENGTH(col) <= n)` | "if length limits needed, use CHECK (LENGTH(col) <= n) instead of VARCHAR(n)" |
| **时间戳** | `TIMESTAMPTZ`（禁 `TIMESTAMP`、`timestamptz(n)`）| "DO NOT use timestamp (without time zone); DO NOT use timestamptz(0) or any precision" |
| **整数 ID** | `BIGINT`（应用层 snowflake，无 DEFAULT/IDENTITY）| 见 §1.2 论证（skill 默认 IDENTITY，本平台分布式例外）|
| 普通整数 | `BIGINT`（除非空间敏感才 `INTEGER`）| "prefer BIGINT unless storage space is critical" |
| **金额** | `BIGINT` 存整数分（`*_cents` 后缀）+ 独立 `currency TEXT` | **偏离 skill 默认**（skill 建议 NUMERIC），但整数分是业界共识（Stripe/Shopify），避免浮点；偏离理由记此 |
| **禁用** | `money` 类型、`serial`/`bigserial`、`timetz` | skill DO NOT use 清单 |
| **布尔** | `BOOLEAN NOT NULL`（除非三态）| skill Data Types > Booleans |
| **JSONB** | 半结构化可选属性用 `JSONB`，禁 `JSON`（保序才用 JSON）| "JSONB preferred over JSON; index with GIN" |
| **二进制** | `BYTEA` | skill Data Types > Strings |

### 1.7 索引规范（对账 skill Indexing 段）

| 场景 | 索引类型 | 示例 |
|---|---|---|
| FK 列（**强制**，见 §1.3）| B-tree | `CREATE INDEX ix_projects_team_id ON projects(team_id);` |
| 高频过滤/排序 | B-tree | `CREATE INDEX ix_requests_status ON requests(status);` |
| JSONB 按内容过滤 | GIN | `CREATE INDEX ix_catalog_items_visibility ON catalog_items USING GIN(visibility_json);` |
| 软删未删行唯一 | partial unique | 见 §2.1 |
| 高频 status 子集（可选）| partial | `CREATE INDEX ix_requests_active ON requests(team_id) WHERE status IN ('submitted','generating');` |

**JSONB GIN 索引取舍**（skill JSONB Guidance）：
- 默认 opclass（支持 `@>`/`?`/`?|`/`?&`）：通用，多数场景用这个。
- `jsonb_path_ops`（仅支持 `@>`，索引更小更快）：确认只做 containment 不做 key existence 时用。
- **不建索引的 JSONB**：schema 文档（form_schema_json）、只读快照（before_json/after_json）、低频访问的配置。

**复合索引列序**（skill Composite）：等值过滤列在前，范围列在后。如 `requests(team_id, status)` 服务"团队 X 的 submitted 工单"。

**partial index**（skill Partial）：热查询只查子集时用，缩小索引体积。MVP 可选，按实际查询模式决定。

**fillfactor / update-heavy**（skill Update-Heavy，可选）：update-heavy 表（requests/approval_runs/executor_runs）可设 `fillfactor=90` 提升 HOT update 命中。MVP 不做，记一笔供调优。

## 02-审计属性规范（sqlc/pgx 集成）

### 2.1 统一审计字段

**所有业务表**（非 append-only）必须有：
```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
```

> **P0 硬规则（对账 skill）**：所有时间列一律 `TIMESTAMPTZ`，**禁用** `TIMESTAMP`（without tz）和 `timestamptz(n)`（带精度）。落迁移时每张表的时间列都按此确认。

**append-only 表**（request_events, audit_logs, approval_decisions）只有：
```sql
occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 或 created_at
```

**软删除**（可选，按表需要）：
```sql
deleted_at TIMESTAMPTZ NULL,
```

> **P0 修复（对账 skill UNIQUE NULLs）**：软删除 + 唯一性用 **partial unique index**，**不用** `deleted_at` 进 UNIQUE 列。
>
> 旧写法（错）：`CONSTRAINT uq_stacks_path UNIQUE (bundle_id, component, env, deleted_at)` —— PG 默认允许多个 NULL，未删行（deleted_at NULL）多行共存，唯一性实际不成立。
>
> 正确写法（skill 软删业界标准，GitHub/Linear 同款）：
> ```sql
> -- 只约束未删行；删了可重建同名
> CREATE UNIQUE INDEX uq_stacks_path_active
>     ON stacks (bundle_id, component, env)
>     WHERE deleted_at IS NULL;
> ```
> 命名约定：`uq_<table>_<cols>_active`（`_active` 后缀表明只约束未删行）。
>
> 若确有"全局唯一含 NULL"需求（非软删场景），用 `UNIQUE (...) NULLS NOT DISTINCT`（PG15+，skill 推荐）。本平台软删场景一律走 partial index。

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
layer_logical_refs(logical_id TEXT PK, current_display_name, notes, created_at)
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
| A2 requests 缺 cost 字段 | 补 `cost_estimate_cents` BIGINT + `cost_currency` TEXT |
| A3 requests 缺 team_id | 补 `team_id` BIGINT FK |
| A4 requests 缺 correlation_id | 补 `correlation_id` TEXT |
| A5 requests 缺 source | 补 `source` TEXT CHECK |
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
2. 重写 teams（对齐雪花 ID BIGINT + 审计统一）
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
