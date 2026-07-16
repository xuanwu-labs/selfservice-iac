# 16-CICD 集成与审批门禁

> 对应 design `D16`、spec `specs/13-CICD集成与审批门禁`。重点回应：CI 完成后资源申请/变更如何提交平台、CD 如何阻塞等待审批、审批后如何释放 CD。

## 1. 场景流程

```
[CI] 构建应用 → 产出制品/镜像
   ↓ (CI 步骤: 提交资源申请)
[平台] POST /api/v1/requests (yaml 申请单) → 解析为工单 → 进入流水线(generating→planning)
   ↓
[审批 gate] 审批流(approval-engine, D11) → 多级/会签
   ↓ (CD 流水线在 gate 步骤阻塞)
[CD] 轮询 gate API 或 等平台 webhook 回调
   ↓
[审批结果]
   ├─ 通过 → gate=approval_granted → 平台 apply → gate=apply_succeeded → CD 继续 deploy
   └─ 驳回/超时 → gate=rejected/timeout → CD 终止
```

## 2. 声明式 yaml 申请单（与表单同构）

```yaml
# req.yaml —— 与 Web 表单同一套 schema（来自 catalog_items.form_schema_json）
apiVersion: iac.platform/v1
kind: ResourceRequest
metadata:
  name: order-service-prod-scale-up
spec:
  catalogItem: dba/rds              # 引用服务目录项
  space: team-a/orders
  form:                              # 等价于表单值
    instance_type: mysql.n2.large.1c
    storage: 200
trigger:
  cicd:                              # CI 上下文（幂等键 + 审计）
    pipeline: "gitlab-ci #1234"
    commit: "abc123def"
    artifact: "registry/order-svc:v1.8.2"
    triggered_by: "git:user@corp"
approval:
  flow: rds-approval                # 引用审批 DSL（D11）
  notify: webhook                   # 回调 CD
gate:
  timeout: 48h                      # gate 超时
```

**关键**：yaml 与表单产生**等价工单**，复用同一审批引擎与流水线——人工与自动化两条入口统一。

## 3. 平台 API（gate 三件套）

| API | 作用 |
|-----|------|
| `POST /api/v1/requests`（支持 `application/json` / `application/yaml`） | 提交申请单，返回 `request_id` + `gate_token` |
| `GET /api/v1/requests/{id}/gate` | 返回 gate 状态：`pending/admission_approved/plan_ready/approval_granted/applying/apply_succeeded/rejected/timeout/failed` + 摘要 |
| `POST /api/v1/requests/{id}/subscribe` | 注册 webhook，审批终态回调 |

幂等键：`trigger.cicd.pipeline + commit + catalogItem + form_hash`，重复提交返回既有工单。同一 commit 下参数变化必须生成新工单，避免复用旧审批/旧 plan。

### 3.1 幂等键去重逻辑（精确化）

`idempotency_key = sha256(pipeline + ":" + commit + ":" + catalogItem + ":" + form_hash)`，存 `cicd_triggers.idempotency_key UNIQUE`。`form_hash = sha256(canonical_json(spec.form))`，必须对 YAML/JSON 同构输入规范化后计算。重复提交时按工单当前状态分类响应：

| 既有工单状态 | 重复提交响应 | HTTP |
|------------|------------|------|
| `generating` / `pending-admission` / `planning` / `plan-ready` / `pending-approval` / `applying` / `reconciling`（进行中）| 返回既有 `request_id` + `gate_token` + 当前状态（**不新建、不重置审批**）| 200 OK + body |
| `succeeded`（终态：已成功）| 返回既有工单 + 资源信息，**不重 apply**（资源已存在）；如需变更必须改 yaml 的 `spec.form`、catalogItem 或 commit 触发新幂等键 | 200 OK + `already_applied:true` 标识 |
| `rejected` / `timeout` / `failed`（终态：失败）| 返回既有工单 + 失败原因；**允许同 pipeline+commit 重新提交时强制要求加 `force: true`** 或改 commit（避免 CI 重试死循环失败工单）| 409 Conflict + `force_required:true` |
| 不存在 | 正常建单 | 201 Created |

**强制 force:true 的设计**：避免 CI 因 flaky 失败无限重试同一组合（pipeline+commit+catalogItem）造成噪音；运维明确"我知道之前失败了，重试"才允许。

### 3.1.1 Gate 状态语义

| gate 状态 | 含义 | CD 默认是否继续 |
|-----------|------|----------------|
| `pending` | 平台已接收，尚未进入关键门禁 | 否 |
| `admission_approved` | pre-plan 准入已通过，尚未完成 plan | 否 |
| `plan_ready` | plan 产物已生成，等待执行确认 | 否 |
| `approval_granted` | pre-apply 审批已通过，准备 apply | 否，除非 CD 明确只等待审批 |
| `applying` | 资源正在落地 | 否 |
| `apply_succeeded` | Terraform apply 成功，reconcile 通过或不影响部署 | 是 |
| `rejected` / `timeout` / `failed` | 审批或执行失败 | 否 |

`gate_status` 是面向 CI/CD 的投影状态，不是 `requests.status`。例如 `apply_succeeded` 投影自 request `succeeded`，`timeout` 投影自审批子状态或 request `expired/rejected`，`failed` 投影自 `failed-retryable` / `failed-terminal`。Gate 状态不得反向扩展 RequestLifecycle 枚举。

默认 `tm gate --wait` 等待 `apply_succeeded`。若少数流水线只需要“审批已同意但资源不由平台 apply”，必须显式传 `--until approval_granted`，并在审计中记录该语义差异。

### 3.1.2 Run Lifecycle Gate（商业级扩展）

Phase 2 起，gate 不只是“等审批”，而是 run lifecycle 的统一状态投影：

| gate 状态 | 来源 | 说明 |
|-----------|------|------|
| `policy_blocked` | OPA / Run Hooks | 策略或外部扫描阻断 |
| `needs_replan` | Drift guard / artifact TTL | plan 过期或世界已变 |
| `manual_intervention_required` | Orchestrator | apply 中断或 state 不确定 |
| `reconcile_pending` | CMDB/FinOps/通知 | apply 成功但旁路补偿未完成 |

CICD 默认只在 `apply_succeeded` 继续；如遇 `reconcile_pending`，可按环境策略决定继续 deploy 或等待补偿完成。

### 3.2 yaml schema 校验与错误处理（精确化）

提交 `POST /api/v1/requests` 时平台 MUST 按以下顺序校验，**任一失败立即拒绝**（不进入流水线）：

| 校验顺序 | 校验项 | 失败响应 |
|---------|--------|---------|
| 1 | yaml 语法（YAML parse）| 400 + `{"error":"yaml_parse","detail":"..."}` |
| 2 | 平台 schema（apiVersion/kind/metadata.name/spec 必填）| 400 + `{"error":"schema_invalid","field":"spec.catalogItem","msg":"required"}` |
| 3 | `spec.catalogItem` 存在 + 当前 team 可见（visibility ∋ team_id）| 403 + `{"error":"catalog_not_visible","catalog":"dba/rds"}` |
| 4 | `spec.space` 归属当前 team（tenancy 校验）| 403 + `{"error":"space_not_owned","space":"team-a/orders"}` |
| 5 | `spec.form` 字段 vs `catalog_items.form_schema_json`（类型/必填/枚举/范围）| 422 + `{"error":"form_invalid","field":"spec.form.storage","msg":"must be >= 50","expected":"number","got":"string"}` |
| 6 | team_cloud_grants 校验（team 对该云账号+layer 有授权）| 403 + `{"error":"no_cloud_grant","cloud":"alicloud:corp-prod-1","layer":"Middleware"}` |
| 7 | 幂等键（§3.1）| 200/409 按表 |

**所有错误响应 MUST 含 `correlation_id`**（即使失败也写 `audit_logs`，actor=sa:cicd-xxx，便于 CI 日志关联排查）。校验失败的工单不写 `requests` 表（不入流水线），但写一条 `audit_logs` 留痕。

## 4. 两种 gate 模式

| 模式 | 机制 | 适用 |
|------|------|------|
| **阻塞轮询** | CD 步骤循环 `GET .../gate` | 无 inbound webhook 能力的 CICD（如自建 Jenkins 防火墙后） |
| **回调异步** | `POST .../subscribe` 注册 webhook，平台回调触发下游 CD job | Argo/Flux/GitLab 等支持 webhook 触发的 |

平台提供权威 CLI `tm gate`，内部实现轮询+断线重连，适配**任意** CICD。历史 `tm-gate` 仅作为兼容 shim / shell alias，不作为独立产品或独立代码路径维护。

## 5. CICD 适配器与示例

| CICD | 集成方式 |
|------|----------|
| Jenkins | Pipeline `input` 或 `waitUntil` + 平台 API；或 `sh 'tm gate request --yaml req.yaml --wait'` |
| GitLab CI | `bridge` + manual approval；或 job 调 `tm gate request --wait` |
| Argo CD | Sync `PreSync` hook 调 `tm gate request`；或 notification 触发平台 webhook |
| Flux | `webhook接收器` + 平台回调 |
| GitHub Actions | `environment` protection + step 调 `tm gate request --wait` |
| 通用 | `tm gate request --yaml req.yaml --wait --timeout 48h`（退出码 0/1/2） |

**`tm gate` 伪流程**：
```
tm gate request --yaml req.yaml --wait --timeout 48h
  → POST /requests (yaml)
  → 循环 GET .../gate（指数退避）直到终态 or timeout
  → exit 0(approved) / 1(rejected) / 2(timeout)
```

## 6. 业界对照

| 平台 | 对应能力 |
|------|----------|
| Spacelift | GitHub/GitLab 集成 + 审批 gate + run policy |
| Terraform Cloud | run task（pre-apply hook）对接外部审批 |
| Argo CD | Sync hook + notification + OPA gatekeeper |
| Jenkins | `input` step 人工审批（本方案将其对接到平台 RBAC 审批） |

本方案差异化：**声明式 yaml 入口 + 复用平台审批引擎(RBAC)+多 CICD 适配器 + 阻塞/回调双模**，避免审批逻辑散落各 CICD。

## 7. 关键时序

```
CI --POST yaml--> 平台(建单+生成+plan) --通知--> 审批人
CD --GET gate(轮询) or 订阅 webhook--> (阻塞)
审批人 --批准/驳回--> 平台(apply or 终止) --gate终态/callback--> CD
CD --apply_succeeded: deploy / rejected|timeout|failed: 终止
```

## 8. 数据库表（补充 docs/04）

```
-- AI Generated Start
cicd_triggers(request_id nullable, cicd_kind, pipeline, commit, artifact,
              catalog_item_id, form_hash, triggered_by, idempotency_key UNIQUE)
gate_subscriptions(request_id, mode[poll|webhook], webhook_url, secret_ref, expires_at)
gate_events(request_id,
            status[pending|admission_approved|plan_ready|approval_granted|
                   applying|apply_succeeded|rejected|timeout|failed],
            occurred_at, actor_id, detail)
-- AI Generated End
```
