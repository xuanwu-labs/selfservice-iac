# workspace-governance Specification

## Purpose
TBD - created by archiving change w2-workspace. Update Purpose after archive.
## Requirements
### Requirement: WorkspaceManager（go-git worktree 管理 + codegen 写入 + commit）

平台 MUST 在 `server/core/workspace/` 实现 WorkspaceManager，满足 orchestrator.WorkspaceManager 接口。CloneOrFetch 克隆/更新工作仓库到本地；WriteFiles 创建 worktree + 写入 codegen FileSet + git commit + 返回 pinned_commit；CheckoutWorktree 按 pinned_commit 检出 worktree（plan/apply 用）；ReleaseWorktree 释放。

#### Scenario: codegen 写入 + commit

- **WHEN** WriteFiles(ctx, requestID=123, files=FileSet{...})
- **THEN** 创建 worktree req-123-generate（共享 .git）
- **AND** 写入 FileSet 到 worktree 目录
- **AND** git add + commit
- **AND** 返回 commit SHA（pinned_commit）

#### Scenario: checkout pinned_commit

- **WHEN** CheckoutWorktree(ctx, requestID=123, pinned_commit="abc123")
- **THEN** 在 worktree 内 git checkout abc123
- **AND** 返回 worktree 绝对路径

#### Scenario: 并发隔离

- **WHEN** 工单 123 和 124 同时 WriteFiles
- **THEN** 各占独立 worktree（req-123-generate/ 和 req-124-generate/）
- **AND** 互不干扰

### Requirement: Checkout 租约（workspace_checkouts 并发锁）

平台 MUST 实现 CheckoutLease 管理 workspace_checkouts 表。Acquire 分配 worktree + leased_by_request_id 锁（同一工单不重复分配）。Release 释放锁 + 标 released。并发隔离保证同一 workspace 不同工单各占独立 worktree。

#### Scenario: Acquire 租约

- **WHEN** Acquire(ctx, workspaceID=1, requestID=123, purpose="plan_apply")
- **THEN** workspace_checkouts 新增行（leased_by_request_id=123, status=active）
- **AND** 返回 worktree 路径

#### Scenario: Release 租约

- **WHEN** Release(ctx, requestID=123)
- **THEN** workspace_checkouts.status = released
- **AND** leased_by_request_id = NULL

### Requirement: apply 成功后 squash merge + push main

平台 MUST 实现 SquashMergeAndPush。apply 成功（exit 0）后：checkout main + pull --ff-only + merge --squash + commit + push origin main + 删除工单分支。失败不 push（保留 worktree 供排障）。

#### Scenario: apply 成功 push

- **WHEN** SquashMergeAndPush(ctx, workspace, requestID=123, branch="req-123", commitMsg="req-123: rds applied")
- **THEN** main 分支新增一个 squash commit
- **AND** req-123 分支删除
- **AND** push 成功

### Requirement: 重启 reconcile（pinned_commit 恢复）

平台 MUST 实现 Reconciler。启动时读 workspace_checkouts where status in (active, stale)：本地 worktree 存在则 fetch + checkout pinned_commit；本地缺失则重新 clone + worktree + checkout。读 requests where status=applying → 转 waiting_manual（不自动重试 apply）。

#### Scenario: worktree 重建

- **WHEN** Reconcile(ctx) 且 workspace_checkouts 有 active 行但本地 worktree 缺失
- **THEN** 按 remote_url + pinned_commit 重建 worktree
- **AND** 标记 status=active

#### Scenario: apply 中断转人工

- **WHEN** Reconcile(ctx) 且 requests 有 status=applying（心跳丢失）
- **THEN** 状态转为 waiting_manual（不自动重试）

