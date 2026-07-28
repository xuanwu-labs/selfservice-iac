## 1. 依赖 + GitProvider 真实实现

- [x] 1.1 在 `server/go.mod` 添加 `github.com/hashicorp/terraform-config-inspect` 依赖（HCL 解析）
- [x] 1.2 实现 `server/core/adapters/git/gogit.go`：GitProvider 的 go-git 真实实现（Clone git_source+ref → 临时目录 + Fetch + CommitSHA）。MVP 凭据用环境变量（GIT_SSH_COMMAND / token），D23 完整注入留 W2
- [x] 1.3 实现 `server/core/adapters/git/gogit_test.go`：用本地 fixture repo 测 Clone + CommitSHA（不依赖外部网络，用 testdata 创建临时 git repo）

## 2. ContractExtractor（HCL → scalar 契约，D25 零侵入）

- [x] 2.1 实现 `server/core/registry/extractor.go`：ContractExtractor 用 tfconfig.LoadDir(path) 解析 `.tf` 文件，提取 variables + outputs → `[]VariableContract`（纯 scalar：name/type/default/description/sensitive/required）
- [x] 2.2 实现 scalar 过滤逻辑：default 是复杂类型（list/map/object）时置 nil（保留 type 声明），符合 D25 原子模块单实例语义
- [x] 2.3 实现 `server/core/registry/extractor_test.go`：用 testdata fixture 模块（2-3 个典型 tf：RDS/ECS/SLB）断言契约提取正确（scalar 保留、complex default 置 nil、sensitive 标记）

## 3. RegistryService（task 3.1 + 3.2）

- [x] 3.1 实现 `server/core/registry/service.go`：RegistryService 注入 ModuleRepo + GitProvider，实现 RegisterModule 流程（clone → extract → CreateWithVersion 事务 → 状态机）
- [x] 3.2 实现状态机：`pending_validation`（CreateWithVersion 时初始）→ `validated`（HCL 解析成功，UpdateStatus）/ `validation_failed`（解析失败，记录错误）
- [x] 3.3 实现 `server/core/registry/service_test.go`：用 fake GitProvider（返回 testdata 路径）+ ModuleRepo 测注册流程（契约落 DB + 状态机转换）

## 4. CatalogService 补全（task 3.3）

> 现有 `core/catalog/validator.go`（D40）已实现。本组补 publish/formgen/defaults/visibility。

- [x] 4.1 实现 `server/core/catalog/formgen.go`：FormSchemaGenerator 从 `variables_contract_json` 裁剪出 `form_schema_json`（隐藏 sensitive + 平台可推断字段，暴露用户必填 + 可改字段）。输出 Draft 2020-12 JSON Schema（D40 校验器兼容）
- [x] 4.2 实现 `server/core/catalog/defaults.go`：DefaultsApplier 维护 `defaults_json`（最佳实践默认值覆盖，如 instance_type 默认 rds.mysql.s2.large）。MVP 硬编码典型默认值表，后续可配置化
- [x] 4.3 实现 `server/core/catalog/service.go`：CatalogService 注入 CatalogRepo + ModuleVersionRepo，实现 PublishCatalogItem（formgen 裁剪 + defaults 注入 + D40 validator 校验 + visibility_json 写入 + CatalogRepo.Publish）
- [x] 4.4 实现 `server/core/catalog/service_test.go`：用 ModuleVersion fixture 测 publish 流程（form_schema 自动裁剪 + defaults 注入 + visibility 过滤 + D40 校验通过）

## 5. api/connect handler 接 DB

- [x] 5.1 新建 `server/api/connect/registry.go`：RegistryAdminServiceHandler 实现（RegisterModule/ListModules/GetModule/DeprecateModule → 调 RegistryService）。注入 RegistryService + TeamRepo（owner_team_id 校验）
- [x] 5.2 改造 `server/api/connect/catalog.go`：CatalogServiceHandler 从占位改为实现（ListCatalogItems/GetCatalogItem → 调 CatalogService）。注入 CatalogService
- [x] 5.3 在 `server/api/connect/provider.go` 注册 RegistryHandler + CatalogHandler 到 wire ProviderSet（注入 RegistryService + CatalogService）

## 6. testdata fixture

- [x] 6.1 创建 `server/core/registry/testdata/rds-mysql/`：典型 alicloud RDS 模块 fixture（variables.tf 含 scalar + complex + sensitive 变量；outputs.tf 含 scalar 输出）
- [x] 6.2 创建 `server/core/registry/testdata/ecs-cluster/`：ECS 模块 fixture（含 for_each 友好变量）
- [x] 6.3 创建 `server/core/registry/testdata/minimal/`：最小可用模块（仅 1 个 required string 变量，无 outputs）

## 7. 验证与提交

- [x] 7.1 `go build ./... && go vet ./...` 通过
- [x] 7.2 `go test ./server/core/registry/... ./server/core/catalog/... ./server/core/adapters/git/... -short` 通过（fixture 驱动，不需 Docker）
- [x] 7.3 `gofmt -l server/` 无输出
- [x] 7.4 提交到 `feat/w1-registry-catalog` 分支，commit message 英文
