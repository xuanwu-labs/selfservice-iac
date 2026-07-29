## Why

W2 状态后端与漂移检测是 MVP 主干的最后一个模块——state backend 适配器让 terraform 有地方存 state（每 stack 独立 key），漂移检测器让平台能发现"云上资源 vs 期望状态"的偏差。没有漂移检测，用户不知道谁手动改了云资源；没有 state backend，terraform apply 无处持久化。

**影响层级**：业务核心层（`server/core/{drift}/`）+ 适配器层（`server/core/adapters/state/`），不改 DB schema / proto 契约。

**为什么现在做**：workspace（W2-07）已归档，TerramateAdapter（W1-01）+ Executor 接口已就位，codegen（W2-05）生成 backend.tf 已含 state_key。漂移检测器复用 Executor + workspace checkout。

## What Changes

### 1. State Backend 适配器实现（task 8.1）

扩展 `server/core/adapters/state/`（W1-01 只有 noop stub）：
- **S3Backend**：实现 StateBackend 接口（Read/Write/Delete/Lock/Unlock）
  - 支持 S3 兼容（AWS S3 / 阿里云 OSS / MinIO）
  - 锁用 DynamoDB（AWS）或 etcd（通用）— Phase 1 用 S3 native lock（conditional write）
  - 每 stack 独立 key（从 PathGenerator.StateKey 推导）
- **noop 保留**：W1-01 的 NoopState 不删除（测试用）

### 2. 漂移检测引擎（task 8.2）

新建 `server/core/drift/`：
- **DriftScheduler**：分层调度（Global 每日/Middleware 每 12h/Application 每 6h）
  - 令牌桶限流（per-layer 并发上限）
  - 时间窗（凌晨/工作时间外/任意）
  - Phase 1 简化：固定调度（不做 leader 选举，单进程定时器）
- **DriftWorker**：单 stack 检测流程
  1. workspace checkout（只读，检出 pinned_commit）
  2. terramate run -- terraform plan -detailed-exitcode
  3. terraform show -json → 解析 resource_changes 差异
  4. 写 drift_runs + drift_records
  5. 发 drift.detected 事件

### 3. 漂移事件 + 同步策略（task 8.3）

- **drift.detected 事件**：Notifier 通知归属团队
- **同步策略**：
  - adopt-cloud（接受云侧现状）：terraform import/state 调整
  - restore-desired（恢复期望状态）：走工单 + 审批 + apply
- Phase 1 简化：只记录 + 通知，不自动执行同步策略（人工触发）

### 不做（本次范围外）

- drift-scheduler 独立二进制（doc 13 提到 `server/cmd/drift-scheduler`，Phase 1 嵌入主进程）
- leader 选举（Phase 2 多实例）
- Scheduled Runs 扩展（Phase 2）
- 脚本（task 8.5 推迟）
- plan artifact 对象存储（W2-07 已预留接口，Phase 1 drift 不持久化 plan）
- outbox 事件补偿（W2-08 简化为直接 Notifier.Notify）

## Capabilities

### New Capabilities

- `state-backend-impl`: S3 兼容 StateBackend 实现（Read/Write/Delete/Lock）+ 每 stack 独立 key
- `drift-detection`: 漂移检测引擎（分层调度 + 令牌桶限流 + 只读 plan + 差异解析 + 事件通知 + 同步策略占位）

### Modified Capabilities

（无）

## Impact

- **代码**：新建 `core/drift/`（scheduler + worker + plan_parser）。扩展 `core/adapters/state/`（S3Backend）。
- **API**：不新增 proto RPC（drift 是内部调度，工单 API 在 W3）
- **依赖**：无新外部依赖（S3 用 AWS SDK Go v2 或 minio-go，但 Phase 1 可以先用接口 + stub，真实 S3 连接 W3/Phase 2）
- **DB**：不改 schema（用 drift_runs + drift_records 表，migration 011 已建 state_backends）
- **测试**：plan JSON 样本解析测试 + 限流逻辑测试 + 调度频率测试
