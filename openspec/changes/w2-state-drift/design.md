## Context

W2 最后一个模块——state backend 实现 + 漂移检测引擎。

**现有架构约束**（已落地）：
- StateBackend 接口（W1-01）：Read/Write/Delete/Lock/Unlock + NoopState stub
- state_backends 表（migration 011）：bucket/region/kind
- drift_runs + drift_records 表（migration 011，需核查是否存在）
- TerramateAdapter（W1-01）：Run（terramate run -- terraform plan）
- WorkspaceManager（W2-07）：CheckoutCommit（只读检出 pinned_commit）
- Notifier 接口（W1-01）：Notify（noop stub）
- doc 13：漂移检测完整设计（架构 + 流程 + 调度 + 限流 + 同步策略）

## Goals / Non-Goals

**Goals:**
- S3Backend 实现（StateBackend 接口，S3 兼容 + 锁）
- DriftScheduler（分层调度 + 令牌桶限流）
- DriftWorker（只读 plan + terraform show -json 差异解析）
- drift.detected 事件 + 同步策略占位

**Non-Goals:**
- 不做独立 drift-scheduler 二进制（Phase 1 嵌入主进程）
- 不做 leader 选举（Phase 2）
- 不做自动同步策略执行（Phase 1 只记录 + 通知）
- 不做 Scheduled Runs 扩展（Phase 2）
- 不做真实 S3 连接（Phase 1 用接口 + mock 测试；真实 S3 SDK 集成 W3/Phase 2）

## Decisions

### D1：S3Backend 用接口 + mock（Phase 1 不连真实 S3）

**决策**：S3Backend 实现 StateBackend 接口，但 Phase 1 的 Read/Write/Delete/Lock/Unlock 用**内存 mock**（不连真实 S3）。真实 S3 SDK 集成（AWS SDK Go v2 或 minio-go）在 W3 或 Phase 2。

**理由**：
- Phase 1 目标是"架构正确 + 流程通"，不是"连真实云"
- S3 SDK 引入大量依赖（AWS SDK ~50MB），Phase 1 不值得
- 测试不需要真实 S3（mock 足够验证逻辑）

### D2：DriftScheduler 用 Go time.Ticker（Phase 1 单进程）

**决策**：Phase 1 DriftScheduler 用 `time.Ticker` 定时触发（嵌入主进程），不做独立二进制 + leader 选举。

```go
type Scheduler struct {
    interval time.Duration  // 分层间隔
    limiter  *tokenBucket   // 令牌桶限流
    worker   *Worker
}
func (s *Scheduler) Start(ctx) // 启动 ticker，按间隔触发
func (s *Scheduler) Stop()     // 停止
```

**理由**：Phase 1 单进程（process 模式 Executor），定时器够用。Phase 2 改 River job + leader 选举。

### D3：DriftWorker 只读 plan 经 Executor 接口

**决策**：DriftWorker 调 TerramateRunner.Run（复用 orchestrator 的接口），执行 `terramate run -- terraform plan -detailed-exitcode`。

```go
type Worker struct {
    runner    TerramateRunner  // 复用 orchestrator 接口
    workspace WorkspaceManager // 只读 checkout
    repo      DriftRepo        // drift_runs/records CRUD
    notifier  Notifier         // 通知
}
func (w *Worker) CheckStack(ctx, stackID) (DriftResult, error)
```

**理由**：doc 13 §2 "漂移只读 plan 走与工单完全相同的 Executor"。复用 Executor + workspace checkout。

### D4：plan JSON 解析提取 resource_changes

**决策**：`terraform show -json <plan>` 输出 JSON，解析 `resource_changes` 数组，提取 `change.actions != ["no-op"]` 的资源。

```go
type PlanDiff struct {
    Resources []ResourceChange `json:"resource_changes"`
}
type ResourceChange struct {
    Address string   `json:"address"`      // "alicloud_db_instance.this"
    Action  []string `json:"change.actions"` // ["create"]/["delete"]/["update"]
}
```

**理由**：terraform plan JSON 格式稳定（Terraform 1.0+ 文档化）；解析逻辑简单（JSON unmarshal + 过滤）。

### D5：分层调度配置硬编码（Phase 1）

**决策**：Phase 1 调度频率硬编码（doc 13 §3 的默认值）：
```
Global:     每 24h，并发上限 2
Middleware: 每 12h，并发上限 5
Application:每 6h, 并发上限 10
```

Phase 2 改 DB 配置（drift_schedule 表）。

## Risks / Trade-offs

- **[不连真实 S3] → Phase 1 mock 足够**：架构正确性靠接口保证，真实集成 Phase 2。
- **[单进程调度] → Phase 1 容量小够用**：< 50 stack，6h 间隔，单进程 ticker 足够。
- **[不自动同步] → 人工触发**：Phase 1 只记录 + 通知，adopt/restore 由运维手动决定。
- **[plan JSON 解析] → 简单 JSON unmarshal**：terraform plan JSON 格式稳定。

## Migration Plan

1. 实现 S3Backend（接口 + mock 实现）
2. 实现 DriftScheduler（ticker + 令牌桶）
3. 实现 DriftWorker（checkout + plan + 解析 + 记录 + 通知）
4. 实现 PlanParser（terraform show -json 解析）
5. 测试（plan JSON 样本 + 限流 + 调度）
6. `go build ./... && go vet ./... && go test ./server/core/{drift,adapters/state}/...`

## Open Questions

- **drift_runs/drift_records 表是否在 migration 011？** 需要核查。如果不存在，Phase 1 用内存记录（不写 DB）。
- **令牌桶用什么实现？** golang.org/x/time/rate（标准限流库）或自写简单版。
