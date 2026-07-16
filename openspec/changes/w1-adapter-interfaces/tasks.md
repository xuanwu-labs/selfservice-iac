# 任务：w1-adapter-interfaces

> **状态：proposed（已提案，待维护者 apply）。** 以下任务在维护者显式执行 apply 前不算开始（config.yaml lifecycle-ownership 规则）。

## 01-适配器接口（6 个适配器 + noop stub）

- [ ] 1.1 创建 `server/core/adapters/git/git.go`：GitProvider 接口（Clone/Fetch/CommitSHA）+ NoopGit
- [ ] 1.2 创建 `server/core/adapters/cloud/cloud.go`：CloudProvider 接口（ValidateCredentials/ListRegions）+ NoopCloud + Credentials struct
- [ ] 1.3 创建 `server/core/adapters/state/state.go`：StateBackend 接口（Read/Write/Delete/Lock/Unlock）+ NoopState
- [ ] 1.4 创建 `server/core/adapters/policy/policy.go`：PolicyEngine 接口（Evaluate）+ NoopPolicy + Result struct
- [ ] 1.5 创建 `server/core/adapters/cost/cost.go`：CostEstimator 接口（Estimate）+ NoopCost + Result struct
- [ ] 1.6 创建 `server/core/adapters/notify/notify.go`：Notifier 接口（Notify）+ NoopNotifier + Notification struct
- [ ] 1.7 创建 `server/core/adapters/provider.go`：wire ProviderSet 绑定 6 个 noop 适配器

## 02-TerramateAdapter（D1 exec 边界）

- [ ] 2.1 创建 `server/core/terramate/terramate.go`：Adapter 接口（Run/Version）+ RunResult struct
- [ ] 2.2 创建 `server/core/terramate/exec.go`：ExecAdapter 实现（exec.CommandContext，stdout/stderr 捕获，exit code）
- [ ] 2.3 创建 `server/core/terramate/exec_test.go`：用 fake terramate 脚本测试（exit code 传播、stdout 捕获、ctx 取消、工作目录）

## 03-D1 边界守护测试

- [ ] 3.1 创建 `server/internal/audit/d1_guard_test.go`：AST 遍历 server/**/*.go，断言无 `github.com/terramate-io/terramate` import

## 04-测试 + 验证

- [ ] 4.1 `go build ./...` 通过
- [ ] 4.2 `go vet ./...` 通过
- [ ] 4.3 `go test ./server/core/adapters/... ./server/core/terramate/... ./server/internal/audit/...` 通过（short 模式，无需 Docker）
- [ ] 4.4 gofmt clean

## 备注：本分支已有实现代码

代码在维护者显式 apply 之前就写了。实现已存在于本分支的 commit 中，但 tasks 保持未勾选，直到 apply 确认。如果维护者批准，标记 tasks 完成；如果不批准，分支可 reset。
