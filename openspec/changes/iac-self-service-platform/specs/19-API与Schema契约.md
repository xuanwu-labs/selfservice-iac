# 19-API 与 Schema 契约（api-schema-contract）

能力 ID：`api-schema-contract`

覆盖：平台 HTTP API、gRPC API、错误 envelope、幂等、correlation id、分页、核心 schema、协议映射、Webhook payload。对应 `docs/00-工程契约.md` 与 `docs/00a-contract-artifacts-plan.md`。

## ADDED Requirements

### Requirement: 统一 API Envelope
平台 SHALL 对所有 `/api/v1` 响应使用统一 envelope。成功响应 MUST 包含 `data` 与 `correlation_id`；失败响应 MUST 包含 `error.code`、`error.message`、`error.details`、可选 `error.remediation` 与 `correlation_id`。

#### Scenario: 结构化错误
- **WHEN** 用户提交缺少 `catalog_item_id` 的申请
- **THEN** 平台返回 400，body 含 `error.code="schema_invalid"`、`error.details.field="catalog_item_id"`、`correlation_id`

### Requirement: 幂等与追踪
创建类 API SHALL 支持 `Idempotency-Key`；若客户端未传，平台 MUST 按业务字段计算幂等键。所有请求 MUST 使用或生成 `X-Correlation-Id`，并贯穿 `request_events`、`audit_logs`、`outbox_events`。

#### Scenario: 重复创建申请
- **WHEN** 同一 actor 在 24 小时窗口内重复提交相同 `catalog_item_id + form_hash`
- **THEN** 平台返回已有 `request_id`，不得创建新工单

### Requirement: 核心 API 列表
平台 SHALL 在 OpenAPI 描述中至少覆盖以下 API：`POST /requests`、`GET /requests/{id}`、`GET /requests/{id}/events`、`GET /requests/{id}/gate`、`POST /requests/{id}/plan`、`GET /artifacts/{artifact_id}`、`POST /approvals/{run_id}/decide`、`POST /requests/{id}/apply`、`GET /catalog`、`GET /requestable-cloud-accounts`。`POST /webhooks/subscriptions` SHALL 作为 P1 预留接口描述，MUST NOT 阻塞 Phase 0 golden path。

平台 SHALL 在 Proto 描述中至少覆盖等价 gRPC service：`RequestService.CreateRequest/GetRequest/ListRequestEvents`、`PlanningService.StartPlan`、`ArtifactService.GetArtifact`、`ApprovalService.DecideApproval`、`ApplyService.StartApply`、`CatalogService.ListCatalogItems/GetCatalogItem`、`EntitlementService.ListRequestableCloudAccounts`。

#### Scenario: API 文档可驱动前端与 CLI
- **WHEN** 前端和 `tm` CLI 需要接入平台
- **THEN** 它们 SHALL 从同一 OpenAPI 契约生成或校验客户端模型，不能各自猜字段

### Requirement: gRPC 与协议边界
平台 SHALL 在核心平台内提供与 HTTP API 语义一致的 gRPC Proto 契约。HTTP 与 gRPC MUST 共享同一领域模型、错误码、状态机、幂等语义和 correlation id。Dubbo、OA、ITSM、Legacy protocol 兼容 SHALL 由未来独立 Go Gateway 转换到 HTTP/gRPC，MUST NOT 进入核心平台服务。平台 identity、RBAC、approval、CMDB、FinOps SHALL 是内建能力；外部用户中心、OA、ITSM 只是可选对接源。

#### Scenario: Dubbo 不污染核心平台
- **WHEN** 外部系统只能通过 Dubbo 调用资源申请能力
- **THEN** 未来 Go Gateway SHALL 将 Dubbo method 转换为平台 gRPC/HTTP 调用，核心平台不得暴露 Dubbo service，也不得为 Dubbo 定义独立状态机、权限模型、CMDB 模型或 FinOps 模型

### Requirement: Phase 0 契约产物
平台 SHALL 在进入大规模业务开发前产出可机器校验的契约产物，包括 `contracts/openapi.yaml`、`contracts/proto/platform/v1/*.proto`、`contracts/protocol-mapping.md`、`contracts/schemas/*.schema.json`、`contracts/error-codes.yaml`、`contracts/fixtures/state-machine/*.json`、`contracts/fixtures/adapter/*.json`、`contracts/fixtures/walking-skeleton/*.json`。这些产物 SHALL 遵循 `docs/00a-contract-artifacts-plan.md`。

#### Scenario: 契约不是 Markdown 说明
- **WHEN** Phase 0 验收 API、schema、状态机、Adapter 和 walking skeleton
- **THEN** 验收依据 SHALL 是 `contracts/` 下的 OpenAPI、Proto、Protocol Mapping、JSON Schema 和 fixture 产物，而不是仅阅读设计文档

### Requirement: RequestCreate Schema
`POST /requests` 的请求 schema SHALL 包含 `catalog_item_id`、`env_id`、`team_id`、`form_values`、`source`，并可选包含 `tenant_id`、`bundle_id`、`source_context`。平台 SHALL 对 `form_values` 做 canonical JSON 计算 `form_hash`。

#### Scenario: CICD YAML 与 Web 表单等价
- **WHEN** Web JSON 与 CICD YAML 表示同一 `form_values`
- **THEN** canonical 后的 `form_hash` MUST 相同，生成同一幂等语义

### Requirement: ResolvedParams Schema
codegen 输出 SHALL 写入 `requests.resolved_params_json`，包含 `values`、`provenance`、`tags`、`policy_flags`、`schema_version`。每个变量 MUST 有 `source` 与 `rank`，治理覆盖用户值时 MUST 记录 `user_attempted_override`。

#### Scenario: 参数来源可审计
- **WHEN** 审批人查看 `encrypt_at_rest`
- **THEN** 平台可展示其来源为 `layer_rule`、rank 为 2、用户曾尝试传 false

### Requirement: PathGeneratorOutput Schema
PathGenerator SHALL 返回 `repo_path`、`state_key`、`stack_id`、`terramate_tags`、`after`、`watch`。codegen、workspace、CMDB、drift、CICD gate MUST 引用此输出，不得重新拼接路径。

#### Scenario: 路径单一来源
- **WHEN** 创建 Application stack
- **THEN** backend key、Terramate stack ID、CMDB stack path 和 drift checkout 路径均来自同一个 PathGeneratorOutput

### Requirement: PlanArtifact Schema
plan 产物记录 SHALL 包含 `artifact_id`、`request_id`、`pinned_commit`、`sha256`、`toolchain_profile_hash`、`provider_lock_hash`、`created_at`、`expires_at`、`status`。

#### Scenario: apply 前校验 plan
- **WHEN** apply 阶段下载 plan artifact
- **THEN** 平台 MUST 校验 `sha256`、`pinned_commit`、`toolchain_profile_hash`、`provider_lock_hash` 与当前沙箱一致

### Requirement: Webhook Payload
Webhook 事件 payload SHALL 包含 `event_id`、`event_type`、`occurred_at`、`correlation_id`、`resource_type`、`resource_id`、`data`、`signature`。Webhook MUST 支持重放保护。

#### Scenario: Gate 回调
- **WHEN** request 进入 `apply_succeeded`
- **THEN** 平台发送 `gate.apply_succeeded` webhook，payload 含 request id、gate status、plan artifact id 和 correlation id
