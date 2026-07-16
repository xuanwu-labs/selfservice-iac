# Design: w1-adapter-interfaces

> Corresponds to design.md D7 (pluggable adapters) + D1 (terramate exec boundary) + D29 (TerramateAdapter usage).

## 01-Adapter Interface Design

### 1.1 Directory layout

```
server/core/adapters/
├── git/
│   ├── git.go          # GitProvider interface + NoopGit
│   └── git_test.go
├── cloud/
│   ├── cloud.go        # CloudProvider interface + NoopCloud
│   └── cloud_test.go
├── state/
│   ├── state.go        # StateBackend interface + NoopState
│   └── state_test.go
├── policy/
│   ├── policy.go       # PolicyEngine interface + NoopPolicy
│   └── policy_test.go
├── cost/
│   ├── cost.go         # CostEstimator interface + NoopCost
│   └── cost_test.go
├── notify/
│   ├── notify.go       # Notifier interface + NoopNotifier
│   └── notify_test.go
└── provider.go         # wire ProviderSet for all adapters
```

### 1.2 Interface contracts

Each interface follows the same pattern: domain-specific methods + a noop stub that fails loud.

**GitProvider** (consumed by registry module 03 for module clone):
```go
type GitProvider interface {
    Clone(ctx context.Context, url, ref, dest string) error
    Fetch(ctx context.Context, dir string) error
    CommitSHA(ctx context.Context, dir string) (string, error)
}
```

**CloudProvider** (consumed by cloudcreds module for credential validation):
```go
type CloudProvider interface {
    ValidateCredentials(ctx context.Context, creds CloudCredentials) error
    ListRegions(ctx context.Context, creds CloudCredentials) ([]string, error)
}
```

**StateBackend** (consumed by codegen module 05 for state read/write):
```go
type StateBackend interface {
    Read(ctx context.Context, key string) ([]byte, error)
    Write(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    Lock(ctx context.Context, key string) (string, error)
    Unlock(ctx context.Context, key, lockID string) error
}
```

**PolicyEngine** (consumed by gate module for OPA evaluation, D28 S3/S6):
```go
type PolicyEngine interface {
    Evaluate(ctx context.Context, policy string, input any) (PolicyResult, error)
}

type PolicyResult struct {
    Allow       bool
    Violations  []string
}
```

**CostEstimator** (consumed by finops module for Infracost estimation):
```go
type CostEstimator interface {
    Estimate(ctx context.Context, planPath string) (CostResult, error)
}

type CostResult struct {
    MonthlyCostCents int64
    Currency         string
}
```

**Notifier** (consumed by events module for IM/webhook notifications):
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

### 1.3 Noop stub pattern

Every stub returns a structured error, not a silent pass:

```go
type NoopGit struct{}

func (NoopGit) Clone(ctx context.Context, url, ref, dest string) error {
    return fmt.Errorf("git adapter not configured: set adapters.git.impl in config")
}
```

This ensures missing adapters fail immediately at runtime, not silently degrade.

## 02-TerramateAdapter (D1 boundary)

### 2.1 Interface

```go
// server/core/terramate/terramate.go
type TerramateAdapter interface {
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

### 2.2 Exec implementation

```go
type ExecAdapter struct {
    binaryPath string  // path to terramate binary, default "terramate"
}

func (a *ExecAdapter) Run(ctx context.Context, dir string, args []string) (RunResult, error) {
    cmd := exec.CommandContext(ctx, a.binaryPath, args...)
    cmd.Dir = dir  // run in stack directory (D29)
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

### 2.3 Testing strategy

Tests use a fake `terramate` shell script (like Terramate's own test fixtures) to assert:
- Exit code propagation
- stdout/stderr capture
- Context cancellation
- Working directory set correctly

## 03-D1 Guard Test

### 3.1 Problem

`.golangci.yml` depguard denies `github.com/terramate-io/terramate` imports, but the rule is non-enforcing: when terramate is not in go.mod, the typechecker silently drops the rule. A dedicated test closes this gap.

### 3.2 Implementation

```go
// server/internal/audit/d1_guard_test.go
func TestNoTerramateImports(t *testing.T) {
    // Walk all .go files under server/, parse imports, assert none match
    // "github.com/terramate-io/terramate"
}
```

Uses `go/parser` + `go/ast` to walk the AST of every `.go` file under `server/`, collecting import paths, and asserting none start with `github.com/terramate-io/terramate`.

This is a pure compile-time check — no runtime, no dependencies, runs in `go test -short` mode.

## 04-wire ProviderSet

All adapters + TerramateAdapter registered in a single ProviderSet:

```go
// server/core/adapters/provider.go
var ProviderSet = wire.NewSet(
    git.NoopGit{},
    cloud.NoopCloud{},
    state.NoopState{},
    policy.NoopPolicy{},
    cost.NoopCost{},
    notify.NoopNotifier{},
    terramate.NewExecAdapter,
)
```

Consumers inject the interface, not the concrete type. Wire resolves to the noop stub by default; future changes replace stubs with real implementations via `wire.Bind`.
