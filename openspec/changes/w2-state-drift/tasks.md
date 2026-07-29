## 1. State Backend 适配器（task 8.1）

- [ ] 1.1 实现 `server/core/adapters/state/s3.go`：S3Backend 实现 StateBackend 接口（Read/Write/Delete/Lock/Unlock）。Phase 1 用内存 mock（不连真实 S3 SDK）。接口 + 签名正确，真实 S3 SDK 集成 Phase 2
- [ ] 1.2 扩展 `server/core/adapters/state/provider.go`：wire ProviderSet 加 S3Backend 绑定（替换 noop 为 S3Backend 作为默认实现，noop 保留测试用）
- [ ] 1.3 实现 `server/core/adapters/state/s3_test.go`：测 Read/Write/Delete/Lock/Unlock mock 逻辑

## 2. DriftScheduler 调度器（task 8.2）

- [ ] 2.1 实现 `server/core/drift/scheduler.go`：DriftScheduler。分层调度（Global 24h / Middleware 12h / Application 6h）+ 令牌桶限流（per-layer 并发上限）。Start(ctx) / Stop()。Phase 1 用 time.Ticker（单进程）
- [ ] 2.2 实现 `server/core/drift/scheduler_test.go`：测限流不超上限 + 调度频率正确

## 3. DriftWorker 检测流程（task 8.2）

- [ ] 3.1 实现 `server/core/drift/worker.go`：DriftWorker。CheckStack(ctx, stackID)：workspace checkout（只读）→ terramate plan -detailed-exitcode → 解析差异 → 记录 drift_runs/records（Phase 1 内存）→ 发事件。接口注入（TerramateRunner + WorkspaceManager + Notifier）
- [ ] 3.2 实现 `server/core/drift/worker_test.go`：mock TerramateRunner（返回 exit 0/2/1）→ 验证 drift 判定正确

## 4. PlanParser（terraform show -json 解析，task 8.2）

- [ ] 4.1 实现 `server/core/drift/parser.go`：PlanParser。解析 terraform show -json 输出的 resource_changes，提取 change.actions != no-op 的资源。返回 DiffSummary
- [ ] 4.2 实现 `server/core/drift/parser_test.go`：用 testdata plan JSON 样本断言解析正确（有差异/无差异/创建/删除/更新）

## 5. testdata

- [ ] 5.1 创建 `server/core/drift/testdata/plan-no-drift.json`：terraform plan exit 0 的 JSON 样本（resource_changes 全 no-op）
- [ ] 5.2 创建 `server/core/drift/testdata/plan-with-drift.json`：terraform plan exit 2 的 JSON 样本（resource_changes 含 create/update/delete）

## 6. wire + 验证

- [ ] 6.1 实现 `server/core/drift/provider.go`：wire ProviderSet
- [ ] 6.2 更新 `server/core/core.go`：加 drift.ProviderSet + state.S3Backend 绑定
- [ ] 6.3 `go build ./... && go vet ./...` 通过
- [ ] 6.4 `go test ./server/core/drift/... ./server/core/adapters/state/... -short` 通过
- [ ] 6.5 `gofmt -l server/` 无输出
- [ ] 6.6 提交到 `feat/w2-state-drift` 分支
