## 1. State Backend 适配器（task 8.1）

- [ ] 1.1 实现 `server/core/adapters/state/s3.go`：S3Backend 实现 StateBackend 接口。接口签名：`Read(ctx, key) ([]byte, error)` / `Write(ctx, key, data)` / `Delete(ctx, key)` / `Lock(ctx, key) (lockID string, err error)` / `Unlock(ctx, key, lockID) error`。Phase 1 内存 mock（不连 S3 SDK）。**NoopState 保持为 wire 默认**（不替换为 mock）
- [ ] 1.2 扩展 `server/core/adapters/provider.go`：**不改默认绑定**（NoopState 保持 line 25）。S3Backend 仅在测试中使用（不绑 wire.Bind 替换 noop）
- [ ] 1.3 实现 `server/core/adapters/state/s3_test.go`：测 Read/Write/Delete/Lock（返回 lockID）/Unlock（接收 lockID）mock 逻辑

## 2. DriftScheduler 调度器（task 8.2）

- [ ] 2.1 实现 `server/core/drift/scheduler.go`：DriftScheduler。分层调度（DefaultIntervals 返回 map，构造器接受 intervals 参数可注入测试用 100ms）+ 令牌桶限流（`golang.org/x/time/rate`，per-layer 并发上限 DefaultConcurrency）。Start(ctx) / Stop(ctx)（优雅 drain）。注入 `clock.Clock`（server/core/clock/）不用 time.Now 直接调用
- [ ] 2.2 实现 `server/core/drift/scheduler_test.go`：测限流不超上限 + 调度频率正确（用 100ms intervals 避免慢测试）

## 3. DriftWorker 检测流程（task 8.2）

- [ ] 3.1 实现 `server/core/drift/worker.go`：DriftWorker。声明**本地接口**（Runner/CheckoutProvider/Notifier），不导入 orchestrator 包。`*terramate.ExecAdapter` 隐式满足 Runner。CheckStack(ctx, stackID)：checkout（只读）→ terramate plan -detailed-exitcode → exit code 映射（0=无漂移 / 2=有漂移 error=nil / 1=错误）→ 记录（MemDriftRepo 内存 Phase 1）→ 通知
- [ ] 3.2 实现 `server/core/drift/worker_test.go`：mock Runner（返回 exit 0/2/1）→ 验证 drift 判定正确。含 exit 1 error path 测试

## 4. PlanParser（terraform show -json 解析，task 8.2）

- [ ] 4.1 实现 `server/core/drift/parser.go`：PlanParser。解析 terraform plan JSON 的 resource_changes。**嵌套 struct**（ChangeBody.Actions，不用 "change.actions" 点号 tag）。提取 actions != ["no-op"] 的资源
- [ ] 4.2 实现 `server/core/drift/parser_test.go`：用 testdata plan JSON 样本断言解析正确（有差异/无差异/创建/删除/更新）

## 5. testdata

- [ ] 5.1 创建 `server/core/drift/testdata/plan-no-drift.json`：exit 0 的 JSON 样本（resource_changes 全 no-op）
- [ ] 5.2 创建 `server/core/drift/testdata/plan-with-drift.json`：exit 2 的 JSON 样本（resource_changes 含 create/update/delete）
- [ ] 5.3 创建 `server/core/drift/testdata/plan-error.json`：exit 1 error path 的 JSON 样本

## 6. wire + 验证

- [ ] 6.1 实现 `server/core/drift/provider.go`：wire ProviderSet
- [ ] 6.2 更新 `server/core/core.go`：加 drift.ProviderSet（S3Backend 不替换 noop 默认）
- [ ] 6.3 `go build ./... && go vet ./...` 通过
- [ ] 6.4 `go test ./server/core/drift/... ./server/core/adapters/state/... -short` 通过
- [ ] 6.5 `gofmt -l server/` 无输出
- [ ] 6.6 提交到 `feat/w2-state-drift` 分支
