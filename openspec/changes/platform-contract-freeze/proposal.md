# Proposal: platform-contract-freeze

## What

将工程契约冻结（Phase 0 Contract Freeze）从 `iac-self-service-platform` 独立为单独的 change。产出完整的 proto 契约（Connect-native，唯一真相源）、错误码注册表、状态机 fixture、适配器 golden cases、walking skeleton seed。

## Why

工程契约冻结是一个自然里程碑，独立成 change 后有清晰的 propose → apply → archive 生命周期。`iac-self-service-platform` 是巨型 change（D1-D30 + 26 Wave），Phase 0 契约不该混在其中——每阶段（Phase/Wave）都应独立 change。

## Impact

- **影响层级**：平台层（API 契约定义）
- **兼容性**：不破坏任何已有功能——这是首次冻结契约
- **依赖**：`platform-tech-stack-and-scaffold`（脚手架，已归档）提供 buf + Connect-RPC 基础设施

## Deliverables

| 产物 | 位置 | 状态 |
|---|---|---|
| proto service 定义（7 service / 15+ RPC） | `contracts/platform/v1/*.proto` | ✅ 已产出 |
| error-codes.yaml（20 错误码） | `contracts/error-codes.yaml` | ✅ 已产出 |
| state-machine fixtures（17 test cases） | `contracts/fixtures/state-machine/` | ✅ 已产出 |
| adapter golden cases（4 cases） | `contracts/fixtures/adapter/` | ✅ 已产出 |
| walking-skeleton seed data | `contracts/fixtures/walking-skeleton/` | ✅ 已产出 |
| admin proto（ModuleRegistry + CatalogAdmin） | `contracts/platform/v1/admin.proto` | ⏳ 本 change 补充 |
| dependency proto（DependencyService） | `contracts/platform/v1/dependency.proto` | ⏳ 本 change 补充 |

## Proto 范式（参照 ferret + 业界规范）

- Connect-native：proto 是唯一契约源，不手写 openapi/schema
- 文件组织：按业务域分文件（request/planning/approval/catalog/entitlement/admin/dependency）
- admin 分离：admin 操作独立 service（ModuleRegistryService / CatalogAdminService），拦截器按 service 名前缀做权限控制
- 共享类型：cross-domain message 放 `common.proto`（pagination/envelope 等）
- 枚举命名：proto3 标准（ENUM_NAME_ 前缀）
