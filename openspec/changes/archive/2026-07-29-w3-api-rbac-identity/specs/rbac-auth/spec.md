## ADDED Requirements

### Requirement: OIDC token 验证 + RBAC 评估 + Connect 拦截器

平台 MUST 在 `server/internal/auth/` 填充两个 Connect 拦截器（middleware.ConnectAuth + middleware.ConnectRBAC，已有占位）。ConnectAuth：从 Authorization header 提取 Bearer JWT → 用 zitadel/oidc（已在 go.mod）验证签名（JWKS from OIDC issuer）+ claim → claim_mapping 解析 platform identity（首次登录自动创建）。ConnectRBAC：读 role_bindings 表（subject × scope × action）评估权限，Phase 1 三角色：bootstrap admin（actions=`["*"]`）、team member（`["read","request"]`）、team owner（`["read","request","approve","reject"]`）。

#### Scenario: 有效 token 解析出 identity

- **WHEN** 请求带 `Authorization: Bearer <valid-JWT>` 且 JWKS 验签通过 + claim 含 sub/email
- **THEN** ConnectAuth 解析出 identity（sub → identity_id）
- **AND** 首次登录自动创建 identity（claim_mapping）
- **AND** identity 注入 ctx 供下游 handler 使用

#### Scenario: 无效或缺失 token 返回 401

- **WHEN** 请求无 Authorization header，或 JWT 签名无效，或 token 已过期
- **THEN** ConnectAuth 返回 `401 Unauthorized`
- **AND** 不进入 handler

#### Scenario: admin 通配放行

- **WHEN** subject 的 role_binding actions=`["*"]`（bootstrap admin）且访问任意 RPC
- **THEN** ConnectRBAC 评估通过
- **AND** 请求进入 handler

#### Scenario: member 只读被拦截 approve

- **WHEN** subject 是 team member（actions=`["read","request"]`）且调用 ApproveRequest（action=`approve`）
- **THEN** ConnectRBAC 评估失败
- **AND** 返回 `403 Forbidden`

#### Scenario: owner 可审批其 team 工单

- **WHEN** subject 是 request.owner_team 的 owner（actions 含 `approve`）且 scope 匹配
- **THEN** ConnectRBAC 评估通过
- **AND** ApproveRequest 进入 handler
