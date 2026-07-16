# 提案：w1-adapter-interfaces

## 变更内容

定义 D7 六大可插拔适配器接口（GitProvider、CloudProvider、StateBackend、PolicyEngine、CostEstimator、Notifier）及 noop/stub 默认实现，加上封装 `terramate run` 子进程调用的 TerramateAdapter exec 适配器。同时添加 D1 边界守护测试，确保 `server/**` 不得 import `github.com/terramate-io/terramate` 内部包。

这是 `iac-self-service-platform` tasks.md 的 **W1 模块 01**（平台骨架与适配器接口）。W1 四个模块中的第一个——适配器接口零依赖，被下游模块消费（03 注册用 GitProvider、05 代码生成用 StateBackend、06 编排用 TerramateAdapter）。

## 为什么

1. **D7 可插拔是核心用户诉求**——平台必须支持切换云厂商、策略引擎（OPA）、成本估算（Infracost）、通知渠道，且不需要改代码。接口优先设计使这成为可能。
2. **TerramateAdapter 是 D1 边界的守护者**——平台通过 `exec`（子进程）调用 terramate，绝不通过 Go import。适配器封装这个契约，让其余代码永远不碰 terramate 内部。
3. **D1 守护测试弥补已知缺口**——`.golangci.yml` depguard 已配置但不生效（terramate 不在 go.mod 时 typechecker 静默丢弃规则）。专用编译期测试让边界显式化、CI 可执行。
4. **下游模块无法在接口就绪前启动**——W1 模块 03（注册）和 W2 模块 05（代码生成）依赖 GitProvider/StateBackend；W2 模块 06（编排）依赖 TerramateAdapter。

## 范围

### 本次范围内
- `server/core/adapters/{git,state,policy,cost,notify,cloud}/` 六个适配器接口 + noop 默认实现
- `server/core/terramate/` TerramateAdapter 接口 + exec 实现
- `server/internal/audit/` D1 边界守护测试
- 所有适配器的 wire ProviderSet 装配

### 本次范围外（后续 change）
- 适配器真实实现（go-git clone、OPA eval、Infracost、S3 state 读写）—— 本次只做 stub
- DB store 层（W1 模块 02：`feat/w1-db-store`）
- 模块注册（W1 模块 03：`feat/w1-module-registry`）
- 分层模型（W1 模块 04：`feat/w1-layer-model`）
- 适配器配置持久化（`adapters_config` 表为非 MVP）
- 配置加载脚本（task 1.6，随 W2 实现）

## 决策

- **不用 entity 类**——core 直接用 sqlc 生成的 `generated.*` 类型；`internal/mapping/` 负责 generated↔proto 转换。这是 sqlc 标准模式（不同于 ferret 手写 ORM 需要 entity 类）。
- **Noop stub 返回结构化错误**——stub 不是静默的；返回 `errors.New("adapter not configured")` 让缺失的适配器在运行时立即失败，不静默降级。
- **TerramateAdapter 返回 Result struct**——捕获 exit code、stdout、stderr、duration；支持用 fake terramate 脚本做确定性测试。

## 影响

- 新增包：`server/core/adapters/{git,state,policy,cost,notify,cloud}/`、`server/core/terramate/`、`server/internal/audit/`
- 不修改现有代码（纯增量）
- 不改 DB schema
- 不改 proto 契约
