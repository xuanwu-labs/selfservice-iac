# 模块注册表深度评估：变量契约 + 依赖 + 注册黄金路径

> 对照 terraform-alicloud-modules/atomic/rds 的真实变量契约 + proto RegisterModuleRequest + DB 表结构，评估模块注册表设计是否遵循 TF 变量的黄金路径原则。

## 真实 TF 模块变量结构（以 RDS 为例）

RDS 模块有 30+ 个变量，分 5 类：
1. **控制开关**（create）——平台不暴露
2. **必填依赖**（vswitch_id, security_group_ids）——来自上游 stack output
3. **业务参数**（engine, instance_type, storage...）——用户表单填
4. **敏感变量**（account_password, sensitive=true）——不进 git，运行期注入
5. **复杂对象**（serverless_config=object, databases=list(object)）——嵌套结构

每个 TF 变量有 5 个属性：`type / description / default / sensitive / validation`

## 当前 DB 表结构评估

### ✅ 已支撑

| 能力 | 表/字段 | 说明 |
|---|---|---|
| 模块基本信息 | modules(name, git_source, module_path, provider) | 对齐 proto RegisterModuleRequest |
| 三层架构 | modules.module_type(atomic/control/declarative) | 区分原子/编排/声明 |
| 版本管理 | module_versions(version, commit_sha, is_current) | 版本 + commit pin |
| 变量契约存储 | module_versions.variables_contract_json | JSONB 可存任意结构 |
| 输出契约存储 | module_versions.outputs_contract_json | 支撑跨层依赖 output_key 校验 |
| Provider 要求 | module_versions.required_providers_json | e.g. ["alicloud@1.280.0"] |
| 跨层依赖 | module_dependencies(variable_name, depends_on_layer, depends_on_module, output_key, required) | 声明性依赖 |
| 模块状态 | modules.status(pending_validation/validated/validation_failed/deprecated) | 生命周期 |

### ⚠️ 需规范（不改表结构，规范 JSON 结构）

**variables_contract_json 的结构规范**：当前是自由 JSONB，没有约束结构。应该规范为每个变量含完整元数据：

```json
{
  "vswitch_id": {
    "type": "string",
    "description": "RDS 所在交换机 ID",
    "default": null,
    "required": true,
    "sensitive": false
  },
  "account_password": {
    "type": "string",
    "description": "RDS 账号密码",
    "default": "",
    "required": false,
    "sensitive": true
  },
  "databases": {
    "type": "list(object)",
    "description": "数据库列表",
    "default": [],
    "required": false,
    "sensitive": false,
    "schema": [{"name": "string", "character_set": "string", "description": "string"}]
  }
}
```

**关键**：`sensitive` 字段必须提取——codegen 用它判断哪些变量用占位符（`__TM_SECRET_*__`）运行期注入，不进 git。当前 variables_contract_json 如果不存 sensitive 标记，codegen 无法区分敏感变量。

**outputs_contract_json 的结构规范**：
```json
{
  "instance_id": {"type": "string", "description": "RDS 实例 ID"},
  "connection_string": {"type": "string", "description": "内网连接地址"},
  "database_names": {"type": "list(string)", "description": "数据库名称列表"}
}
```

### ❌ proto 缺字段（需 proto change）

`RegisterModuleRequest` 缺 `module_type` + `owner_team_id`——注册时无法指定模块类型和负责团队。

**改法**：
```protobuf
message RegisterModuleRequest {
  string git_source = 1;
  string module_path = 2;
  string version = 3;
  string provider = 4;
  string name = 5;
  string description = 6;
  // 新增 ↓
  string module_type = 7;    // atomic/control/declarative
  string owner_team_id = 8;  // 负责团队 ID
}
```

## 注册黄金路径

```
管理员调 RegisterModule(git_source, module_path="atomic/rds", version, provider, name, module_type="atomic", owner_team_id)
  ↓
平台 git clone + checkout commit_sha
  ↓
平台解析 versions.tf → required_providers_json (["alicloud@1.280.0"])
  ↓
平台解析 variables.tf → variables_contract_json（含 type/description/default/sensitive）
  ↓
平台解析 outputs.tf → outputs_contract_json（含 type/description）
  ↓
管理员手动声明 module_dependencies（MVP 不自动推断）
  或平台从 description 推断（"通过 module.vswitch 引用" → 依赖 vpc 模块）
  ↓
INSERT modules(status='validated') + module_versions(is_current=true) + module_dependencies
  ↓
后续：管理员发布 catalog_item（绑 layer + form_schema + visibility）
  → codegen 消费 variables_contract_json 生成表单
  → codegen 消费 module_dependencies 生成 terraform_remote_state
  → codegen 用 sensitive 标记判断哪些变量用占位符
```

## 结论

DB 表结构**支撑注册黄金路径**。需规范的：
1. **variables_contract_json 结构**——必须含 `sensitive` 字段（codegen 区分敏感变量）
2. **outputs_contract_json 结构**——含 `type` + `description`（跨层依赖校验）
3. **proto RegisterModuleRequest 加 module_type + owner_team_id**（proto change）

这些不改表结构（JSONB 能存），是 handler 实现时的规范 + proto 补字段。
