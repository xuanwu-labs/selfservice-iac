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

> **偏离 docs 说明（soft-delete policy）**：docs（04/05/06/07）的软删除一律用 status 枚举（`active|deprecated|archived` / `active|frozen|deprecated` 等），全文无 `deleted_at` 列。本 design **刻意保留 `deleted_at` 列**作为统一的软删机制，理由：
> 1. **统一性**：所有业务表一个软删机制（deleted_at），不按表混用 status 语义（有些表 status 是业务状态机如 requests，软删塞进去会污染状态机）。
> 2. **查询清晰**：`WHERE deleted_at IS NULL` 统一过滤，不需记住"这张表用 status='active' 那张表用 status!='deprecated'"。
> 3. **docs 的 status 枚举保留业务含义**：catalog_items.status(active/deprecated/archived) 表达**业务生命周期**（上架/下架/归档），不是软删；deleted_at 表达**记录删除**。两者正交。
> 4. **代价**：与 docs 表述不一致，docs 标注修订时需说明本 change 的偏离。
>
> **规则**：业务表用 deleted_at 软删；status 枚举只表达业务生命周期（不动）。查询活跃记录一律 `WHERE deleted_at IS NULL`。

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

## 03-全量表清单（初版设计全部 ~68 张，分优先级实现）

> **原则**：所有表都在本 change **推导初版**（字段/类型/约束/FK 全规范化），不是只设计 MVP 子集。区别只在实现顺序——MVP 表先落迁移 SQL + sqlc 查询，非 MVP 表设计已就绪、迁移 SQL 随对应 Wave 实现时再落。这样后续 Wave 不用回头重新设计表结构，只需按已定稿的 schema 落迁移。

### 优先级 A：MVP 主链路表（本 change 落迁移 + sqlc）

#### A1. 组织归属（3 张）
```
teams(id, name, slug, kind[platform|dba|middleware|business], status[active|deprecated],
      tags_json, policy_json, created_at, updated_at, deleted_at)
  -- policy_json: S6 team 策略（allowed_regions/cost_cap/mandatory_tags，doc 08 S6）
  -- FK 索引: 无（被引用方）
  -- 唯一: uq_teams_slug_active ON (slug) WHERE deleted_at IS NULL
projects(id, name, team_id FK→teams, created_at, updated_at, deleted_at)
  -- FK 索引: ix_projects_team_id
bundles(id, name, project_id FK→projects, layer_logical_id FK→layer_logical_refs,
        repo_path, tags_json, created_at, updated_at, deleted_at)
  -- FK 索引: ix_bundles_project_id, ix_bundles_layer_logical_id
```
**对账 docs**：teams 加 status（doc 04 §2.1 + 业务生命周期）；policy_json 是 doc 08 S6 team 策略落点（allowed_regions/cost_cap/mandatory_tags）。

#### A2. 模块注册（3 张）
```
modules(id, name, git_source, module_path, provider, layer DEFAULT '',
  owner_team_id FK→teams, status CHECK(pending_validation|validated|validation_failed|deprecated),
  description, created_at, updated_at, deleted_at)
  -- registry 只接受 atomic 模块（D19/D25）：控制层编排由平台 codegen 承担，
  -- 声明层由 Web 表单承担。原 module_type CHECK(atomic|control|declarative) 已删除
  -- （它把 terraform-alicloud-modules 的人工三层误植为平台 registry 三层）。
  -- layer: 信息性字段，权威层归属看 catalog_items.layer_logical_id
module_versions(id, module_id FK→modules, version, commit_sha,
  required_providers_json, variables_contract_json, outputs_contract_json,
  is_current BOOL, registered_at, created_at)
  -- variables_contract_json: 纯 scalar 契约（D25），S1 管道输入
  -- outputs_contract_json: outputs.tf 提取，跨层依赖校验（module_dependencies.output_key 引用上游 output）
  -- FK 索引: ix_module_versions_module_id
module_dependencies(id, module_version_id FK→module_versions, variable_name,
  depends_on_layer, depends_on_module, output_key, required BOOL, description, created_at)
  -- FK 索引: ix_module_dependencies_module_version_id
```
**修复断裂**：modules 补 name/provider/description + status 4 值 CHECK；module_versions 补 is_current（doc 04 §2.2）；module_dependencies 新增（proto ModuleDependency，docs/04 无表）。

#### A3. 服务目录（1 张）
```
catalog_items(id, module_version_id FK→module_versions, display_name, description, category,
  status TEXT CHECK(status IN ('draft','active','deprecated','archived','blocked')),  -- proto CatalogItemStatus
  form_schema_json, defaults_json,   -- defaults_json = S2 catalog defaults（doc 08）
  cardinality TEXT CHECK(cardinality IN ('single','list','map')),   -- D25
  instance_key, per_instance_fields_json, shared_fields_json,
  layer_logical_id FK→layer_logical_refs, stack_grouping TEXT CHECK(stack_grouping IN
    ('per-component','per-bundle','per-team','custom')),   -- D24
  owner_team_id FK→teams, default_tags_json,   -- L6 catalog defaults（doc 08）
  user_allowed_tag_keys_json,   -- L7 用户 tag 白名单（doc 08）
  visibility_json,   -- 团可见性（team_ids 数组）
  created_at, updated_at, deleted_at)
  -- FK 索引: ix_catalog_items_module_version_id, ix_catalog_items_layer_logical_id, ix_catalog_items_owner_team_id
  -- GIN: ix_catalog_items_visibility ON (visibility_json), ix_catalog_items_user_allowed_tag_keys ON (user_allowed_tag_keys_json)
  -- 唯一: uq_catalog_items_(module_version_id, display_name)_active WHERE deleted_at IS NULL
```
**修复断裂**：status 补全 5 值（draft/active/deprecated/archived/blocked，doc 19 §1）；visibility_json + user_allowed_tag_keys_json 建 GIN（按团队过滤 + tag 白名单高频查询）。

#### A4. 工单生命周期（4 张）
```
requests(
  id, catalog_item_id FK→catalog_items, bundle_id FK→bundles nullable,
  env_id, tenant_id, team_id FK→teams, requester_id,
  kind TEXT CHECK(kind IN ('standard','drift_remediation','legacy_import','maintenance_apply')),
  source TEXT CHECK(source IN ('web','cli','cicd','ai','gateway')),
  status TEXT CHECK(status IN (
    'submitted','generating','pending_admission','planning','plan_ready',
    'pending_approval','applying','reconciling','succeeded','reconcile_pending',
    'rejected','cancelled','expired','failed_retryable','failed_terminal',
    'waiting_manual','blocked_policy','blocked_state_health','paused_drift')),  -- 19 值对齐 proto RequestStatus
  current_stage, form_values_json, form_hash, resolved_params_json, source_context_json,
  idempotency_key,   -- sha256(actor+catalog+form_hash+24h_window)，UNIQUE
  pinned_commit, plan_artifact_id FK→plan_artifacts nullable,
  cost_estimate_cents, cost_currency, correlation_id,
  retry_count INT DEFAULT 0, version INT NOT NULL DEFAULT 0,
  layer_rule_set_version_id FK→layer_rule_set_versions,
  created_at, updated_at)
request_events(id, request_id FK→requests, event_type, stage, from_status, to_status,
  actor_id, actor_team_id, actor_type TEXT CHECK(actor_type IN ('unspecified','human','ai','system')),
  message, correlation_id, occurred_at)  -- append-only
plan_artifacts(id, request_id FK→requests,
  status TEXT CHECK(status IN ('ready','expired','consumed','superseded')),  -- proto ArtifactStatus
  plan_hash, storage_uri, sha256, size_bytes,
  pinned_commit, toolchain_profile_hash, provider_lock_hash, tf_version_sha256,
  stack_id, state_key, resources_to_add, resources_to_change, resources_to_destroy,
  cost_estimate_cents, expires_at, created_at)
gate_results(id, request_id FK→requests, gate_id, passed BOOL, policy, message,
  severity TEXT CHECK(severity IN ('unspecified','error','warning')), evaluated_at)
```
> **枚举值域以 proto enum.proto 为准**（剥前缀 lowercase + snake_case）。docs（doc 04/12）的枚举值是设计草稿，proto 是冻结契约唯一源（D45）。

#### A5. 审批（4 张）
```
approval_flows(id, name, trigger, dsl_yaml, version INT, active BOOL,
               created_at, updated_at)
approval_runs(id, request_id FK→requests, flow_id FK→approval_flows,
  gate TEXT CHECK(gate IN ('pre_plan','pre_apply','break_glass_retroactive')),  -- proto ApprovalGate
  current_node, status TEXT CHECK(status IN ('pending','approved','rejected','expired')),  -- proto ApprovalRunStatus
  decided_by, decided_at, started_at, finished_at, expires_at, version INT)
  -- 唯一: uq_approval_runs_req_gate_pending WHERE status='pending'
approval_node_runs(id, run_id FK→approval_runs, node_id,
  mode TEXT CHECK(mode IN ('any','all','majority','quorum')),  -- proto ApprovalNodeMode（quorum+N=Count>=N）
  decided_count INT, required_count INT,
  status TEXT CHECK(status IN ('pending','approved','rejected','skipped','timeout')),  -- proto ApprovalNodeStatus
  timeout_at)
approval_decisions(id, node_run_id FK→approval_node_runs, approver_id,
  decision TEXT CHECK(decision IN ('approved','rejected')),  -- proto ApprovalDecision
  comment, decided_at)  -- append-only
  -- 唯一: uq_approval_decisions_node_approver ON (node_run_id, approver_id)（IDEMP-004）
```
**对齐 proto**：gate/mode/status/decision 全部对齐 proto enum（剥前缀 lowercase + snake_case）。mode 用 `any/all/majority/quorum`（决策聚合语义），`count>=N` 用 `quorum` + `required_count=N` 表达，conditional 路由靠 DSL 不进 enum。

#### A6. 云账号（1 张）
```
cloud_accounts(id,
  provider TEXT CHECK(provider IN ('aws','aliyun','azure','gcp')),  -- proto CloudProvider
  account_id, alias, display_name,
  status TEXT CHECK(status IN ('active','suspended')),  -- proto CloudAccountStatus（deprecating/deprecated 延后）
  default_region, regions_json, credentials_ref, billing_enabled BOOL,
  default_team_id FK→teams nullable, tags_json,
  bootstrap_status TEXT CHECK(bootstrap_status IN ('ok','rotate','none')),
  oidc_trust_configured BOOL, created_at, updated_at)
  -- FK 索引: ix_cloud_accounts_default_team_id
  -- 对齐 doc 04 §2.12 权威（doc 06 footnote 指向）；status 含 deprecating/deprecated 支持 doc 07c §14 注销 cascade
```

#### A7. 分层（2 张，Phase 1 只 seed）
```
layer_logical_refs(logical_id TEXT PK, current_display_name, notes, created_at)
layer_rule_set_versions(version_id INT PK, layers_json, status[active|superseded|deprecated|archived],
                        is_default, created_at, created_by, superseded_at, superseded_by)
```
Phase 1 出厂 seed：3 条 layer_logical_refs（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active）。

#### A8. 审计/事件（2 张）
```
audit_logs(id, actor_id, actor_team_id, actor_type TEXT CHECK(actor_type IN ('unspecified','human','ai','system')),
  action, target_type, target_id, before_json, after_json,
  ai_metadata_json,   -- nullable，仅 actor_type=ai 时填（doc 17 §9.2: prompt_hash/skill_name/llm_model/tool_calls_json/confidence_score）
  correlation_id, occurred_at)  -- append-only
  -- 合规可选（doc 20 §2，HMAC 链启用时加列）: prev_hash, entry_hash, signing_key_id, sealed_at
  -- 索引: ix_audit_logs_target(target_type, target_id), ix_audit_logs_correlation_id, ix_audit_logs_occurred_at
outbox_events(id, event_id,   -- event_id UNIQUE，幂等（doc 12a IDEMP-005）
  aggregate_type, aggregate_id, event_type, payload_json,
  status TEXT CHECK(status IN ('pending','processing','succeeded','failed','dead-letter')),
  retry_count INT, next_retry_at, correlation_id, created_at, updated_at, processed_at)
  -- 索引: ix_outbox_events_status, ix_outbox_events_next_retry_at
```
**修复**：audit_logs 补 ai_metadata_json（doc 17 §9.2，AI 操作审计链）；outbox_events 枚举改 5 值（pending/processing/succeeded/failed/dead-letter，doc 04 §2.8a）+ 补 event_id（幂等 UNIQUE）/next_retry_at/correlation_id。

#### A9. 执行面（5 张，2026-07-16 从 B 列提升为 MVP）
```
state_backends(id, name, kind CHECK(s3|oss|local), bucket, region, endpoint,
  encrypt BOOL, lock_table, access_style CHECK(oidc|aksk|anonymous),
  credentials_ref, is_default BOOL, created_at, updated_at)
  -- 父级后端配置唯一源（替代 doc 02/09 硬编码 bucket="tm-state"）。
  -- codegen 读 is_default 行渲染 backend.tf；account-per-env 场景用 stacks.state_backend_id override。
  -- 约束: uq_state_backends_single_default（部分唯一索引，保证至多 1 个 is_default=true）
  -- seed: id=0, name='default', bucket='tm-state'（初始 dev 流可用）
workspaces(id, name, remote_url, default_branch DEFAULT 'main', created_at, updated_at)
  -- 平台管理的 infra-repo 注册（D4/D29 单仓 monorepo）。
  -- 约束: uq_workspaces_name
workspace_checkouts(id, workspace_id FK→workspaces, node_id, worktree_path, branch,
  pinned_commit, purpose CHECK(plan_apply|drift|import),
  leased_by_request_id FK→requests nullable, leased_until,
  status CHECK(active|released|stale), created_at, updated_at)
  -- 每工单独占 worktree；pinned_commit 是 requests.pinned_commit 的归属目标（消除孤儿）。
  -- 索引: ix_workspace_checkouts_status（重启 reconcile 扫 stale，doc 10 §4）
stacks(id, bundle_id FK→bundles nullable, catalog_item_id FK→catalog_items,
  layer_logical_id FK→layer_logical_refs, layer_rule_set_version_id FK→layer_rule_set_versions(version_id),
  owner_team_id FK→teams, layer, component, env, tenant_id DEFAULT 'platform-default',
  stack_id, repo_path, state_key, terramate_tags_json,
  state_backend_id FK→state_backends nullable, pinned_commit,
  migration_status CHECK(stable|migration_pending|migrating|deprecated), sunset_deadline,
  version, created_at, updated_at)
  -- codegen 产出的 stack 身份落库（D29 stack.tm.hcl 镜像）。
  -- stacks.tenant_id MVP 为字符串（tenants 表 B11 非 MVP），Phase 2 落表后加 FK。
  -- 约束: uq_stacks_stack_id（全局唯一）；uq_stacks_repo_path_active（同路径非 deprecated 唯一）
stack_dependencies(id, from_stack_id FK→stacks CASCADE, to_stack_id FK→stacks RESTRICT,
  kind CHECK(remote_state|data_source|watch_only), variable_name, output_key, inject_as,
  status CHECK(active|deprecated), created_at)
  -- 运行时跨层依赖图（D29 after/watch）。codegen 从 module_dependencies + (env,tenant,layer) binding 物化。
```
**2026-07-16 提升 MVP 理由**：原 MVP 20 张表只穿通 catalog → request → codegen 输入；后半段（stack 落地 → git → 执行）完全断裂——requests.pinned_commit 是孤儿（workspace_checkouts 在 B 列）、codegen 产出无 stacks 表持久化、backend.tf 硬编码无 state_backends 源。这 5 张表是端到端穿通的必要条件，不是可选优化。

### 优先级 B：非 MVP 表（本 change 设计定稿，迁移随 Wave 落）

> 这些表**字段/类型/约束/FK 本 change 全部推导定稿**，但不写迁移 SQL——等对应 Wave 实现功能时直接按定稿 schema 落迁移。设计依据来自 docs/04 §2.1-2.16 全部读完 + proto 契约对齐。

#### B1. 执行与工作仓库（Wave 2-3 实现，部分已提升 A9）
```
executor_runs(id, request_id FK→requests,
  phase TEXT CHECK(phase IN ('plan','apply','drift','import')),
  pinned_commit, artifact_id FK→plan_artifacts nullable,
  toolchain_profile_hash, provider_lock_hash,
  credential_session_id nullable, exit_code INT, started_at, finished_at,
  status TEXT CHECK(status IN ('running','succeeded','failed','interrupted')),
  failure_category TEXT CHECK(failure_category IN
    ('user_input','policy_denied','cloud_quota','cloud_api','toolchain','state_backend',
     'platform_bug','manual_required')) nullable)   -- doc 18 §4/§6 Phase 1 验收门
  -- FK 索引: ix_executor_runs_request_id, ix_executor_runs_artifact_id
-- NOTE: workspaces/workspace_checkouts 已于 2026-07-16 提升至 A9（MVP），此处不再重复。
toolchain_manifest(node_id, mode TEXT CHECK(mode IN ('process','container','kubernetes','remote')),
  terramate_version, tf_version, tofu_version, providers_json,
  checked_at, status TEXT CHECK(status IN ('active','superseded')), created_at, updated_at)
  -- doc 11 §6 节点工具链版本真相源（DB+节点双写），校验时机③的输入
  -- PK: (node_id, mode) 或独立 id；此处用 (node_id) 单节点单 manifest
```
注意：executor_runs.artifact_id → plan_artifacts（非"并入"——plan_artifacts 独立，executor_runs 引用它）。这解决了 docs/04 A1 三处矛盾。补 failure_category（doc 18 §4 结构化失败归因，dashboard 必需）+ toolchain_manifest（doc 11 §6 节点工具链真相源）。

#### B2. stack 注册表（已于 2026-07-16 提升至 A9 MVP）
> stacks + stack_dependencies 已在 A9 落迁移。详见 A9 段。此处保留标题供历史索引。
executor_runs(id, request_id FK→requests,
  phase TEXT CHECK(phase IN ('plan','apply','drift','import')),
  pinned_commit, artifact_id FK→plan_artifacts nullable,
  toolchain_profile_hash, provider_lock_hash,
  credential_session_id nullable, exit_code INT, started_at, finished_at,
  status TEXT CHECK(status IN ('running','succeeded','failed','interrupted')),
  failure_category TEXT CHECK(failure_category IN
    ('user_input','policy_denied','cloud_quota','cloud_api','toolchain','state_backend',
     'platform_bug','manual_required')) nullable)   -- doc 18 §4/§6 Phase 1 验收门
  -- FK 索引: ix_executor_runs_request_id, ix_executor_runs_artifact_id
workspaces(id, name, remote_url, default_branch, created_at, updated_at)
workspace_checkouts(id, workspace_id FK→workspaces, node_id, worktree_path, branch,
  pinned_commit, purpose, leased_by_request_id FK→requests nullable,
  leased_until, status TEXT CHECK(status IN ('active','released','stale')),
  created_at, updated_at)
  -- FK 索引: ix_workspace_checkouts_workspace_id, ix_workspace_checkouts_leased_by_request_id
  -- 索引: ix_workspace_checkouts_status（reconcile 扫 stale）
toolchain_manifest(node_id, mode TEXT CHECK(mode IN ('process','container','kubernetes','remote')),
  terramate_version, tf_version, tofu_version, providers_json,
  checked_at, status TEXT CHECK(status IN ('active','superseded')), created_at, updated_at)
  -- doc 11 §6 节点工具链版本真相源（DB+节点双写），校验时机③的输入
  -- PK: (node_id, mode) 或独立 id；此处用 (node_id) 单节点单 manifest
```
注意：executor_runs.artifact_id → plan_artifacts（非"并入"——plan_artifacts 独立，executor_runs 引用它）。这解决了 docs/04 A1 三处矛盾。补 failure_category（doc 18 §4 结构化失败归因，dashboard 必需）+ toolchain_manifest（doc 11 §6 节点工具链真相源）。

#### B2. 组合模板 catalog_blueprints（Wave 2 实现，2026-07-16 新增设计）
```
catalog_blueprints(id, name, display_name, description, category,
  status CHECK(draft|active|deprecated|archived),
  owner_team_id FK→teams, layer_logical_id FK→layer_logical_refs nullable,
  visibility_json, version INT DEFAULT 1, created_at, updated_at, deleted_at)
  -- 组合模板：用户一次申请一套资源（如"订单微服务套件"= VPC+RDS+Redis+ECS+SLB）。
  -- 与 bundles（路径分组）正交：bundles 管"目录/成本/标签分组"，
  -- catalog_blueprints 管"一次申请多个 catalog_item 的模板 + 参数映射"。
  -- 编排顺序由 stacks 的 stack.tm.hcl after/watch 表达（Terramate DAG），本表只管模板定义。
catalog_blueprint_items(id, blueprint_id FK→catalog_blueprints CASCADE,
  catalog_item_id FK→catalog_items RESTRICT,
  role, sort_order INT,             -- 部署拓扑序（codegen 据此生成 stack after 链）
  param_mappings_json,              -- 本 item 变量 ← 其他 item output 映射（如 db_conn ← {{outputs.primary-db.conn}}）
  overrides_json,                   -- 对 catalog defaults 的覆盖
  required BOOL DEFAULT TRUE,       -- false=可选加购
  created_at)
```
**设计依据**（2026-07-16 架构审查）：用户场景"一次申请一套组合"当前无产品入口——catalog_items 是单 atomic，bundles 是路径分组，stack_dependencies 是运行时关系。组合模板填补"设计时模板 + 参数映射"缺口，codegen 展开后生成 N 个 stack，每 stack 仍独立 state/审批/回滚（防爆炸）。
**与 Terramate bundle 的区分**：Terramate bundle 是静态 HCL 求值（cty.Value 单次计算），无 provenance/拒绝门；Aether blueprint 的参数映射走 codegen 9 阶段管道（每 item 参数经 S1-S9 合并 + provenance 审计）。MPL-2.0 开源无商业限制，但技术不匹配（静态 vs 9 阶段动态），故自研表。

#### B2a. stack 注册表（已于 2026-07-16 提升至 A9 MVP）
> stacks + stack_dependencies 已在 A9 落迁移（含补 layer_logical_id + layer_rule_set_version_id 闭合动态分层链路）。此处保留索引。

#### B3. 漂移检测（Wave 4 实现）
```
drift_runs(id, stack_id FK→stacks, started_at, finished_at, exit_code INT,
  has_drift BOOL, diff_summary_json, created_at)
  -- FK 索引: ix_drift_runs_stack_id
drift_records(id, stack_id FK→stacks, drift_run_id FK→drift_runs,
  status TEXT CHECK(status IN ('open','adopted','reverted')),
  detected_at, resolved_at, resolver_id,
  resolution TEXT CHECK(resolution IN ('adopt-cloud','restore-desired','mark-failed-terminal')),
  remediation_request_id FK→requests nullable)   -- doc 13 §5.1：漂移修复创建的 request（kind=drift-remediation），闭合追溯链
  -- doc 21 RB-001 第三路径 mark-failed-terminal
  -- FK 索引: ix_drift_records_stack_id, ix_drift_records_drift_run_id, ix_drift_records_remediation_request_id
  -- 索引: ix_drift_records_(stack_id, status, detected_at)（连续漂移计数，doc 15 §3.1 3 次暂停判据）
```

#### B4. 鉴权与身份（Wave 2-3 实现，docs/04 §2.7 + docs/05 §5）
```
oidc_providers(name PK, issuer_url, client_id, client_secret_encrypted, claim_mapping_json,
  scopes_json, bridge_mode BOOL, is_default BOOL,
  status TEXT CHECK(status IN ('active','disabled')), created_at, updated_at)
identities(id, external_id, display_name, email, provider_name FK→oidc_providers,
  primary_source, status TEXT CHECK(status IN ('active','disabled','merged')),
  merged_into_id FK→identities nullable, last_synced_at, created_at)
  -- FK 索引: ix_identities_provider_name, ix_identities_merged_into_id
sessions(id, identity_id FK→identities, idp_session_id, issued_at, expires_at, revoked_at)
  -- FK 索引: ix_sessions_identity_id
role_bindings(id, subject_id, role,
  scope_type TEXT CHECK(scope_type IN ('team','project','bundle','stack','layer')),
  scope_id, actions_json)
  -- 索引: ix_role_bindings_(subject_id), ix_role_bindings_(scope_type, scope_id)
emergency_runs(id, request_id FK→requests, break_glass_operator_ids_json, reason, recording_url,
  retroactive_approval_id FK→approval_runs nullable, retroactive_deadline,
  status TEXT CHECK(status IN ('active','retroactively_approved','overdue')), created_at)
  -- FK 索引: ix_emergency_runs_request_id
sensitive_field_blacklist(pattern PK, added_at)
-- ↓ docs/05 §5 目录同步层（4 张，原 design.md 漏）
identity_sources(id, kind TEXT CHECK(kind IN ('oidc','scim','feishu','dingtalk','ldap')),
  config_json, priority INT, enabled BOOL, created_at, updated_at)
  -- 目录同步源配置（config_json 含敏感连接信息，加密）
org_nodes(id, source_id FK→identity_sources, external_id, parent_id FK→org_nodes nullable,
  name, path, created_at, updated_at)
  -- 同步的组织树（自引用 parent_id，path 物化路径）
  -- FK 索引: ix_org_nodes_source_id, ix_org_nodes_parent_id
org_mappings(id, match_type TEXT CHECK(match_type IN ('dept','group')), match_expr,
  target_team_id FK→teams, target_role, target_layer, created_at, updated_at)
  -- 组织→团队/角色映射规则，event-driven 推 role_bindings（doc 05 §7）
  -- FK 索引: ix_org_mappings_target_team_id
sync_runs(id, source_id FK→identity_sources, started_at, finished_at,
  status TEXT CHECK(status IN ('pending','running','succeeded','failed')),
  stats_json, created_at)
  -- FK 索引: ix_sync_runs_source_id
```

#### B5. 适配器配置（Wave 1 实现，docs/04 §2.8）
```
adapters_config(id, adapter_type[git|state|policy|cost|notify|cloud|identity|credentials],
                name, provider, config_json, status[active|disabled], created_at, updated_at)
```

#### B6. Saga 补偿（Wave 3 实现，docs/04 §2.8a）
```
reconcile_jobs(id, aggregate_type, aggregate_id, job_type, payload_json,
  status TEXT CHECK(status IN ('pending','running','succeeded','failed')),
  diff_summary_json, retry_count INT, last_error, started_at, finished_at, created_at, updated_at)
  -- 对齐 doc 04 §2.8a（补 diff_summary_json/started_at/finished_at）
manual_intervention_tasks(id,
  source_type TEXT CHECK(source_type IN ('request','stack','cloud_account','import','migration','executor')),
  source_id, reason_code, owner_team_id FK→teams nullable, assignee_id nullable,
  severity TEXT CHECK(severity IN ('p0','p1','p2','p3')),
  status TEXT CHECK(status IN ('open','acknowledged','resolved','cancelled')),
  context_json, recovery_action_json, sla_deadline, correlation_id,
  created_at, updated_at, resolved_at)
  -- 对齐 doc 04 §2.8a 权威字段（source_type/severity/sla_deadline/recovery_action_json）
  -- FK 索引: ix_manual_intervention_tasks_owner_team_id
```

#### B7. 商业级 Run Hooks / 运营（Phase 2+ 实现，docs/04 §2.8b）
```
run_hooks(id, name, phase TEXT CHECK(phase IN ('pre-plan','post-plan','pre-apply','post-apply')),
  target_url, auth_secret_ref, timeout_seconds INT,
  failure_policy TEXT CHECK(failure_policy IN ('fail-open','fail-closed','warn-only')),
  requires_credentials BOOL, status TEXT CHECK(status IN ('active','disabled')),
  created_at, updated_at)
run_hook_results(id, hook_id FK→run_hooks, request_id FK→requests,
  phase TEXT CHECK(phase IN ('pre-plan','post-plan','pre-apply','post-apply')),
  status TEXT CHECK(status IN ('running','succeeded','failed','timeout')),
  decision TEXT CHECK(decision IN ('allow','deny','warn')),
  summary, details_ref, started_at, finished_at)
  -- FK 索引: ix_run_hook_results_hook_id, ix_run_hook_results_request_id
incidents(id, severity TEXT CHECK(severity IN ('p0','p1','p2','p3')), title,
  status TEXT CHECK(status IN ('open','mitigated','resolved','closed')),
  source_type TEXT CHECK(source_type IN ('request','stack','executor','state','security','platform')),
  source_id, owner_team_id FK→teams nullable, legal_hold BOOL,
  started_at, mitigated_at, resolved_at, postmortem_ref, correlation_id, created_at)
  -- 对齐 doc 04 §2.8b + doc 20 §3.1（legal_hold/source_type）
  -- FK 索引: ix_incidents_owner_team_id
runbook_executions(id, runbook_id FK→runbooks, incident_id FK→incidents nullable,
  operator_id, status TEXT CHECK(status IN ('started','completed','failed')),
  evidence_ref, started_at, finished_at, created_at)
  -- FK 索引: ix_runbook_executions_runbook_id, ix_runbook_executions_incident_id
drill_results(id, drill_type TEXT CHECK(drill_type IN
    ('state_restore','db_restore','break_glass','oidc_jwks_rotation','provider_mirror_outage')),
  runbook_id FK→runbooks nullable, scheduled_frequency,
  status TEXT CHECK(status IN ('passed','failed')),
  findings_json, follow_up_task_id FK→manual_intervention_tasks nullable,
  executed_at, executed_by, created_at)
  -- doc 20 §7 + doc 21 §10（5 种演练类型）
platform_scorecards(id, subject_type TEXT CHECK(subject_type IN ('catalog_item','team','platform')),
  subject_id, period, score INT,   -- 0-100
  confidence TEXT CHECK(confidence IN ('low','medium','high')),   -- doc 19 §3.1
  sample_size INT, data_completeness INT, missing_dimensions_json,
  dimension_scores_json,   -- 6 维度子分（reliability30/governance20/usage15/cost15/version10/ownership10）
  calculated_at, created_at)
catalog_health_checks(id, catalog_item_id FK→catalog_items,
  usage_json, reliability_json, governance_json, cost_json, version_json, ownership_json,
  status TEXT CHECK(status IN ('healthy','degraded','blocked')),
  calculated_at, created_at)
  -- FK 索引: ix_catalog_health_checks_catalog_item_id
-- ↓ docs 丢失表（doc 20 §3.2 HMAC 签名密钥 + doc 21 §1 runbook 引用）
signing_keys(signing_key_id PK, status TEXT CHECK(status IN ('active','readonly','compromised')),
  rotated_at, compromised_window_start nullable, compromised_window_end nullable, created_at)
  -- doc 20 §3.2 HMAC 审计链签名密钥轮换（90 天默认）
runbooks(runbook_id PK, title, severity TEXT CHECK(severity IN ('p0','p1','p2')),
  scenario, steps_json, created_at, updated_at)
  -- doc 21 §1 9 个 runbook 种子（RB-001~RB-009）
```

#### B8. 分层迁移（Phase 2-3 实现，docs/04 §2.9）
```
stack_grouping_rules(id, scope_type TEXT CHECK(scope_type IN ('catalog_item','layer','global')),
  scope_id, granularity TEXT CHECK(granularity IN ('per-component','per-bundle','per-team','custom')),
  custom_rule_json, priority INT, created_at, updated_at)
layer_migrations(id, from_version_id FK→layer_rule_set_versions, to_version_id FK→layer_rule_set_versions,
  batch_id, stack_id FK→stacks, tier TEXT CHECK(tier IN ('1','2','3')),
  old_path, new_path, state_mv_script_path, backup_token,
  status TEXT CHECK(status IN ('planned','running','succeeded','rolled_back','failed')),
  operator, approved_by, silence_until, created_at, completed_at)
  -- FK 索引: ix_layer_migrations_stack_id, ix_layer_migrations_from_version_id, ix_layer_migrations_to_version_id
```

#### B9. CMDB 与 FinOps（Wave 4 实现，docs/04 §2.11 + docs/14）
```
resources(id, stack_id FK→stacks nullable, bundle_id FK→bundles nullable,
  team_id FK→teams nullable, tenant_id,   -- doc 07 §7 tenant_id 反规范化（租户级成本归集）
  layer, address, type, cloud_provider, region, cloud_resource_id, name,
  tags_json, attributes_json, monthly_cost_estimate_cents, currency, managed BOOL,
  status TEXT CHECK(status IN ('active','drifted','orphan','destroyed')),   -- doc 14 §2
  first_seen_at, last_synced_at, created_at, updated_at)
  -- FK 索引: ix_resources_stack_id, ix_resources_bundle_id, ix_resources_team_id
  -- GIN: ix_resources_tags ON (tags_json)（成本归集锚点，doc 18）
resource_relations(id, source_resource_id FK→resources, target_resource_id FK→resources,
  relation_type, created_at)
  -- doc 14 §2 资源关系图（dependency/contains/part-of）
  -- FK 索引: ix_resource_relations_source, ix_resource_relations_target
cost_records(id, period_month, team_id FK→teams, bundle_id FK→bundles, stack_id FK→stacks,
  resource_id FK→resources nullable,   -- nullable：unallocated cost（doc 14 §2）
  cloud_provider, service_code, amount_cents, currency,
  cost_source TEXT CHECK(cost_source IN ('bill','estimate')), tags_json, recorded_at, created_at)
  -- FK 索引: ix_cost_records_resource_id, ix_cost_records_(team_id, period_month)
cost_budgets(id, scope_type TEXT CHECK(scope_type IN ('team','bundle','stack','layer')),
  scope_id, period_month, budget_cents, alert_thresholds_json,
  alert_status TEXT CHECK(alert_status IN ('ok','warning','exceeded')), created_at, updated_at)
finops_recommendations(id,
  kind TEXT CHECK(kind IN ('rightsize','release_orphan','reserved_instance','tag_missing')),
  resource_id FK→resources, detail_json, estimated_saving_cents,
  confidence TEXT CHECK(confidence IN ('low','medium','high')),   -- doc 14 §4.5
  metric_source, utilization_summary_json, data_days INT, sample_count INT,   -- doc 14 §4.5 支撑字段
  status TEXT CHECK(status IN ('open','dismissed','applied')), created_at, updated_at)
  -- FK 索引: ix_finops_recommendations_resource_id
  -- 对齐 doc 14 §4.5：补 confidence_score + 4 个支撑字段
```

#### B10. 云凭据管理（Wave 5 实现，docs/04 §2.12 权威）
```
team_cloud_grants(id, team_id FK→teams, bundle_id FK→bundles nullable,
  cloud_account_id FK→cloud_accounts, allowed_layers_json, iam_role_template,
  budget_quota_cents, expires_at, env_scope_json, granted_by, granted_at, created_at, updated_at)
  -- FK 索引: ix_team_cloud_grants_team_id, ix_team_cloud_grants_cloud_account_id
  -- GIN: ix_team_cloud_grants_env_scope ON (env_scope_json)（doc 06 §8 JSON_CONTAINS 查询）
cloud_credentials(id, cloud_account_id FK→cloud_accounts, team_id FK→teams nullable,
  bundle_id FK→bundles nullable,
  credential_type TEXT CHECK(credential_type IN ('bootstrap','execution_long_lived','execution_oidc')),
  name, secret_ref, iam_role_assumed,
  status TEXT CHECK(status IN ('active','rotating','revoked','expired')),
  rotated_at, rotate_after, guardians_json, last_used_at, last_used_by_request, created_at, updated_at)
  -- FK 索引: ix_cloud_credentials_cloud_account_id, ix_cloud_credentials_team_id
catalog_items_required_permissions(catalog_item_id FK→catalog_items, provider, permission, created_at)
  -- 复合 PK: (catalog_item_id, provider, permission)
iam_role_templates(id, name, team_id FK→teams, cloud_account_id FK→cloud_accounts,
  permissions_json, cloud_role_name, oidc_audience, oidc_sub, managed_by_bootstrap BOOL,
  version INT, created_at, updated_at)
  -- FK 索引: ix_iam_role_templates_team_id, ix_iam_role_templates_cloud_account_id
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
tag_policies(id, scope_type TEXT CHECK(scope_type IN ('platform','env','tenant','team','bundle','catalog_item')),
  scope_id, tag_namespace_json, mandatory_keys_json, user_allowed_tag_keys_json,
  version INT, created_at, updated_at)
tag_policy_versions(id, tag_policy_id FK→tag_policies, version INT,
  tag_namespace_json, mandatory_keys_json, user_allowed_tag_keys_json,
  superseded_by FK→tag_policy_versions nullable, created_at)
  -- doc 04 §2.14 版本化子表（结构同 tag_policies + superseded_by）
  -- FK 索引: ix_tag_policy_versions_tag_policy_id
```

#### B13. CICD 触发（Wave 5 实现，docs/04 §2.15 + docs/16）
```
cicd_triggers(id, request_id FK→requests nullable,
  cicd_kind TEXT CHECK(cicd_kind IN ('pipeline','webhook','manual')),
  pipeline, commit, artifact, catalog_item_id FK→catalog_items, form_hash,
  triggered_by, idempotency_key UNIQUE,   -- sha256(pipeline:commit:catalogItem:form_hash)，doc 16 §3.1
  created_at)
  -- FK 索引: ix_cicd_triggers_request_id, ix_cicd_triggers_catalog_item_id
gate_subscriptions(id, request_id FK→requests,
  mode TEXT CHECK(mode IN ('poll','webhook')), webhook_url, secret_ref, expires_at, created_at)
  -- FK 索引: ix_gate_subscriptions_request_id
gate_events(id, request_id FK→requests,
  status TEXT CHECK(status IN ('pending','admission_approved','plan_ready','approval_granted',
    'applying','apply_succeeded','rejected','timeout','failed',   -- 主集（doc 16 §3.1）
    'policy_blocked','needs_replan','manual_intervention_required','reconcile_pending')),  -- Phase 2（doc 16 §3.1.2）
  occurred_at, actor_id, detail_json, created_at)
  -- gate_status 是 requests.status 的投影，不反向扩展 RequestLifecycle（doc 16 §3.1.1）
  -- FK 索引: ix_gate_events_request_id
```

#### B14. 存量导入（Wave 5 实现，docs/04 §2.16 + docs/15）
```
import_jobs(id, request_id FK→requests nullable, module_version_id FK→module_versions,
  catalog_item_id FK→catalog_items, bundle_id FK→bundles nullable, requester_id,
  lifecycle TEXT CHECK(lifecycle IN ('discovered','candidate','imported-limited','managed-readonly',
    'managed-changeable','standard','failed')),   -- doc 15 §1 七态
  status TEXT CHECK(status IN ('pending','running','waiting-review','succeeded','failed','cancelled')),
  review_required BOOL,
  created_stack_id FK→stacks nullable,   -- doc 15 §6 step 4：import 创建的 stack（nullable 直到 step 4 完成）
  exemption_expires_at nullable, exemption_reason nullable, exemption_review_cycle nullable,  -- doc 15 §3.1
  created_at, updated_at)
  -- FK 索引: ix_import_jobs_request_id, ix_import_jobs_module_version_id, ix_import_jobs_catalog_item_id, ix_import_jobs_created_stack_id
import_resources(id, import_job_id FK→import_jobs, cloud_id, tf_address,
  import_status TEXT CHECK(import_status IN ('pending','imported','limited','sensitive','failed')),
  limited BOOL, sensitive_fields_json, last_plan_summary_json, created_at)
  -- FK 索引: ix_import_resources_import_job_id
```

#### B15. AI 与机器身份（Wave 8+ 实现，docs/17 + docs/20）
> docs/17 §3/§6/§9.2 + docs/20 §3.2 要求的表，原 design.md §03 完全缺失。
```
service_accounts(id, name, display_name,
  status TEXT CHECK(status IN ('active','disabled','rotating')), created_at, updated_at)
  -- 机器身份（doc 17 §3），AK 明文存、SK 只存 hash
service_account_keys(id, service_account_id FK→service_accounts, ak, sk_hash,
  scope_json, status TEXT CHECK(status IN ('active','revoked')), rotated_at, created_at)
  -- 双 AK 轮换支持（doc 17 §3），同一 service_account 可有多 active key
  -- FK 索引: ix_service_account_keys_service_account_id
ai_prompts(prompt_hash PK, prompt_ciphertext, prompt_length INT, created_at)
  -- doc 17 §9.2 AI prompt 加密存储（按 hash 查，不存明文）
skills(id, name, description, trigger_patterns_json, steps_yaml, output_contract,
  owner_team_id FK→teams nullable, version INT, builtin BOOL, created_at, updated_at)
  -- doc 17 §6 声明式 skills（builtin + team-custom）
  -- FK 索引: ix_skills_owner_team_id
```

### 全量表统计

| 优先级 | 域 | 表数 | 本 change 动作 |
|---|---|---|---|
| A（MVP）| 组织3/registry3/catalog1/lifecycle4（+approval_flows 共5）/cloud1/layer2/审计2/**执行面5（2026-07-16 提升）** | **25** | 落迁移 SQL + sqlc |
| B（非 MVP）| exec2（+toolchain_manifest，workspaces/checkout 已提升 A9）/**catalog_blueprints2（2026-07-16 新增）**/stacks0（已提升 A9）/drift2/auth11（+identity_sources/org_nodes/org_mappings/sync_runs）/adapters1/saga2/hooks运营10（+signing_keys/runbooks）/layer-migration2/cmdb7（+resource_relations）/finops/cloud-creds4/env-tenant3/tag2（+tag_policy_versions）/cicd3/import2/AI机器身份4（B15 新增） | **~45** | **设计定稿**，迁移随 Wave 落 |
| **合计** | | **~68** | 全部初版设计完成（docs 全量对账后） |

> **表数变化说明**：原 ~52 张基于 docs/04 §2.1-2.16 + proto 契约；docs 全量通读（含 docs/05 §5 目录同步层、docs/11 §6 toolchain_manifest、docs/17 §3/§6/§9.2 AI/机器身份、docs/20 §3.2 signing_keys、docs/21 §1 runbooks、docs/04 §2.14 tag_policy_versions、docs/14 §2 resource_relations）后补全至 ~68 张。新增表见 audit-docs-sweep.md B-致命-1。

## 04-断裂修复方案（docs 全量对账后更新）

### A 级（proto 契约断裂，原 11 项 + 新增）

| 断裂 | 修复 |
|---|---|
| A1 PlanArtifact 三处矛盾 | 独立 `plan_artifacts` 表（不并入 executor_runs），MVP 只建这张 |
| A2 requests 缺 cost 字段 | 补 `cost_estimate_cents` BIGINT + `cost_currency` TEXT |
| A3 requests 缺 team_id | 补 `team_id` BIGINT FK |
| A4 requests 缺 correlation_id | 补 `correlation_id` TEXT |
| A5 requests 缺 source | 补 `source` TEXT CHECK |
| A6 stacks 无 layer_logical_id | B2 stacks 补 `layer_logical_id` 列 |
| A7 CatalogItem proto 暴露少 | 表有全部列；proto 暴露调整属 proto change |
| A8 module_versions 缺 dependencies | 新增 `module_dependencies` 表 |
| A9 modules 缺 provider | 补 `provider` 列 |
| A10 variables_contract_json 归属 | 移到 `module_versions` |
| A11 layer 表不在 Phase 1 清单 | 纳入 `layer_logical_refs` + `layer_rule_set_versions`（seed） |
| **A12（新）requests 缺 kind/resolved_params_json/retry_count** | 补 `kind`/`resolved_params_json`/`retry_count`（doc 08/12/13/15）|
| **A13（新）plan_artifacts 缺版本校验字段** | 补 `toolchain_profile_hash`/`provider_lock_hash`/`tf_version_sha256`/`stack_id`/`state_key`/`sha256`/`size_bytes`/`pinned_commit`（doc 09 §5.2 + doc 12 invariant 0）|
| **A14（新）approval_flows 表缺失** | 新增 `approval_flows`（doc 12 §6 审批流 DSL 持久化）|
| **A15（新）outbox_events 枚举/列不全** | 枚举改 5 值 + 补 `event_id`/`next_retry_at`/`correlation_id`（doc 04 §2.8a）|
| **A16（新）requests.status CHECK 域缺失** | 列 19 值 CHECK（doc 00 §5 + doc 12a）|

### B 级（docs 不一致/短路，docs 全量通读后新增）

| 断裂 | 修复 | 来源 |
|---|---|---|
| B1 cloud_accounts 字段不全 | 对齐 doc 04 §2.12 权威（alias/display_name/billing_enabled/default_team_id/tags_json）| A6 |
| B2 catalog_items.status 缺 blocked | 5 值 CHECK（doc 19 §1）| A3 |
| B3 approval_node_runs.mode 漂移 | 对齐 proto ApprovalNodeMode（any/all/majority/quorum；quorum+N 表达 count>=N）| A5 |
| B4 audit_logs 缺 ai_metadata_json | 补列（doc 17 §9.2）| A8 |
| B5 resources 缺 tenant_id/status | 补列 + 新增 resource_relations（doc 07 §7 + doc 14 §2）| B9 |
| B6 finops_recommendations 缺 confidence | 补 confidence_score + 4 支撑字段（doc 14 §4.5）| B9 |
| B7 executor_runs 缺 failure_category | 补列（doc 18 §4 Phase 1 验收门）| B1 |
| B8 manual_intervention_tasks 字段不全 | 对齐 doc 04 §2.8a（source_type/severity/sla_deadline 等）| B6 |
| B9 import_jobs 缺 exemption + lifecycle/status CHECK | 补 3 豁免列 + CHECK（doc 15 §3.1）| B14 |
| B10 drift_records.resolution 缺第三路径 | 加 mark-failed-terminal（doc 21 RB-001）| B3 |
| B11 incidents 用 `...` 占位 | 展开全字段 + legal_hold/source_type（doc 04 §2.8b + doc 20）| B7 |
| B12 11 张表缺失 | 新增（见 audit-docs-sweep B-致命-1）| B1/B4/B7/B12/B15 |

### C 级（跨表链路断裂，docs 全量通读 + 链路审计后新增）

| 断裂 | 修复 | 来源 |
|---|---|---|
| C1 approval_runs 无 (request_id, gate) 唯一约束 | 加 partial unique `WHERE status='pending'`（防并发双 pre-apply race，doc 12 §3）| A5 |
| C2 drift_records 无 remediation_request_id | 加列 FK→requests nullable（闭合漂移修复追溯链，doc 13 §5.1）| B3 |
| C3 import_jobs 无 created_stack_id | 加列 FK→stacks nullable（闭合 import→stack 创建链，doc 15 §6 step 4）| B14 |
| C4 layer_migrations 缺 to_version_id 索引 | 补索引 | B8 |
| C5 MVP requester_id/env_id/tenant_id 悬挂 | 显式标注 MVP 边界（identities B4 / environments B11 非 MVP），Wave 落表后加 FK | A4 |
| **C6（2026-07-16）module_type 误植** | 删除 modules.module_type（registry 只接受 atomic；control 由平台 codegen 承担，declarative 由 Web 表单承担）| migration 011 |
| **C7（2026-07-16）state backend 硬编码** | 新增 state_backends 表（A9），替代 doc 02 §4.1 / doc 09 §6 硬编码 `bucket="tm-state"`；codegen 读 is_default 行渲染 backend.tf | migration 011 |
| **C8（2026-07-16）requests.pinned_commit 孤儿** | 新增 workspaces + workspace_checkouts 表（A9），pinned_commit 有归属表；重启 reconcile 有扫描目标（doc 10 §4）| migration 011 |
| **C9（2026-07-16）codegen 产出无持久化** | 新增 stacks + stack_dependencies 表（A9），codegen 输出 stack 身份（id/path/state_key/tags）落库；DB↔repo↔state 三方对账有数据源 | migration 011 |
| **C10（2026-07-16）组合模板缺失** | 设计定稿 catalog_blueprints + catalog_blueprint_items（B2，非 MVP）；一次申请多 atomic 的产品入口 + 参数映射，codegen 展开成 N stack | Wave 2 |
| **C11（2026-07-16）catalog_item_defaults 缺失** | 设计待定（per-env/per-team 默认值覆盖，S2 管道读）；当前 catalog_items.defaults_json 全局单一，prod/dev 差异无法表达 | Wave 2 |

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
