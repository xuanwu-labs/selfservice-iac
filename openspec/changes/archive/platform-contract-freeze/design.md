# Design: platform-contract-freeze

## 01-设计决策

### D1: Connect-native（proto 是唯一契约源）

proto 文件是唯一契约源。不手写 openapi.yaml / schemas/*.json / protocol-mapping.md。
Connect-RPC 天然支持 gRPC / gRPC-Web / Connect-JSON 三种协议，一个 handler 全覆盖。

**理由**：减少双源漂移；改接口只改 proto 一处；前端用 connect-es 客户端自带类型。

### D2: 三层服务模型（Server / Admin / Internal）

每个 proto service 归入三层之一，决定鉴权与网络暴露：

| 层 | 调用方 | 鉴权 | 网络 |
|---|---|---|---|
| **Server**（用户） | Web/CLI/AI 终端用户 | OIDC JWT + RBAC | Connect `/api/` 暴露 |
| **Admin**（操作员） | 平台管理员 | OIDC JWT + **admin 角色** | Connect `/api/` 暴露，拦截器按 service 名前缀鉴权 |
| **Internal**（进程内） | 平台自身（codegen/executor/CMDB/drift） | **无 RPC**，直接函数调用 | 不暴露，不写 proto |

**Admin 分离实现**：admin 操作（`RegistryAdminService` / `CatalogAdminService`）与用户操作（`CatalogService`）拆为不同 service，放在同一域的 `srv.proto`。Connect 拦截器按 procedure 名前缀（`/aether.platform.v1.RegistryAdmin*`、`/aether.platform.v1.CatalogAdmin*`）要求 admin 角色。

**为什么 Internal 不写 proto**：本项目是单体（D39：进程内直接函数调用，无消息中间件）。内部操作（codegen、executor、CMDB ingest、drift scan）是同进程函数调用，只有跨进程/跨团队边界才需要 proto service。

### D3: proto 文件组织（按业务域 × srv/dto/enum 分离）

每个业务域一个目录（`common/` / `lifecycle/` / `catalog/` / `cloud/`），域内按职责拆三个文件：

| 文件 | 内容 | 规则 |
|---|---|---|
| `srv.proto` | 仅 `service` 定义，不含 message | 每域一个 |
| `dto.proto` | 该域全部 message（领域对象 + RPC 请求/响应） | 每域一个 |
| `enum.proto` | 仅枚举 | **`common/enum.proto` 是跨域枚举的唯一归属** |

**理由**：`srv.proto` 作为纯接口清单可读、可审查；dto/enum 演进不动 service 签名；跨域共享类型集中在 `common/` 避免 import 循环。

当前规模（6 service / 21 RPC）已按此结构落地：

| 域 | srv.proto 中的 service | 层 |
|---|---|---|
| `lifecycle/` | `LifecycleService`（9 RPC） | Server |
| `catalog/` | `CatalogService`（4 RPC）+ `RegistryAdminService`（4 RPC）+ `CatalogAdminService`（3 RPC） | Server + Admin |
| `cloud/` | `EntitlementService`（1 RPC） | Server |
| `common/` | （无 service，仅共享 `dto.proto` + `enum.proto`） | — |

### D4: 每阶段独立 change

Phase 0 / Wave 1 / Wave 2 ... 各自独立 OpenSpec change，不混在一个巨型 change 里（OpenSpec config `tasks` 规则）。每个 change 有清晰的 propose → apply → archive 生命周期。本 change 是从 `iac-self-service-platform`（设计总纲）拆出的 Phase 0 契约里程碑。

## 02-MVP 全链路覆盖

```
管理员链路（catalog 域 Admin service）：
  RegistryAdminService.RegisterModule
    → CatalogAdminService.PublishCatalogItem
    → 用户可见（CatalogService.ListItems）

用户链路（lifecycle 域 + catalog 域 Server service）：
  CatalogService.GetCatalogItem
    → LifecycleService.CreateRequest
    → (codegen 占位)
    → LifecycleService.StartPlan
    → LifecycleService.GetArtifact
    → LifecycleService.EvaluateGate
    → LifecycleService.DecideApproval
    → LifecycleService.StartApply
    → LifecycleService.GetRequest（= succeeded）
```

MVP 全链路由 6 service / 21 RPC 完整覆盖，无 admin 占位未实现缺口——`RegistryAdminService` 是 plan→apply 的前置（无 module 注册则无法 PublishCatalogItem，用户链路无法启动），因此 Phase 0 必须包含 admin 链路契约。

## 03-依赖关系

本 change 依赖 `platform-tech-stack-and-scaffold`（已归档）的 buf + Connect-RPC 基础设施。
本 change 产出被 `iac-self-service-platform`（Wave 1-8 业务实现）消费：handler 按 proto 接口实现。

## 04-验收标准

- [x] `buf lint` + `buf generate` + `go build ./...` 全绿。
- [x] Connect-native 原则确认：仓库无 openapi.yaml / schemas / protocol-mapping.md 手写契约文件。
- [x] MVP 全链路覆盖确认：6 service / 21 RPC 覆盖 admin 注册 → 用户申请 → plan → gate → 审批 → apply。
- [x] 三层模型落地确认：每个 service 明确归入 Server / Admin；Internal 操作无 proto。
- [x] 枚举命名规范确认：所有枚举值带类型前缀（proto3 规范）。

## 05-proto 与 docs/04 表设计的一致性

proto message 与 `iac-self-service-platform/docs/04-数据库设计.md` 表结构**结构一致但 proto 更精简**——这是有意的边界划分：

| 边界 | 归属 |
|---|---|
| 用户可见字段（申请/查询/审批/目录浏览） | proto message（契约层） |
| 平台内部字段（`resolved_params_json`、`toolchain_profile_hash`、`provider_lock_hash`、`layer_rule_set_version_id`、`idempotency_key` 等） | DB 表（实现层），不进 proto |

已交叉验证的核心对应（无结构冲突）：

| proto message | DB 表 | 关键字段对应 |
|---|---|---|
| `LifecycleRequest` | `requests` | id / catalog_item_id / bundle_id / env_id / tenant_id / form_values(map↔json) / form_hash / pinned_commit / status / version |
| `LifecycleEvent` | `request_events` | id / request_id / event_type(↔stage) / actor / correlation_id / occurred_at |
| `PlanArtifact` | `artifacts` | id / request_id / status / plan_hash(↔sha256) / summary / storage_uri(↔ref) / expires_at |
| `ApprovalRun` | `approval_runs` | run_id / status / decided_by / decided_at |
| `CatalogItem` / `Module` / `ModuleVersion` | `catalog_items` / `modules` / `module_versions` | name / git_source / status / variables_contract_json / cardinality / instance_key / form_schema_json / default_tags |
| `CloudAccount` | `cloud_accounts` | id / provider / name / status / regions |

**结论**：proto 与 DB 表无结构冲突；proto 精简是设计意图（用户契约 vs 实现细节分离），不构成漂移。运行时（Wave 1+）的 sqlc query 与 handler 负责在两者间映射。
