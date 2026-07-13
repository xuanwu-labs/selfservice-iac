# Tasks: platform-contract-mvp-completeness

## 01-Registry 独立成域（职责划分修正）

- [x] 1.1 新建 `contracts/platform/v1/registry/srv.proto`（RegistryAdminService: Register/List/Get/DeprecateModule）
- [x] 1.2 新建 `contracts/platform/v1/registry/dto.proto`（Module / ModuleVersion / ModuleDependency + 4 RPC req/resp）
- [x] 1.3 `catalog/srv.proto` 移除 RegistryAdminService，仅留 CatalogService + CatalogAdminService
- [x] 1.4 `catalog/dto.proto` 移除 Module/ModuleVersion/ModuleDependency/Register*/ListModules/GetModule/DeprecateModule*；CatalogItem.module_version 改为引用 registry.ModuleVersion（import registry/dto.proto）
- [x] 1.5 验证 buf lint 通过（含跨域 import 校验）

## 02-MVP RPC 补全（消除主链路断点）

- [x] 2.1 `lifecycle/srv.proto` 增 `ListRequests`（按 requester_id/team_id/status[]/catalog_item_id 过滤 + 分页）
- [x] 2.2 `lifecycle/srv.proto` 增 `ListPendingApprovals`（按 approver_id 经 RBAC 解析取待办 + 分页）
- [x] 2.3 `lifecycle/srv.proto` 增 `GetApprovalRun`（返回 run + 节点链 + 决策历史）
- [x] 2.4 `lifecycle/dto.proto` 补 3 RPC 的 request/response message

## 03-Message 字段补全（对齐 docs/04 表 + docs/12 审批模型）

- [x] 3.1 `LifecycleRequest` 增 `requester_id`(19) / `current_stage`(20)
- [x] 3.2 `ApprovalRun` 增 `request_id`(5) / `gate`(6) / `current_node`(7) / `started_at`(8) / `finished_at`(9) / `expires_at`(10)
- [x] 3.3 新增 `ApprovalNodeRun` message（node_id/mode/decided_count/required_count/status/timeout_at）
- [x] 3.4 新增 `ApprovalDecisionRecord` message（approver_id/decision/comment/decided_at）
- [x] 3.5 `common/enum.proto` 增 `ApprovalGate` / `ApprovalNodeMode` / `ApprovalNodeStatus`

## 04-代码生成与验证

- [x] 4.1 删除 stale `server/internal/proto/platform/v1/catalog/`（含已迁出的 Module* 类型）
- [x] 4.2 `buf generate` 重新生成（catalog 瘦身 + registry 新增 + lifecycle 增 RPC）
- [x] 4.3 `go build ./...` + `go vet ./...` 全绿（CatalogHandler 仅用 ListItems/CatalogItem，不受影响）
- [x] 4.4 确认 registry 域 Module 类型生成正确（grep 验证）

## 05-契约文档对齐

- [x] 5.1 `contracts/README.md` 更新域清单（5 域）、目录结构（加 registry/）、capability 对齐说明
- [x] 5.2 `contracts/README.md` 更新 RPC 总数（6 service / 24 RPC）
- [x] 5.3 父提案 `iac-self-service-platform/proposal.md` 加 Phase 拆分索引（标注 Phase 0 = freeze[已归档] + 本 change）
- [x] 5.4 能力规格 `specs/03-平台契约.md` MODIFIED（apply 时产出，反映 5 域 / 24 RPC / 审批节点链）

## 06-验收

- [x] 6.1 `buf lint && buf generate && go build ./... && go vet ./...` 全绿
- [x] 6.2 MVP 全链路无断点（registry 注册 → catalog 发布 → lifecycle 全链路 → 审批人队列/详情/决策）
- [x] 6.3 职责划分干净（registry 与 catalog 分域，capability 对齐）
- [x] 6.4 proto 范式确认（域目录 × srv/dto/enum；枚举类型前缀；三层模型）
