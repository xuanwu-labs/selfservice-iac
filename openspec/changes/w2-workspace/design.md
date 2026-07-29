## Context

W2 执行目录治理提供 orchestrator Pipeline 的 WorkspaceManager 真实实现。

**现有架构约束**（已落地）：
- workspaces 表（migration 011）：id/name/remote_url/default_branch
- workspace_checkouts 表（migration 011）：workspace_id/node_id/worktree_path/branch/pinned_commit/purpose/leased_by_request_id/leased_until/status
- orchestrator.WorkspaceManager 接口（W2-06）：WriteFiles(ctx, requestID, FileSet) → commitSHA
- GoGitProvider（W1-01）：Clone/Fetch/CommitSHA（go-git/v5）
- doc 10：工作仓库拓扑 + checkout 生命周期 + 重启恢复 + 合并策略

## Goals / Non-Goals

**Goals:**
- WorkspaceManager 实现（满足 orchestrator.WorkspaceManager 接口）
- clone/fetch + worktree 分配 + codegen 写入 + commit + pinned_commit
- checkout 租约（并发隔离）
- apply 成功后 squash merge + push main
- 重启 reconcile（pinned_commit 恢复）

**Non-Goals:**
- 不做 plan artifact 对象存储（W2-08）
- 不做 outbox 事件补偿（W2-08）
- 不做一致性对账完整实现（Phase 1 只做 stub：Reconciler.ConsistencyCheck 返回 not-implemented）
- 不做 scripts/ 脚本（推迟）
- 不改 orchestrator 接口签名（WriteFiles 是唯一接口方法；其余方法内部使用）

## Decisions

### D1：WorkspaceManager 用 git CLI worktree（shell out，非 go-git API）

**决策**：`git worktree add` 和 `git merge --squash` 通过 **shell out 到 `git` CLI** 实现（exec.CommandContext），不用 go-git API。

**理由**（P0 修正）：go-git v5.19.1 验证发现：
- ❌ 无 `git worktree add` API（Repository.Worktree() 只返回主工作树）
- ❌ 无 squash merge（Merge 只支持 FastForwardMerge）
- ✅ go-git 用于 clone/fetch/commit/add/checkout（这些 API 可用）

**混合策略**：
- clone/fetch/commit/add/checkout → go-git（纯 Go，已有 GoGitProvider）
- worktree add/remove + merge --squash + push → `git` CLI（exec.CommandContext）

**依赖**：process 模式 Executor 预装 git（doc 11 §3 "process 模式预装工具链"，git 是基础工具）。

```
/var/tm/worktrees/infra-prod/
  ├── repo/                          ← 主 clone（go-git PlainClone）
  └── worktrees/
      ├── req-123-generate/          ← git worktree add（CLI）
      └── req-124-apply/             ← git worktree add（CLI）
```

**注意**：这与 W1-01 GoGitProvider 的"纯 Go 无外部 git"不同——GoGitProvider 用于**模块注册**（clone 外部模块仓库），workspace manager 用于**执行目录治理**（需要 worktree 高级特性）。process 模式 Executor 预装 git 是 D20 的前提。

**理由**：doc 10 §2 明确"用 git worktree 而非 git clone：多 worktree 共享 .git，省盘省时"。go-git/v5 支持 worktree API。

### D2：WriteFiles 流程（codegen → commit → pinned_commit）

**决策**：
```
1. 确保 workspace 主 clone 存在（CloneOrFetch）
2. git worktree add -b req-<id> <worktrees>/req-<id>-generate
3. codegen FileSet 写入 worktree 目录
4. git add -A + git commit -m "req-<id>: codegen generated"
5. 记录 pinned_commit = commit SHA
6. 返回 pinned_commit
```

**理由**：doc 10 §3 "codegen 产物写入内置执行仓库的工单分支，commit SHA 成为 pinned_commit"。

### D3：CheckoutWorktree（plan/apply 阶段检出）

**决策**：plan 和 apply 阶段按 pinned_commit 检出 worktree。

```
CheckoutWorktree(ctx, requestID, pinned_commit):
  1. 查 workspace_checkouts（已分配的 worktree）
  2. git checkout <pinned_commit>（在该 worktree 内）
  3. 返回 worktree 绝对路径（给 TerramateAdapter.Run 用）
```

**D21 plan/apply 解耦**：plan 用 worktree A，apply 用 worktree B（新沙箱，同一 pinned_commit）。Phase 1 简化：同一 worktree（process 模式无沙箱隔离）。

### D4：apply 成功后 squash merge + push（doc 10 §3.1，git CLI）

**决策**：apply 成功（exit 0）后，用 **git CLI**（exec.CommandContext）执行 squash merge：
```
1. git checkout main + git pull --ff-only origin main（go-git checkout + CLI pull）
2. git merge --squash req-<id>（★ CLI，go-git 不支持 squash merge）
3. git commit -m "req-<id>: <component> applied (stack: <stack_id>)"（go-git commit）
4. git push origin main（go-git push）
5. 删除 req-<id> 分支（go-git 删除引用）
6. 更新 workspace_checkouts.status = released
```

**P1-1 修正**：WorkspaceManager 对 orchestrator 只暴露 `WriteFiles`（接口方法）。CheckoutWorktree/ReleaseWorktree/SquashMergeAndPush 是**内部方法**，被 Executor（W2-08）和 Reconciler 调用，不在 orchestrator.WorkspaceManager 接口上。

**P1-2 修正**：WriteFiles 返回 commitSHA 后，**orchestrator 负责写入 `requests.pinned_commit`**（Pipeline.runGenerating 调 `RequestStore.UpdatePinnedCommit`）。workspace_checkouts.pinned_commit 由 lease.Acquire 写入。两个字段都写。

**P1-3 修正**：Reconciler 的 applying→waiting_manual **不直接写 DB**，而是调 orchestrator 的 Transition + EventLogger（复用 advance 路径，保证乐观锁 + request_events + manual_intervention_tasks）。

**P1-4 修正**：node_id 从 config 注入（`config.workspace.node_id`，默认 `"node-1"` Phase 1 单节点）。

**失败不 push**：保留 worktree + 分支供排障（doc 10 §7）。

### D5：重启 reconcile（doc 10 §4）

**决策**：平台启动时：
```
1. 读 workspace_checkouts where status in (active, stale)
2. 对每条记录：
   - 本地 worktree 存在 → fetch + checkout pinned_commit → 标 active
   - 本地缺失 → 按 remote_url clone → worktree add → checkout pinned_commit
3. 读 requests where status in (generating, planning, applying):
   - generating/planning → 可重试（转 failed_retryable 让 Pipeline 重跑）
   - applying（心跳丢失）→ 转 waiting_manual（不自动重试 apply）
```

**幂等保证**：同一 pinned_commit 的 worktree 检出是幂等的（git checkout 同一 commit）。

### D6：Phase 1 worktree 根路径可配置

**决策**：worktree 根路径从配置读（默认 `/var/tm/worktrees/`），开发环境可改（如 `./tmp/worktrees/`）。

## Risks / Trade-offs

- **[worktree 并发安全] → go-git worktree add 是文件系统级隔离**：不同工单的 worktree 目录不同，天然隔离。主 clone 的 `.git` 只读（fetch 时不冲突）。
- **[push main 冲突] → squash merge + pull rebase**：不同 stack 改不同目录，squash merge 不冲突（doc 10 §5）。冲突极少（D20 双锁保证同 stack 不并发）。
- **[重启恢复复杂度] → 只恢复 worktree，不重跑 apply**：generating/planning 可重试，applying 转 waiting_manual。人工确认后手动恢复。
- **[Phase 1 无沙箱隔离] → process 模式同 worktree**：D21 plan/apply 解耦需要新沙箱，Phase 1 简化为同一 worktree（process 模式无容器隔离）。

## Migration Plan

1. 实现 WorkspaceManager（CloneOrFetch + WriteFiles + CheckoutWorktree + ReleaseWorktree）
2. 实现 CheckoutLease（Acquire + Release + 并发锁）
3. 实现 SquashMergeAndPush（apply 成功后）
4. 实现 Reconciler（重启恢复）
5. 测试（本地 file:// bare repo）
6. `go build ./... && go vet ./... && go test ./server/core/workspace/...`

## Open Questions

- **worktree 路径用什么分隔？** 用 `req-<id>-<purpose>` 格式（如 `req-123-generate`、`req-123-apply`）。
- **多节点支持？** Phase 1 单节点（node_id 固定）。Phase 2 多节点（workspace_checkouts.node_id 区分）。
