# adapter-interfaces Specification

## Purpose
TBD - created by archiving change w1-adapter-interfaces. Update Purpose after archive.
## Requirements
### Requirement: 六大可插拔适配器接口

平台 MUST 在 `server/core/adapters/` 定义 D7 六大适配器接口：GitProvider、CloudProvider、StateBackend、PolicyEngine、CostEstimator、Notifier。每个接口 MUST 有 noop/stub 默认实现，返回结构化错误（不静默通过）。

#### Scenario: NoopGit 未配置时返回错误
- **WHEN** GitProvider 是 noop stub
- **AND** 调用 Clone
- **THEN** 返回包含 "git adapter not configured" 的错误

#### Scenario: NoopPolicy 未配置时返回错误
- **WHEN** PolicyEngine 是 noop stub
- **AND** 调用 Evaluate
- **THEN** 返回包含 "policy adapter not configured" 的错误

#### Scenario: NoopState 未配置时返回错误
- **WHEN** StateBackend 是 noop stub
- **AND** 调用 Read
- **THEN** 返回包含 "state adapter not configured" 的错误

### Requirement: TerramateAdapter exec 边界

平台 MUST 在 `server/core/terramate/` 定义 TerramateAdapter 接口，封装 terramate CLI 作为子进程调用。exec 实现 MUST 设置 cmd.Dir 为 stack 目录（D29），捕获 stdout/stderr/exit code，尊重 context 取消，且绝不 import github.com/terramate-io/terramate 内部包。

#### Scenario: 在 stack 目录运行 terramate
- **WHEN** 调用 Run，dir="/path/to/stack"，args=["run", "--tags", "env:prod", "--", "terraform", "plan"]
- **THEN** exec.Command 以 cmd.Dir="/path/to/stack" 执行
- **AND** RunResult 捕获 exit code、stdout、stderr

#### Scenario: Context 取消传播
- **WHEN** 调用 Run 时 context 已取消
- **THEN** 子进程被杀死，返回错误

#### Scenario: exit code 提取
- **WHEN** terramate 子进程退出码为 1
- **THEN** RunResult.ExitCode == 1
- **AND** 返回 error 包含 ExitError

### Requirement: D1 边界守护测试

平台 MUST 在 `server/internal/audit/` 有测试，遍历 `server/` 下所有 .go 文件，断言无 import github.com/terramate-io/terramate。

#### Scenario: server 下无 terramate import
- **WHEN** D1 守护测试运行
- **THEN** 通过（server/ 下无文件 import terramate 内部包）

### Requirement: wire ProviderSet 绑定

所有六个适配器 MUST 注册在一个 wire ProviderSet（`server/core/adapters/provider.go`），用 wire.Bind 绑定接口到 noop 默认实现。

#### Scenario: ProviderSet 绑定全部六个接口
- **WHEN** wire 生成依赖图
- **THEN** GitProvider、CloudProvider、StateBackend、PolicyEngine、CostEstimator、Notifier 全部绑定到各自的 noop stub

