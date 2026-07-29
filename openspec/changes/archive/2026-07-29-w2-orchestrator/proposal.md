## Why

W2 编排引擎是 MVP 主干的第二关键模块——把 codegen（W2-05）生成的代码串到 Terramate 执行。没有编排引擎，codegen 的输出只是"文件"——没有人调 Terramate plan/apply，没有状态机驱动工单流转，没有审批门禁。

**影响层级**：业务核心层（`server/core/orchestrator/`），不改 DB schema / proto 契约。

**为什么现在做**：codegen（W2-05）已归档，TerramateAdapter（W1-01）已就位，requests 表 + request_events 表已落地（migration 005）。

## What Changes

### 1. 工单状态机（task 6.1）

新建 `server/core/orchestrator/`：
- **StateMachine**：19 状态 + 合法转换矩阵（doc 12a）
- Phase 1 主链路（简化，doc 00b）：
  ```
  submitted → generating → planning → plan_ready → pending_approval → applying → reconciling → succeeded
  ```
- 异常分支：`failed_retryable` / `failed_terminal` / `waiting_manual` / `cancelled` / `expired`
- Phase 1 **不做** `pending_admission`（准入审批，Phase 2 双门禁才加）
- 每步状态转换持久化 `request_events`（who/when/from/to/context）
- 乐观锁 `version`（防并发覆盖）

### 2. 流水线串接（task 6.2）

- **Pipeline**：工单各阶段执行逻辑
  - `generating`：调 codegen.Generator 生成 FileSet → 交给 workspace manager 写入 worktree（W2-07 实现，这里先接口占位）
  - `planning`：调 TerramateAdapter.RunPlan（terramate run terraform plan）
  - `pending_approval`：阻塞等待审批（Phase 1 单 pre-apply 门）
  - `applying`：调 TerramateAdapter.RunApplySavedPlan（terramate run terraform apply plan.tfplan）
  - `reconciling`：apply 后回写 + outbox 事件

### 3. 审批流程（Phase 1 简化，task 6.2 子项）

Phase 1 审批策略（doc 00b 权威）：
- **单 pre-apply 门**（不做准入审批 pre-plan）
- **多人或签**（任一审批人通过即可，不做会签/条件/超时）
- 审批人 = 资源 owner team 的成员（从 stacks.owner_team_id → teams）
- 审批 API：approve / reject（状态从 pending_approval → applying 或 rejected）
- Phase 2 才做：双门禁（pre-plan + pre-apply）+ 会签 + 条件 + 超时 + OPA 策略自动放行

### 4. OPA 策略占位（task 6.3）

- Phase 1 OPA 不接入真实策略评估（PolicyEngine 是 noop，W1-01）
- 预留策略评估钩子（plan 后 → policy evaluate → deny 阻断）
- Phase 2 接入真实 OPA

### 不做（本次范围外）

- 双门禁准入审批（Phase 2）
- 会签/条件/超时（Phase 2）
- OPA 真实策略评估（Phase 2）
- workspace git worktree 管理（W2-07 单独模块）
- plan artifact 持久化到对象存储（W2-07/08）
- 重启 reconcile 恢复（W2-07）
- Temporal 升级（Phase 3）

## Capabilities

### New Capabilities

- `request-orchestration`: 工单状态机（19 状态 + 合法转换）+ 流水线串接（codegen→plan→审批→apply→reconcile）+ Phase 1 单门审批（pre-apply 或签）

### Modified Capabilities

（无）

## Impact

- **代码**：新建 `core/orchestrator/`（state_machine + pipeline + approval_stub + provider）。不改现有代码。
- **API**：不新增 proto RPC（orchestrator 是内部编排，工单 API 在 W3 task 09）
- **依赖**：无新外部依赖（W1 TerramateAdapter + W2 codegen + W1 Repo）
- **DB**：不改 schema（用 requests + request_events + approval_* 表）
- **测试**：状态机转换矩阵测试 + 流水线 mock 测试（fake TerramateAdapter + fake codegen）
