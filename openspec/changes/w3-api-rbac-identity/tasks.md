## 0. Migration 015（P0-2：identities + role_bindings 表）

- [ ] 0.1 编写 `server/cmd/migrate/migrations/015_identity_rbac.sql`：identities 表（id/external_id/display_name/email/provider_name/primary_source/status/merged_into_id/last_synced_at）+ role_bindings 表（subject_id/role/scope_type/scope_id/actions jsonb）。含 COMMENT + UNIQUE INDEX + set_updated_at trigger
- [ ] 0.2 同步追加 schema 到 `server/pkg/db/queries/schema.sql` + sqlc generate

## 1. LifecycleService handler（task 9.1 + 9.3，P0-1 RPC 对齐）

- [ ] 1.1 实现 `server/api/connect/lifecycle.go`：LifecycleHandler 实现 LifecycleService 11 个 RPC（CreateRequest/GetRequest/ListRequests/ListRequestEvents/CancelRequest/StartPlan/GetArtifact/EvaluateGate/ListPendingApprovals/GetApprovalRun/DecideApproval/StartApply）。薄包装 orchestrator Pipeline + ApprovalService。DecideApproval → ApprovalService.Approve/Reject。CreateRequest 支持 idempotency_key（P1-5）
- [ ] 1.2 在 `server/api/connect/provider.go` 注册 LifecycleHandler
- [ ] 1.3 在 `server/internal/server/connect.go` 注册 LifecycleService 路由

## 2. OIDC token 验证（task 11.2，P2-10 用 golang-jwt/v5）

- [ ] 2.1 实现 `server/internal/auth/oidc_verifier.go`：OIDCVerifier。VerifyToken(ctx, bearerToken) → (Identity, error)。用 golang-jwt/v5 + keyfunc（JWKS fetch）。Phase 1 单 issuer（config.auth.oidc.issuer_url + audience + jwks_url）
- [ ] 2.2 实现 `server/internal/auth/oidc_verifier_test.go`：mock JWKS + 验证 token 解析 + 过期 token 拒绝

## 3. RBAC 评估 + Connect 拦截器（task 9.2 + 11.3）

- [ ] 3.1 实现 `server/internal/auth/rbac.go`：EvaluateRBAC(ctx, subjectID, action, scopeType, scopeID) (bool, reason string)。读 role_bindings 表。Phase 1 三角色（admin=*/member=read+request/owner=read+request+approve+reject）
- [ ] 3.2 填充 `server/internal/middleware/connect.go` 的 ConnectAuth() + ConnectRBAC()：Auth 提取 Bearer → OIDCVerifier → 注入 identity + correlation_id 到 ctx。RBAC 读 ctx → EvaluateRBAC → 拒绝 403
- [ ] 3.3 实现 `server/internal/auth/rbac_test.go`：admin 全通过 + member 只读 + owner 可审批 + 越权 403

## 4. 本地 Identity 管理（task 11.1）

- [ ] 4.1 实现 `server/core/identity/service.go`：IdentityService。identity CRUD（Create/GetByID/GetByExternalID/List）+ BootstrapAdmin(ctx, cfg)（启动时 idempotent 检查 + seed）
- [ ] 4.2 实现 `server/core/identity/provider.go`：wire ProviderSet

## 5. 审批（P0-4 复用 W2-06，不新建 Engine）

- [ ] 5.1 LifecycleHandler 的 DecideApproval 直接调 ApprovalService.Approve/Reject（W2-06 已实现）。ListPendingApprovals 查 approval_runs。GetApprovalRun 查 approval_run 详情。不新建 approval.Engine
- [ ] 5.2 审批人解析：实现 ResolveApprovers(ctx, teamID) → role_bindings where role=owner + scope=team → identities

## 6. 事件 + 审计（task 9.4，P1-6/7 字段对齐）

- [ ] 6.1 实现 `server/core/events/bus.go`：EventBus（进程内直接调用 + handler 注册 + error-tolerant）
- [ ] 6.2 实现 `server/core/audit/logger.go`：AuditLogger 写 audit_logs 表。字段对齐 migration 009（actor_id/actor_team_id/actor_type/action/target_type/target_id/before_json/after_json/correlation_id/occurred_at）。correlation_id 从 ctx 读
- [ ] 6.3 实现 `server/core/events/provider.go`：wire ProviderSet

## 7. wire + 验证

- [ ] 7.1 更新 `server/core/core.go`：加 identity.ProviderSet + events.ProviderSet
- [ ] 7.2 更新 `server/api/connect/provider.go`：加 LifecycleHandler
- [ ] 7.3 更新 `server/internal/server/connect.go`：注册 LifecycleService
- [ ] 7.4 `go build ./... && go vet ./...` 通过
- [ ] 7.5 `go test ./server/api/connect/... ./server/internal/auth/... ./server/core/{identity,events}/... -short` 通过
- [ ] 7.6 `gofmt -l server/` 无输出
- [ ] 7.7 提交到 `feat/w3-api-rbac-identity` 分支
