# event-audit Specification

## Purpose
TBD - created by archiving change w3-api-rbac-identity. Update Purpose after archive.
## Requirements
### Requirement: 进程内事件分发 + append-only 审计日志

平台 MUST 在 `server/core/events/` 实现 EventBus（进程内直接函数调用，Phase 1 不做消息队列）+ AuditLogger（append-only 写 audit_logs 表，migration 009 已建）。EventBus.Publish(ctx, event) 同步遍历已注册 handlers 调用（handler 签名 `func(ctx, Event) error`）。AuditLogger.Log(ctx, actor, action, targetType, targetID, before, after) 写一行 audit_logs，表只允许追加（不更新/删除）。AuditLogger 注册为 EventBus 的 handler，关键操作（工单创建/状态转换/审批/鉴权失败）通过事件触发审计写入。

#### Scenario: Publish 调用所有注册 handlers

- **WHEN** 调用 `EventBus.Publish(ctx, event)` 且 bus 注册了 N 个 handler
- **THEN** 同步依次调用每个 handler(ctx, event)
- **AND** 任一 handler 返回 error 不中断后续 handler（记录错误继续）

#### Scenario: 审计日志记录 actor/action/target

- **WHEN** 关键事件（如 ApproveRequest 成功）经 EventBus 触发 AuditLogger
- **THEN** audit_logs 新增一行：actor=identity_id, action="approve", target_type="request", target_id=request_id
- **AND** before/after JSON 快照记录状态变化（from=pending_approval, to=applying）
- **AND** 记录 occurred_at 时间戳

#### Scenario: audit_logs append-only 不可变

- **WHEN** 尝试 UPDATE 或 DELETE audit_logs 已有行
- **THEN** 数据库拒绝（append-only 约束 / 无更新路径）
- **AND** 仅允许 INSERT 新行

