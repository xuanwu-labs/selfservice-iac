# Tasks: platform-contract-freeze

## 01-Proto 契约产出（Connect-native）

- [x] 1.1 产出 `contracts/platform/v1/common/{enum.proto,dto.proto}`（跨域共享：13 枚举 + PageRequest/PageResponse/Actor）
- [x] 1.2 产出 `contracts/platform/v1/lifecycle/srv.proto`（LifecycleService: Create/Get/ListEvents/Cancel/StartPlan/GetArtifact/EvaluateGate/DecideApproval/StartApply）
- [x] 1.3 产出 `contracts/platform/v1/lifecycle/dto.proto`（LifecycleRequest/PlanArtifact/GateResult/ApprovalRun + 9 RPC req/resp）
- [x] 1.4 产出 `contracts/platform/v1/catalog/srv.proto`（CatalogService[Server] + RegistryAdminService[Admin] + CatalogAdminService[Admin]）
- [x] 1.5 产出 `contracts/platform/v1/catalog/dto.proto`（CatalogItem/ModuleVersion/Module/AvailableStack + 11 RPC req/resp）
- [x] 1.6 产出 `contracts/platform/v1/cloud/srv.proto` + `dto.proto`（EntitlementService[Server]: ListRequestableCloudAccounts）
- [x] 1.7 验证 buf lint + buf generate + Go build 通过

## 02-错误码 + Fixture

- [x] 2.1 产出 `contracts/error-codes.yaml`（20 错误码 + http/grpc/retryable/remediation/owner）
- [x] 2.2 产出 `contracts/fixtures/state-machine/main-lifecycle.json`（17 test cases: RLC/ERR/IDEMP/CONC/EXPR）
- [x] 2.3 产出 `contracts/fixtures/adapter/terramate-adapter.json`（4 golden cases: plan/apply/drift/import）
- [x] 2.4 产出 `contracts/fixtures/walking-skeleton/seed-data.json`（完整 seed + 8 步 golden path）

## 03-Admin 契约（管理员链路，归入 catalog 域）

> Admin 操作不单独建 `admin.proto`——按域归属原则，模块注册与目录发布属于 catalog 域，放在 `catalog/srv.proto` + `catalog/dto.proto`。

- [x] 3.1 `catalog/srv.proto` 产出 `RegistryAdminService`（RegisterModule/ListModules/GetModule/DeprecateModule）
- [x] 3.2 `catalog/srv.proto` 产出 `CatalogAdminService`（PublishCatalogItem/UpdateCatalogItem/DeprecateCatalogItem）
- [x] 3.3 `catalog/srv.proto` 产出 `CatalogService.ListModuleDependencies` + `ListAvailableStacks`（依赖查询归入用户侧 Server service）
- [x] 3.4 `catalog/dto.proto` 的 `ModuleVersion` 已含 dependency contract 字段（`dependencies: repeated ModuleDependency`）
- [x] 3.5 验证 buf lint + buf generate + Go build 通过

## 04-验收

- [x] 4.1 MVP 全链路覆盖检查（RegistryAdmin.Register → CatalogAdmin.Publish → Catalog.ListItems → Lifecycle 全链路 → succeeded）
- [x] 4.2 Connect-native 原则确认（仓库无 openapi.yaml / schemas / protocol-mapping.md 手写契约）
- [x] 4.3 proto 范式确认（域目录 × srv/dto/enum 分离；枚举类型前缀；三层服务模型 Server/Admin/Internal）
- [x] 4.4 `buf lint && buf generate && go build ./...` 全绿
