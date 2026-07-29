## Why

W3 是 MVP 的"门面层"——把 W1+W2 的全部后端能力通过 API + RBAC + 身份 + 审批暴露给用户。没有 W3，后端全通但用户无法操作：不能提交工单、不能审批、不能查看漂移、没有权限控制。

**影响层级**：传输层（`server/api/connect/`）+ 安全层（`server/internal/auth/`）+ 业务核心（`server/core/{identity,approval,events}`）。

**为什么现在做**：W1+W2 全部归档（8 模块），编排引擎 + codegen + workspace + state/drift 全部就位。W3 把它们串到 API + 前端可用。

## What Changes

### 1. LifecycleService API handler（task 9.1 + 9.3）

新建 `server/api/connect/lifecycle.go`：
- 实现 LifecycleService（proto 已定义）：CreateRequest / GetRequest / ListRequests / CancelRequest / StartPlan / GetArtifact / EvaluateGate / StartApply / ApproveRequest / RejectRequest / ListApprovals
- 接 orchestrator Pipeline + ApprovalService（W2-06）
- `POST CreateRequest` 同步建单 → 异步推进（调 Pipeline.Execute）

### 2. RBAC 鉴权（task 9.2 + 11.3）

扩展 `server/internal/auth/`：
- **OIDC token 验证**：从 Authorization header 提取 JWT → 验证签名 + claim → 提取 identity
- **RBAC 评估**：role_bindings 表评估（subject × scope × action）。Phase 1 简化：
  - bootstrap admin（本地 identity + admin role）
  - team member（OIDC 登录 → team 成员 → read + request 权限）
  - team owner（审批权限）
- **Connect 拦截器**：已有 `ConnectAuth()` + `ConnectRBAC()` 拦截器占位（middleware），填充真实逻辑

### 3. 本地 Identity 管理（task 11.1 Phase 1）

新建 `server/core/identity/`：
- **IdentityService**：identity CRUD + bootstrap admin 初始化
- Phase 1 简化：不对接外部用户中心（SCIM/飞书/钉钉 推迟 Phase 2）
- bootstrap admin = 本地 identity + admin role（平台首次启动 seed）

### 4. OIDC 登录（task 11.2 Phase 1）

扩展 `server/internal/auth/oidc.go`：
- 单 OIDC issuer 登录（zitadel/oidc 已在 go.mod）
- claim_mapping：OIDC token claim → platform identity
- Phase 1 不做多 issuer（Phase 3）

### 5. 审批引擎扩展（task 12.1-12.3 Phase 1）

新建 `server/core/approval/`：
- **ApprovalEngine**：Start / Decide / Status 接口
- Phase 1 简化（W2-06 已做基础 ApprovalService，这里扩展为引擎）：
  - 单级审批（不做多级/会签/条件分支/超时升级——Phase 2）
  - 审批人解析（team + role → identities）
  - 持久化 approval_runs + approval_decisions

### 6. 事件 + 审计（task 9.4）

新建 `server/core/events/`：
- **EventBus**：进程内事件分发（Phase 1 直接函数调用，不做消息队列）
- **AuditLogger**：append-only 审计日志（audit_logs 表）

### 不做（Phase 2/3 推迟）

- 多级/会签/条件/超时审批（Phase 2）
- 双门禁 pre-plan（Phase 2，D21）
- SCIM / 飞书 / 钉钉 目录同步（Phase 2）
- 多 OIDC issuer（Phase 3）
- Temporal 升级（Phase 3）
- Webhook 分发（Phase 2）
- 前端（W5 task 10）

## Capabilities

### New Capabilities

- `lifecycle-api`: LifecycleService handler（工单 CRUD + plan/apply/approve/reject API）
- `rbac-auth`: OIDC token 验证 + RBAC 评估 + Connect 拦截器
- `identity-management`: 本地 identity CRUD + bootstrap admin + Phase 1 单 OIDC
- `approval-engine`: 审批引擎（Phase 1 单级 + 审批人解析 + 持久化）
- `event-audit`: 进程内事件分发 + append-only 审计日志

## Impact

- **代码**：新建 `api/connect/lifecycle.go` + `core/identity/` + `core/approval/` + `core/events/` + 扩展 `internal/auth/`
- **API**：LifecycleService 从 proto 定义变为真实实现
- **依赖**：zitadel/oidc（已在 go.mod）
- **DB**：不改 schema（用 identities/role_bindings/approval_*/audit_logs 表，migration 007/009 已建）
- **测试**：API 契约测试 + RBAC 越权 403 + 审批流转 + 审计写入
