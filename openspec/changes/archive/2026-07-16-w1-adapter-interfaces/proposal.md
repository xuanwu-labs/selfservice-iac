## Why

W1 模块 01（平台骨架与适配器接口）是 iac-self-service-platform 的第一个实现模块。平台需要 D7 六大可插拔适配器接口（GitProvider/CloudProvider/StateBackend/PolicyEngine/CostEstimator/Notifier）+ D1 TerramateAdapter（exec 边界），才能启动后续模块——03 注册用 GitProvider clone 模块、05 代码生成用 StateBackend 读写 state、06 编排用 TerramateAdapter 执行。当前 `server/core/adapters/` 和 `server/core/terramate/` 只有 `.gitkeep` 空目录。

**影响层级**：平台层（新增适配器接口 + TerramateAdapter exec 边界守护）。

**兼容性**：不破坏已有 Terramate 配置或 CLI 用法——平台通过 exec 调用 terramate 二进制（D1 边界），不修改 terramate 本身。

## What Changes

- 新增 `server/core/adapters/{git,cloud,state,policy,cost,notify}/` 六个适配器接口，每个含 noop/stub 默认实现（返回结构化错误，不静默通过）
- 新增 `server/core/terramate/` TerramateAdapter 接口 + ExecAdapter 实现（exec.CommandContext 封装 terramate 子进程，cmd.Dir=stack 目录，stdout/stderr/exit code 捕获）
- 新增 `server/core/adapters/provider.go` wire ProviderSet 绑定六接口到 noop 默认
- 新增 `server/internal/audit/d1_guard_test.go` D1 边界守护测试（AST 遍历 server/**/*.go，断言无 terramate import）
- 不修改现有代码（纯增量）
- 不改 DB schema / proto 契约
- 不含适配器真实实现（go-git clone / OPA / Infracost / S3）——本次只做 stub

## Capabilities

### New Capabilities

- `adapter-interfaces`: D7 六大可插拔适配器接口 + noop stub + TerramateAdapter exec 边界 + D1 守护测试 + wire ProviderSet

### Modified Capabilities

（无——本次不修改已有 spec 的 requirements）

## Impact

- **代码**：新增 3 个包（adapters/ terramate/ audit/），纯增量，不改现有
- **API**：不改 proto 契约（适配器是 Go 内部接口，不是 RPC）
- **依赖**：无新外部依赖（exec 是标准库；wire 已在 go.mod）
- **DB**：不改 schema
- **配置**：未来需要 adapters_config 表（非 MVP），本次不涉及
- **测试**：TerramateAdapter 用 fake terramate 脚本测试（跨平台）；D1 guard 用 go/parser AST 测试（-short 模式可跑）
