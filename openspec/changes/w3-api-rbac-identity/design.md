## Context

W3 是 MVP 的门面层——API + RBAC + 身份 + 审批，把 W1+W2 后端能力暴露给用户。

**现有架构约束**（已落地）：
- LifecycleService proto（已定义）：CreateRequest/GetRequest/ListRequests/CancelRequest/StartPlan/GetArtifact/EvaluateGate/StartApply/ApproveRequest/RejectRequest/ListApprovals
- Connect 拦截器占位：`middleware.ConnectAuth()` + `middleware.ConnectRBAC()`（已有，空实现）
- auth 包：argon2id + JWT 签名/验证 + OIDC 依赖锁定（zitadel/oidc）
- identities / role_bindings 表（migration 009）：已建
- approval_flows/runs/node_runs/decisions 表（migration 007）：已建
- audit_logs 表（migration 009）：已建
- orchestrator Pipeline + ApprovalService（W2-06）：已实现

**Phase 1 简化策略**（doc 00b）：
- 审批：单 pre-apply 门 + 或签（Phase 2 双门禁）
- 身份：单 OIDC issuer + 本地 identity（Phase 2 SCIM/飞书）
- RBAC：bootstrap admin + team member/owner（Phase 2 细粒度）

## Goals / Non-Goals

**Goals:**
- LifecycleService handler（工单 API 全流程）
- OIDC token 验证 + RBAC 评估 + Connect 拦截器
- 本地 identity 管理 + bootstrap admin
- 审批引擎（Phase 1 单级 + 审批人解析）
- 事件 + 审计日志

**Non-Goals:**
- 不做多级/会签/条件/超时审批（Phase 2）
- 不做双门禁 pre-plan（Phase 2）
- 不做 SCIM/飞书/钉钉（Phase 2）
- 不做多 OIDC issuer（Phase 3）
- 不做 Temporal（Phase 3）
- 不做前端（W5）

## Decisions

### D1：LifecycleService handler 薄包装 orchestrator

**决策**：LifecycleService handler 是薄包装——每个 RPC 调 orchestrator 或 approval 对应方法。不在此层做业务逻辑。

```go
type LifecycleHandler struct {
    pipeline  *orchestrator.Pipeline
    approval  *approval.Engine
    repo      *repo.RequestRepo
}
```

**理由**：业务逻辑在 orchestrator（W2-06），handler 只做 proto↔Go 转换 + 鉴权。

### D2：RBAC 评估用 role_bindings 表 + scope 匹配

**决策**：RBAC 评估从 role_bindings 表读 (subject_id, role, scope_type, scope_id, actions)。

```go
func EvaluateRBAC(ctx, subjectID string, action string, scopeType string, scopeID string) bool
// SELECT FROM role_bindings WHERE subject_id=? AND scope_type=? AND scope_id=?
// 检查 actions 数组是否包含 action
```

Phase 1 简化：
- bootstrap admin：subject_id="admin", role="admin", actions=["*"]
- team member：subject_id=<identity_id>, role="member", actions=["read","request"]
- team owner：subject_id=<identity_id>, role="owner", actions=["read","request","approve","reject"]

### D3：OIDC 验证用 zitadel/oidc（已在 go.mod）

**决策**：OIDC token 验证用 zitadel/oidc 库（D43 已锁定依赖）。

流程：
1. 从 Authorization header 提取 Bearer token
2. 验证 JWT 签名（JWKS from OIDC issuer）
3. 提取 sub/email/groups claim
4. claim_mapping → platform identity（首次登录自动创建 identity）

### D4：审批引擎 Phase 1 单级（扩展 W2-06 ApprovalService）

**决策**：approval.Engine 扩展 W2-06 的 ApprovalService：
- Start(requestID) → 创建 approval_run（Phase 1 单节点）
- Decide(approvalRunID, approverID, decision) → 写 approval_decisions + 状态转换
- Status(requestID) → 查审批状态
- 审批人解析：team.owner → identities 列表

Phase 1 不做：多级/会签/条件分支/超时升级/DSL。

### D5：事件 + 审计用进程内直接调用

**决策**：Phase 1 EventBus 用进程内直接函数调用（不做消息队列）。AuditLogger 直接写 audit_logs 表。

```go
type EventBus struct { handlers []EventHandler }
func (b *EventBus) Publish(ctx, event Event) // 直接遍历 handlers 调用
type AuditLogger struct { db *pgxpool.Pool }
func (l *AuditLogger) Log(ctx, actor, action, targetType, targetID, before, after) error
```

## Risks / Trade-offs

- **[OIDC 验证复杂度] → Phase 1 单 issuer 简化**：JWKS 缓存 + token 过期处理。Phase 2 多 issuer。
- **[RBAC 粒度] → Phase 1 三角色够用**：admin/member/owner。Phase 2 细粒度 scope × action。
- **[审批简化] → Phase 1 单级或签**：Phase 2 双门禁 + DSL + 会签。
- **[进程内事件] → Phase 1 单进程够用**：Phase 2 改 River job + outbox。

## Migration Plan

1. 实现 LifecycleService handler
2. 实现 OIDC token 验证 + Connect 拦截器
3. 实现 RBAC 评估
4. 实现 identity 管理 + bootstrap admin
5. 实现审批引擎
6. 实现事件 + 审计
7. 测试

## Open Questions

- **bootstrap admin 怎么初始化？** 平台首次启动时 seed（migration 或启动时检查）。用配置文件指定 admin identity 信息。
- **OIDC issuer 配置在哪？** config.yaml（Phase 1 单 issuer URL + client_id + client_secret）。
