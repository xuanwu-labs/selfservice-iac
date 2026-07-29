# request-orchestration Specification

## Purpose
TBD - created by archiving change w2-orchestrator. Update Purpose after archive.
## Requirements
### Requirement: 工单状态机（19 状态 + 合法转换）

平台 MUST 在 `server/core/orchestrator/` 实现 StateMachine。StateMachine 是纯函数 `Transition(current, event) → (new, error)`，不查 DB。Phase 1 主链路：`submitted→generating→planning→plan_ready→pending_approval→applying→reconciling→succeeded`。异常分支：`failed_retryable`/`failed_terminal`/`rejected`/`cancelled`/`expired`/`waiting_manual`。非法转换 MUST 返回错误。

#### Scenario: 主链路合法转换

- **WHEN** Transition("submitted", SubmitEvent)
- **THEN** 返回 "generating", nil
- **AND** Transition("generating", GenDoneEvent) 返回 "planning", nil
- **AND** Transition("planning", PlanDoneEvent) 返回 "plan_ready", nil

#### Scenario: 非法转换报错

- **WHEN** Transition("succeeded", SubmitEvent)
- **THEN** 返回 "", error（succeeded 是终态，不能转换）

#### Scenario: 异常分支

- **WHEN** Transition("generating", GenFailEvent)
- **THEN** 返回 "failed_retryable", nil

### Requirement: 流水线串接（codegen→plan→审批→apply→reconcile）

平台 MUST 实现 Pipeline，通过接口注入依赖（CodeGenerator/TerramateRunner/WorkspaceManager/RequestRepo/EventLogger）。Execute(ctx, requestID) 按当前状态执行对应阶段，每阶段完成后状态机转换 + request_events 记录。

#### Scenario: generating 阶段执行

- **WHEN** Pipeline.Execute(ctx, requestID) 且当前状态 = "generating"
- **THEN** 调 CodeGenerator.Generate → FileSet
- **AND** 调 WorkspaceManager.WriteFiles（stub 或真实实现）
- **AND** 状态转换为 generating→planning
- **AND** 写一条 request_events

#### Scenario: planning 阶段执行

- **WHEN** 当前状态 = "planning"
- **THEN** 调 TerramateRunner.RunPlan
- **AND** 状态转换为 planning→plan_ready

#### Scenario: applying 阶段执行

- **WHEN** 当前状态 = "applying"（审批通过后）
- **THEN** 调 TerramateRunner.RunApplySavedPlan
- **AND** 状态转换为 applying→reconciling→succeeded

### Requirement: Phase 1 单门审批（pre-apply 或签）

平台 MUST 实现 ApprovalService。Phase 1 单 pre-apply 门：`pending_approval` 状态时，owner_team 的任一成员可 Approve（→applying）或 Reject（→rejected）。不做会签/条件/超时/多级（Phase 2）。

#### Scenario: 审批通过

- **WHEN** ApprovalService.Approve(ctx, requestID, approverID) 且 approverID 属于 owner_team
- **THEN** 写 approval_decisions（decision=approved）
- **AND** 状态从 pending_approval → applying

#### Scenario: 审批拒绝

- **WHEN** ApprovalService.Reject(ctx, requestID, approverID, reason)
- **THEN** 写 approval_decisions（decision=rejected）
- **AND** 状态从 pending_approval → rejected

#### Scenario: 非审批人被拒绝

- **WHEN** approverID 不属于 owner_team
- **THEN** 返回权限错误

### Requirement: request_events 审计记录

平台 MUST 在每次状态转换写一条 request_events（request_id/from_status/to_status/actor/context_json/occurred_at）。

#### Scenario: 状态转换记录

- **WHEN** 状态从 generating 转为 planning
- **THEN** request_events 新增一行：from=generating, to=planning, actor=system

