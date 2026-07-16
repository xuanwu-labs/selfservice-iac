# Tasks: w1-adapter-interfaces

## 01-Adapter interfaces (6 adapters + noop stubs)

- [x] 1.1 Create `server/core/adapters/git/git.go`: GitProvider interface (Clone/Fetch/CommitSHA) + NoopGit
- [x] 1.2 Create `server/core/adapters/cloud/cloud.go`: CloudProvider interface (ValidateCredentials/ListRegions) + NoopCloud + Credentials struct
- [x] 1.3 Create `server/core/adapters/state/state.go`: StateBackend interface (Read/Write/Delete/Lock/Unlock) + NoopState
- [x] 1.4 Create `server/core/adapters/policy/policy.go`: PolicyEngine interface (Evaluate) + NoopPolicy + Result struct
- [x] 1.5 Create `server/core/adapters/cost/cost.go`: CostEstimator interface (Estimate) + NoopCost + Result struct
- [x] 1.6 Create `server/core/adapters/notify/notify.go`: Notifier interface (Notify) + NoopNotifier + Notification struct
- [x] 1.7 Create `server/core/adapters/provider.go`: wire ProviderSet binding all 6 noop adapters

## 02-TerramateAdapter (D1 exec boundary)

- [x] 2.1 Create `server/core/terramate/terramate.go`: Adapter interface (Run/Version) + RunResult struct
- [x] 2.2 Create `server/core/terramate/exec.go`: ExecAdapter implementation (exec.CommandContext, stdout/stderr capture, exit code)
- [x] 2.3 Create `server/core/terramate/exec_test.go`: test with fake terramate script (exit code propagation, stdout capture, ctx cancel, working dir)

## 03-D1 guard test

- [x] 3.1 Create `server/internal/audit/d1_guard_test.go`: AST-walk all server/**/*.go, assert no import of `github.com/terramate-io/terramate`

## 04-Tests + verification

- [x] 4.1 `go build ./...` pass
- [x] 4.2 `go vet ./...` pass
- [x] 4.3 `go test ./server/core/adapters/... ./server/core/terramate/... ./server/internal/audit/...` pass (short mode, no Docker needed)
- [x] 4.4 gofmt clean
