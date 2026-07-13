# Tasks: platform-contract-freeze

## 01-Proto 契约产出（Connect-native）

- [x] 1.1 产出 `contracts/platform/v1/request.proto`（RequestService: Create/Get/ListEvents/Cancel）
- [x] 1.2 产出 `contracts/platform/v1/planning.proto`（PlanningService + ArtifactService + GateService）
- [x] 1.3 产出 `contracts/platform/v1/approval.proto`（ApprovalService + ApplyService）
- [x] 1.4 产出 `contracts/platform/v1/catalog.proto`（CatalogService: ListItems/GetCatalogItem）
- [x] 1.5 产出 `contracts/platform/v1/entitlement.proto`（EntitlementService）
- [x] 1.6 验证 buf lint + buf generate + Go build 通过

## 02-错误码 + Fixture

- [x] 2.1 产出 `contracts/error-codes.yaml`（20 错误码 + http/grpc/retryable/remediation/owner）
- [x] 2.2 产出 `contracts/fixtures/state-machine/main-lifecycle.json`（17 test cases: RLC/ERR/IDEMP/CONC/EXPR）
- [x] 2.3 产出 `contracts/fixtures/adapter/terramate-adapter.json`（4 golden cases: plan/apply/drift/import）
- [x] 2.4 产出 `contracts/fixtures/walking-skeleton/seed-data.json`（完整 seed + 8 步 golden path）

## 03-Admin 契约补充（管理员链路）

- [ ] 3.1 产出 `contracts/platform/v1/admin.proto`（ModuleRegistryService: Register/List/Get/Deprecate）
- [ ] 3.2 产出 `contracts/platform/v1/admin.proto`（CatalogAdminService: Publish/Update/Deprecate）
- [ ] 3.3 产出 `contracts/platform/v1/dependency.proto`（DependencyService: ListDependencies）
- [ ] 3.4 补充 ModuleVersion message 加 dependency contract 字段
- [ ] 3.5 验证 buf lint + buf generate + Go build 通过

## 04-验收

- [ ] 4.1 MVP 全链路覆盖检查（管理员注册→用户申请→审批→执行→reconcile）
- [ ] 4.2 Connect-native 原则确认（无 openapi/schema/mapping 手写文件）
- [ ] 4.3 proto 范式确认（参照 ferret 风格，枚举前缀，admin 分离）
