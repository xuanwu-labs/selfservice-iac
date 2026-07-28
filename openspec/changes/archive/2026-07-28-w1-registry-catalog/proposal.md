## Why

W1-03 是 `iac-self-service-platform` 的第三个实现模块，把 proto 契约接到 W1-02 数据层，落地**第一个真实业务流**：模块注册（Git clone → 解析 variables.tf → 生成 scalar 契约）+ 服务目录发布（catalog_item 上架 + 可见性控制）。这是用户从"有代码框架"到"能注册一个模块并发布到货架"的关键一步。

**影响层级**：业务核心层（`server/core/{registry,catalog}/`）+ 传输层（`server/api/connect/`），不改 DB schema / proto 契约。

**为什么现在做**：W1-02 的 ModuleRepo/CatalogRepo + catalog/validator.go（D40）+ proto 契约已全部就位，前置依赖满足。

## What Changes

### 1. core/registry/（task 3.1）—— 模块注册引擎

新建 `server/core/registry/`：
- **RegistryService**：注入 ModuleRepo + GitProvider，实现 RegisterModule 流程
- **ContractExtractor**：用 `terraform-config-inspect`（HashiCorp 官方库）解析 `.tf` 文件，提取 `variables.tf` 的纯 scalar 契约（name/type/default/description）→ `variables_contract_json`（D25 零侵入）
- **状态机**：`pending_validation → validated | validation_failed`（task 3.2）
- **terraform validate**（可选）：注册时校验模块可 `init + validate`（MVP 可降级为"仅 HCL 解析成功即 validated"，真实 terraform validate 留 W2）

### 2. GitProvider 真实实现（task 3.1 依赖）

W1-01 的 GitProvider 是 noop stub。本 change 用 `go-git` 实现 `data/repo/git_provider.go`（或 `core/adapters/git/gogit.go`）：
- Clone(git_source, ref) → 本地临时目录
- 支持 SSH + HTTPS（credential 注入，D23 后续完善，MVP 用环境变量 GIT_SSH_COMMAND）

### 3. core/catalog/（task 3.3）—— 补全服务目录

现有 `core/catalog/validator.go`（D40 JSON Schema 校验）。补全：
- **CatalogService**：注入 CatalogRepo + ModuleVersionRepo，实现 PublishCatalogItem
- **FormSchemaGenerator**：从 `variables_contract_json` 裁剪出 `form_schema_json`（用户可见字段 + 校验规则）
- **DefaultsApplier**：维护 `defaults_json`（最佳实践默认值覆盖）
- **VisibilityControl**：`visibility_json` 团队可见性管理

### 4. api/connect/ handler（接 DB）

- 新建 `api/connect/registry.go`：RegistryAdminService handler（接 RegistryService）
- 改造 `api/connect/catalog.go`：CatalogService handler（从占位改为接 CatalogService）

### 5. HCL 解析依赖

引入 `github.com/hashicorp/terraform-config-inspect/tfconfig`（HashiCorp 官方，专门解析 tf 模块结构）。

### 不做（本次范围外）

- terraform validate 真实执行（需装 terraform + provider 下载，MVP 降级为 HCL 解析校验）
- 批量注册脚本（task 3.5，推迟 YAGNI）
- module_dependencies 自动推断（W2，需静态分析 module 间调用）
- 多维分类（category.l1/l2 标签，推到后续）
- D23 凭据注入完整实现（MVP 用环境变量）

## Capabilities

### New Capabilities

- `module-registry`: 模块注册引擎——Git clone + terraform-config-inspect 解析 variables.tf → scalar 契约 + 状态机（pending_validation → validated）
- `catalog-publishing`: 服务目录发布——从契约裁剪 form_schema + defaults 注入 + visibility 可见性控制 + D40 校验集成

### Modified Capabilities

（无——不修改已归档 spec 的 requirements。`adapter-interfaces` + `metadata-store` 已归档。）

## Impact

- **代码**：新建 `core/registry/`（service + extractor）+ `core/catalog/`（补 service + formgen + defaults）+ `api/connect/registry.go` + GitProvider 实现。改造 `api/connect/catalog.go`。
- **API**：RegistryAdminService + CatalogService 从占位变为真实实现（proto 契约不变）。
- **依赖**：新增 `github.com/hashicorp/terraform-config-inspect`（HCL 解析）；go-git 已在 go.mod。
- **DB**：不改 schema（用 W1-02 的 modules/module_versions/catalog_items 表）。
- **配置**：GitProvider 凭据用环境变量（GIT_SSH_COMMAND / token），D23 完整实现留 W2。
- **测试**：registry 用 fixture 模块（testdata/）断言契约提取；catalog 断言 form_schema 裁剪 + defaults 注入 + visibility 过滤。
