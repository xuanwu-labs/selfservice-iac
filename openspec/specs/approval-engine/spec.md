# approval-engine Specification

## Purpose
TBD - created by archiving change w3-api-rbac-identity. Update Purpose after archive.
## Requirements
### Requirement: Phase 1 单级审批引擎（Start/Decide/Status + 审批人解析）

平台 MUST 在 `server/core/approval/` 实现 Engine，扩展 W2-06 的 ApprovalService。三个接口：`Start(requestID)` 创建 approval_run（Phase 1 单节点，写入 approval_runs/approval_node_runs 表 migration 007 已建）；`Decide(approvalRunID, approverID, decision)` 写 approval_decisions + 状态转换（approved → 工单 applying；rejected → 工单 rejected）；`Status(requestID)` 查审批状态。审批人解析：从 request.owner_team 的 owner 角色成员（role_bindings）解析出 identities 列表（Phase 1 或签——任一 owner approve 即通过）。不做多级/会签/条件分支/超时升级/DSL（Phase 2）。

#### Scenario: Start 创建 approval_run

- **WHEN** 调用 `Engine.Start(requestID)` 且工单状态=`pending_approval`
- **THEN** 创建 approval_run（status=open，关联 requestID）
- **AND** 创建单节点 approval_node_run（type=single_approval）
- **AND** 解析 owner_team owners 为候选审批人 identities 列表

#### Scenario: Decide 批准触发工单 applying

- **WHEN** 调用 `Engine.Decide(approvalRunID, approverID, "approved")` 且 approverID 属于候选审批人列表
- **THEN** 写 approval_decisions（decision=approved, approver_id, ts）
- **AND** approval_run.status=closed（Phase 1 或签，单决策即结束）
- **AND** 工单状态从 pending_approval → applying

#### Scenario: Decide 拒绝触发工单 rejected

- **WHEN** 调用 `Engine.Decide(approvalRunID, approverID, "rejected")` 且 approverID 属于候选审批人列表
- **THEN** 写 approval_decisions（decision=rejected）
- **AND** approval_run.status=closed
- **AND** 工单状态从 pending_approval → rejected

#### Scenario: 非候选审批人 Decide 被拒

- **WHEN** 调用 `Engine.Decide(...)` 且 approverID 不在 owner_team owners 列表
- **THEN** 返回权限错误（不写 approval_decisions）
- **AND** approval_run 状态不变

