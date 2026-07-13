# Proposal: platform-contract-freeze

## What

将工程契约冻结（Phase 0 Contract Freeze）从 `iac-self-service-platform` 独立为单独的 change。产出完整的 proto 契约（Connect-native，唯一真相源）、错误码注册表、状态机 fixture、适配器 golden cases、walking skeleton seed。

## Why

工程契约冻结是一个自然里程碑，独立成 change 后有清晰的 propose → apply → archive 生命周期。`iac-self-service-platform` 是巨型 change（D1-D30 + 26 Wave），Phase 0 契约不该混在其中——每阶段（Phase/Wave）都应独立 change（OpenSpec config `git` / `tasks` 规则）。

## Impact

- **影响层级**：平台层（API 契约定义）
- **兼容性**：不破坏任何已有功能——这是首次冻结契约
- **依赖**：`platform-tech-stack-and-scaffold`（脚手架，已归档）提供 buf + Connect-RPC 基础设施
- **被消费方**：`iac-self-service-platform`（Wave 1-8 业务实现）按本契约实现 handler

## Deliverables

| 产物 | 位置 | 状态 |
|---|---|---|
| proto service 定义（6 service / 24 RPC） | `contracts/platform/v1/{common,lifecycle,catalog,registry,cloud}/` | ✅ 已产出 |
| error-codes.yaml（20 错误码） | `contracts/error-codes.yaml` | ✅ 已产出 |
| state-machine fixtures（17 test cases） | `contracts/fixtures/state-machine/` | ✅ 已产出 |
| adapter golden cases（4 cases） | `contracts/fixtures/adapter/` | ✅ 已产出 |
| walking-skeleton seed data | `contracts/fixtures/walking-skeleton/` | ✅ 已产出 |

## Proto 范式（Connect-native，遵循 OpenSpec config `design` 规则）

- **Connect-native**：proto 是唯一契约源，不手写 openapi.yaml / schemas / protocol-mapping.md（D1）。
- **域目录组织**：每个业务域一个目录（`common/` / `lifecycle/` / `catalog/` / `registry/` / `cloud/`）。
- **srv/dto/enum 分离**：域内 `srv.proto` 只放 service 定义；`dto.proto` 放该域全部 message；跨域枚举集中在 `common/enum.proto`。
- **三层服务模型**：Server（用户，OIDC+RBAC）/ Admin（操作员，admin 角色）/ Internal（进程内函数调用，不写 proto）。详见 `contracts/README.md`。
- **域与 capability 对齐**：registry = module-registry（specs/01）、catalog = service-catalog（specs/02），不混域。
- **枚举命名**：proto3 标准，类型前缀（如 `REQUEST_STATUS_SUBMITTED`、`CLOUD_PROVIDER_AWS`、`APPROVAL_GATE_PRE_PLAN`）。
- **注释全英文**：开源标准。

## 本 change 范围（Phase 0 = MVP 主链路契约，非全部平台接口）

只冻结 MVP 主链路需要的接口：管理员注册模块 → 发布目录项 → 用户申请 → 代码生成（占位）→ plan → gate → 审批 → apply。Phase 2+ 扩展能力（PR-first、Run Hooks、Scheduled Runs、AI/MCP、半自动 StateMover）只保留 schema 预留，不进入主链路阻塞条件（遵循 `iac-self-service-platform/tasks.md` 00b Phase 1 feature flag 约束）。

运行时验证（云账号 bootstrap、Day-1 资源栈、walking skeleton 跑通）属 Wave 5（依赖 Wave 1-4 实现），不在本契约 change 范围。
