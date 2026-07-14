# 审计报告：docs 全量通读 × design.md §03 表清单

> 三 Explore agent 并行通读全部 24 个 docs（5107 行）+ 大提案 proposal/design/tasks（1224 行），逐 doc 提取表/列/枚举/FK/状态机/JSONB/审计字段/金额/状态机。本报告汇总与 design.md §03 当前 ~52 表设计的差距，按 **MVP（A1-A8）** 与 **非 MVP（B1-B14）** 分级，再按严重度（**致命/重要/规范**）排序。
>
> 验收目标：**MVP 完全支撑 W1 全过程**（design.md §03 A1-A8 落迁移即可跑通主链路）；**非 MVP 防短路**（B1-B14 字段齐全，落迁移时不返工）。

---

## 汇总

| 类别 | 致命（阻塞 MVP 闭环）| 重要（短路/返工风险）| 规范（缺索引/枚举值）|
|---|---|---|---|
| MVP（A1-A8）| 6 | 9 | 11 |
| 非 MVP（B1-B14）| 4 | 14 | 8 |
| **丢失表**（docs 有、design.md §03 无）| — | 11 张 | — |

---

## 一、MVP 致命断裂（阻塞 W1 主链路闭环）

### M-致命-1 ❌ `requests.status` 枚举不全（20 值，design.md 未列 CHECK 域）
**docs 权威**：doc 00 §5 + doc 12 §3.2 + doc 12a = **20 个状态**：
```
主路径 10：submitted, generating, pending-admission, planning, plan-ready,
          pending-approval, applying, reconciling, succeeded, reconcile-pending
异常 10： rejected, cancelled, expired, failed-retryable, failed-terminal,
          waiting-manual, blocked-policy, blocked-state-health, paused-drift,
          (+ reconcile-pending 已在主路径计)
```
**design.md §03 A4 当前**：`status` 列只标了列名，没列 CHECK 取值域。
**后果**：落迁移时若 CHECK 写错（漏 blocked-policy/blocked-state-health/paused-drift），W1 状态机跑不过——这是 W1 主链路的核心枚举。
**修复**：§03 A4 requests 列出 20 值 CHECK。

### M-致命-2 ❌ `requests` 缺 `form_hash` / `version` / `retry_count` / `kind`（doc 12a + doc 00 硬要求）
- `form_hash`：doc 12a IDEMP-001/002/003 的幂等判别字段（独立于 idempotency_key）。design.md 有 `form_hash` ✅。
- `version`：doc 00 §5 + doc 12a CONC-003 乐观锁，每个状态转换 `WHERE id=? AND status=? AND version=?`。design.md 有 `version` ✅。
- `retry_count`：doc 12 reject.terminal 判据（同 catalog+user+24h 拒绝 ≥3 次 → terminal）。design.md **缺** ❌。
- `kind`：doc 13/15 漂移修复/存量导入工单的判别列（`standard`/`drift-remediation`/`legacy-import`/`maintenance-apply`）。design.md **缺** ❌。
- `resolved_params_json`：doc 08 §4.2 MUST 存（每变量 source/rank）。design.md **缺** ❌——这是参数管道 provenance 的落点。
- `estimated_cost_cents` vs `cost_estimate_cents`：design.md 用 `cost_estimate_cents`，doc 12 用 `estimated_cost_cents`。**命名漂移**，需统一（design.md 已有 cost_estimate_cents，保留）。
**修复**：§03 A4 requests 补 `retry_count INT`、`kind TEXT CHECK`、`resolved_params_json JSONB`。

### M-致命-3 ❌ `plan_artifacts` 缺 plan/apply 版本一致性校验字段（doc 09 §5.2 + doc 12 invariant 0）
doc 09 §5.2 + doc 12 invariant 0 要求 plan artifact MUST 存：
- `pinned_commit`、`toolchain_profile_hash`、`provider_lock_hash`、`tf_version_sha256`、`stack_id`、`state_key`、`sha256`、`size_bytes`、`expires_at`、`storage_uri`、superseded 标志
**design.md §03 A4 当前**：`plan_artifacts(id, request_id, status, plan_hash, storage_uri, resources_to_add/change/destroy, cost_estimate_cents, expires_at, created_at)` —— 缺 `toolchain_profile_hash`/`provider_lock_hash`/`tf_version_sha256`/`stack_id`/`state_key`/`pinned_commit`/`sha256`/`size_bytes`。
**后果**：apply 阶段无法校验 plan 时与 apply 时工具链版本一致（D21 plan/apply 解耦的安全前提），MVP 主链路安全闭环断。
**修复**：§03 A4 plan_artifacts 补全 8 个字段。

### M-致命-4 ❌ MVP 缺 `approval_flows` 表（doc 12 §6 硬要求）
doc 12 §6 审批引擎**权威定义** 4 表：`approval_flows, approval_runs, approval_node_runs, approval_decisions`。
**design.md §03 A5 当前**：只有 `approval_runs, approval_node_runs, approval_decisions` —— **缺 approval_flows**（审批流定义表，存 DSL YAML + trigger + version + active）。
**后果**：MVP 审批引擎无流定义存储，DSL 无处持久化，W1 审批跑不通。
**修复**：§03 A5 新增 `approval_flows(id, name, trigger, dsl_yaml, version, active, created_at, updated_at)`。

### M-致命-5 ❌ `approval_runs` 缺 `gate` 列（D21 双门禁判别）
doc 12 §6 footnote：`approval_runs.gate` ∈ {`pre-plan`, `pre-apply`, `break-glass-retroactive`} 是 D21 双门禁的判别列。
**design.md §03 A5 当前**：approval_runs 列里**有 `gate`** ✅（已修复），但未列取值域。
**修复**：§03 A5 approval_runs.gate 列出 3 值 CHECK。

### M-致命-6 ❌ `outbox_events` 枚举不全 + 缺 correlation_id/next_retry_at
doc 04 §2.8a + doc 10 §8.1：outbox_events `status` ∈ {`pending, processing, succeeded, failed, dead-letter`}（5 值），且有 `correlation_id`、`next_retry_at`、`event_id`（幂等）。
**design.md §03 A8 当前**：`status[pending|processed|failed]`（3 值，缺 processing/succeeded/dead-letter），缺 `correlation_id`/`next_retry_at`/`event_id`。
**后果**：Saga outbox 状态机不完整，dead-letter 兜底丢失，W1 reconcile 链路断。
**修复**：§03 A8 outbox_events 枚举改 5 值 + 补 3 列。

---

## 二、MVP 重要短路（返工风险）

### M-重要-1 ⚠️ soft-delete policy 矛盾：design.md 用 deleted_at，docs 全用 status 枚举
**docs 全量扫描**（agent 1 明确）：**所有 4 个核心 docs（04/05/06/07）无任何 `deleted_at` 列**。soft-delete 一律用 status 枚举：
- `active|deprecated|archived`（catalog_items, layer_rule_set_versions）
- `active|frozen|deprecated`（environments, tenants）
- `active|disabled`（oidc_providers, run_hooks）
- `active|revoked|expired`（cloud_credentials）

**design.md §03 当前**：teams/catalog_items 用 `deleted_at`（§2.1 还专门写了 partial unique index 规则）。
**矛盾**：docs 的语义是"软删除 = 状态降级（deprecated/archived）"，design.md 的语义是"软删除 = 时间戳标记"。两者并存会导致：同一实体两种软删机制，查询时 `WHERE status='active' AND deleted_at IS NULL` 双重过滤，逻辑割裂。
**决策点（需定）**：
- 方案 A（贴合 docs，推荐）：MVP 表用 status 枚举表达软删，删 `deleted_at` 列。catalog_items 已有 status（active/deprecated/archived），teams 加 status 列。§2.1 partial unique index 规则改为"基于 status 的 partial index `WHERE status='active'`"。
- 方案 B（保留 deleted_at，偏离 docs）：保留 deleted_at，docs 视为"未覆盖"，design.md 自定。需在 §01 写明偏离理由。
**影响范围**：teams, catalog_items,（非 MVP 的 environments/tenants/oidc_providers/cloud_accounts 等若用 deleted_at 也要统一）。
**修复**：用户决策后统一。倾向方案 A（与 docs 一致，减少短路）。

### M-重要-2 ⚠️ `cloud_accounts` 字段不全（doc 04 §2.12 权威）
doc 04 §2.12（doc 06 footnote 指向其为权威）：cloud_accounts 字段 = `id, provider, account_id, alias, display_name, default_region, credentials_ref, billing_enabled, default_team_id, tags_json, oidc_trust_configured, bootstrap_status, created_at`。
**design.md §03 A6 当前**：`cloud_accounts(id, provider, account_id, name, status, regions_json, default_region, credentials_ref, bootstrap_status, oidc_trust_configured, created_at, updated_at)` —— 缺 `alias`/`display_name`/`billing_enabled`/`default_team_id`/`tags_json`；`name` vs `alias`/`display_name` 命名漂移；`regions_json`（design）vs doc 无（doc 只有 default_region）。
**修复**：§03 A6 对齐 doc 04 权威字段。

### M-重要-3 ⚠️ `teams` 缺 status 列（若采 M-重要-1 方案 A）
teams 当前 `kind[platform|dba|middleware|business]` 无 status。若软删走 status 枚举，teams 需加 `status[active|deprecated]`。
**修复**：随 M-重要-1 决策。

### M-重要-4 ⚠️ `audit_logs` 缺 ai 扩展列（doc 17 §9.2）
doc 17 §9.2：audit_logs 扩展 `actor_type` 加 `ai` 值（design.md 已有 [human|ai|system] ✅）+ 新增 `ai_metadata_json`（prompt_hash/prompt_length/skill_name/skill_version/llm_model/tool_calls_json/confidence_score）。
**design.md §03 A8 当前**：缺 `ai_metadata_json`。
**MVP 影响**：W1 若含 AI 入口（doc 00 §4.1 source 含 `ai`），审计链断。若 W1 不含 AI，可推迟到非 MVP。但列预留无害。
**修复**：§03 A8 audit_logs 补 `ai_metadata_json`（nullable，仅 actor_type=ai 时填）。

### M-重要-5 ⚠️ JSONB GIN 索引未在 MVP 表全量标注
agent 1 明确标注需 GIN 的 MVP 字段：
- `catalog_items.visibility_json`（按团队过滤目录高频）✅ design.md 已标
- `catalog_items.user_allowed_tag_keys_json`（白名单过滤）❌ 未标
- `requests.resolved_params_json`（provenance 查询，但主要是整体读，GIN 可选）
**修复**：§03 A3 补 user_allowed_tag_keys_json 的 GIN 标注。

### M-重要-6 ⚠️ MVP 表的 FK 索引未逐表标注
§1.3 已立"每 FK 必索引"硬规则，但 §03 A1-A8 各表的 FK 列后**未逐个标 `ix_<table>_<fk>`**。落迁移时易漏。
**修复**：§03 每张 MVP 表的 FK 列后标注索引（或在表块末尾统一列索引清单）。

### M-重要-7 ⚠️ `requests.idempotency_key` 的 24h 窗口语义未说明
doc 00 §3 + doc 12 invariant 0a：Web/CLI 幂等 = `sha256(actor + catalog + form_hash + 24h_window)`。design.md 有 idempotency_key 但没说明 24h 窗口如何进 hash（是按天取整还是 TTL 过期）。
**修复**：§03 A4 加注释说明 idempotency_key 构造（24h 窗口 = 按提交时间的小时桶或 UTC 日期）。

### M-重要-8 ⚠️ `approval_node_runs.mode` 取值域
doc 12 §2.3：mode ∈ {`any, all, majority, count>=N`}。design.md 列了 mode 但没列取值域。
**修复**：§03 A5 approval_node_runs.mode 列 CHECK。

### M-重要-9 ⚠️ `catalog_items.status` 缺 `blocked` 值
doc 19 §1：catalog_items.status ∈ {`draft, active, deprecated, archived, blocked`}（5 值，blocked = 安全/合规/工具链问题）。design.md §03 A3 只标了 status 列名没列域，但 doc 04 用 {active|deprecated|archived}（3 值）。
**修复**：§03 A3 catalog_items.status 列 5 值 CHECK（对齐 doc 19）。

---

## 三、MVP 规范缺失（缺索引/枚举/CHECK）

| # | 项 | 修复 |
|---|---|---|
| M-规范-1 | requests.status 列名有但无 CHECK 域 | §03 A4 列 20 值 |
| M-规范-2 | request_events.event_type 无域 | 列值（state_transition/log/error）|
| M-规范-3 | gate_results 无 status/severity 域 | 列值 |
| M-规范-4 | cloud_accounts.provider 无 CHECK | 列 [alicloud\|aws\|azure] |
| M-规范-5 | cloud_accounts.bootstrap_status 无 CHECK | 列 [ok\|rotate\|none] |
| M-规范-6 | modules.status 无 CHECK | 列 [pending_validation\|validated\|validation_failed\|deprecated] |
| M-规范-7 | catalog_items.cardinality 已有域 ✅ | — |
| M-规范-8 | catalog_items.stack_grouping 无 CHECK | 列 [per-component\|per-bundle\|per-team\|custom] |
| M-规范-9 | approval_decisions.decision 无 CHECK | 列 [approve\|reject\|abstain] |
| M-规范-10 | approval_runs.status 无 CHECK | 列 [pending\|approved\|rejected\|timeout\|cancelled] |
| M-规范-11 | outbox_events.event_id（幂等）缺 | 补列 + UNIQUE |

---

## 四、非 MVP 致命短路（B1-B14 落迁移时返工）

### B-致命-1 ❌ 丢失 11 张表（docs 有、design.md §03 无）

| # | 表 | 来源 doc | 用途 | 归属 B 段 |
|---|---|---|---|---|
| 1 | `identity_sources` | doc 05 §5 | 目录同步源配置（oidc/scim/feishu/dingtalk）| B4 扩展 |
| 2 | `org_nodes` | doc 05 §5 | 同步的组织树（自引用 parent_id）| B4 扩展 |
| 3 | `org_mappings` | doc 05 §5 | 组织→团队/角色映射规则 | B4 扩展 |
| 4 | `sync_runs` | doc 05 §5 | 目录同步执行记录 | B4 扩展 |
| 5 | `toolchain_manifest` | doc 11 §6 | 节点工具链版本真相源（DB+节点双写）| B1 扩展 |
| 6 | `tag_policy_versions` | doc 04 §2.14 | tag_policies 版本化子表 | B12 扩展 |
| 7 | `ai_prompts` | doc 17 §9.2 | AI prompt 加密存储（按 hash 查）| 新增 B15 |
| 8 | `skills` | doc 17 §6 | 声明式 AI skills（builtin + team-custom）| 新增 B15 |
| 9 | `service_accounts` (+`service_account_keys`) | doc 17 §3 | AK/SK 机器身份 + 双 AK 轮换 | 新增 B15 |
| 10 | `signing_keys` | doc 20 §3.2 | HMAC 审计链签名密钥轮换 | B7 扩展 |
| 11 | `runbooks` | doc 21 §1 | 9 个 runbook 种子引用表 | B7 扩展 |

**修复**：§03 新增 B15（AI/机器身份：ai_prompts, skills, service_accounts, service_account_keys）；B4 扩展（identity_sources, org_nodes, org_mappings, sync_runs）；B1 扩展（toolchain_manifest）；B7 扩展（signing_keys, runbooks）；B12 扩展（tag_policy_versions）。

### B-致命-2 ❌ `resources` 缺 `tenant_id` + `status`（doc 07 §7 + doc 14 §2）
- doc 07 §7：resources 加 `tenant_id` 反规范化（租户级成本归集）。design.md §03 B9 resources **缺 tenant_id** ❌。
- doc 14 §2：resources 有 `status[active|drifted|orphan|destroyed]`。design.md **缺 status** ❌。
- doc 14 §2：resources 有 `resource_relations` 子表（source/target/relation_type）。design.md **缺** ❌。
**修复**：§03 B9 resources 补 tenant_id/status + 新增 resource_relations 表。

### B-致命-3 ❌ `finops_recommendations` 缺 confidence_score + 支撑字段（doc 14 §4.5）
doc 14 §4.5：rightsize 建议必须带 `confidence_score[low|medium|high]` + `metric_source` + `utilization_summary_json` + `data_days`/`sample_count`。
**design.md §03 B9 当前**：finops_recommendations 只有 `kind/resource_id/detail_json/estimated_saving_cents/status/created_at`。
**修复**：§03 B9 finops_recommendations 补 confidence_score/metric_source/utilization_summary_json/data_days/sample_count。

### B-致命-4 ❌ `audit_logs` HMAC 链字段缺失（doc 20 §2，合规客户必需）
doc 20 §2（合规可选模块）：audit_logs 启用 HMAC 链时需 `prev_hash, entry_hash, signing_key_id, sealed_at`。
**design.md §03 A8 当前**：无。
**MVP 影响**：MVP 不启用 HMAC 链（默认 append-only），但列预留或标注"合规启用时加列"。
**修复**：§03 A8 audit_logs 标注"HMAC 链字段（合规启用，doc 20 §2）：prev_hash/entry_hash/signing_key_id/sealed_at"。

---

## 五、非 MVP 重要短路

### B-重要-1 ⚠️ `executor_runs` 缺 failure_category（doc 18 §4 Phase 1 验收门）
doc 18 §6：`failed_run_reason` 结构化字段是 **Phase 1 验收门**（dashboard 必需）。值域：`user_input|policy_denied|cloud_quota|cloud_api|toolchain|state_backend|platform_bug|manual_required`。
**修复**：§03 B1 executor_runs 补 `failure_category`。

### B-重要-2 ⚠️ `manual_intervention_tasks` 字段不全（doc 04 §2.8a）
doc 04 §2.8a：mit 字段 = `source_type, source_id, reason_code, owner_team_id, assignee_id, severity, status, context_json, recovery_action_json, sla_deadline, correlation_id`。
**design.md §03 B6 当前**：`request_id, task_type, description, status, assigned_to, resolution_notes` —— 缺 `source_type/source_id/severity/sla_deadline/recovery_action_json/reason_code`。
**修复**：§03 B6 manual_intervention_tasks 对齐 doc 04。

### B-重要-3 ⚠️ `import_jobs` 缺 exemption 字段（doc 15 §3.1）
doc 15 §3.1：managed-readonly 30/60/90 天升级 + 豁免机制，需 `exemption_expires_at, exemption_reason, exemption_review_cycle`。
**修复**：§03 B14 import_jobs 补 3 列。

### B-重要-4 ⚠️ `drift_records` 第三种解决路径缺失（doc 21 RB-001）
doc 21 RB-001 step 5：apply 中断的解决路径含 `mark-failed-terminal`（除 adopt-cloud/restore-desired 外）。
**修复**：§03 B3 drift_records.resolution CHECK 加 `mark-failed-terminal`。

### B-重要-5 ⚠️ `cicd_triggers` 缺 created_at + gate_events 缺 Phase 2 状态（doc 16）
doc 16 §3.1.2：gate_events.status Phase 2 加 `policy_blocked|needs_replan|manual_intervention_required|reconcile_pending`。cicd_triggers 缺 created_at。
**修复**：§03 B13 补全。

### B-重要-6 ⚠️ `cloud_credentials` 命名对齐（doc 04 权威 vs doc 06 漂移）
doc 06 有 typo `guardisans`（应 `guardians_json`）；doc 04 有 `bundle_id`（doc 06 漏）。design.md §03 B10 需用 doc 04 权威。
**修复**：§03 B10 cloud_credentials 补 bundle_id，guardians_json 拼写确认。

### B-重要-7 ⚠️ `team_cloud_grants.budget_quota_cents`（doc 04 vs doc 06 漂移）
doc 06 写 `budget_quota`，doc 04 写 `budget_quota_cents`。design.md §03 B10 已用 `_cents` ✅。补 `bundle_id`（doc 04 有，doc 06 漏）。

### B-重要-8 ⚠️ `incidents` 缺 legal_hold + source_type（doc 20 §3.1）
doc 20 §3.1：incidents 需 `legal_hold` + `source_type[request|stack|executor|state|security|platform]`。design.md §03 B7 incidents 用 `...` 占位。
**修复**：§03 B7 incidents 展开全字段。

### B-重要-9 ⚠️ `platform_scorecards` 缺 confidence/dimensions（doc 19 §3.1）
doc 19 §3.1：scorecards 需 `confidence[low|medium|high], sample_size, data_completeness, missing_dimensions_json` + 6 维度子分。
**修复**：§03 B7 platform_scorecards 展开。

### B-重要-10 ⚠️ `approval_node_runs.mode` 枚举值漂移
doc 12 §2.3：`any|all|majority|count>=N`。doc 04 §2.10：`single|countersign|conditional`。**两 doc 不一致**。
**修复**：采 doc 12（审批引擎权威）的 `any|all|majority|count>=N`。

### B-重要-11 ~ B-重要-14 ⚠️ 其余字段展开（B7 的 run_hooks/run_hook_results/drill_results/catalog_health_checks 用 `...` 占位，需展开全字段对齐 doc 04 §2.8b）

---

## 六、跨域逻辑断裂（短路风险）

### L-1 ⚠️ codegen 输出契约未落表（doc 09 §5.1）
doc 09 §5.1：codegen Phase 0 输出契约（ResolvedParams/PathGeneratorOutput/GeneratedFile）含 `content_sha256`（确定性 golden test）、`stack_id`、`terramate_tags` 等。这些字段部分落 requests（resolved_params_json/pinned_commit），部分落 stacks（repo_path/state_key/stack_id/terramate_tags_json）。
**design.md §03 B2 stacks 已有 stack_id/repo_path/state_key/terramate_tags_json ✅**。但 `content_sha256`（生成文件哈希，golden test 用）无落点。
**修复**：可选新增 `codegen_artifacts` 表（或落 request_events）。非 MVP，记一笔。

### L-2 ⚠️ `requests.kind` 与 drift/import 工单的衔接（doc 13/15）
doc 13：漂移修复复用 requests 表，`requests.kind = drift-remediation`。doc 15：存量导入 `requests.kind = legacy-import`。
**design.md §03 A4 requests 缺 kind 列**（M-致命-2 已列）。补 kind 后，drift/import 工单与标准工单同表，靠 kind 判别。

### L-3 ⚠️ gate_status 是 requests.status 的投影，不是新枚举（doc 16 §3.1.1）
doc 16 §3.1.1 明确：gate_events.status 是 requests.status 的投影，**不得反向扩展 RequestLifecycle 枚举**。
**design.md 影响**：gate_events.status 的取值域（pending/admission_approved/...）独立于 requests.status，不要混。design.md §03 B13 gate_events 已独立 ✅。

### L-4 ⚠️ 两套幂等公式不能混淆（doc 00 §3 vs doc 16 §3.1）
- Web/CLI/AI：`sha256(actor + catalog + form_hash + 24h_window)` → requests.idempotency_key
- CICD：`sha256(pipeline:commit:catalogItem:form_hash)` → cicd_triggers.idempotency_key（无时间窗）
**design.md**：requests.idempotency_key（M-重要-7）+ cicd_triggers.idempotency_key（B13）两套独立 ✅，但需在 design.md 注明公式差异。

---

## 七、整改优先级（apply 顺序）

### 第一批（MVP 致命 + 重要，W1 必须先解决）
1. **M-致命-1/2/3/4/5/6**：requests 枚举/列补全、plan_artifacts 版本字段、approval_flows 新增、outbox 枚举
2. **M-重要-1**：soft-delete policy 决策（deleted_at vs status）—— **需用户拍板**
3. **M-重要-2~9** + **M-规范-1~11**：字段对齐 + 枚举 CHECK + FK 索引标注

### 第二批（非 MVP 致命，B 段落迁移前必须解决）
4. **B-致命-1**：新增 11 张丢失表
5. **B-致命-2/3/4**：resources/finops/audit_logs 字段补全

### 第三批（非 MVP 重要 + 规范）
6. **B-重要-1~14** + **L-1~4**

---

## 决策点（需用户拍板，唯一阻塞项）

**M-重要-1 soft-delete policy**：
- **方案 A（推荐，贴合 docs）**：MVP 表用 status 枚举表达软删（catalog_items 已有 status；teams 加 status），删 deleted_at 列。§2.1 partial unique index 规则改为 `WHERE status='active'`。
- **方案 B（保留 deleted_at）**：保留 deleted_at，与 docs 的 status 枚举并存，design.md §01 写明偏离理由。

其余项（MVP 致命 6 项 + 非 MVP 致命 4 项 + 重要/规范）均可按报告直接 apply，不需决策。
