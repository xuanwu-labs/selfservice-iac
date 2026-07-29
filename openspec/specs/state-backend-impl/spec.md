# state-backend-impl Specification

## Purpose
TBD - created by archiving change w2-state-drift. Update Purpose after archive.
## Requirements
### Requirement: S3 兼容 StateBackend 实现

平台 MUST 在 `server/core/adapters/state/` 实现 S3Backend，满足 W1-01 StateBackend 接口（Read/Write/Delete/Lock/Unlock）。Phase 1 用内存 mock（不连真实 S3 SDK，接口签名正确）。支持 S3 兼容协议（AWS S3 / 阿里云 OSS / MinIO）。每 stack 独立 state key（从 PathGenerator.StateKey 推导）。

#### Scenario: Write + Read state

- **WHEN** S3Backend.Write(ctx, key, data)
- **AND** S3Backend.Read(ctx, key)
- **THEN** 返回写入的 data

#### Scenario: Lock/Unlock

- **WHEN** S3Backend.Lock(ctx, key)
- **THEN** 后续 Lock 同一 key 返回错误（已锁定）
- **AND** S3Backend.Unlock(ctx, key) 后可再次 Lock

### Requirement: S3Backend 作为默认实现替换 noop

平台 MUST 将 S3Backend 作为 StateBackend 的默认实现（替换 W1-01 的 NoopState）。noop 保留用于测试。

#### Scenario: wire 绑定 S3Backend

- **WHEN** wire 生成依赖图
- **THEN** StateBackend 接口绑定到 S3Backend（而非 NoopState）

