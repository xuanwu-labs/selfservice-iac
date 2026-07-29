## Context

W2 最后一个模块——state backend 实现 + 漂移检测引擎。

**现有架构约束**（已落地）：
- StateBackend 接口（W1-01）：`Read(ctx, key) ([]byte, error)` / `Write(ctx, key, data)` / `Delete(ctx, key)` / `Lock(ctx, key) (lockID string, err error)` / `Unlock(ctx, key, lockID)` + NoopState stub
- state_backends 表（migration 011）：bucket/region/kind/endpoint/encrypt
- **drift_runs / drift_records 表不存在**（migration 011 只有 state_backends/workspaces/stacks 等）→ Phase 1 用内存记录，Phase 2 补 migration
- terramate.Adapter（W1-01）：`Run(ctx, dir, args) (RunResult, error)` — **已实现**（ExecAdapter）
- WorkspaceManager（W2-07）：CheckoutCommit（只读检出 pinned_commit）
- Notifier 接口（W1-01）：Notify（noop stub）
- clock 包（`server/core/clock/`）：Clock 接口 + FakeClock（D44）
- **`server/core/drift/` 已存在**（空目录 + .gitkeep），不是新建
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

### D1：S3Backend 用接口 + mock（Phase 1 不连真实 S3）；保留 noop 为默认

**决策**：S3Backend 实现 StateBackend 接口（Lock 返回 lockID，Unlock 接收 lockID），Phase 1 用内存 mock。**NoopState 保持为 wire 默认绑定**（P2-10 修正：mock 不作默认——会静默丢数据）。S3Backend 仅在测试中使用，真实 S3 SDK 集成 Phase 2。

**理由**：
- Phase 1 目标是"架构正确 + 流程通"，不是"连真实云"
- S3 SDK 引入大量依赖（AWS SDK ~50MB），Phase 1 不值得
- noop 返回 fail-loud 错误（比 mock 静默丢数据安全）
- 测试不需要真实 S3（mock 足够验证逻辑）

### D2：DriftScheduler 用 Go time.Ticker（Phase 1 单进程）

**决策**：Phase 1 DriftScheduler 用 `time.Ticker` 定时触发（嵌入主进程），不做独立二进制 + leader 选举。

```go
type Scheduler struct {
    intervals map[string]time.Duration // 分层间隔（可注入，测试用 100ms）
    limiter  *rate.Limiter            // 令牌桶限流（golang.org/x/time/rate）
    worker   *Worker
    clock    clock.Clock              // 注入时钟（P3-14 修正：用 clock 包不用 time.Now）
}
func (s *Scheduler) Start(ctx context.Context) // 启动 ticker，按间隔触发
func (s *Scheduler) Stop(ctx context.Context)  // 停止 + 优雅 drain（P2-12 修正）
```

**理由**：Phase 1 单进程（process 模式 Executor），定时器够用。Phase 2 改 River job + leader 选举。**intervals 可注入**（测试用 100ms 避免慢测试）。**用 clock.Clock 接口**（server/core/clock/，D44 专门为 drift 设计）。

### D3：DriftWorker 用本地接口引用 terramate.Adapter（不导入 orchestrator）

**决策**（P1-5 修正）：DriftWorker 声明**本地接口**，被 `*terramate.ExecAdapter` 隐式满足。**不导入 orchestrator 包**（避免层耦合 + orchestrator.TerramateRunner 无具体实现）。

```go
// drift 包内部定义，被 *terramate.ExecAdapter 隐式满足
type Runner interface {
    Run(ctx context.Context, dir string, args []string) (terramate.RunResult, error)
}

type Worker struct {
    runner    Runner           // 本地接口（terramate.ExecAdapter 满足）
    workspace CheckoutProvider // 本地接口（workspace.Manager.CheckoutCommit 满足）
    notifier  Notifier         // 本地接口（adapters/notify.Notifier 满足）
}
func (w *Worker) CheckStack(ctx context.Context, stackID int64) (DriftResult, error)
```

**理由**：与 workspace.Manager 满足 orchestrator.WorkspaceManager 的模式一致（DIP 隐式接口）。orchestrator.TerramateRunner 是 orchestrator 内部接口（无具体实现），不应用作 drift 的依赖。

**exit code 映射**（P2-9 修正）：
- `RunResult.ExitCode == 0` → 无漂移（has_drift=false）
- `RunResult.ExitCode == 2` → 有漂移（has_drift=true，error=nil）
- `RunResult.ExitCode == 1` → 错误（记录失败，not drift）

### D4：plan JSON 解析提取 resource_changes

**决策**：`terraform show -json <plan>` 输出 JSON，解析 `resource_changes` 数组，提取 `change.actions != ["no-op"]` 的资源。

```go
type PlanDiff struct {
    ResourceChanges []ResourceChange `json:"resource_changes"`
}
type ResourceChange struct {
    Address string      `json:"address"`       // "alicloud_db_instance.this"
    Change  ChangeBody  `json:"change"`        // P2-8 修正：嵌套 struct（不能用 "change.actions" 点号 tag）
}
type ChangeBody struct {
    Actions []string `json:"actions"`           // ["create"]/["delete"]/["update"]/["no-op"]
}
```

**理由**：terraform plan JSON 格式稳定（Terraform 1.0+ 文档化）；解析逻辑简单（JSON unmarshal + 过滤）。

### D5：分层调度配置可注入（Phase 1 默认值，测试可覆盖）

**决策**：调度频率用 `DefaultIntervals()` 函数返回默认值，但构造器接受 `intervals map[string]time.Duration` 参数（测试用 100ms）。令牌桶用 `golang.org/x/time/rate`（P2-11 关闭）。

```go
func DefaultIntervals() map[string]time.Duration {
    return map[string]time.Duration{
        "global":      24 * time.Hour,
        "middleware":  12 * time.Hour,
        "application":  6 * time.Hour,
    }
}
func DefaultConcurrency() map[string]int {
    return map[string]int{"global": 2, "middleware": 5, "application": 10}
}
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

- **drift_runs/drift_records 表：** 已确认 migration 011 不含这两张表。Phase 1 用**内存记录**（MemDriftRepo），Phase 2 补 migration 015。这是显式 Non-Goal（记录在此）。
- **令牌桶实现：** 已决定用 `golang.org/x/time/rate`（Go 标准限流库，codebase 已有 golang.org/x 依赖）。
- **audit_logs for drift：** Phase 1 不写 audit_logs（drift run 不是 request）。Phase 2 补审计。显式 Non-Goal。
