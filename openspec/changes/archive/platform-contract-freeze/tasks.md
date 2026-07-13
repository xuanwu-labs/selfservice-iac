# Tasks: platform-contract-freeze

## 01-Proto 契约产出（Connect-native，5 域）

- [x] 1.1 产出 `contracts/platform/v1/common/{enum.proto,dto.proto}`（跨域共享：16 枚举 + PageRequest/PageResponse/Actor）
- [x] 1.2 产出 `contracts/platform/v1/lifecycle/srv.proto`（LifecycleService: Create/Get/ListRequests/ListEvents/Cancel/StartPlan/GetArtifact/EvaluateGate/ListPendingApprovals/GetApprovalRun/DecideApproval/StartApply）
- [x] 1.3 产出 `contracts/platform/v1/lifecycle/dto.proto`（LifecycleRequest[含 requester_id/current_stage]/PlanArtifact/GateResult/ApprovalRun[含 gate/节点链]/ApprovalNodeRun/ApprovalDecisionRecord + 12 RPC req/resp）
- [x] 1.4 产出 `contracts/platform/v1/catalog/srv.proto`（CatalogService[Server] + CatalogAdminService[Admin]）
- [x] 1.5 产出 `contracts/platform/v1/catalog/dto.proto`（CatalogItem/ModuleDependencyInfo/AvailableStack + 7 RPC req/resp；CatalogItem 跨域引用 registry.ModuleVersion）
- [x] 1.6 产出 `contracts/platform/v1/registry/srv.proto`（RegistryAdminService[Admin]: Register/List/Get/DeprecateModule）
- [x] 1.7 产出 `contracts/platform/v1/registry/dto.proto`（Module/ModuleVersion/ModuleDependency + 4 RPC req/resp）
- [x] 1.8 产出 `contracts/platform/v1/cloud/srv.proto` + `dto.proto`（EntitlementService[Server]: ListRequestableCloudAccounts）
- [x] 1.9 验证 buf lint + buf generate + Go build 通过

## 02-错误码 + Fixture

- [x] 2.1 产出 `contracts/error-codes.yaml`（20 错误码 + http/grpc/retryable/remediation/owner）
- [x] 2.2 产出 `contracts/fixtures/state-machine/main-lifecycle.json`（17 test cases: RLC/ERR/IDEMP/CONC/EXPR）
- [x] 2.3 产出 `contracts/fixtures/adapter/terramate-adapter.json`（4 golden cases: plan/apply/drift/import）
- [x] 2.4 产出 `contracts/fixtures/walking-skeleton/seed-data.json`（完整 seed + 8 步 golden path）

## 03-职责划分与 MVP 主链路完整性

- [x] 3.1 registry 域独立（RegistryAdminService + Module/ModuleVersion/ModuleDependency 从 catalog 迁出，对齐 capability module-registry vs service-catalog）
- [x] 3.2 catalog 域只留服务目录职责（CatalogService + CatalogAdminService + CatalogItem）
- [x] 3.3 `LifecycleService.ListRequests`（用户/审批人列工单，对齐 CLI `tm request list`）
- [x] 3.4 `LifecycleService.ListPendingApprovals`（审批人按 RBAC 取待办，对齐 CLI `tm approval list`）
- [x] 3.5 `LifecycleService.GetApprovalRun`（审批详情含会签节点链 + 决策历史）
- [x] 3.6 `ApprovalRun` 字段补全（request_id/gate[D21 双门禁]/current_node/started_at/finished_at/expires_at）
- [x] 3.7 新增 `ApprovalNodeRun` + `ApprovalDecisionRecord` message + ApprovalGate/ApprovalNodeMode/ApprovalNodeStatus 枚举

## 04-验收

- [x] 4.1 MVP 全链路覆盖检查（RegistryAdmin.Register → CatalogAdmin.Publish → Catalog.ListItems → Lifecycle 全链路含审批队列/详情/决策 → succeeded，无断点）
- [x] 4.2 Connect-native 原则确认（仓库无 openapi.yaml / schemas / protocol-mapping.md 手写契约）
- [x] 4.3 proto 范式确认（5 域目录 × srv/dto/enum 分离；枚举类型前缀；三层服务模型 Server/Admin/Internal；域与 capability 对齐）
- [x] 4.4 `buf lint && buf generate && go build ./... && go vet ./...` 全绿
