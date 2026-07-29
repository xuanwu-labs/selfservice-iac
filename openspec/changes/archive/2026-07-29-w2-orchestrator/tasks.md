## 1. StateMachine（task 6.1）

- [x] 1.1 实现 `server/core/orchestrator/state_machine.go`：Transition(current, event) → (new, error) 纯函数。Phase 1 主链路转换矩阵：submitted→generating→planning→plan_ready→pending_approval→applying→reconciling→succeeded + 异常分支（failed_retryable/failed_terminal/rejected/cancelled/expired/waiting_manual）
- [x] 1.2 实现 `server/core/orchestrator/state_machine_test.go`：表驱动测试覆盖所有合法转换（~15 个）+ 非法转换报错（~10 个）

## 2. Pipeline 接口 + 各阶段（task 6.2）

- [x] 2.1 实现 `server/core/orchestrator/pipeline.go`：Pipeline 结构体 + 接口注入（CodeGenerator/TerramateRunner/WorkspaceManager/RequestRepo/EventLogger）。Execute(ctx, requestID) 主入口：按当前状态执行对应阶段
- [x] 2.2 实现 generating 阶段：调 codegen.Generate → FileSet → 调 workspace.WriteFiles（stub 接口，W2-07 实现）→ 状态 generating→planning
- [x] 2.3 实现 planning 阶段：调 terramate.RunPlan → plan 产出 → 状态 planning→plan_ready
- [x] 2.4 实现 applying 阶段：调 terramate.RunApplySavedPlan → 状态 applying→reconciling
- [x] 2.5 实现 reconciling 阶段：回写（outbox 事件占位）→ 状态 reconciling→succeeded

## 3. 审批流程 Phase 1（单 pre-apply 门 或签）

- [x] 3.1 实现 `server/core/orchestrator/approval.go`：ApprovalService。Approve(ctx, requestID, approverID) → pending_approval→applying；Reject(ctx, requestID, approverID, reason) → pending_approval→rejected。写 approval_decisions 表
- [x] 3.2 审批人校验：approverID 必须属于 request 的 owner_team（Phase 1 简化：team 任何人都能审批）

## 4. request_events 记录

- [x] 4.1 实现 `server/core/orchestrator/events.go`：EventLogger 接口 + 实现。每次状态转换写一条 request_events（request_id/from_status/to_status/actor/context_json/occurred_at）

## 5. wire + 验证

- [x] 5.1 实现 `server/core/orchestrator/provider.go`：wire ProviderSet
- [x] 5.2 更新 `server/core/core.go`：加 orchestrator.ProviderSet
- [x] 5.3 `go build ./... && go vet ./...` 通过
- [x] 5.4 `go test ./server/core/orchestrator/... -short` 通过
- [x] 5.5 `gofmt -l server/` 无输出
- [x] 5.6 提交到 `feat/w2-orchestrator` 分支

## 6. 测试

- [x] 6.1 实现 `server/core/orchestrator/pipeline_test.go`：用 fake CodeGenerator + fake TerramateRunner + fake WorkspaceManager 测流水线（generating→planning→plan_ready→pending_approval→applying→succeeded 全链路）
- [x] 6.2 实现 `server/core/orchestrator/approval_test.go`：Approve/Reject 状态转换 + 非审批人拒绝
