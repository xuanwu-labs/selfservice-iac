# IaC 自助平台整体方案复评 v2

评估对象：`openspec/changes/iac-self-service-platform`

评估日期：2026-06-26

评估口径：只评估整体方案的可落地性、逻辑串联性、系统边界、接口边界、外部集成边界和剩余风险；不把当前文档当作已经完成的工程实现。

## 1. 总体结论

这版方案已经从“概念完整”推进到“可以进入 Phase 0 契约产物化”的状态。主链路、事实源、状态机、协议边界、平台内建能力、外部可选集成、Phase 1/2/3 范围都已经基本收敛。

综合判断：

| 维度 | 评分 | 判断 |
|------|------|------|
| 方案可落地性 | 88 | Phase 1 golden path 已收敛，bootstrap、契约、状态机、执行目录均有明确入口 |
| 逻辑串联性 | 89 | Catalog → Request → Codegen → Git → PlanArtifact → Approval → Apply → Reconcile 主链路连续 |
| 系统边界清晰度 | 88 | 平台内建 identity/RBAC/approval/CMDB/FinOps，外部系统降为可选对接 |
| 接口契约成熟度 | 84 | HTTP/gRPC/protocol mapping 已规划，但实体契约文件尚未生成 |
| 状态机与韧性 | 88 | request 主状态与审批/break-glass 子域状态已拆开，Outbox/Reconcile/manual task 形成闭环 |
| Phase 规划合理性 | 87 | Phase 1/2/3 范围比上一版清晰，仍需要执行时严控 scope |
| 工程就绪度 | 83 | 文档契约成熟，但 `contracts/`、fixture、golden tests、walking skeleton 还未落地 |

综合评分：**88 / 100**。

一句话结论：**方案层已经成立，当前最大风险不是架构想不清，而是 Phase 0 契约产物和 walking skeleton 尚未把方案变成可机器验证的事实。**

## 2. 权威边界复核

### 2.1 平台定位

当前定位是正确的：平台是 **受治理的 IaC 变更控制面**，不是全功能云资源管理平台，也不是替代 Terraform/Terramate 的执行引擎。

主链路：

`CatalogItem → Request → Codegen → Git pinned_commit → PlanArtifact → Approval → Apply(saved plan) → Reconcile → Succeeded`

其他能力只能作为旁路或后置治理：

- CMDB、FinOps、通知、Scorecard、Runbook、Evidence chain 不能反向阻塞 apply，除非涉及安全或 state 一致性。
- PR-first、Run Hooks、Scheduled Runs、Promotion、AI/MCP、半自动 StateMover、FinOps 优化建议属于 Phase 2+ 或 Phase 3 能力。
- Phase 1 只证明 golden path，不追求完整商业平台。

### 2.2 事实源边界

| 事实源 | 权威内容 | 不应承担 |
|--------|----------|----------|
| 元数据 DB | request、approval、RBAC、resolved params、audit、CMDB index、FinOps records | 不存 Terraform state 全文 |
| Git pinned commit | 生成代码、Terramate stack 定义、变更历史 | 不存执行中临时状态 |
| PlanArtifact | 已审 plan 二进制、sha256、toolchain hash、provider lock hash | 不重新解释为新 plan |
| Terraform state | 资源真实执行状态 | 不直接作为 Web 查询全文暴露 |
| 云账单 | 实际成本事实源 | 不替代申请时 Infracost 估算 |
| Infracost | 申请时和 plan 时成本估算 | 不作为月末核销事实源 |
| 平台 RBAC | 权限事实源 | 不由外部用户中心或 Gateway 直接决定权限 |

这个边界已经比前几版明显更自洽。

## 3. 外部系统与内建能力边界

当前已修正为合理口径：

| 能力 | 是否平台内建 | 外部系统角色 |
|------|--------------|--------------|
| Identity | 是 | 外部用户中心只是 identity source，可选 |
| Team / Role binding | 是 | 外部组织树可同步后映射 |
| RBAC | 是 | 外部组/部门不能直接等于权限 |
| Approval | 是 | OA/ITSM 可联动单号或通知，不替代审批引擎 |
| CMDB | 是 | 外部 CMDB 可同步/导出/对账，不是事实源 |
| FinOps | 是 | 外部财务/成本平台可对账，不替代内部成本模型 |
| Audit / Evidence | 是 | 外部系统可接收事件，不替代平台审计链 |
| Dubbo / Legacy protocol | 否 | 未来 Go Gateway 转换为 HTTP/gRPC |

关键判断：**平台不依赖外部用户中心、OA、ITSM、外部 CMDB、外部成本平台也能独立运行。**

这点非常重要。否则平台会变成“集成胶水”，不是可独立演进的 IaC 控制面。

## 4. 接口与协议边界

当前接口规划方向正确：

- HTTP OpenAPI：面向 Web、CLI、CI/CD、AI agent、外部自动化。
- gRPC Proto：面向内部服务、未来 Go Gateway、高可靠自动化调用。
- Protocol Mapping：统一 HTTP/gRPC 的 path、method、status/code、metadata、error、idempotency、pagination、actor 映射。
- Future Go Gateway：只做 Dubbo/OA/ITSM/Legacy 到 HTTP/gRPC 的协议转换。

必须坚持的约束：

- HTTP 与 gRPC 共享同一 service 层语义。
- gRPC 不绕过 service 层直接操作 DB、Executor、TerramateAdapter。
- Gateway 不持有 RequestLifecycle 主状态。
- Gateway 不持有独立权限模型。
- Gateway 不拥有 CMDB/FinOps 模型。
- Gateway 只能透传或规范化外部身份声明，最终 actor/team/role/RBAC 由平台判定。

Phase 0 最小接口应覆盖：

| 分组 | HTTP | gRPC |
|------|------|------|
| Request | `POST /requests`、`GET /requests/{id}`、`GET /requests/{id}/events` | `RequestService` |
| Plan | `POST /requests/{id}/plan` | `PlanningService.StartPlan` |
| Artifact | `GET /artifacts/{artifact_id}` | `ArtifactService.GetArtifact` |
| Approval | `POST /approvals/{run_id}/decide` | `ApprovalService.DecideApproval` |
| Apply | `POST /requests/{id}/apply` | `ApplyService.StartApply` |
| Catalog | `GET /catalog`、`GET /catalog/{id}` | `CatalogService` |
| Entitlement | `GET /requestable-cloud-accounts` | `EntitlementService` |

Webhook 可以预留，但不应作为 Phase 0 golden path 阻塞项。

## 5. 状态机与长流程连续性

当前 RequestLifecycle 的主状态已经基本合理：

`submitted → generating → pending-admission? → planning → plan-ready → pending-approval → applying → reconciling → succeeded`

异常状态：

`rejected / cancelled / expired / failed-retryable / failed-terminal / waiting-manual / reconcile-pending / blocked-policy / blocked-state-health / paused-drift`

已修正的关键点：

- `timeout` 不是 request status，只能是审批/gate 子状态或投影。
- `escalation` 不是 request status，只存在于 `approval_runs` / `approval_nodes`。
- `break-glass-applying` 不是 request status，break-glass 进 `emergency_runs`。
- `incident` 不是 request status，事故进 `incidents`。
- `gate_status` 是 CI/CD 投影状态，不反向扩展 RequestLifecycle。

韧性链路也基本连续：

| 断点 | 处理 |
|------|------|
| codegen / plan 可重试失败 | `failed-retryable` |
| plan artifact 过期或 hash 不一致 | `expired`，要求重 plan |
| apply 中断或 state 不确定 | `waiting-manual` + `manual_intervention_tasks` |
| CMDB/FinOps/通知失败 | `reconcile-pending` + outbox/reconcile |
| 审批超时 | 审批子状态升级或自动驳回到 `rejected` |
| break-glass | `emergency_runs` + retroactive approval + incident |

结论：**长流程已经不再是“中断即失控”的设计。**

## 6. Phase 路线复核

### Phase 0：契约冻结

目标：不做业务功能，只产出可机器校验的契约。

必须产物：

- `contracts/openapi.yaml`
- `contracts/proto/platform/v1/*.proto`
- `contracts/protocol-mapping.md`
- `contracts/schemas/*.schema.json`
- `contracts/error-codes.yaml`
- `contracts/fixtures/state-machine/*.json`
- `contracts/fixtures/adapter/*.json`
- `contracts/fixtures/walking-skeleton/*.json`

当前状态：**已规划，未产出。**

### Phase 1：Golden Path

目标：一个标准 catalog item 跑通端到端。

允许范围：

- 本地 identity / team / role binding / RBAC。
- 单 OIDC issuer + claim mapping。
- 5 阶段简化 codegen。
- 单 Executor 模式。
- 单 pre-apply 审批。
- CMDB resource index。
- Infracost 估算和预算提示。
- Run Health、Approval Health、Catalog Usage 三张基础看板。

明确不进入 Phase 1：

- 多 OIDC issuer、SCIM、飞书、钉钉、CAS。
- PR-first、Run Hooks、Scheduled Runs、Promotion。
- 完整 FinOps 账单核销、孤儿检测、优化建议。
- AI/MCP/skills。
- 半自动 StateMover。

### Phase 2：生产治理

目标：把平台从“能跑通”推进到“可生产治理”。

包括：

- 完整 9 阶段参数解析与 provenance。
- 双门禁。
- SCIM/飞书/钉钉目录同步。
- CI/CD gate。
- 基础账单核销和预算治理。
- Drift coverage 扩展。
- managed-readonly → managed-changeable。

### Phase 3：商业高级能力

包括：

- 多 issuer 联邦和 CAS proxy。
- Environment Promotion。
- PR-first。
- Run Hooks。
- AI/MCP/skills。
- StateMover 半自动。
- 孤儿检测与 FinOps 优化建议。
- 多云高级治理。

结论：**Phase 路线现在可落地，但执行时必须把 `tasks.md` 中标注的 Phase 边界当成 scope gate。**

## 7. 当前剩余风险

### P0

目前没有发现新的方案级 P0。此前的 request status 污染、API 契约漏 plan/apply、Gateway 边界误放 CMDB/FinOps 等问题已经修正。

### P1

| 风险 | 影响 | 建议 |
|------|------|------|
| `contracts/` 仍无实体产物 | 文档契约无法被测试和生成客户端 | 下一步先产出 OpenAPI/Proto/Schema/fixture |
| 状态机还没有执行模型 | 工程实现可能散落在 handler 中 | Phase 0 定义 `StateTransitionEngine` 或等价表驱动核心 |
| TerramateAdapter 还没有 golden tests | 命令契约可能随实现漂移 | fake Terramate 固化 plan/apply/drift/import-verify |
| Walking skeleton 未运行 | 可落地性还停留在设计可信 | 选择一个 catalog item 真实跑通 |
| Phase 1 scope 仍需执行纪律 | 任务很多，容易把 Phase 2/3 提前 | 每个任务进入 sprint 前检查 Phase 标记 |

### P2

| 风险 | 影响 | 建议 |
|------|------|------|
| Webhook / Gate projection 需要实体映射 | CI/CD 集成时可能混用状态 | 在 `protocol-mapping.md` 明确 gate_status 映射 |
| 外部系统同步策略还没实体化 | 对接企业系统时可能反向污染核心模型 | Gateway 和 DirectorySyncer 只做输入源，不做事实源 |
| 证据链配置未参数化 | 合规行业需要不同 retention / legal hold | 后续转成配置项和演练 checklist |
| Scorecard 数据可信度未实体化 | 早期看板误导运营动作 | Phase 1 仅观察，不触发自动治理 |

## 8. 重新评分

| 项目 | 评分 |
|------|------|
| 方案成熟度 | 89 |
| 可落地性 | 88 |
| 逻辑串联性 | 89 |
| 系统边界清晰度 | 88 |
| 接口契约成熟度 | 84 |
| 工程就绪度 | 83 |
| 商业平台完整度 | 87 |

综合评分：**88 / 100**。

和上一版相比，分数没有大幅上升，但质量更可信。原因是这次不是继续堆能力，而是把几处会误导实现的边界冲突消掉了。

## 9. 最优下一步

不要继续扩文档。下一步应该直接进入 Phase 0 契约产物化：

1. 先产出 `contracts/openapi.yaml` 和 `contracts/proto/platform/v1/*.proto`。
2. 再产出 `contracts/protocol-mapping.md` 和 `contracts/error-codes.yaml`。
3. 产出 P0 JSON Schema。
4. 把 `docs/12a` 转为 state-machine fixtures。
5. 把 `docs/10` 的 TerramateAdapter 命令矩阵转为 adapter golden fixtures。
6. 准备 walking skeleton seed。
7. 跑通一个 golden catalog item。

验收标准只有一个：**不再靠人读 Markdown 判断可行，而是让契约、fixture、golden tests 和 walking skeleton 证明可行。**
