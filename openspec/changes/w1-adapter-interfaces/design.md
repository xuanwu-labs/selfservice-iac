# 设计：w1-adapter-interfaces

> 对应 design.md D7（可插拔适配器）+ D1（terramate exec 边界）+ D29（TerramateAdapter 使用方式）。

## 01-适配器接口设计

### 1.1 目录布局

```
server/core/adapters/
├── git/
│   ├── git.go          # GitProvider 接口 + NoopGit
│   └── git_test.go
├── cloud/
│   ├── cloud.go        # CloudProvider 接口 + NoopCloud
│   └── cloud_test.go
├── state/
│   ├── state.go        # StateBackend 接口 + NoopState
│   └── state_test.go
├── policy/
│   ├── policy.go       # PolicyEngine 接口 + NoopPolicy
│   └── policy_test.go
├── cost/
│   ├── cost.go         # CostEstimator 接口 + NoopCost
│   └── cost_test.go
├── notify/
│   ├── notify.go       # Notifier 接口 + NoopNotifier
│   └── notify_test.go
└── provider.go         # 所有适配器的 wire ProviderSet
```

### 1.2 接口契约

每个接口遵循相同模式：领域方法 + 失败即报错的 noop stub。

**GitProvider**（被注册模块 03 消费，用于模块 clone）：
```go
type GitProvider interface {
    Clone(ctx context.Context, url, ref, dest string) error
    Fetch(ctx context.Context, dir string) error
    CommitSHA(ctx context.Context, dir string) (string, error)
}
```

**CloudProvider**（被 cloudcreds 模块消费，用于凭据校验）：
```go
type CloudProvider interface {
    ValidateCredentials(ctx context.Context, creds Credentials) error
    ListRegions(ctx context.Context, creds Credentials) ([]string, error)
}
```

**StateBackend**（被代码生成模块 05 消费，用于 state 读写）：
```go
type StateBackend interface {
    Read(ctx context.Context, key string) ([]byte, error)
    Write(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    Lock(ctx context.Context, key string) (string, error)
    Unlock(ctx context.Context, key, lockID string) error
}
```

**PolicyEngine**（被 gate 模块消费，用于 OPA 评估，D28 S3/S6）：
```go
type PolicyEngine interface {
    Evaluate(ctx context.Context, policy string, input any) (Result, error)
}

type Result struct {
    Allow      bool
    Violations []string
}
```

**CostEstimator**（被 finops 模块消费，用于 Infracost 估算）：
```go
type CostEstimator interface {
    Estimate(ctx context.Context, planPath string) (Result, error)
}

type Result struct {
    MonthlyCostCents int64
    Currency         string
}
```

**Notifier**（被 events 模块消费，用于 IM/webhook 通知）：
```go
type Notifier interface {
    Notify(ctx context.Context, event Notification) error
}

type Notification struct {
    Type    string
    Title   string
    Message string
}
```

### 1.3 Noop stub 模式

每个 stub 返回结构化错误，不静默通过：

```go
type NoopGit struct{}

func (NoopGit) Clone(ctx context.Context, url, ref, dest string) error {
    return fmt.Errorf("git adapter not configured: set adapters.git.impl in config")
}
```

确保缺失的适配器在运行时立即失败，不静默降级。

## 02-TerramateAdapter（D1 边界）

### 2.1 接口

```go
// server/core/terramate/terramate.go
type Adapter interface {
    Run(ctx context.Context, dir string, args []string) (RunResult, error)
    Version(ctx context.Context) (string, error)
}

type RunResult struct {
    ExitCode   int
    Stdout     string
    Stderr     string
    DurationMs int64
}
```

### 2.2 Exec 实现

```go
type ExecAdapter struct {
    binaryPath string  // terramate 二进制路径，默认 "terramate"
}

func (a *ExecAdapter) Run(ctx context.Context, dir string, args []string) (RunResult, error) {
    cmd := exec.CommandContext(ctx, a.binaryPath, args...)
    cmd.Dir = dir  // 在 stack 目录运行（D29）
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    start := time.Now()
    err := cmd.Run()
    return RunResult{
        ExitCode:   exitCode(err),
        Stdout:     stdout.String(),
        Stderr:     stderr.String(),
        DurationMs: time.Since(start).Milliseconds(),
    }, err
}
```

### 2.3 测试策略

测试用 fake `terramate` shell 脚本（类似 Terramate 自己的测试 fixture），断言：
- exit code 传播
- stdout/stderr 捕获
- context 取消传播
- 工作目录正确设置

## 03-D1 边界守护测试

### 3.1 问题

`.golangci.yml` depguard 禁止 `github.com/terramate-io/terramate` import，但规则不生效：terramate 不在 go.mod 时 typechecker 静默丢弃规则。专用测试弥补此缺口。

### 3.2 实现

```go
// server/internal/audit/d1_guard_test.go
func TestNoTerramateImports(t *testing.T) {
    // 遍历 server/ 下所有 .go 文件，解析 import，断言无 "github.com/terramate-io/terramate"
}
```

用 `go/parser` + `go/ast` 遍历 `server/` 下每个 `.go` 文件的 AST，收集 import 路径，断言无 `github.com/terramate-io/terramate`。

纯编译期检查——无运行时、无依赖、`go test -short` 可跑。

## 04-wire ProviderSet

所有适配器 + TerramateAdapter 注册在一个 ProviderSet：

```go
// server/core/adapters/provider.go
var ProviderSet = wire.NewSet(
    wire.Bind(new(git.GitProvider), new(git.NoopGit)),
    wire.Bind(new(cloud.CloudProvider), new(cloud.NoopCloud)),
    wire.Bind(new(state.StateBackend), new(state.NoopState)),
    wire.Bind(new(policy.PolicyEngine), new(policy.NoopPolicy)),
    wire.Bind(new(cost.CostEstimator), new(cost.NoopCost)),
    wire.Bind(new(notify.Notifier), new(notify.NoopNotifier)),
)
```

消费方注入接口，不注入具体类型。Wire 默认解析到 noop stub；后续 change 通过 `wire.Bind` 替换 stub 为真实实现。
