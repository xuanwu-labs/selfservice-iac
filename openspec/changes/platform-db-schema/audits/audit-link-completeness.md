# 审计报告：跨表链路完整性（link completeness）

> 逐链验证 design.md §03 ~68 表的跨表引用是否闭合（无悬挂引用、无缺失 join 列）。对每条链路回答："平台业务逻辑要求的这条链，表结构能否支撑？哪一列/哪张表缺？"
>
> 这是 **完备性 + 链路串联性** 审计（不是 skill 规范审计，不是枚举/CHECK 审计——那两轮已做）。

## 汇总

| 链路 | 状态 | MVP 严重度 | 缺失 |
|---|---|---|---|
| 1 Request→Plan→Approval→Apply 溯源 | **BROKEN** | RISK（非 BLOCKER）| requester_id 无 FK 目标（identities 是 B4 非 MVP）|
| 2 Catalog→Module→Version→Dependency | CLOSED | OK | — |
| 3 Layer 身份链（D24/D26）| PARTIAL | OK | MVP 经 requests.layer_rule_set_version_id 闭合；stacks 属 B2 |
| 4 MVP requests 的 env/tenant | **BROKEN** | RISK | env_id/tenant_id 无 FK（environments/tenants 是 B11 非 MVP）|
| 5 Team 归属链 | CLOSED | OK | — |
| 6 审批双门禁（D21）| **PARTIAL** | RISK | 缺 uq_approval_runs_(request_id, gate) 唯一约束 |
| 7 Stack→Binding→VPC（D27）| CLOSED | OK（非 MVP）| — |
| 8 CMDB→成本归集（D18）| CLOSED | OK（非 MVP）| — |
| 9 凭据注入（D23）| PARTIAL | OK（intentional）| credential_session_id 是 ephemeral，故意无 FK |
| 10 Drift→Request→Stack | **BROKEN** | RISK（非 MVP）| drift_records 无 remediation_request_id；requests 无 drift_record_id |
| 11 Import→Stack | **BROKEN** | RISK（非 MVP）| import_jobs 无 created_stack_id |
| 12 Layer 迁移（D26）| CLOSED | OK（非 MVP）| minor: 缺 ix_layer_migrations_to_version_id |
| 13 Saga/outbox | PARTIAL | RISK（非 MVP）| MIT/reconcile 无 FK 指向 outbox（多态设计，可接受）|
| 14 审计 correlation | PARTIAL | RISK（soft）| correlation_id 是约定非 FK，6 表 nullable |

## 需 apply 修复（4 项，明确 docs 要求）

### L-修复-1：approval_runs 加 (request_id, gate) 唯一约束（链 6）
**doc 12 §3 L136**："两道门，不是一个审批"——一个 request 每个 gate 至多一个 active run。
**缺失**：design.md A5 approval_runs 无 `uq_approval_runs_(request_id, gate)`。
**后果**：并发下可能插入两条 pre-apply run，状态机 race。
**修复**：A5 加 `uq_approval_runs_(request_id, gate)_active WHERE status='pending'`（partial unique，只约束 pending 态，允许历史 completed run 共存）。

### L-修复-2：drift_records 加 remediation_request_id（链 10）
**doc 13 §5.1 L95**："apply 成功 → 关闭 drift_record"——漂移修复创建 request（kind=drift-remediation），但 drift_records 无列指向该 request，无法 JOIN 追溯。
**缺失**：drift_records 无 `remediation_request_id FK→requests nullable`。
**修复**：B3 drift_records 加列。

### L-修复-3：import_jobs 加 created_stack_id（链 11）
**doc 15 §6 L153**：import 第 4 步"创建 stack + managed-readonly"——但 import_jobs 无列记录创建的 stack。
**缺失**：import_jobs 无 `created_stack_id FK→stacks nullable`。
**修复**：B14 import_jobs 加列。

### L-修复-4：layer_migrations 补 to_version_id 索引（链 12 minor）
**修复**：B8 layer_migrations 加 `ix_layer_migrations_to_version_id`。

## 需 design.md 显式标注的 MVP 边界（2 项，非 bug）

### L-边界-1：requester_id 在 MVP 是悬挂字符串（链 1）
**事实**：identities 表属 B4（非 MVP），MVP 无用户身份表。
**doc 依据**：docs/05 §5 identities（非 MVP Wave 2-3）。
**处理**：design.md A4 requests 标注 `requester_id TEXT`（MVP 悬挂字符串，Wave 2-3 identities 落地后加 FK）。同理 request_events.actor_id / approval_decisions.approver_id / audit_logs.actor_id。
**非 bug**：MVP 鉴权在网关层完成，requester_id 是外部 SSO subject，平台不 own 用户身份。

### L-边界-2：env_id/tenant_id 在 MVP 是悬挂字符串（链 4）
**事实**：environments/tenants 属 B11（非 MVP Wave 6）。
**doc 依据**：docs/07 L108 Phase 1 用固定字符串 `platform-default` 单租户。
**处理**：design.md A4 requests 标注 `env_id/tenant_id TEXT`（MVP 悬挂，Phase 1 tenant 恒为 `platform-default`，Wave 6 落 environments/tenants 后加 FK）。
**非 bug**：D27 明确 Phase 1 单租户，env/tenant 表延后。

## 按设计 intentional（3 项，非 bug，不改）

- **链 9** credential_session_id 是 OIDC ephemeral STS，故意不入库（D23 凭据不落盘）。
- **链 13** outbox/MIT/reconcile 是多态引用（aggregate_type+aggregate_id），故意无 FK（doc 04 §2.8a 设计）。
- **链 14** correlation_id 是 trace 约定（业界标准，非 FK），doc 00 §"可追踪默认"要求 app 层贯穿。
