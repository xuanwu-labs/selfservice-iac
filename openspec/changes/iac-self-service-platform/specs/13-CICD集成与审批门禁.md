# 15-CICD 集成与审批门禁（cicd-integration）

能力 ID：`cicd-integration`
覆盖：CI 完成后通过声明式 yaml 向平台提交资源申请/变更 → CD 阻塞 → 平台审批 → 释放或终止 CD。平台作为审批 gate 嵌入 CICD 流水线。

## ADDED Requirements

### Requirement: 声明式 yaml 申请单
平台 SHALL 接受声明式 yaml 申请单（与 Web 表单同构），POST 到 `/api/v1/requests`（支持 `application/yaml`），平台 MUST 解析为工单并复用同一套审批引擎（`approval-engine`）；yaml 与表单两种入口产生等价工单。

#### Scenario: CI 提交 yaml 申请
- **WHEN** CI 流水线在某步骤 `POST /api/v1/requests`，body 为 yaml：`catalog_item: dba/rds; bundle: team-a/orders; form: {instance_type: ..., storage: ...}; trigger.cicd: {pipeline: ci#123, commit: abc123}`
- **THEN** 平台解析为工单 `REQ-xxx`，状态 `generating`，进入与表单相同的流水线与审批流

### Requirement: 申请单幂等与上下文绑定
平台 SHALL 按 `trigger.cicd.pipeline + commit + catalog_item + form_hash` 做幂等去重；`form_hash` MUST 基于 canonical JSON 的 `spec.form` 计算。申请单 MUST 携带 CI 上下文（pipeline、commit、制品、触发人、form_hash）并随工单持久化，便于审计与回溯。

#### Scenario: CI 重试不重复建单
- **WHEN** 同一 pipeline+commit+catalog_item+form_hash 重复提交相同申请
- **THEN** 平台返回既有工单 ID 而不新建，避免重复审批

#### Scenario: 同 commit 参数变化生成新工单
- **WHEN** 同一 pipeline+commit+catalog_item 下 `spec.form.storage` 从 100 改为 200
- **THEN** `form_hash` 改变，平台创建新工单，不复用旧 plan/旧审批

### Requirement: 审批 gate 状态查询与订阅
平台 SHALL 提供 gate 状态 API `GET /api/v1/requests/{id}/gate`（返回 `pending/admission_approved/plan_ready/approval_granted/applying/apply_succeeded/rejected/timeout/failed`）供 CD 轮询。Webhook 订阅 SHALL 作为可选增强模式，审批或 apply 终态时回调；MUST NOT 作为 Phase 0 golden path 的阻塞条件。

`gate_status` SHALL 是面向 CI/CD 的投影状态，不是 `requests.status`。平台 MUST 在契约中维护 gate status 到 RequestLifecycle 的映射，且 gate status 不得反向扩展 request 主状态枚举。

#### Scenario: CD 轮询 gate
- **WHEN** CD 步骤循环 `GET .../gate` 直到非 `pending`
- **THEN** 默认等待 `apply_succeeded` 才继续；驳回返回 `rejected`，CD 终止

#### Scenario: webhook 回调释放 CD
- **WHEN** 审批完成，平台回调 CD 注册的 webhook
- **THEN** CD 收到结果（approved/rejected）继续或终止，无需轮询

### Requirement: CD 阻塞与释放语义
平台作为审批 gate MUST 支持 CD 阻塞语义：工单未达终态前 CD 不得继续。默认语义为审批通过后由平台执行 apply，`apply_succeeded` 才释放 CD；`approval_granted` 只表示审批已通过，不等于资源已落地。驳回或 gate 超时 → CD MUST 终止并标记失败原因。

#### Scenario: 审批通过释放 CD
- **WHEN** 工单审批通过并 apply 成功
- **THEN** gate 返回 `apply_succeeded`，CD 继续 deploy 阶段

#### Scenario: 驳回终止 CD
- **WHEN** 审批被驳回
- **THEN** gate 返回 `rejected`，CD 立即终止，记录驳回原因

### Requirement: CICD 适配器
平台 SHALL 提供 CICD 适配器与示例：Jenkins（`input`/等待步骤 + 平台 API）、GitLab CI（`bridge`/manual approval + webhook）、Argo CD（Sync hook + notification）、Flux（webhook 接收）、GitHub Actions（environment protection + API）；并 SHALL 提供通用 CLI `tm gate` 适配任意 CICD。历史 `tm-gate` MAY 作为兼容 shim，但 MUST NOT 成为独立 CLI 产品。

#### Scenario: 通用 CLI 阻塞
- **WHEN** 任意 CICD 步骤执行 `tm gate request --yaml req.yaml --wait --timeout 48h`
- **THEN** CLI 提交申请并阻塞至审批完成，退出码反映 approved(0)/rejected(1)/timeout(2)，CICD 据此继续或终止

### Requirement: gate 超时与升级联动
gate SHALL 支持超时配置；超时 MUST 联动审批引擎（`approval-engine`）的升级策略（转上级/自动驳回），并通知 CD。

#### Scenario: gate 超时
- **WHEN** gate 配置 48h，审批未完成
- **THEN** 平台按审批 DSL 的 `on_timeout` 升级或驳回，gate 返回终态，CD 据此处理
