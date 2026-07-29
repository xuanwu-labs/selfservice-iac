## 1. LifecycleService handler（task 9.1 + 9.3）

- [ ] 1.1 实现 `server/api/connect/lifecycle.go`：LifecycleHandler 实现 LifecycleService（CreateRequest/GetRequest/ListRequests/CancelRequest/StartPlan/StartApply/ApproveRequest/RejectRequest/ListApprovals）。注入 Pipeline + ApprovalEngine + RequestRepo
- [ ] 1.2 在 `server/api/connect/provider.go` 注册 LifecycleHandler
- [ ] 1.3 在 `server/internal/server/connect.go` 注册 LifecycleService 路由

## 2. OIDC token 验证（task 11.2）

- [ ] 2.1 实现 `server/internal/auth/oidc_verifier.go`：OIDCVerifier。VerifyToken(ctx, bearerToken) → (Identity, error)。用 zitadel/oidc 验证 JWT 签名 + 提取 claim。Phase 1 单 issuer
- [ ] 2.2 实现 `server/internal/auth/oidc_verifier_test.go`：mock JWKS + 验证 token 解析 + 过期 token 拒绝

## 3. RBAC 评估 + Connect 拦截器（task 9.2 + 11.3）

- [ ] 3.1 实现 `server/internal/auth/rbac.go`：EvaluateRBAC(ctx, subjectID, action, scopeType, scopeID) bool。读 role_bindings 表。Phase 1 三角色（admin/member/owner）
- [ ] 3.2 填充 `server/internal/middleware` 的 ConnectAuth() + ConnectRBAC() 拦截器（已有占位）：Auth 提取 Bearer token → OIDC 验证 → 注入 identity 到 context。RBAC 读 context identity → EvaluateRBAC → 拒绝 403
- [ ] 3.3 实现 `server/internal/auth/rbac_test.go`：admin 全通过 + member 只读 + owner 可审批 + 越权 403

## 4. 本地 Identity 管理（task 11.1）

- [ ] 4.1 实现 `server/core/identity/service.go`：IdentityService。identity CRUD（Create/GetByID/GetByExternalID/List）+ bootstrap admin 初始化（首次启动 seed admin identity + admin role_binding）
- [ ] 4.2 实现 `server/core/identity/provider.go`：wire ProviderSet

## 5. 审批引擎（task 12.1-12.3 Phase 1）

- [ ] 5.1 实现 `server/core/approval/engine.go`：Engine。Start(ctx, requestID) → 创建 approval_run（单节点 Phase 1）。Decide(ctx, runID, approverID, decision, reason) → 写 approval_decisions + 状态转换。Status(ctx, requestID) → 审批状态
- [ ] 5.2 审批人解析：ResolveApprovers(ctx, teamID) → 该 team 的 owner 角色成员列表
- [ ] 5.3 实现 `server/core/approval/provider.go`：wire ProviderSet

## 6. 事件 + 审计（task 9.4）

- [ ] 6.1 实现 `server/core/events/bus.go`：EventBus（进程内直接调用 + handler 注册）
- [ ] 6.2 实现 `server/core/audit/logger.go`：AuditLogger（写 audit_logs 表：actor/action/target/before/after/correlation_id）
- [ ] 6.3 实现 `server/core/events/provider.go`：wire ProviderSet

## 7. wire + 验证

- [ ] 7.1 更新 `server/core/core.go`：加 identity.ProviderSet + approval.ProviderSet + events.ProviderSet
- [ ] 7.2 更新 `server/api/connect/provider.go`：加 LifecycleHandler
- [ ] 7.3 更新 `server/internal/server/connect.go`：注册 LifecycleService
- [ ] 7.4 `go build ./... && go vet ./...` 通过
- [ ] 7.5 `go test ./server/api/connect/... ./server/internal/auth/... ./server/core/{identity,approval,events}/... -short` 通过
- [ ] 7.6 `gofmt -l server/` 无输出
- [ ] 7.7 提交到 `feat/w3-api-rbac-identity` 分支
