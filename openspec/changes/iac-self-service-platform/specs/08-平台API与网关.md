# 10-平台 API 与网关（platform-api）

能力 ID：`platform-api`
覆盖：面向前端与集成的 HTTP API、面向内部服务和未来网关的 gRPC API、表单提交触发编排、鉴权与 RBAC、审批流、Webhook / 通知、审计。

## ADDED Requirements

### Requirement: HTTP 与 gRPC API
平台 SHALL 同时提供一致业务语义的 HTTP REST API 与 gRPC API，覆盖：目录浏览、模块管理、服务申请、工单 / 状态查询、漂移查看、审批操作；HTTP API MUST 有版本前缀（如 `/api/v1`）与 OpenAPI 描述，gRPC API MUST 有 Proto 契约。HTTP 与 gRPC MUST 共享同一 service 层语义、错误码、幂等、correlation id 和状态机。

#### Scenario: 列出可见目录项
- **WHEN** 已认证用户 `GET /api/v1/catalog?team=team-a`
- **THEN** 返回该用户可见的目录项列表，含归属团队、分层、表单 schema 引用

#### Scenario: 未来网关通过 gRPC 接入
- **WHEN** 未来 Go Gateway 需要把 Dubbo/OA/ITSM 调用转换为平台调用
- **THEN** Gateway SHALL 调用平台 gRPC 或 HTTP API，核心平台不得直接暴露 Dubbo service，也不得为外部协议维护独立状态机

### Requirement: 表单提交触发编排
平台 SHALL 把"服务申请表单提交"作为编排引擎的入口：`POST /api/v1/requests` 校验表单、创建工单、触发流水线，同步返回工单 ID。

#### Scenario: 提交 RDS 申请
- **WHEN** 前端 `POST /api/v1/requests` body 携带 `catalog_item=rds`、`bundle=team-a/orders`、`form={instance_type:..., storage_size:...}`
- **THEN** 平台校验权限与表单，创建工单并立即触发 `generating` 阶段，返回 `request_id`，前端可轮询或订阅状态

### Requirement: 鉴权与 RBAC
平台 SHALL 对所有 API 强制鉴权（平台本地身份、OIDC / SSO / token），并按"团队 / 项目组 / bundle / 资源类型 × 动作"做内建 RBAC 鉴权，拒绝越权访问并记录。外部用户中心、OA、ITSM 只作为可选身份源、组织源或流程联动源；平台 RBAC 始终是权限事实源。

#### Scenario: 越权访问被拒
- **WHEN** 业务用户带合法 token 调用 `POST /api/v1/requests` 申请 Global 层 VPC 变更
- **THEN** 平台返回 403，错误指向"归属平台运维团队，无权操作"，并写审计日志

#### Scenario: 未对接外部用户中心
- **WHEN** 企业尚未接入 OIDC/SSO、OA 或 ITSM
- **THEN** 平台 SHALL 允许管理员通过本地 identity、team、role binding 管理用户与权限，并完整使用申请、审批、执行、CMDB 和 FinOps 能力

### Requirement: 审批流
平台 SHALL 内置多级审批流：按目录项 / 团队 / 影响面路由到对应审批人，支持串行 / 并行 / 会签；plan 摘要与 OPA 结果 MUST 一并呈给审批人。

#### Scenario: RDS 账号申请需 DBA 审批
- **WHEN** 业务团队 A 申请 `rds-orders-prod` 的只读账号
- **THEN** 工单路由到 DBA 团队审批人，审批人可看到 plan 摘要后批准 / 驳回

### Requirement: Webhook 与通知
平台 SHALL 支持配置 Webhook 与内置通知通道（站内信 / 邮件 / IM 机器人），在工单状态变更、审批、漂移、策略阻断时推送事件，事件 payload MUST 包含足够信息供外部系统处理。

#### Scenario: 推送工单完成到 IM
- **WHEN** 工单进入 `succeeded`
- **THEN** 平台向已配置的 IM Webhook 推送消息，含资源名、归属团队、执行人、关键输出（如 RDS 连接地址，敏感信息脱敏）

### Requirement: 审计日志
平台 SHALL 记录所有敏感操作（申请、审批、执行、漂移处置、策略变更、权限变更）的审计日志，含操作人、时间、目标、前后值摘要，日志 MUST 不可篡改且可导出。

#### Scenario: 审计 apply 操作
- **WHEN** 审批人批准并触发 `apply`
- **THEN** 审计日志记录"审批人 X 于 T 批准工单 Y，apply 由执行器执行，影响资源 Z"
