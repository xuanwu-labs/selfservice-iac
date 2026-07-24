## ADDED Requirements

### Requirement: 服务目录发布（CatalogService + FormSchema + Defaults + Visibility）

平台 MUST 在 `server/core/catalog/` 补全 CatalogService（现有 validator.go D40 不变）。CatalogService 注入 CatalogRepo（W1-02）+ ModuleVersionRepo，实现 PublishCatalogItem：从 module_version 的 variables_contract_json 裁剪 form_schema_json + 注入 defaults_json + D40 校验 + 写入 visibility_json → CatalogRepo.Publish 落 DB。

#### Scenario: 发布一个 catalog_item（自动生成 form_schema）

- **WHEN** 调用 PublishCatalogItem(module_version_id=100, display_name="RDS MySQL", layer_logical_id="middleware", owner_team_id=1, visibility=["dba"])
- **THEN** FormSchemaGenerator 从 variables_contract_json 裁剪出 form_schema_json（隐藏 sensitive + 平台可推断字段）
- **AND** DefaultsApplier 注入 defaults_json（最佳实践默认值）
- **AND** D40 Validator.ValidateSchema 校验 form_schema_json 合法（Draft 2020-12）
- **AND** CatalogRepo.Publish 写入 catalog_items（含 form_schema + defaults + visibility）
- **AND** 返回 catalog_item_id

#### Scenario: 可见性控制写入 visibility_json

- **WHEN** publish 时 visibility=["team-a","team-b"]
- **THEN** catalog_items.visibility_json = ["team-a","team-b"]（仅这些团队可见）
- **AND** 空数组 visibility_json=[] 表示全局可见

### Requirement: FormSchemaGenerator 从契约裁剪（非全暴露）

平台 MUST 实现 FormSchemaGenerator 从 `variables_contract_json` 裁剪出 `form_schema_json`。裁剪规则：隐藏 sensitive=true（占位符，运行期注入）+ 平台可推断字段（region 从 env 推断）；暴露 required=true 且平台不可推断（用户必填）+ required=false 且有 default（用户可改）。输出 JSON Schema Draft 2020-12（D40 校验器兼容）。

#### Scenario: 隐藏 sensitive 字段

- **WHEN** 契约含 `{name:"password", sensitive:true}`
- **THEN** form_schema_json 的 properties 不含 password（隐藏，运行期注入）

#### Scenario: 暴露 required 用户必填字段

- **WHEN** 契约含 `{name:"instance_type", required:true}`（无 default，平台不可推断）
- **THEN** form_schema_json 的 properties 含 instance_type
- **AND** form_schema_json.required 数组含 "instance_type"

#### Scenario: 暴露可选字段（带 default）

- **WHEN** 契约含 `{name:"storage_size", default:100, required:false}`
- **THEN** form_schema_json 的 properties 含 storage_size（带 default:100）
- **AND** form_schema_json.required 数组不含 "storage_size"

### Requirement: DefaultsApplier 注入最佳实践默认值

平台 MUST 实现 DefaultsApplier 维护 `defaults_json`（最佳实践默认值覆盖）。MVP 可硬编码典型默认值表（如 RDS instance_type 默认 rds.mysql.s2.large），后续可配置化。defaults_json MUST 与 form_schema_json 的 properties 对齐（key 一致）。

#### Scenario: 注入 RDS 默认值

- **WHEN** 发布 RDS MySQL catalog_item
- **THEN** defaults_json 含 `{"instance_type":"rds.mysql.s2.large","storage_size":100}` 等最佳实践默认值
- **AND** 这些默认值出现在 form_schema_json 的 properties.default（用户表单预填）

### Requirement: D40 校验集成（catalog publish 前校验 form_schema）

平台 MUST 在 CatalogService.Publish 调用 D40 Validator.ValidateSchema 校验 form_schema_json（自动生成或手动覆盖）合法。校验失败 MUST 返回结构化错误，阻止 publish。

#### Scenario: 自动生成的 form_schema 通过 D40 校验

- **WHEN** FormSchemaGenerator 生成 form_schema_json 后调用 D40 Validator
- **THEN** 校验通过（Draft 2020-12 合法 schema）
- **AND** publish 继续

#### Scenario: 手动覆盖的非法 form_schema 被拦截

- **WHEN** publish 时手动传入的 form_schema_json 违反 Draft 2020-12（如 properties 是 string 而非 object）
- **THEN** D40 Validator 返回结构化错误
- **AND** publish 被拦截，返回错误给调用方
