## 1. WorkspaceManager 核心实现（task 7.1 + 7.2）

- [x] 1.1 实现 `server/core/workspace/manager.go`：WorkspaceManager 结构体。CloneOrFetch(ctx, workspace) → clone bare repo 到 worktree 根/repo/（首次）或 fetch（已存在）。使用 go-git/v5
- [x] 1.2 实现 WriteFiles(ctx, requestID, files codegen.FileSet) → (commitSHA string, err error)：git worktree add → 写入 FileSet → git add + commit → 返回 commit SHA
- [x] 1.3 实现 CheckoutWorktree(ctx, requestID, pinned_commit) → (worktreePath string, err error)：按 pinned_commit 检出 worktree（plan/apply 阶段用）
- [x] 1.4 实现 ReleaseWorktree(ctx, requestID) → error：释放 worktree（apply 成功后或失败保留）

## 2. Checkout 租约（task 7.2）

- [x] 2.1 实现 `server/core/workspace/lease.go`：CheckoutLease 管理 workspace_checkouts 表。Acquire(ctx, workspaceID, requestID, purpose) → 分配 worktree + leased_by_request_id 锁。Release(ctx, requestID) → 释放锁 + 标 released
- [x] 2.2 并发隔离：同一 workspace 不同工单各占独立 worktree（不同 worktree_path + branch）

## 3. apply 成功后 push main（doc 10 §3.1）

- [x] 3.1 实现 `server/core/workspace/merge.go`：SquashMergeAndPush(ctx, workspace, requestID, branch, commitMsg) → checkout main + pull --ff-only + merge --squash + commit + push origin main + 删除工单分支

## 4. 重启 reconcile（task 7.3）

- [x] 4.1 实现 `server/core/workspace/reconcile.go`：Reconciler。Reconcile(ctx) → 读 workspace_checkouts where status in (active, stale) → 重建/校验 worktree。读 requests where applying → 转 waiting_manual

## 5. wire + 验证

- [x] 5.1 实现 `server/core/workspace/provider.go`：wire ProviderSet（NewWorkspaceManager）
- [x] 5.2 更新 `server/core/core.go`：加 workspace.ProviderSet + wire.Bind(orchestrator.WorkspaceManager → *workspace.WorkspaceManager)
- [x] 5.3 `go build ./... && go vet ./...` 通过
- [x] 5.4 `go test ./server/core/workspace/... -short` 通过
- [x] 5.5 `gofmt -l server/` 无输出
- [x] 5.6 提交到 `feat/w2-workspace` 分支

## 6. 测试（task 7.5）

- [x] 6.1 实现 `server/core/workspace/manager_test.go`：用本地 file:// bare repo 测 CloneOrFetch + WriteFiles + CheckoutWorktree。验证 codegen FileSet 写入 + commit SHA 返回 + worktree checkout 正确
- [x] 6.2 实现 `server/core/workspace/reconcile_test.go`：测重启恢复（模拟 worktree 丢失 → reconcile 重建）
