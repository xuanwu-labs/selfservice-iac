# e2e-validation Specification

## Purpose
TBD - created by archiving change w5-e2e-validation. Update Purpose after archive.
## Requirements
### Requirement: E2E 端到端测试（null/random provider + local backend）

平台 MUST 在 `server/e2e/` 实现端到端测试，使用 null_resource + random_id provider + local backend（零云凭证，业界最佳实践参考 Atlantis + Terramate）。验证完整链路：模块注册 → catalog 发布 → 工单提交 → codegen → git commit → terramate plan → 审批 → apply → state 写入。

#### Scenario: 模块注册 + 契约提取

- **WHEN** 用 ContractExtractor 解析 test-fixtures/atomic-null/variables.tf
- **THEN** variables_contract_json 含 instance_name(required) + ttl(default=300) + secret_key(sensitive) + vswitch_id(platform-inferred)
- **AND** providers 含 null + random

#### Scenario: catalog 发布 + form_schema 裁剪

- **WHEN** FormSchemaGenerator 从契约裁剪
- **THEN** form_schema_json 暴露 instance_name + ttl
- **AND** 隐藏 secret_key（sensitive）+ vswitch_id（platform-inferred）

#### Scenario: codegen 生成 + git commit

- **WHEN** codegen.Generate 输出 FileSet
- **THEN** 含 main.tf（null_resource + random_id module 块）
- **AND** 含 backend.tf（local backend）
- **AND** workspace.WriteFiles 返回 commit SHA

#### Scenario: terramate plan + apply

- **WHEN** TerramateAdapter.Run 执行 terramate run -- terraform plan
- **THEN** exit code = 0（无漂移，首次 plan）
- **AND** terramate run -- terraform apply exit code = 0
- **AND** terraform.tfstate 文件存在 + 含 null_resource.demo

#### Scenario: 完整生命周期串联

- **WHEN** 运行 TestE2E_FullLifecycle
- **THEN** 注册→发布→工单→codegen→plan→审批→apply→state 全链路通过
- **AND** 无云凭证、无 Docker 依赖（仅 terramate + terraform CLI）

### Requirement: 测试 fixture（atomic-null 模块）

平台 MUST 创建 `test-fixtures/atomic-null/` 测试模块，使用 null_resource + random_id（Terraform 内置 provider）。variables.tf 含 4 种字段类型：scalar required / scalar default / sensitive / platform-inferred。无需任何云凭证或 provider 下载。

#### Scenario: atomic-null 模块可被 ContractExtractor 解析

- **WHEN** tfconfig.LoadModule(test-fixtures/atomic-null/)
- **THEN** 返回 4+ 个变量 + 2+ 个输出 + 2 个 provider（null + random）

