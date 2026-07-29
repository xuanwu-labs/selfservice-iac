## Context

W2 编排引擎是 MVP 主干的编排核心——驱动工单从"提交"到"执行成功"。

**现有架构约束**（已落地）：
- requests 表（migration 005）：19 状态 + form_values_json + resolved_params_json + idempotency_key
- request_events 表（migration 005）：who/when/from/to/context
- approval_flows/runs/node_runs/decisions 表（migration 007）：审批四件套
- TerramateAdapter（W1-01）：Run/RunPlan/RunApplySavedPlan 接口
- codegen Generator（W2-05）：CodegenInput → FileSet
- plan_artifacts 表（migration 006）：plan 产物存储

**Phase 1 审批简化策略**（doc 00b 权威）：
- 单 pre-apply 门（不做准入审批 pre-plan）
- 多人或签（不做会签/条件/超时）
- Phase 2 才做双门禁 + 会签 + OPA 自动放行

## Goals / Non-Goals

**Goals:**
- StateMachine：19 状态 + 合法转换 + 非法转换报错
- Pipeline：各阶段执行逻辑（generating/planning/applying/reconciling）
- ApprovalStub：Phase 1 单门审批（pre-apply 或签）
- request_events 持久化（每步状态转换记录）
- 乐观锁（version 防并发）

**Non-Goals:**
- 不做双门禁准入审批（Phase 2）
- 不做 workspace git worktree（W2-07）
- 不做 plan artifact 对象存储（W2-07/08）
- 不做重启 reconcile（W2-07）
- 不做 OPA 真实策略（Phase 2）
- 不做 Temporal（Phase 3）

## Decisions

### D1：StateMachine 是纯函数（转换矩阵 + 校验）

**决策**：StateMachine 是纯函数——输入 (currentStatus, event) → 输出 (newStatus, error)。不查 DB、不调外部。

```go
func Transition(current string, event Event) (string, error)
```

**转换矩阵**（Phase 1 主链路 + 异常分支）：
```
submitted      +submit      → generating
generating     +gen_done    → planning
generating     +gen_fail    → failed_retryable
planning       +plan_done   → plan_ready
planning       +plan_fail   → failed_retryable
plan_ready     +request_approval → pending_approval
pending_approval +approve   → applying
pending_approval +reject    → rejected
pending_approval +timeout   → expired
applying       +apply_done  → reconciling
applying       +apply_fail  → failed_terminal
reconciling    +reconcile_done → succeeded
任意           +cancel      → cancelled
任意（除 succeeded/cancelled） +manual_intervention → waiting_manual
```

**理由**：纯函数可测试（表驱动测试覆盖所有合法/非法转换）；解耦状态逻辑与执行逻辑。

### D2：Pipeline 各阶段注入接口（不硬编码依赖）

**决策**：Pipeline 通过接口注入依赖，不硬编码：

```go
type Pipeline struct {
    codegen     CodeGenerator     // 接口（W2-05 Generator 实现它）
    terramate   TerramateRunner   // 接口（W1-01 TerramateAdapter 实现它）
    workspace   WorkspaceManager  // 接口（W2-07 实现它，MVP stub）
    repo        RequestRepo       // 接口（W1-02 Repo 实现它）
}
```

**理由**：可 mock 测试（fake codegen + fake terramate）；解耦编排与执行。

### D3：Phase 1 审批用简化 ApprovalService（不接 approval_flows DSL）

**决策**：Phase 1 审批用简化 ApprovalService——直接 approve/reject API，不走 approval_flows YAML DSL。

```go
type ApprovalService struct {
    repo ApprovalRepo  // 接口
}
func (s *ApprovalService) Approve(ctx, requestID, approverID) error  // pending_approval → applying
func (s *ApprovalService) Reject(ctx, requestID, approverID, reason) error // pending_approval → rejected
```

**理由**：
- approval_flows DSL 是 Phase 2（W4 task 12）
- Phase 1 只需"或签"（任一审批人通过即可）
- 不做会签/条件/超时/多级

**审批人**：从 stacks.owner_team_id → teams → team 成员列表（Phase 1 简化：team 的任何人都能审批）。

### D4：Pipeline 用同步执行（Phase 1 单进程）

**决策**：Phase 1 Pipeline 同步执行（generating → planning 阻塞等 TerramateAdapter 返回）。

**理由**：
- Phase 1 单进程（process 模式 Executor）
- River 队列已引入（go.mod），但 Phase 1 先同步
- Phase 2 改异步（River job per stage）

### D5：request_events 每步记录

**决策**：StateMachine 每次转换都写一条 request_events（who/when/from/to/context_json）。

**理由**：审计要求（doc 00 §5 "所有状态变化必须写 request_events"）；排障追溯。

## Risks / Trade-offs

- **[同步执行阻塞] → Phase 1 单进程可接受**：TerramateAdapter plan 可能耗时 30s-5min，同步阻塞但 Phase 1 容量小（< 50 stack / < 5 并发）。
- **[审批简化无 OPA] → Phase 2 补**：Phase 1 审批是人工或签（没有 OPA 自动放行低危）。Phase 2 加 OPA + 双门禁。
- **[workspace stub] → W2-07 实现**：Pipeline 的 workspace 接口 MVP 用 stub（返回固定路径），W2-07 补真实 go-git worktree。

## Migration Plan

1. 实现 StateMachine（转换矩阵 + Transition 纯函数）
2. 实现 Pipeline 接口（CodeGenerator/TerramateRunner/WorkspaceManager/RequestRepo）
3. 实现 Pipeline 各阶段（generating/planning/applying/reconciling）
4. 实现 ApprovalService（Approve/Reject 或签）
5. 实现 request_events 记录
6. 测试（状态机表驱动 + Pipeline mock）
7. `go build ./... && go vet ./... && go test ./server/core/orchestrator/...`

## Open Questions

- **Pipeline 是否需要 context 超时？** Phase 1 同步执行，建议每阶段加 context 超时（generating 60s，planning 10min，applying 30min）。
- **approval_decisions 表怎么写？** Phase 1 直接写 approval_decisions（decision=approved/rejected + approver_id + reason）。不走 approval_runs/node_runs（Phase 2 DSL）。
