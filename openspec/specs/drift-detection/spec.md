# drift-detection Specification

## Purpose
TBD - created by archiving change w2-state-drift. Update Purpose after archive.
## Requirements
### Requirement: DriftScheduler 分层调度 + 令牌桶限流

平台 MUST 在 `server/core/drift/` 实现 DriftScheduler。分层调度（Global 24h / Middleware 12h / Application 6h）+ 令牌桶限流（per-layer 并发上限：Global 2 / Middleware 5 / Application 10）。Phase 1 用 time.Ticker 单进程。Start(ctx) 启动定时器，Stop() 停止。

#### Scenario: 分层调度频率

- **WHEN** DriftScheduler.Start(ctx)
- **THEN** Global 层每 24h 触发一次
- **AND** Middleware 层每 12h 触发一次
- **AND** Application 层每 6h 触发一次

#### Scenario: 令牌桶限流

- **WHEN** Global 层并发检测请求超过 2
- **THEN** 超出的请求等待（令牌桶阻塞）

### Requirement: DriftWorker 只读 plan + 差异解析

平台 MUST 实现 DriftWorker。CheckStack(ctx, stackID)：workspace 只读 checkout → terramate plan -detailed-exitcode → 解析差异 → 记录 drift。退出码 0 = 无漂移，2 = 有漂移，1 = 错误。

#### Scenario: 无漂移（exit 0）

- **WHEN** CheckStack 且 terraform plan 退出码 = 0
- **THEN** drift_run 记录 has_drift = false
- **AND** 不创建 drift_record

#### Scenario: 有漂移（exit 2）

- **WHEN** CheckStack 且 terraform plan 退出码 = 2
- **THEN** drift_run 记录 has_drift = true
- **AND** 创建 drift_record（status = open）
- **AND** 发送 drift.detected 事件

### Requirement: PlanParser 解析 terraform show -json

平台 MUST 实现 PlanParser。解析 terraform plan JSON 输出的 resource_changes，提取 change.actions != ["no-op"] 的资源，返回 DiffSummary。

#### Scenario: 解析有差异的 plan

- **WHEN** PlanParser.Parse(planJSON) 且 JSON 含 resource_changes[0].change.actions = ["update"]
- **THEN** 返回 DiffSummary 含 1 个 update 资源

#### Scenario: 解析无差异的 plan

- **WHEN** PlanParser.Parse(planJSON) 且所有 resource_changes.actions = ["no-op"]
- **THEN** 返回 DiffSummary 空（0 个资源变更）

