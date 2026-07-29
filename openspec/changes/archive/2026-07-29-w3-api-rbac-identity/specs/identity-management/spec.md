## ADDED Requirements

### Requirement: 本地 identity CRUD + bootstrap admin 初始化

平台 MUST 在 `server/core/identity/` 实现 IdentityService：identity CRUD（identities 表，migration 009 已建）+ bootstrap admin 初始化。Phase 1 不对接外部用户中心（SCIM/飞书/钉钉 推迟 Phase 2），仅维护本地 identity + role_bindings。bootstrap admin 在平台首次启动时 seed（检查 admin identity 是否存在，不存在则按 config 创建 identity + role_binding actions=`["*"]`）。OIDC 登录首次出现的 subject 自动创建 identity（claim_mapping）。

#### Scenario: 首次启动 seed bootstrap admin

- **WHEN** 平台启动且 identities 表无 admin 记录
- **THEN** IdentityService 按 config（admin subject/email）创建 identity（role=admin）
- **AND** 写 role_binding（subject_id=admin, actions=`["*"]`, scope=platform）
- **AND** 再次启动若 admin 已存在则跳过（幂等）

#### Scenario: OIDC 首次登录自动创建 identity

- **WHEN** OIDC token 验证通过且 claim.sub 在 identities 表无记录
- **THEN** IdentityService 自动创建 identity（subject=claim.sub, email=claim.email）
- **AND** 默认 role=member（写 role_binding actions=`["read","request"]`）
- **AND** 后续同一 subject 登录复用已有 identity（不重复创建）
