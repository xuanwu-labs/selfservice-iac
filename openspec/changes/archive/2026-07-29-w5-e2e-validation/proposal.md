## Why

W5 是 MVP 的闭环验证——证明 W1+W2+W3 的后端能力能端到端跑通。使用 **null/random provider + local backend**（业界最佳实践，零云凭证、零容器依赖），验证完整链路：模块注册 → catalog 发布 → 工单提交 → codegen → git commit → terramate plan → 审批 → apply → state 写入。

**影响层级**：测试层（`server/e2e/`）+ 测试 fixture（`test-fixtures/`）。

**为什么现在做**：W1-W3 全部归档（9 个模块），所有后端能力就位。W5 验证它们能串通。

## What Changes

### 1. E2E 测试 fixture（null/random provider）

创建 `test-fixtures/atomic-null/`：
- 一个使用 `null_resource` + `random_id` 的原子模块（替代真实 alicloud 模块）
- variables.tf（含 scalar required + scalar default + sensitive + platform-inferred 字段）
- outputs.tf（scalar 输出）
- 不需要任何云凭证，不需要 provider 下载（null + random 是 Terraform 内置）

### 2. E2E 测试用例（Go test）

新建 `server/e2e/`：
- **TestE2E_RegisterModule**：注册 null 模块 → 契约提取 → 验证 variables_contract_json
- **TestE2E_PublishCatalog**：发布 catalog_item → FormSchemaGenerator 裁剪 → 验证 form_schema_json
- **TestE2E_CreateRequest**：提交工单 → Pipeline.Execute → codegen 生成 stack 目录
- **TestE2E_GitCommit**：验证 workspace.WriteFiles → commit SHA 返回
- **TestE2E_Plan**：TerramateAdapter.RunPlan（null provider + local backend）→ exit code 0
- **TestE2E_Approval**：ApprovalService.Approve → 状态 pending_approval → applying
- **TestE2E_Apply**：TerramateAdapter.RunApply → exit code 0 → terraform.tfstate 存在
- **TestE2E_StateWrite**：验证 local backend terraform.tfstate 含 null_resource

### 3. Walking Skeleton（手动验证脚本）

创建 `scripts/walking-skeleton/`：
- 一个 shell 脚本引导完整流程（不依赖 Go test）
- 用于 demo / 演示

### 不做

- 前端实现（W5 后续单独 change）
- 真实云测试（Phase 2 单独 smoke pipeline）
- LocalStack / MinIO（null provider + local backend 够用）
- 多 stack 依赖测试（Phase 2）

## Capabilities

### New Capabilities

- `e2e-validation`: 端到端测试——null/random provider + local backend，验证模块注册→catalog→工单→codegen→plan→审批→apply→state 全链路

## Impact

- **代码**：新建 `server/e2e/`（Go test）+ `test-fixtures/atomic-null/`（TF 模块）+ `scripts/walking-skeleton/`
- **依赖**：需要 terramate CLI + terraform CLI 安装在本机（CI 环境预装）
- **DB**：不改 schema（用已有表）
- **测试**：本身就是测试
