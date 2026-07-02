# 00a-Phase 0 契约产物规划

> 本文承接 `docs/00-工程契约.md` 与 `specs/19-API与Schema契约.md`。目标不是继续扩展业务功能，而是把方案中的核心契约标准化为可评审、可生成、可测试、可回归的工程产物。

## 1. 定位

Phase 0 的交付物必须回答五个问题：

| 问题 | 交付物 |
|------|--------|
| 前端、CLI、CI/CD、AI agent 调什么接口 | HTTP OpenAPI |
| 内部服务、未来网关、自动化系统调用什么接口 | gRPC Proto |
| HTTP 与 gRPC 的字段、错误、状态如何对齐 | Protocol Mapping |
| 核心对象字段、类型、枚举、必填项是什么 | JSON Schema |
| RequestLifecycle 是否可恢复、可重试、可人工介入 | State machine fixtures |
| 平台是否始终通过 TerramateAdapter 调用 Terramate/Terraform | Adapter golden tests |
| 第一条真实链路能否从申请跑到 reconcile | Walking skeleton |

Phase 0 不以“功能多”为目标，只以“契约不可误解”为目标。

## 2. 标准选型

| 领域 | 标准 | 采用原因 |
|------|------|----------|
| API 描述 | OpenAPI 3.1 | 与 JSON Schema 2020-12 对齐，可驱动前端/CLI SDK 和 contract tests |
| gRPC 描述 | Protocol Buffers v3 | 面向内部服务、未来 Go Gateway 和高可靠自动化调用 |
| 协议映射 | HTTP/gRPC mapping 文档 | 确保同一业务语义不会因协议不同产生状态、错误码或幂等差异 |
| Schema | JSON Schema 2020-12 | 支持条件约束、枚举、格式校验、schema 复用 |
| 错误模型 | RFC 7807 思路 + 平台 error registry | 机器可读 code + 人可读 remediation |
| 事件 envelope | CloudEvents 语义子集 | 统一 `event_id`、`type`、`source`、`time`、`correlation_id` |
| 幂等 | `Idempotency-Key` + 业务 canonical hash | 支持 Web/CLI/CI/CD 重试与重复提交 |
| Trace | W3C Trace Context 兼容字段 | 兼容后续可观测系统 |
| Fixture | Golden file + table-driven case | 便于单测、回归和跨语言消费 |

## 2.1 协议边界

核心平台只承担 HTTP 与 gRPC 两类一等接口：

| 协议 | 面向对象 | 边界 |
|------|----------|------|
| HTTP OpenAPI | Web、CLI、CI/CD、AI agent、外部自动化 | 对外主入口，承载表单申请、查询、审批、gate、artifact、catalog |
| gRPC Proto | 平台内部服务、未来 Go Gateway、高可靠自动化 | 内部主入口，承载同一业务语义，不另造状态机和错误码 |
| Dubbo / OA / ITSM / Legacy protocol | 不进入核心平台 | 未来由独立 Go Gateway 做协议转换 |

不变量：

- HTTP 与 gRPC 共享同一领域模型、错误码、状态机、幂等语义和 correlation id。
- gRPC 不得绕过 service 层直接操作 DB、Executor 或 TerramateAdapter。
- 平台内建认证、RBAC、审批、CMDB、FinOps；外部用户中心、OA、ITSM 只是可选对接源，不是平台运行前提。
- Dubbo 兼容不进入本项目核心；只在未来 Gateway 中作为 adapter。
- Gateway 只能做协议转换、外部身份声明转换、字段映射和限流熔断，不能拥有独立业务状态机、权限模型、CMDB 或 FinOps。

推荐边界：

| 组件 | 职责 |
|------|------|
| Platform API Service | 暴露 HTTP/gRPC，承载核心业务语义 |
| Platform Service Layer | RequestLifecycle、auth/RBAC、approval、codegen、workspace、executor、CMDB、FinOps |
| Future Go Gateway | Dubbo/OA/ITSM/legacy protocol 到 HTTP/gRPC 的转换 |
| External Systems | 用户中心、OA、ITSM、企业网关、老系统；均为可选对接 |

## 3. 产物目录规划

| 产物 | 建议路径 | Phase 0 验收 |
|------|----------|--------------|
| OpenAPI | `openspec/changes/iac-self-service-platform/contracts/openapi.yaml` | 覆盖 request、approval、catalog、artifact、events、webhooks 最小 API |
| Proto | `openspec/changes/iac-self-service-platform/contracts/proto/platform/v1/*.proto` | 覆盖与 OpenAPI 等价的核心 service、message、enum、error metadata |
| Protocol Mapping | `openspec/changes/iac-self-service-platform/contracts/protocol-mapping.md` | 说明 HTTP path/status/header 与 gRPC method/code/metadata 的映射 |
| JSON Schema | `openspec/changes/iac-self-service-platform/contracts/schemas/*.schema.json` | 覆盖 P0 核心对象，能被 schema validator 校验 |
| Error Registry | `openspec/changes/iac-self-service-platform/contracts/error-codes.yaml` | 每个错误有 code、HTTP status、retryable、manual_required、remediation |
| State Fixtures | `openspec/changes/iac-self-service-platform/contracts/fixtures/state-machine/*.json` | 覆盖 RLC、IDEMP、CONC 主用例 |
| Adapter Golden Cases | `openspec/changes/iac-self-service-platform/contracts/fixtures/adapter/*.json` | 覆盖 plan、saved-plan apply、drift-plan、import-verify |
| Walking Skeleton Seed | `openspec/changes/iac-self-service-platform/contracts/fixtures/walking-skeleton/*.json` | 一个 golden catalog item 能串起完整链路 |

## 4. P0 数据结构清单

| 数据结构 | 归属产物 | 用途 | Phase 0 必须冻结 |
|----------|----------|------|------------------|
| `ApiEnvelope` | OpenAPI / Schema | 统一成功响应 | 是 |
| `ApiError` | OpenAPI / Error Registry | 统一失败响应 | 是 |
| `GrpcServiceContract` | Proto / Protocol Mapping | gRPC service、method、metadata、deadline 约束 | 是 |
| `ProtocolMapping` | Protocol Mapping | HTTP/gRPC 字段、状态、错误码映射 | 是 |
| `RequestCreate` | OpenAPI / Schema | 用户创建申请 | 是 |
| `Request` | OpenAPI / Schema | 工单查询与状态机主实体 | 是 |
| `RequestEvent` | Schema / State Fixture | 状态轨迹与审计串联 | 是 |
| `CatalogItem` | OpenAPI / Schema | 服务目录展示与申请入口 | 是 |
| `ModuleVersion` | Schema | 模块版本、变量契约、provider lock | 是 |
| `ResolvedParams` | Schema | 参数解析结果和 provenance | 是 |
| `PathGeneratorOutput` | Schema | repo path、state key、Terramate tags 单一来源 | 是 |
| `PlanArtifact` | OpenAPI / Schema | plan/apply 解耦与 apply 前校验 | 是 |
| `ApprovalDecision` | OpenAPI / Schema | 审批动作与幂等 | 是 |
| `WebhookEvent` | OpenAPI / Schema | 对外事件通知 | P1，可在 Phase 0 预留 |
| `StateTransitionCase` | State Fixture | 状态迁移可测试 | 是 |
| `TerramateAdapterCase` | Adapter Fixture | 命令契约可回归 | 是 |
| `WalkingSkeletonSeed` | Walking Fixture | 第一条 golden path 数据 | 是 |

## 5. 字段标准

所有 P0 schema 遵守以下字段规范：

| 规则 | 标准 |
|------|------|
| ID | 使用带前缀的字符串，如 `req_`、`cat_`、`art_`、`stk_` |
| 时间 | RFC 3339 UTC 字符串 |
| 枚举 | 必须集中定义，禁止自由字符串散落 |
| 金额 | 使用整数最小单位或 decimal string，禁止 float |
| Hash | 使用 `sha256:<hex>` 格式 |
| URI | artifact 使用对象存储 URI，不暴露临时下载 URL |
| Secret | 只保存 `secret_ref`，禁止 schema 中出现 secret value |
| Version | 可变对象必须有 `version` 或 `schema_version` |
| Correlation | 外部入口、状态事件、审计、outbox 均携带 `correlation_id` |
| Idempotency | 创建类和决策类 API 必须声明幂等键来源 |

## 6. OpenAPI 最小边界

| API 分组 | 最小接口 | Phase 0 目的 |
|----------|----------|--------------|
| Requests | `POST /api/v1/requests`、`GET /api/v1/requests/{id}`、`GET /api/v1/requests/{id}/events` | Web/CLI/CI/CD 统一创建与查询 |
| Planning | `POST /api/v1/requests/{id}/plan`、`GET /api/v1/artifacts/{artifact_id}` | 固化 plan artifact 产物引用 |
| Approval | `POST /api/v1/approvals/{run_id}/decide` | 固化审批决策模型 |
| Apply | `POST /api/v1/requests/{id}/apply` | 固化 saved-plan apply 触发语义 |
| Catalog | `GET /api/v1/catalog`、`GET /api/v1/catalog/{id}` | 固化服务目录最小读取模型 |
| Entitlement | `GET /api/v1/requestable-cloud-accounts` | 固化团队可申请范围 |
| Webhook | `POST /api/v1/webhooks/subscriptions` | P1 预留，不阻塞 Phase 1 主链路 |

说明：Phase 0 可以先定义接口契约和 mock，不要求所有接口具备完整业务实现。

## 6.1 gRPC Proto 最小边界

gRPC service 与 HTTP API 应保持语义等价，但不要求字段命名完全相同；差异必须写入 `protocol-mapping.md`。

| Service | Method | 对应 HTTP | Phase 0 目的 |
|---------|--------|-----------|--------------|
| `RequestService` | `CreateRequest` | `POST /api/v1/requests` | 创建申请 |
| `RequestService` | `GetRequest` | `GET /api/v1/requests/{id}` | 查询工单 |
| `RequestService` | `ListRequestEvents` | `GET /api/v1/requests/{id}/events` | 查询事件 |
| `PlanningService` | `StartPlan` | `POST /api/v1/requests/{id}/plan` | 触发 plan |
| `ArtifactService` | `GetArtifact` | `GET /api/v1/artifacts/{artifact_id}` | 查询 artifact metadata |
| `ApprovalService` | `DecideApproval` | `POST /api/v1/approvals/{run_id}/decide` | 审批决策 |
| `ApplyService` | `StartApply` | `POST /api/v1/requests/{id}/apply` | 触发 saved-plan apply |
| `CatalogService` | `ListCatalogItems` / `GetCatalogItem` | `GET /api/v1/catalog` / `GET /api/v1/catalog/{id}` | 查询服务目录 |
| `EntitlementService` | `ListRequestableCloudAccounts` | `GET /api/v1/requestable-cloud-accounts` | 查询可申请云账号 |

gRPC metadata 约束：

| Metadata | HTTP 对应 | 说明 |
|----------|-----------|------|
| `x-correlation-id` | `X-Correlation-Id` | 全链路追踪 |
| `idempotency-key` | `Idempotency-Key` | 创建/决策幂等 |
| `authorization` | `Authorization` | OIDC session、AK/SK 或 Gateway 转发的外部身份凭证 |
| `x-external-actor-claim` | `X-External-Actor-Claim` | Gateway 仅透传/规范化外部身份声明，平台负责解析为 actor |
| `x-source` | `source` 字段或 header | `web` / `cli` / `cicd` / `ai` / `gateway` |

## 6.2 Protocol Mapping 标准

`protocol-mapping.md` 必须至少覆盖：

| 映射 | 标准 |
|------|------|
| API 映射 | HTTP method/path 到 gRPC service/method |
| 状态码映射 | HTTP status 到 gRPC code |
| 错误码映射 | `error.code` 到 gRPC `ErrorInfo.reason` 或等价 metadata |
| 幂等映射 | `Idempotency-Key` 到 gRPC metadata |
| 追踪映射 | `X-Correlation-Id` 到 gRPC metadata |
| 分页映射 | `page_size/page_token` 到 proto request field |
| 乐观锁映射 | `version/expected_status` 到 proto request field |
| Actor 映射 | OIDC/AKSK/Gateway external claim 到平台统一 actor model；最终 team/role/RBAC 由平台判定 |

推荐错误映射：

| 平台错误类型 | HTTP | gRPC |
|--------------|------|------|
| schema invalid | 400 | `INVALID_ARGUMENT` |
| unauthenticated | 401 | `UNAUTHENTICATED` |
| permission denied | 403 | `PERMISSION_DENIED` |
| not found | 404 | `NOT_FOUND` |
| conflict / stale version | 409 | `ABORTED` |
| idempotency replay | 200/201 with existing resource | `OK` with existing resource |
| rate limited | 429 | `RESOURCE_EXHAUSTED` |
| retryable platform error | 503 | `UNAVAILABLE` |
| manual required | 409/422 | `FAILED_PRECONDITION` |

## 6.3 Future Go Gateway 边界

未来 Go Gateway 是独立组件，不属于 Phase 1 主链路。

| 能力 | Gateway 负责 | 核心平台负责 |
|------|--------------|--------------|
| Dubbo 兼容 | Dubbo method 到 gRPC/HTTP 映射 | 不直接暴露 Dubbo |
| OA/ITSM 接入 | 外部字段适配、签名、回调重试、外部单号映射 | 内建审批、工单状态、审计和 HTTP/gRPC |
| 用户中心接入 | 外部 identity claim / org payload 规范化与转发 | 内建 identity、team、role_binding、RBAC，可独立管理并做最终判定 |
| 限流熔断 | per-system rate limit、circuit breaker | 核心服务保护与统一错误 |
| 身份透传 | 透传或规范化外部身份声明与 source | 平台解析 actor/team/role，并执行 RBAC 与审计 |
| CMDB / FinOps | 不负责 | 平台内建 CMDB resource index、cost records、budget、FinOps recommendations |

Gateway 禁止事项：

- 禁止持久化 RequestLifecycle 主状态。
- 禁止直接调用 Terraform、Terramate 或 state backend。
- 禁止绕过核心平台审批、审计、幂等和 evidence chain。
- 禁止持有独立权限模型；权限事实源是平台 RBAC。
- 禁止把外部组/部门直接当成平台权限；必须进入平台 identity/team/role_binding 映射后再授权。
- 禁止为 Dubbo/OA/ITSM 定义独立资源模型、CMDB 模型或 FinOps 模型。

## 7. JSON Schema 分层

| 层 | Schema | 说明 |
|----|--------|------|
| API envelope | `api-envelope.schema.json`、`api-error.schema.json` | 全 API 共享 |
| Request | `request-create.schema.json`、`request.schema.json`、`request-event.schema.json` | 主链路核心 |
| Catalog | `catalog-item.schema.json`、`module-version.schema.json` | 服务目录与模块契约 |
| Resolution | `resolved-params.schema.json`、`path-generator-output.schema.json` | codegen 输入输出 |
| Execution | `plan-artifact.schema.json`、`toolchain-profile.schema.json` | plan/apply 解耦 |
| Approval | `approval-run.schema.json`、`approval-decision.schema.json` | 审批模型 |
| Fixture | `state-transition-case.schema.json`、`terramate-adapter-case.schema.json`、`walking-skeleton-seed.schema.json` | 测试输入 |

## 8. 状态机 Fixture 标准

每个状态迁移用例必须包含：

| 字段 | 说明 |
|------|------|
| `case_id` | 对齐 `docs/12a` 的 RLC/IDEMP/CONC 编号 |
| `initial_status` | 迁移前 request status |
| `event_type` | 触发事件 |
| `guards` | 前置条件，如 artifact 存在、审批通过、hash 有效 |
| `expected_status` | 迁移后 request status |
| `expected_events` | 必须追加的 `request_events` |
| `expected_outbox` | 必须写入的 outbox 事件 |
| `expected_manual_tasks` | 需要人工介入时必须生成 |
| `idempotency` | 重放同一事件时的期望 |

Phase 0 必须覆盖：

| 分类 | 必须用例 |
|------|----------|
| 主链路 | submitted、generating、planning、plan-ready、pending-approval、applying、reconciling、succeeded |
| 异常链路 | plan failed、artifact expired、apply interrupted、state unhealthy、policy blocked |
| 人工兜底 | waiting-manual、manual task created、manual task resolved |
| 幂等 | duplicate request、duplicate approval、duplicate outbox delivery |
| 并发 | concurrent approval、concurrent apply、outbox worker claim |

## 9. TerramateAdapter Golden Test 标准

Adapter golden case 必须验证“平台没有绕过 Terramate”。

| 动作 | 必测内容 |
|------|----------|
| `RunPlan` | workdir、tags、parallel、`terraform plan -out`、stdout/stderr、exit code |
| `RunApplySavedPlan` | 新 worktree、artifact path、`terraform apply -input=false <plan>`、hash 校验前置 |
| `RunDriftPlan` | read-only credential、`-detailed-exitcode`、exit code 0/2/1 语义 |
| `RunImportVerify` | import worktree、zero-diff 判断、diff summary |

每个 golden case 至少包含：

| 字段 | 说明 |
|------|------|
| `case_id` | Adapter 用例编号 |
| `action` | Adapter 方法名 |
| `workdir` | 执行目录 |
| `scope_tags` | Terramate tags |
| `inputs` | plan path、toolchain、provider lock 等 |
| `expected_argv` | 期望命令参数数组 |
| `expected_env` | 必要环境变量，禁止 secret value |
| `expected_outputs` | exit code、artifact、log、summary |
| `forbidden` | 禁止出现的命令，如 Orchestrator 直接调用 terraform |

## 10. Walking Skeleton 标准

Walking skeleton 只验证一条 golden path，不验证完整商业平台。

| Seed 数据 | 最小要求 |
|-----------|----------|
| Tenant | 一个租户，如 `corp-a` |
| Env | 一个 dev 环境 |
| Team | 一个 owner 明确的团队 |
| Bundle | 一个业务 bundle，可为空但建议保留 |
| CatalogItem | 一个 active golden catalog item |
| ModuleVersion | 一个 validated module version |
| CloudAccount | 一个已 bootstrap 的云账号 |
| TeamCloudGrant | team 对 dev 云账号有申请权限 |
| ToolchainProfile | 固定 Terramate、Terraform/OpenTofu、provider 版本 |
| ApprovalPolicy | dev 环境最小审批策略 |
| PathGeneratorOutput | layer-first 路径、state key、tags |
| PlanArtifact | 可下载、可校验、未过期 |

验收链路：

| 步骤 | 验收 |
|------|------|
| Request | 幂等创建，返回 `request_id` |
| Codegen | 生成确定性文件树和 `pinned_commit` |
| Plan | 产出 `PlanArtifact`，hash 和摘要可查 |
| Approval | 决策写入 approval run 和 request event |
| Apply | 使用 saved plan，不重新 plan |
| Reconcile | 写 CMDB index、request event、基础运营指标 |
| Query | `GET /requests/{id}` 能返回完整状态和 artifact 引用 |

## 11. 验收门禁

Phase 0 通过必须满足：

| 门禁 | 标准 |
|------|------|
| Contract completeness | P0 schema 和最小 API 全部存在 |
| Contract validation | 示例 payload 能通过 JSON Schema 校验 |
| State validation | RLC/IDEMP/CONC fixture 能驱动表测试 |
| Adapter validation | Golden cases 能证明命令契约和 forbidden command |
| Skeleton validation | 一条 golden path 可从 request 跑到 reconcile |
| Documentation alignment | `docs/00`、`specs/19`、`docs/12a`、`docs/10`、`tasks.md` 口径一致 |

没有通过以上门禁，不进入大规模业务模块开发。
