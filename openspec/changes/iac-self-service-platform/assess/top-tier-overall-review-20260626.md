# IaC 自助平台顶级优化后整体复评

评估对象：`openspec/changes/iac-self-service-platform`

评估日期：2026-06-26

评估视角：平台架构师、代码工程师、云架构师、商业 IaC 平台产品负责人

## 1. 总体结论

本轮顶级优化后，方案已经从“可实施的内部 IaC 平台设计”提升到“具备商业级产品骨架的平台方案”。此前复评中指出的 API 契约、状态机测试矩阵、云账号 bootstrap、产品运营、证据链、runbook 等关键缺口，已经被补入独立文档和任务入口。

综合评分采用双口径：

- **方案成熟度：89 / 100**。主链路、Phase 边界、运营、证据链、Terramate 适配、存量 read-only 过渡均已形成闭环。
- **工程就绪度：82 / 100**。若进入实现，还需把 Phase 0 契约变成机器可校验产物和 walking skeleton。

方案级综合判断：**88 / 100**。

| 维度 | 当前评分 | 判断 |
|------|----------|------|
| 可落地性 | 88 | Phase 1 边界、feature flag、bootstrap 顺序和主链路职责清楚 |
| 逻辑连续性 | 88 | RequestLifecycle、Outbox、Reconcile、manual task 和子状态边界已形成连续链路 |
| 自洽性 | 88 | CLI、路径、5 阶段 codegen、TerramateAdapter、资源栈 bootstrap 口径已统一 |
| 架构优雅性 | 86 | 主链路收敛，商业能力后置，避免 MVP 被拖重 |
| 商业平台成熟度 | 88 | PR-first、Run Hooks、Scorecard、证据包、Runbook 已有方案级闭环 |
| 工程施工成熟度 | 82 | 契约已成形，Adapter/Bootstrap/状态测试补齐；仍缺 OpenAPI/JSON Schema/fixture 实体产物 |

一句话结论：**仅按方案校验，这套 IaC 自助平台设计已经可行且逻辑自洽；剩余问题主要是实现阶段的验证产物，而非方案级断点。**

## 2. 已显著增强的部分

### 2.1 Phase 0 Contract Freeze 被补齐

新增 `docs/00-工程契约.md`、`specs/19-API与Schema契约.md`、`docs/12a-状态机测试矩阵.md`，把原先偏抽象的设计意图转成实施前置契约：

- API envelope、错误码、幂等头、correlation id、分页规范。
- `RequestCreate`、`ResolvedParams`、`PathGeneratorOutput`、`PlanArtifact` schema。
- 主链路、异常链路、幂等、并发状态机测试矩阵。
- Phase 0 验收项：没有这些契约，不进入大规模编码。

这解决了上一轮最核心的工程风险：前端、CLI、CICD、orchestrator、codegen 并行猜接口。

### 2.2 Day-0 云账号 Bootstrap 断点被补齐

新增 `docs/06a-云账号Bootstrap手册.md` 后，首云账号接入不再只停留在 “OIDC-first” 原则，而有了顺序：

1. 登记云账号。
2. 双人写入 bootstrap 凭据引用。
3. 发布平台 OIDC issuer/JWKS。
4. 配置云账号 OIDC trust。
5. 生成首个 execution role。
6. 创建 `team_cloud_grants`。
7. 首个 dry-run。
8. 首个 apply。

这使 Phase 1 第一条真实 apply 链路具备启动路径。

### 2.3 商业工作流补齐，但没有污染 Phase 1 主链路

`specs/20-VCS与PR工作流.md`、`specs/21-RunHooks与策略扩展.md`、`docs/13`、`docs/16`、`docs/07` 的扩展，使方案对标 Atlantis、Spacelift、Terraform Cloud、env0 的核心能力：

- Form-first + PR-first 双入口。
- PR plan summary。
- Apply requirements。
- Run Hooks。
- Scheduled runs。
- Environment promotion。

这些能力被明确放到 Phase 2+，这很关键。方案没有把商业级能力全部塞进 Phase 1，仍然保留 MVP 可落地性。

### 2.4 平台产品化能力被正式纳入设计

新增 `docs/18-平台运营与指标.md`、`docs/19-服务目录与Scorecard.md`、`specs/22-平台运营指标.md`，把平台从“能创建资源”推进到“能运营平台”：

- Self-service ratio。
- Ticket deflection。
- Successful run rate。
- Approval wait。
- Catalog health。
- Scorecard。
- Failed run reason TopN。

这比传统内部工具设计更接近商业平台：可推广、可复盘、可治理。

### 2.5 企业级证据链从声明变成可落表模型

新增 `docs/20-安全与合规证据链.md`、`docs/21-运维Runbook.md`，并在 `docs/04` 增加 `incidents`、`runbook_executions`、`drill_results`、`run_hooks`、`run_hook_results`、`platform_scorecards`、`catalog_health_checks` 等表。

这让审计、break-glass、state restore、DB restore、OIDC JWKS、provider mirror、CMDB reconcile 等不再是“发生了再说”，而是有证据链、有演练、有 owner 的运维能力。

### 2.6 本轮继续收敛的不足

本轮围绕方案级不足补齐了五类约束：

- `tasks.md` 补充 Phase 1 feature flag 约束，明确 PR-first、Run Hooks、Promotion、AI/MCP、StateMover、FinOps 优化建议不进入主链路。
- `docs/19` 补充 Scorecard `confidence` / `data_completeness`，避免早期低质量数据触发错误治理。
- `docs/20` 补充证据保留周期、legal hold、HMAC key rotation、chain break 处置。
- `docs/10` 补充 Terramate OSS 适配验证策略，降低 CLI 行为变化风险。
- `docs/15` 补充 `managed-readonly` 运营规则，避免半纳管资源长期悬挂。

## 3. 当前剩余关键风险

### 已降级：Terramate saved-plan apply 路径已文档消歧

`docs/10` 已新增 `TerramateAdapter` 命令矩阵，`docs/12`、`docs/04`、`specs/06`、`tasks.md` 已统一为 `TerramateAdapter.RunApplySavedPlan`，明确 saved-plan apply 通过 `terramate run -- terraform apply "<plan>"` 执行，禁止 Orchestrator 绕过 Terramate 直接调用 Terraform。

剩余工作：把该矩阵转成 fake Terramate golden tests。

### 已降级：资源栈 bootstrap 顺序已补齐

`docs/06a` 已新增 Day-1 资源栈 Bootstrap，定义 Global VPC seed stack、outputs 校验、dependency seed / `environment_tenant_bindings`、首个 Application/Middleware catalog item 的顺序，并在 `tasks.md` Phase 0 中加入 0.6。

剩余工作：实际跑通 Global VPC seed 与首个 golden catalog item。

### P1-1：Phase 0 硬门槛尚未实际验收（实现阶段）

`tasks.md` 已列出 0.1-0.8，但目前仍是计划项。文档成熟度已经高于工程产物成熟度。

建议：把 Phase 0 验收变成第一批执行事项：OpenAPI、Proto、Protocol Mapping、JSON Schema、状态机 fixture、golden catalog item 演练记录、correlation id 全链路 trace。

### P1-2：契约仍是文档级，还未机器可校验（实现阶段）

虽然 `docs/00` 和 `specs/19` 已把 API 和 schema 写清楚，但下一步必须把它们落成：

- OpenAPI 文件。
- JSON Schema。
- Golden request/response fixture。
- Error code registry。
- Contract tests。

否则工程团队仍可能在实现阶段出现“看了同一文档但理解不同”的偏差。

建议：Phase 0 第一批任务不要写业务逻辑，先产出 `openapi.yaml`、schema fixture、状态机测试 fixture。

### P1-3：状态机测试矩阵覆盖了 case，但还缺执行模型（实现阶段）

`docs/12a` 已经列出 RLC/IDEMP/CONC 测试 ID，但工程上还需要决定：

- 状态机是纯函数表驱动，还是散落在 orchestrator handler 中。
- 如何模拟 Git/DB/ObjectStore/StateBackend/CMDB 的部分失败。
- apply interrupted 的人工恢复测试如何写。
- outbox worker 并发 claim 如何测。

建议：实现前先定义 `StateTransitionEngine` 或等价的状态迁移核心，让测试矩阵直接驱动单元测试，而不是靠 e2e 后补。

### 已收敛：Phase 1 被商业级能力拖重的风险

`tasks.md` 已新增 Phase 1 feature flag 约束，明确 PR-first、Run Hooks、Scheduled Runs、Promotion、AI/MCP、半自动 StateMover、FinOps 优化建议、孤儿自动释放默认关闭。

剩余要求：执行阶段必须按该边界管理 scope，不因评审或演示诉求提前打开 Phase 2+ 能力。

### 已降级：状态机迁移矩阵与 DB `requests.status` 枚举边界已明确

`docs/12` 已新增 request status 与审批子状态边界，明确 `escalation` 属于 `approval_runs` / `approval_nodes`，break-glass 属于 `emergency_runs`，request 主状态保持 `pending-approval` / `applying` 等主链路状态。

剩余工作：实现时按该边界建立表驱动状态机测试。

### 已降级：`tm-gate` 与单一 `tm` CLI 口径已统一

`design.md`、`docs/16`、`specs/13`、`specs/14`、`tasks.md`、`proposal.md` 已统一为 `tm gate` 权威入口，`tm-gate` 仅作为兼容 shim。

剩余工作：后续脚本示例默认使用 `tm gate`。

### 已降级：路径模型残留已基本清理

`tasks.md` 中的 tenant-first 示例已替换为 D29 layer-first 示例。`docs/02` 的 `globals/middleware/application` 说明保留为出厂默认层名/历史兼容上下文，不再作为新 stack 输出目标。

建议后续 demo/prototype 继续统一：

- `global/<component>-<tenant>-<env>`
- `middleware/<tenant>/<component>-<env>`
- `application/<tenant>/<team>/[bundle/]<component>-<env>`

旧路径只在 import/迁移章节出现。

### 已收敛：企业证据链保留策略与密钥生命周期

`docs/20` 已补充：

- HMAC signing key rotation。
- Object retention policy。
- Evidence package TTL。
- Legal hold。
- 审计日志链断裂后的处置。

剩余要求：进入强合规行业前，需要把这些规则转成平台配置和演练检查项。

### 已收敛：Scorecard 数据质量边界

`docs/19` 已补充 `confidence`、`data_completeness`、`sample_size`、`missing_dimensions`，并明确 `score < 60` 且 `confidence=high` 才能进入阻断或下架候选。

剩余要求：实现阶段保证 Scorecard 仅作为 Phase 1 观察指标，不作为自动治理依据。

## 4. 按原目标复评

### 4.1 可落地性

现在可落地性已经达到方案级可行。最关键的变化是 Phase 0 和 walking skeleton 被提到编码前置，同时 Phase 1 feature flag 规则防止商业能力拖重主链路。

仍需注意：真正开工时第一阶段必须严格限制范围；这是执行治理问题，不再是方案逻辑断点。

### 4.2 逻辑断点

长流程断点已经被系统性补齐：

- `pinned_commit` 作为代码锚点。
- plan artifact sha256/TTL。
- apply interrupted 进入 `waiting-manual`。
- CMDB/FinOps/通知失败进入 outbox/reconcile。
- manual task 承载人工兜底。

剩余问题主要是测试执行模型，不是架构概念缺失。

### 4.3 自洽性

整体自洽性已经较高。权威模型、路径契约、参数 rank、状态机、schema、bootstrap、运营、证据链都已形成明显主次。

主要口径已统一；后续要防止旧原型/demo 或历史评估报告影响实施认知。

### 4.4 开源方案适配

与开源 Terramate/Terraform/OpenTofu 的适配路径总体合理：

- 不改 Terramate 核心，exec adapter 调用 CLI。
- 使用 Terramate stack/tags/DAG 做影响面和编排。
- Terraform/OpenTofu 负责 state 和 provider。
- 平台只做控制面、审计、审批、codegen、运营与证据链。

主要风险：exec adapter 必须锁定 CLI 输出契约和版本范围；`terramate list --changed`、DAG、tags 的行为必须有兼容性测试。

## 5. 新评分

| 版本 | 分数 | 说明 |
|------|------|------|
| 初始评估 | 65-70 | 概念完整但断点多、多权威冲突 |
| 第一轮优化后 | 78 | 主链路收敛，状态机和 schema 明显增强 |
| 顶级优化后（设计成熟度） | 86 | 工程契约、商业工作流、产品运营、证据链补齐 |
| 顶级优化后（工程就绪度） | 82 | Adapter/Bootstrap/状态边界已补齐，Phase 0 fixture 尚未验收 |
| 顶级优化后（保守综合） | 84 | 适合进入 Phase 0 契约产物实施，不适合直接大规模业务编码 |
| 本轮方案级优化后 | 88 | Phase 1 边界、Scorecard 可信度、证据保留、Terramate 适配、read-only 运营补齐 |

短板已经不再是“方案是否成立”，而是“实现阶段能否按边界和契约执行”。

## 6. 最优下一步

> 📌 **后续更新（2026-07）**：本节设想的 `docs/00a-contract-artifacts-plan.md` 及其 OpenAPI+gRPC 双契约源方案已被 **D1 Connect-native** 取代——proto 是唯一契约源，不手写 openapi.yaml / protocol-mapping.md / schemas。实际落地的契约见 `specs/03-平台契约.md` 与 `contracts/`（5 域 / 6 service / 24 RPC）。下方原文保留作历史评审记录。

下一阶段应进入 Phase 0 契约产物化，具体规划见 `docs/00a-contract-artifacts-plan.md`。建议按五类交付推进：

1. **Protocol contract artifacts**：产出 `contracts/openapi.yaml`、`contracts/proto/platform/v1/*.proto`、`contracts/protocol-mapping.md`、`contracts/schemas/*.schema.json`、`contracts/error-codes.yaml`，明确核心平台提供 HTTP/gRPC，Dubbo/OA/ITSM/Legacy 由未来 Go Gateway 转换；identity/RBAC、approval、CMDB、FinOps 是平台内建能力，外部系统只作为可选对接源。
2. **State machine fixtures**：把 RLC/IDEMP/CONC 测试矩阵转成 `contracts/fixtures/state-machine/*.json`。
3. **TerramateAdapter golden tests**：用 fake Terramate 锁定 plan、saved-plan apply、drift-plan、import-verify 命令契约。
4. **Bootstrap and walking skeleton**：用一个 golden catalog item 跑通 Request → Codegen → Git → Plan → Approval → Apply → Reconcile。
5. **Operational minimum**：Run Health、Approval Health、Catalog Usage 三张基础看板接入 walking skeleton 的真实数据。

做到这些，方案就从“顶级设计”进入“可证明可落地”的阶段。
