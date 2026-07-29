## Context

W3 是 MVP 的门面层——API + RBAC + 身份 + 审批，把 W1+W2 后端能力暴露给用户。

**现有架构约束**（已落地）：
- LifecycleService proto（已定义，**实际 RPC**）：CreateRequest / GetRequest / ListRequests / ListRequestEvents / CancelRequest / StartPlan / GetArtifact / EvaluateGate / ListPendingApprovals / GetApprovalRun / DecideApproval / StartApply（**P0-1 修正：不是 ApproveRequest/RejectRequest/ListApprovals**）
- Connect 拦截器占位：`middleware.ConnectAuth()` + `middleware.ConnectRBAC()`（已有 stub，TODO body）
- auth 包：argon2id + JWT 签名/验证（golang-jwt/v5 已在 go.mod）+ OIDC 依赖锁定（zitadel/oidc）
- **identities / role_bindings 表不存在**（P0-2 修正：migration 009 只有 audit_logs + outbox_events，requests.requester_id 是 dangling string "identities table is B4"）→ **本 change 必须新建 migration 015**
- approval_flows/runs/node_runs/decisions 表（migration 007）：已建。CHECK 约束：approval_runs.status ∈ pending|approved|rejected|expired；approval_node_runs.mode ∈ any|all|majority|quorum；approval_node_runs.status ∈ pending|approved|rejected|skipped|timeout
- audit_logs 表（migration 009）：已建。字段：id(snowflake)/actor_id/actor_team_id/actor_type(unspecified|human|ai|system)/action/target_type/target_id/before_json/after_json/ai_metadata_json/correlation_id/occurred_at
- orchestrator Pipeline（Execute(ctx, requestID)）+ ApprovalService（Approve(ctx, requestID, approverID) / Reject(ctx, requestID, approverID, reason)）（W2-06）：已实现

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

**决策**：LifecycleService handler 是薄包装——每个 RPC 调 orchestrator 或 ApprovalService 对应方法。不在此层做业务逻辑。

RPC 映射（**P0-1 修正，对齐 proto 实际 RPC**）：
- CreateRequest → 写 requests 表 + Pipeline.Execute
- GetRequest/ListRequests/ListRequestEvents → 读 requests + request_events
- CancelRequest → StateMachine Transition(cancel)
- StartPlan → Pipeline.Execute（如果当前 plan_ready）
- StartApply → Pipeline.Execute（如果当前 applying）
- EvaluateGate → 读 gate_results（Phase 1 简化：直通）
- **ListPendingApprovals** → 查 approval_runs where status=pending
- **GetApprovalRun** → 查 approval_run 详情
- **DecideApproval** → ApprovalService.Approve/Reject（**P0-4 修正：不新建 Engine.Start/Decide，复用 W2-06 ApprovalService**）
- GetArtifact → 读 plan_artifacts

```go
type LifecycleHandler struct {
    pipeline  *orchestrator.Pipeline
    approval  *orchestrator.ApprovalService  // P0-4: 复用 W2-06，不新建 Engine
    repo      *repo.RequestRepo
}
```

**理由**：业务逻辑在 orchestrator（W2-06），handler 只做 proto↔Go 转换 + 鉴权。

### D2：identities/role_bindings 表新建（P0-2 修正）+ RBAC 评估

**决策**：**新建 migration 015**（identities + role_bindings 表），因为 migration 009 只建了 audit_logs。RBAC 评估从 role_bindings 表读。

```go
func EvaluateRBAC(ctx, subjectID string, action string, scopeType string, scopeID string) bool
// SELECT FROM role_bindings WHERE subject_id=? AND scope_type=? AND scope_id=?
// 检查 actions 数组是否包含 action
```

Phase 1 简化：
- bootstrap admin：subject_id="admin", role="admin", actions=["*"]
- team member：subject_id=<identity_id>, role="member", actions=["read","request"]
- team owner：subject_id=<identity_id>, role="owner", actions=["read","request","approve","reject"]

### D3：OIDC token 验证用 golang-jwt/v5 + keyfunc（P2-10 修正）

**决策**：Phase 1 token 验证用 **golang-jwt/v5**（已在 go.mod）+ **keyfunc**（JWKS fetch），不用 zitadel/oidc（那是完整 OIDC provider/client 库，Phase 1 只需 verify-only，~40 行代码）。

流程：
1. 从 Authorization header 提取 Bearer token
2. keyfunc 从 OIDC issuer JWKS URL 获取签名密钥
3. golang-jwt/v5 验证签名 + exp + iss + aud
4. 提取 sub/email/groups claim
5. claim_mapping → platform identity（首次登录自动创建 identity）

zitadel/oidc 保留在 go.mod（Phase 2 完整 authorization-code flow 用）。

### D4：审批复用 W2-06 ApprovalService（P0-4 修正）

**决策**：**不新建 approval.Engine**。直接复用 W2-06 的 ApprovalService：
- DecideApproval(approved) → ApprovalService.Approve(ctx, requestID, approverID)
- DecideApproval(rejected) → ApprovalService.Reject(ctx, requestID, approverID, reason)
- ListPendingApprovals → 查 approval_runs where status=pending
- GetApprovalRun → 查 approval_run 详情

**P0-3 修正**：审批状态用 DB CHECK 约束值（approval_runs.status ∈ pending|approved|rejected|expired，approval_node_runs.mode = any（或签），approval_node_runs.status ∈ pending|approved|rejected）。不用 open/closed/single_approval。

审批人解析：team owner → role_bindings where scope_type=team, scope_id=teamID, role=owner → subject_id → identities。

**决策**：审批复用 W2-06 ApprovalService（已在 D4 说明，不重复）。

Phase 1 不做：多级/会签/条件分支/超时升级/DSL。

### D5：事件 + 审计用进程内直接调用（P1-7 修正：audit_logs 字段对齐）

**决策**：Phase 1 EventBus 用进程内直接函数调用（不做消息队列）。AuditLogger 写 audit_logs 表。

**P1-7 修正**：audit_logs 字段对齐 migration 009：
```go
type AuditEntry struct {
    ID            int64     // snowflake（app 生成）
    ActorID       string    // identity external_id
    ActorTeamID   *int64    // team_id（可空）
    ActorType     string    // "human"|"ai"|"system"
    Action        string    // "create_request"|"approve"|"apply"...
    TargetType    string    // "request"|"stack"|"module"...
    TargetID      string    // 目标 ID
    BeforeJSON    []byte    // jsonb
    AfterJSON     []byte    // jsonb
    CorrelationID string    // P1-6 修正：从 ctx 读取
    OccurredAt    time.Time
}
```

**P1-6 修正**：correlation_id 从 Connect 请求 header `X-Correlation-Id` 读取（客户端不传则服务端生成 UUID），注入 context，AuditLogger 从 ctx 读。

**P1-5 修正**：CreateRequest 支持 idempotency——计算 `sha256(actor + catalog_item_id + form_hash + 24h_window)` 作为 idempotency_key，重复请求返回已有 request_id。

```go
type EventBus struct { handlers []EventHandler }
func (b *EventBus) Publish(ctx, event Event) // 直接遍历 handlers 调用（error-tolerant）
type AuditLogger struct { db *pgxpool.Pool }
func (l *AuditLogger) Log(ctx, entry AuditEntry) error // 从 ctx 读 correlation_id
```

**P1-8 修正**：middleware.ConnectAudit(logger) = 结构化日志行（保持不变，不改）。core/audit/AuditLogger = DB 行写入（EventBus handler）。两者不冲突（不同 sink）。

## Migration Plan

1. **新建 migration 015**（identities + role_bindings 表）+ seed bootstrap admin
2. 实现 OIDC token 验证（golang-jwt/v5 + keyfunc）
3. 填充 ConnectAuth + ConnectRBAC 拦截器
4. 实现 IdentityService（CRUD + bootstrap admin 启动检查）
5. 实现 LifecycleService handler（11 个 RPC，薄包装 orchestrator）
6. 实现 EventBus + AuditLogger
7. 测试

## Risks / Trade-offs

- **[OIDC 验证复杂度] → Phase 1 单 issuer 简化**：JWKS 缓存 + token 过期处理。Phase 2 多 issuer。
- **[RBAC 粒度] → Phase 1 三角色够用**：admin/member/owner。Phase 2 细粒度 scope × action。
- **[审批简化] → Phase 1 单级或签**：Phase 2 双门禁 + DSL + 会签。
- **[进程内事件] → Phase 1 单进程够用**：Phase 2 改 River job + outbox。

## Open Questions

- **bootstrap admin 初始化**（P3-14 关闭）：**启动时检查**（IdentityService.BootstrapAdmin(ctx, cfg)），idempotent。admin identity 信息从 config 读（external_id + display_name + email）。首次启动 seed identity + role_binding(admin, actions=[*])。
- **OIDC issuer 配置**（P3-15 关闭）：config.yaml 的 `auth.oidc` 段（issuer_url + audience + jwks_url）。Phase 1 单 issuer。
- **wire 依赖边界**（P3-16 关闭）：auth(internal) → identity(core) 允许（internal → core）。identity(core) 不 import auth(internal)。approval(core) 调 identity(core)（ResolveApprovers）。无循环。
7. 测试

## Open Questions

- **bootstrap admin 怎么初始化？** 平台首次启动时 seed（migration 或启动时检查）。用配置文件指定 admin identity 信息。
- **OIDC issuer 配置在哪？** config.yaml（Phase 1 单 issuer URL + client_id + client_secret）。
