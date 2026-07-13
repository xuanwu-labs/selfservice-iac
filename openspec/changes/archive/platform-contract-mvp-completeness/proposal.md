# Proposal: platform-contract-mvp-completeness

## What

对 Phase 0 契约做职责划分修正 + MVP 主链路 RPC 补全，分两类改动：

1. **职责划分修正**：把 `RegistryAdminService`（模块注册）从 `catalog/` 域拆出，独立为 `registry/` 域。模块注册（module-registry, specs/01）与服务目录（service-catalog, specs/02）是两个独立 capability，不该同域。
2. **MVP RPC 补全**：补齐主链路 3 个必需 RPC + 补全 `ApprovalRun` 等 message 的缺失字段，消除审批人侧与"我的工单"的断点。

## Why

`platform-contract-freeze`（已归档）冻结的契约存在两类遗留问题：

- **职责混淆**：`RegistryAdminService` 及其 `Module` / `ModuleVersion` / `ModuleDependency` message 错放在 `catalog/` 域。catalog 的职责是"把已注册模块版本发布为用户可申请的目录项"，而 registry 的职责是"原子 Terraform 模块的注册/版本/契约提取"——两者输入输出、操作者、生命周期都不同（父提案 specs/01 vs specs/02 已明确区分）。混在一起导致 catalog 既管注册又管发布，违反单一职责。
- **MVP 主链路断点**：审计（对照 docs/12 审批引擎、docs/17 CLI、docs/04 表）发现：用户/审批人没有 `ListRequests`（CLI 已承诺 `tm request list`）；审批人没有 `ListPendingApprovals` / `GetApprovalRun`（无法拿到 run_id、无法看会签节点链与 plan_diff 决策上下文）；`ApprovalRun` 仅 4 字段，缺 `request_id` / `gate`（D21 双门禁核心）/ 节点链。契约不冻结完整，Wave 1 实现必然返工。

## Impact

- **影响层级**：平台层（API 契约定义）。
- **兼容性**：**不破坏运行时**——目前只有 `CatalogHandler`（仅实现 `ListItems`）存在，无 RegistryAdminHandler / 业务逻辑依赖被迁字段。但属于 **proto 破坏性重组**（message 迁移路径变更、字段编号扩展），故走独立 change + SemVer 感知，而非悄悄改。
- **依赖**：`platform-contract-freeze`（已归档）产出的初版契约；本 change 在其基础上修订。
- **被消费方**：`iac-self-service-platform`（Wave 1-8）按修订后契约实现 handler。

## Deliverables

| 产物 | 位置 | 状态 |
|---|---|---|
| registry 域 proto（srv + dto） | `contracts/platform/v1/registry/` | ✅ 已产出 |
| catalog 域瘦身（移除 RegistryAdmin + Module*） | `contracts/platform/v1/catalog/` | ✅ 已产出 |
| lifecycle 增 3 RPC（ListRequests/ListPendingApprovals/GetApprovalRun） | `contracts/platform/v1/lifecycle/srv.proto` | ✅ 已产出 |
| message 字段补全（LifecycleRequest/ApprovalRun + 新 message） | `contracts/platform/v1/lifecycle/dto.proto` | ✅ 已产出 |
| common 枚举补全（ApprovalGate/ApprovalNodeMode/ApprovalNodeStatus） | `contracts/platform/v1/common/enum.proto` | ✅ 已产出 |
| 重新生成 Go 代码（catalog 瘦身 + registry 新增） | `server/internal/proto/platform/v1/` | ✅ 已产出 |
| 契约文档对齐（README 5 域 / 24 RPC） | `contracts/README.md` | ✅ 已产出 |
| 能力规格修订（MODIFIED 03-平台契约） | `specs/03-平台契约.md` | ⏳ apply 时产出 |

## 不做的事

- handler 业务实现（Wave 1+ 任务）。
- DB migration（Wave 1+；本 change 只对齐 proto 与 docs/04 表设计，不动 SQL）。
- Phase 2+ RPC（ListStacks/ListDrifts/EstimateCost 等，推迟到对应 Wave）。
