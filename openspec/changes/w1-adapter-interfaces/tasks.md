## 1. 六大适配器接口 + noop stub

- [ ] 1.1 创建 `server/core/adapters/git/git.go`：GitProvider 接口（Clone/Fetch/CommitSHA）+ NoopGit
- [ ] 1.2 创建 `server/core/adapters/cloud/cloud.go`：CloudProvider 接口（ValidateCredentials/ListRegions）+ NoopCloud + Credentials struct
- [ ] 1.3 创建 `server/core/adapters/state/state.go`：StateBackend 接口（Read/Write/Delete/Lock/Unlock）+ NoopState
- [ ] 1.4 创建 `server/core/adapters/policy/policy.go`：PolicyEngine 接口（Evaluate）+ NoopPolicy + Result struct
- [ ] 1.5 创建 `server/core/adapters/cost/cost.go`：CostEstimator 接口（Estimate）+ NoopCost + Result struct
- [ ] 1.6 创建 `server/core/adapters/notify/notify.go`：Notifier 接口（Notify）+ NoopNotifier + Notification struct
- [ ] 1.7 创建 `server/core/adapters/provider.go`：wire ProviderSet 用 wire.Bind 绑定六接口到 noop

## 2. TerramateAdapter（D1 exec 边界）

- [ ] 2.1 创建 `server/core/terramate/terramate.go`：Adapter 接口（Run/Version）+ RunResult struct
- [ ] 2.2 创建 `server/core/terramate/exec.go`：ExecAdapter（exec.CommandContext + cmd.Dir + stdout/stderr 捕获 + exitCode 提取）
- [ ] 2.3 创建 `server/core/terramate/exec_test.go`：fake terramate 脚本测试（stdout 捕获、exit code 1、工作目录、context 取消、binary not found）

## 3. D1 边界守护测试

- [ ] 3.1 创建 `server/internal/audit/d1_guard_test.go`：go/parser AST 遍历 server/**/*.go，断言无 terramate import

## 4. 验证

- [ ] 4.1 `go build ./...` 通过
- [ ] 4.2 `go vet ./...` 通过
- [ ] 4.3 `go test ./core/adapters/... ./core/terramate/... ./internal/audit/... -short` 通过
- [ ] 4.4 gofmt clean
