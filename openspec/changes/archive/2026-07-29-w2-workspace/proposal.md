## Why

W2 执行目录治理是编排引擎（W2-06）的执行底座——orchestrator 的 Pipeline 调 `WorkspaceManager.WriteFiles` 写入 codegen 产物，但当前是 stub 接口。本模块实现真实的 go-git worktree 管理：clone → worktree 分配 → codegen 写入 → commit → pinned_commit 持久化 → push（apply 成功后）。没有它，codegen 生成的代码无处可写，Terramate 无目录可执行。

**影响层级**：业务核心层（`server/core/workspace/`），不改 DB schema / proto 契约。

**为什么现在做**：orchestrator（W2-06）已落地 Pipeline 接口（WorkspaceManager），但当前是 stub。W2-07 提供真实实现，让 Pipeline 的 generating/applying 阶段可以写文件 + 检出 worktree。

## What Changes

### 1. WorkspaceManager 实现（task 7.1 + 7.2）

新建 `server/core/workspace/`：
- **WorkspaceManager**：实现 orchestrator.WorkspaceManager 接口
  - `CloneOrFetch(ctx, workspace)` → clone bare repo 到 `/var/tm/worktrees/<workspace>/repo/`（首次）或 fetch（已存在）
  - `WriteFiles(ctx, requestID, files FileSet)` → 创建 worktree + 分支 → 写入 codegen FileSet → git add + commit → 返回 pinned_commit
  - `CheckoutWorktree(ctx, requestID, pinned_commit)` → 检出指定 commit 的 worktree（plan/apply 阶段用）
  - `ReleaseWorktree(ctx, requestID)` → 释放 worktree（apply 成功后或失败保留）
- **CheckoutLease**：workspace_checkouts 表的租约管理
  - Acquire（分配 worktree + leased_by_request_id 锁）
  - Release（释放锁 + worktree 清理）
  - 并发隔离：同一 workspace 的不同工单各占独立 worktree

### 2. pinned_commit 持久化（task 7.1）

- codegen 写入 worktree → git commit → commit SHA 写入 `workspace_checkouts.pinned_commit`
- Executor（plan/apply）按 pinned_commit 检出 worktree

### 3. apply 成功后 push main（doc 10 §3.1）

- apply 成功 → `git merge --squash req-<id>` → `git commit` → `git push origin main`
- 失败 → 保留 worktree 供排障

### 4. 重启 reconcile（task 7.3）

- 平台启动时读 `workspace_checkouts where status in (active, stale)`
- 本地 worktree 存在 → fetch + checkout pinned_commit → 标 active
- 本地缺失 → 按 remote_url + branch 重新 clone → checkout pinned_commit
- apply 中断的工单 → 转 waiting_manual（不自动重试）

### 5. 一致性对账（task 7.4）

- 定期校验：元数据 stacks（repo_path）↔ 工作仓库目录树 ↔ 远程 state key
- 不一致 → 标记 stale + 生成 manual_intervention_tasks

### 不做（本次范围外）

- scripts/ 脚本（task 7.6，推迟）
- plan artifact 对象存储（W2-08）
- outbox 事件补偿（W2-08）
- CMDB 对账（W2-08）

## Capabilities

### New Capabilities

- `workspace-governance`: 执行目录治理——go-git worktree 管理（clone/fetch/worktree/commit/push）+ checkout 租约（并发隔离）+ 重启 reconcile（pinned_commit 恢复）+ 一致性对账。

### Modified Capabilities

（无）

## Impact

- **代码**：新建 `core/workspace/`（manager + lease + reconcile + 对账）。实现 orchestrator.WorkspaceManager 接口。
- **API**：不新增 proto RPC（workspace 是内部管理，不是用户 API）
- **依赖**：go-git/v5（已在 go.mod，W1-01 GoGitProvider 用）
- **DB**：不改 schema（用 workspaces + workspace_checkouts 表）
- **配置**：worktree 根路径（默认 `/var/tm/worktrees/`，可配置）
- **测试**：用本地 file:// bare repo 测 clone/worktree/commit/checkout/reconcile（不依赖外部网络）
