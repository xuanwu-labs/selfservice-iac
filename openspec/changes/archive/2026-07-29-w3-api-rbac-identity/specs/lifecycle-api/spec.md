## ADDED Requirements

### Requirement: LifecycleService handler（工单 CRUD + plan/apply/approve/reject 薄包装 orchestrator）

平台 MUST 在 `server/api/connect/lifecycle.go` 实现 LifecycleService handler（proto 已定义）。handler 是薄包装：每个 RPC（CreateRequest / GetRequest / ListRequests / CancelRequest / StartPlan / StartApply / ApproveRequest / RejectRequest）注入 Pipeline（orchestrator W2-06）+ ApprovalEngine + RequestRepo，只做 proto↔Go 转换 + 鉴权透传，业务逻辑下沉 orchestrator。CreateRequest 同步建单（状态 `submitted`）后异步推进（调 Pipeline.Execute）。

#### Scenario: CreateRequest 同步建单触发 pipeline

- **WHEN** 调用 `CreateRequest(catalog_item_id, input_values, owner_team_id)` 且鉴权通过（subject 有 `request` action）
- **THEN** 同步写入 requests（status=`submitted`，actor=resolved identity）
- **AND** 异步触发 Pipeline.Execute（状态转换 submitted→generating→planning）
- **AND** 返回 request_id

#### Scenario: ApproveRequest 使状态转入 applying

- **WHEN** 调用 `ApproveRequest(request_id)` 且当前状态=`pending_approval` 且 approver 属于 owner_team
- **THEN** 调 ApprovalEngine.Decide（decision=approved）
- **AND** 状态从 `pending_approval` → `applying`
- **AND** 异步触发 Pipeline.Execute 进入 applying 阶段

#### Scenario: GetRequest/ListRequests 透传 orchestrator 状态

- **WHEN** 调用 `GetRequest(request_id)` 或 `ListRequests(filter)`
- **THEN** 从 RequestRepo 读取工单 + 当前状态 + 最近 request_events
- **AND** 返回 proto Response（无状态副作用）
