# 21-运维 Runbook

> Runbook 是企业级交付物，不是上线后再补的口头经验。每个 runbook 都必须有触发条件、判断步骤、恢复动作、升级路径和演练要求。

## 1. Runbook 索引

| ID | 场景 | 严重度 | 默认 owner |
|----|------|--------|-------------|
| RB-001 | apply interrupted | P1 | Platform Ops |
| RB-002 | state lock stuck | P1 | Platform Ops |
| RB-003 | provider mirror down | P2 | Platform Ops |
| RB-004 | OIDC JWKS expired | P1 | Security / Platform Ops |
| RB-005 | DB restore | P0 | DBA / Platform Ops |
| RB-006 | Executor node lost | P1 | Platform Ops |
| RB-007 | CMDB reconcile failed | P2 | CMDB Owner |
| RB-008 | secret leakage detected | P0 | Security |
| RB-009 | bootstrap credential rotation | P1 | Cloud Guardian |
| TD-001 | wire CLI 工具链失败（NewManager 多参数歧义） | P2 | Platform Dev |
| TD-002 | Windows 开发环境 Docker 不可用（DB 测试 14 FAIL） | P3 | Platform Dev |
| TD-003 | 测试内嵌 migration 滞后（pkg/db/migrations 缺文件） | P3 | Platform Dev |

## 1a. 技术债追踪（2026-07-29 记录）

> 以下技术债在 W1-W3 实现过程中发现，记录在此供追踪。非阻塞 MVP，但需在 Phase 2 前解决。

### TD-001：wire CLI 工具链失败

**现象**：`wire gen ./cmd/server/...` 失败，报错 `multiple parameters of the same type (string, string)` 在 `workspace.NewManager(worktreeRoot, nodeID string)`。

**根因**：wire 通过参数类型推断依赖，`NewManager(string, string)` 两个 string 参数无法区分（worktreeRoot vs nodeID）。这是 wire 的已知限制（同类型多参数）。

**临时方案**：手动编辑 `server/cmd/server/wire_gen.go`（不跑 wire CLI）。

**Phase 2 修复**：
- 方案 A：用 wire `wire.FieldsOf` 或自定义 provider function 包装参数
- 方案 B：改为 config struct 参数 `NewManager(cfg WorkspaceConfig)`（单参数，wire 可注入）
- 方案 C：升级 wire 到支持 named parameters 的版本（如果有）

**影响范围**：每次新增 ProviderSet 到 core.go 后，wire_gen.go 需手动同步。

### TD-002：Windows 开发环境 Docker 不可用

**现象**：`go test` 的 14 个 DB 依赖测试（identity/RBAC/repo）FAIL，报错 "rootless Docker is not supported on Windows"。

**根因**：testcontainers-go 在 Windows 上需要 Docker Desktop（rootless 模式不支持）。

**临时方案**：
- 本地用 `-short` 跳过 DB 测试（`go test -short ./...`）
- CI 环境（Linux + Docker）全量测试

**Phase 2 修复**：
- 方案 A：开发环境配置 Docker Desktop（Windows 原生支持）
- 方案 B：用 WSL2 + Docker（已有方案）
- 方案 C：用 docker-proxy 方式（见项目 memory：我们使用 dockerproxy 的使用方式）

### TD-003：测试内嵌 migration 滞后

**现象**：`server/pkg/db/migrations/`（测试内嵌的 migration 副本）缺少 013/014/015，导致 testcontainers 起的测试 DB 没有 env/tenant/identity 表。

**根因**：migration 文件在 `server/cmd/migrate/migrations/` 新增后，忘记同步到 `server/pkg/db/migrations/`（testcontainers embed 用的是后者）。

**临时方案**：W3 实现时 Agent 已手动同步（复制 013/014/015 到 pkg/db/migrations/）。

**Phase 2 修复**：
- 方案 A：改为 `//go:embed ../../cmd/migrate/migrations/*.sql`（单一真相源）
- 方案 B：CI 检查 `diff server/cmd/migrate/migrations/ server/pkg/db/migrations/`（一致性校验）
- 方案 C：testdb 直接用 cmd/migrate 的 goose runner（不 embed 副本）

## 2. RB-001 apply interrupted

触发：

- `executor_runs.status=interrupted`
- request 卡在 `applying`
- executor heartbeat 超时

处理：

1. 禁止直接重跑 apply。
2. 创建 `manual_intervention_tasks`。
3. 读取 state backend，确认资源真实状态。
4. 若 plan 已部分执行，运行只读 plan 判断差异。
5. 人工选择 `adopt-cloud`、`restore-desired` 或 `mark-failed-terminal`。
6. 记录恢复动作到 audit 和 incident。

## 3. RB-002 state lock stuck

触发：

- Terraform backend lock 超过阈值。
- 无 active executor 持有该 request。

处理：

1. 检查 executor heartbeat。
2. 检查 state backend lock owner。
3. 若确认孤儿锁，双人审批后 unlock。
4. unlock 后必须运行 plan 验证 state 一致性。

## 4. RB-003 provider mirror down

处理：

1. 将新 plan 请求置为 `blocked-state-health`。
2. 已进入 apply 且使用已有 plugin cache 的请求可按风险策略继续。
3. 恢复 mirror 后运行 stack-health-check。

## 5. RB-004 OIDC JWKS expired

处理：

1. 暂停新 apply。
2. 重新发布 JWKS。
3. 对每个 cloud account 执行 assume role smoke test。
4. 恢复 `team_cloud_grants` 可见性。

## 6. RB-005 DB restore

处理：

1. 冻结新 request。
2. 按最近备份恢复 DB。
3. 运行 DB ↔ Git ↔ artifact ↔ state reconcile。
4. 对恢复窗口内的 request 创建人工核对任务。

## 7. RB-006 Executor node lost

处理：

1. 标记节点 unavailable。
2. 对该节点 running executor_runs 分类：plan 可重跑，apply 进入 `waiting-manual`。
3. 回收或重建 workspace checkout。

## 8. RB-007 CMDB reconcile failed

处理：

1. request 可进入 `reconcile-pending`，不回滚资源。
2. outbox 重试。
3. dead-letter 后创建 owner 任务。
4. 补偿成功后把 request 更新为 `succeeded` 或保留 pending 并展示债务。

## 9. RB-008 secret leakage detected

处理：

1. 立即阻断相关 cloud credential。
2. 触发 rotation。
3. 扫描 Git、日志、对象存储。
4. 关联受影响 request 和 executor run。
5. 产出 incident report。

## 10. 演练验收

每个 runbook 必须每年至少演练一次；P0/P1 runbook 每季度抽样演练。演练结果写入 `drill_results`，失败项进入 `manual_intervention_tasks` 或改进 backlog。
