# Protocol Mapping — HTTP ↔ gRPC

> Phase 0 Contract Freeze (task 0.3)。
> 本文档冻结 HTTP API 与 gRPC proto 的语义映射，确保同一业务语义不因协议不同产生状态、错误码或幂等差异。

## 1. API 映射（HTTP path ↔ gRPC method）

| HTTP | gRPC Service.Method | 说明 |
|------|---------------------|------|
| `POST /api/v1/requests` | `RequestService.CreateRequest` | 创建申请 |
| `GET /api/v1/requests/{requestId}` | `RequestService.GetRequest` | 查询工单 |
| `GET /api/v1/requests/{requestId}/events` | `RequestService.ListRequestEvents` | 查询事件 |
| `GET /api/v1/requests/{requestId}/gate` | *(未来 GateService.EvaluateGate)* | 准入门禁（Phase 0 预留 HTTP，gRPC 待定） |
| `POST /api/v1/requests/{requestId}/plan` | `PlanningService.StartPlan` | 触发 plan |
| `GET /api/v1/artifacts/{artifactId}` | `ArtifactService.GetArtifact` | 查询 artifact |
| `POST /api/v1/approvals/{runId}/decide` | `ApprovalService.DecideApproval` | 审批决策 |
| `POST /api/v1/requests/{requestId}/apply` | `ApplyService.StartApply` | 触发 apply |
| `GET /api/v1/catalog` | `CatalogService.ListCatalogItems` | 查询服务目录 |
| `GET /api/v1/catalog/{catalogItemId}` | `CatalogService.GetCatalogItem` | 查询单个目录项 |
| `GET /api/v1/requestable-cloud-accounts` | `EntitlementService.ListRequestableCloudAccounts` | 查询可申请云账号 |
| `POST /api/v1/webhooks/subscriptions` | *(未来 WebhookService)* | P1 预留 |

## 2. 状态码映射（HTTP status ↔ gRPC code）

| 平台错误类型 | HTTP Status | gRPC Code | retryable | manual_required |
|-------------|-------------|-----------|-----------|-----------------|
| Schema 校验失败 | 400 | `INVALID_ARGUMENT` | false | false |
| 未认证 | 401 | `UNAUTHENTICATED` | false | false |
| 权限不足 | 403 | `PERMISSION_DENIED` | false | false |
| 资源不存在 | 404 | `NOT_FOUND` | false | false |
| 状态冲突（乐观锁/状态机非法迁移） | 409 | `ABORTED` | true | false |
| 幂等重放（返回已有资源） | 200/201 | `OK` | false | false |
| 限流 | 429 | `RESOURCE_EXHAUSTED` | true (after backoff) | false |
| 业务校验失败（预算超限、策略不满足） | 422 | `FAILED_PRECONDITION` | false | depends |
| 需要人工介入 | 409 | `FAILED_PRECONDITION` | false | **true** |
| 平台内部错误（可重试） | 503 | `UNAVAILABLE` | true | false |
| 平台内部错误（不可重试） | 500 | `INTERNAL` | false | false |

## 3. Metadata / Header 映射

| HTTP Header | gRPC Metadata Key | 说明 |
|-------------|-------------------|------|
| `Authorization: Bearer <jwt>` | `authorization` | OIDC JWT token（D10/D43） |
| `X-Aether-Signature: <hmac>` | `x-aether-signature` | AK/SK HMAC（D17 机器身份） |
| `Idempotency-Key: <uuid>` | `idempotency-key` | 创建/决策幂等键 |
| `X-Correlation-Id: <uuid>` | `x-correlation-id` | 全链路追踪（W3C Trace Context） |
| `X-External-Actor-Claim: <json>` | `x-external-actor-claim` | Gateway 透传的外部身份声明（D10.1） |
| `source` (query/body field) | `x-source` | 请求来源（web/cli/cicd/ai/gateway） |

### 路径参数映射

HTTP path parameter 在 gRPC 中变为 request message field：

| HTTP Path Param | gRPC Request Field | 示例 |
|-----------------|--------------------|------|
| `{requestId}` | `request_id` | `req_abc123` |
| `{runId}` | `run_id` | `run_xyz789` |
| `{artifactId}` | `artifact_id` | `art_def456` |
| `{catalogItemId}` | `catalog_item_id` | `cat_ghi012` |

## 4. 分页映射

| HTTP | gRPC | 说明 |
|------|------|------|
| `?page_size=20` | `page_size: int32` | 每页大小（1-100，默认 20） |
| `?page_token=abc` | `page_token: string` | 不透明游标 |
| `next_page_token` in response body | `next_page_token: string` in response | 下一页游标（空=无更多） |

## 5. 乐观锁映射

| HTTP | gRPC | 说明 |
|------|------|------|
| `version: 3` in request body | `expected_run_version: int32` (approval) | 审批乐观锁 |
| `version: 5` in Request body | *(由 D21 状态机内部管理)* | 工单状态迁移乐观锁 |

乐观锁失败 → HTTP 409 `ABORTED` / gRPC `ABORTED`，retryable=true。

## 6. Actor / 身份模型映射

平台统一 actor model（D10/D10.1），不论请求来自 HTTP 还是 gRPC：

```
HTTP Authorization: Bearer <jwt>
  → OIDC Verifier 解析 → claims {sub, team_id, roles}
  → Actor{type: HUMAN, user_id: sub, team_id}

HTTP X-Aether-Signature: <hmac>
  → AK/SK Verifier 解析 → service account {sa_id, team_id}
  → Actor{type: SYSTEM, user_id: sa_id, team_id}

HTTP X-External-Actor-Claim: {"external_id":"...", "source":"oa"}
  → Gateway 仅透传，平台解析为 actor（不信任外部 claim，自己做最终判定）
  → Actor{type: HUMAN, user_id: resolved_internal_id}
```

gRPC 走同一逻辑——metadata 里的 `authorization` / `x-aether-signature` / `x-external-actor-claim` 经过同一拦截器链解析为 Actor。

## 7. 错误码→gRPC ErrorInfo 映射

gRPC 错误用 `connect.NewError(code, err)` + `ErrorInfo` detail：

| 平台 error.code（error-codes.yaml） | gRPC Code | ErrorInfo.reason |
|--------------------------------------|-----------|-------------------|
| `SCHEMA_INVALID` | `INVALID_ARGUMENT` | `schema_invalid` |
| `UNAUTHENTICATED` | `UNAUTHENTICATED` | `unauthenticated` |
| `PERMISSION_DENIED` | `PERMISSION_DENIED` | `permission_denied` |
| `REQUEST_NOT_FOUND` | `NOT_FOUND` | `request_not_found` |
| `STATE_CONFLICT` | `ABORTED` | `state_conflict` |
| `IDEMPOTENCY_REPLAY` | `OK` | *(正常返回已有资源)* |
| `RATE_LIMITED` | `RESOURCE_EXHAUSTED` | `rate_limited` |
| `BUDGET_EXCEEDED` | `FAILED_PRECONDITION` | `budget_exceeded` |
| `MANUAL_INTERVENTION_REQUIRED` | `FAILED_PRECONDITION` | `manual_required` |
| `PLATFORM_UNAVAILABLE` | `UNAVAILABLE` | `platform_unavailable` |
| `INTERNAL_ERROR` | `INTERNAL` | `internal_error` |

## 8. Dubbo / OA / ITSM / Legacy 边界

这些协议**不进入核心平台**（00a §2.1）：

| 协议 | 接入方式 | 谁负责 |
|------|----------|--------|
| Dubbo | 未来 Go Gateway 做 Dubbo→gRPC 转换 | Gateway |
| OA / ITSM | Gateway 做字段适配 + 回调重试 | Gateway |
| Legacy HTTP | Gateway 做协议规范化 | Gateway |

Gateway 禁止：持有独立权限模型、绕过审批/审计、直接调用 Terraform/Terramate。
