# Design: platform-contract-freeze

## 01-设计决策

### D1: Connect-native（proto 是唯一契约源）

proto 文件是唯一契约源。不手写 openapi.yaml / schemas/*.json / protocol-mapping.md。
Connect-RPC 天然支持 gRPC / gRPC-Web / Connect-JSON 三种协议，一个 handler 全覆盖。

**理由**：减少双源漂移；改接口只改 proto 一处；前端用 connect-es 客户端自带类型。

### D2: admin 分离（独立 service，同一 package）

管理操作（注册模块/发布目录项）和用户操作（浏览/申请/审批）分到不同 service。
通过 Connect 拦截器按 service 名前缀做权限控制（`ModuleRegistryService.*` 需要 admin 角色）。

**参照**：ferret 每个业务域分 `Server`/`AdminServer`/`InternalServer` 三个 service。
我们简化为：用户 service（已有）+ admin service（`admin.proto` 集中管理）。

### D3: proto 文件组织（按业务域，不按 srv/dto/enum）

当前规模（7 service / 15+ RPC）按业务域分文件足够清晰。
等业务域 > 8 个时再按 ferret 风格拆 `_srv.proto` / `_dto.proto` / `_enum.proto`。

### D4: 每阶段独立 change

Phase 0 / Wave 1 / Wave 2... 各自独立 OpenSpec change，不混在一个巨型 change 里。
每个 change 有清晰的 propose → apply → archive 生命周期。

## 02-MVP 全链路覆盖

```
管理员链路：
  RegisterModule → PublishCatalogItem → 用户可见

用户链路：
  ListItems → CreateRequest → (codegen) → StartPlan → EvaluateGate
  → DecideApproval → StartApply → GetRequest(=succeeded)
```

## 03-依赖关系

本 change 依赖 `platform-tech-stack-and-scaffold`（已归档）的 buf + Connect-RPC 基础设施。
本 change 产出被 `iac-self-service-platform`（Wave 1-8 业务实现）消费。
