## Context

当前 `server/core/adapters/` 和 `server/core/terramate/` 只有 `.gitkeep` 空目录。D7 定义了六大可插拔适配器（GitProvider/CloudProvider/StateBackend/PolicyEngine/CostEstimator/Notifier），D1 要求平台通过 exec 调用 terramate（不 import 内部包）。W1 模块 01 是后续模块（03 注册/05 代码生成/06 编排）的前置依赖。

**现有架构约束**：
- `server/core/<domain>/` 是领域核心顶层包（AGENTS.md 定义）
- `server/data/` 是数据访问层（pgxpool + sqlc）
- wire 做依赖注入（`server/data/data.go` 已有 ProviderSet）
- 不用 entity 类——core 直接用 sqlc `generated.*`（已确认）

## Goals / Non-Goals

**Goals:**
- 定义 6 个适配器接口 + noop stub（fail-loud，不静默降级）
- 定义 TerramateAdapter 接口 + ExecAdapter（exec.CommandContext 封装）
- D1 边界守护测试（AST 遍历，编译期检查）
- wire ProviderSet 绑定

**Non-Goals:**
- 适配器真实实现（go-git clone / OPA / Infracost / S3 state）——stub only
- DB store 层（W1 模块 02）
- 模块注册（W1 模块 03）
- 分层模型（W1 模块 04）
- D20 Executor 四模式（process/container/k8s/remote）——W2 模块 13

## Decisions

### D1：接口 + noop 同文件 vs 分文件

**决策**：noop stub 短小，和接口同文件（如 `git.go` 同时有 GitProvider 接口 + NoopGit）。ExecAdapter 有真实逻辑，分文件（`terramate.go` 接口 + `exec.go` 实现）。

**理由**：Google Go Style 不强制文件拆分；简单 stub 和接口同文件更紧凑，复杂实现分文件更清晰。后续有多个实现再拆。

**备选**：全部接口单独文件 + 实现单独文件（过度拆分，stub 只有 3 行没必要分开）。

### D2：Noop stub 返回错误而非静默

**决策**：每个 noop 方法返回 `fmt.Errorf("xxx adapter not configured: set adapters.xxx.impl in config")`。

**理由**：静默通过会导致下游逻辑拿到空结果继续跑，错误难以定位。fail-loud 在运行时立即暴露未配置的适配器。

**备选**：noop 返回零值静默通过（反模式，debug 噩梦）。

### D3：TerramateAdapter 用 exec.CommandContext

**决策**：ExecAdapter 用 `exec.CommandContext(ctx, binaryPath, args...)` + `cmd.Dir = dir` + stdout/stderr Buffer 捕获 + exit code 提取。

**理由**：D1 要求 exec 不 import；exec.CommandContext 是标准库，支持 context 取消（杀子进程）、stdout/stderr 分离捕获、ExitError 提取 exit code。

**备选**：os/exec.Command（无 context 取消支持）；shell out via sh -c（不必要的 shell 层）。

### D4：D1 guard 用 go/parser AST

**决策**：`go/parser.ParseFile` + `parser.ImportsOnly` 遍历 `server/**/*.go`，收集 import path，断言无 `github.com/terramate-io/terramate`。

**理由**：`.golangci.yml` depguard 不生效（terramate 不在 go.mod 时 typechecker 丢弃规则）。AST 遍历是编译期检查，零运行时依赖，`-short` 模式可跑。

**备选**：只靠 depguard（已证明不生效）；CI 脚本 grep（不够结构化）。

### D5：wire.Bind 绑定接口到 noop

**决策**：`wire.Bind(new(git.GitProvider), new(git.NoopGit))` 在 ProviderSet 里绑定接口到 noop struct。后续替换实现时改 Bind 目标。

**理由**：消费方注入接口不注入具体类型；wire 在编译期解析绑定，类型安全。

**备选**：ProviderSet 返回具体类型让 wire 自动推断（不显式，不够清晰）。

## Risks / Trade-offs

- **[Noop stub 不是真实实现] → 下游模块需要真实实现时再替换**：W1 只验证接口设计，不验证真实集成。风险可控——接口签名在真实实现前会被下游消费方校验。
- **[ExecAdapter 混淆了 TerramateAdapter 和 Executor 职责] → W2 模块 13 落地 Executor 后重构**：当前 ExecAdapter 自己 exec（process 模式），正确架构是 TerramateAdapter 只构造命令、Executor 选沙箱。临时方案，等 W2 拆分。
- **[D1 guard 测试遍历全 server/ 慢] → AST ImportsOnly 模式只解析 import 声明**：实测 < 1s，可接受。
- **[跨平台测试（Windows bat vs Unix sh）] → fake terramate 脚本双版本**：已在 exec_test.go 实现 Windows bat + Unix sh 双路径。
