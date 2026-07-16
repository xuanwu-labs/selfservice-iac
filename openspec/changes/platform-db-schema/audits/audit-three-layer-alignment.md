# 深度审计：三层架构（atomic/control/declarative）↔ 表结构支撑度

> 读完 `terraform-alicloud-modules` 真实三层架构（62 atomic + 7 control + declarative）+ docs 分层/依赖设计 + 当前 DB 表结构后，逐项审计表结构是否支撑未来。

## 真实三层架构（terraform-alicloud-modules）

```
declarative/     声明层（用户表单/yaml 提交，含 provider + state）
  └── simple/    7 个阶段：00-shared → 01-network → 02-security → 03-compute → 04-database → 05-middleware → 06-web → 07-bigdata
control/         控制层（编排逻辑，调 atomic，平台自动生成）
  ├── network-topology    调 vpc/vswitch/route_table/nat/eip/vpn/cen/cen_attachment（8 个 atomic）
  ├── security            调 security_group/kms/waf/...
  ├── web-cluster         调 ecs/alb/slb/nlb + listeners/rules/server_groups（15 个 atomic）
  ├── database-cluster    调 rds/polardb/redis/kafka/es/mongodb/...（13 个 atomic）
  ├── middleware-cluster  调 kafka/mse/rocketmq/es/...
  ├── ack-cluster         调 ack/ack-node-pool/acr/...
  └── big-data-cluster    调 emr/hologres/flink/...
atomic/          原子层（62 个单资源封装，零 provider 零 state）
```

### 关键发现

1. **atomic 层**：62 个原子模块，每个是单资源封装（ecs/rds/vpc/...），有 variables.tf（输入契约）+ outputs.tf（输出供下游引用）
2. **control 层**：7 个组合模块，每个调 N 个 atomic（如 web-cluster 调 15 个），**编排逻辑在这里——平台自动生成**
3. **declarative 层**：用户填 tfvars（map 驱动），**等价于平台的前端表单或 webhook yaml 提交**
4. **跨层依赖**：declarative 层用 `terraform_remote_state` 引用上游层的 outputs（如 03-compute 引用 01-network 的 vswitch_ids_map）
5. **map 驱动**：control 层用 `for_each` 遍历 declarative 传的 map（如 `vswitches = { "j-ecs" = {...} }`）

## 逐项审计

### A. 自定义层级是否支撑

**需求**：管理员可自定义层（默认 3 层 Global/Middleware/Application，可加 security/data 层）

**当前表结构**：
- `layer_logical_refs(logical_id PK, current_display_name, notes)` — 层定义
- `layer_rule_set_versions(version_id PK, layers_json, status, is_default)` — 层规则集版本化

**审计**：
- ✅ `layer_logical_refs` 支撑任意层名（TEXT PK，不硬编码）
- ✅ `layers_json` 存完整层配置（name/order/path_template/owning_team_pattern/depends_on）
- ✅ 版本化（D26 v1→v2）支撑层规则演进
- ⚠️ **但 layers_json 是 JSONB，不是关系表**——如果要做"列出所有层的所有配置"查询，JSONB 内部结构不可索引。MVP 够用（3 层 seed），但 Wave 2 层管理 UI 上线后可能需要拆成 `layer_rules` 关系表（每行一层配置）

**结论**：MVP ✅ 支撑；Wave 2 层管理 UI 时可能需要拆 JSONB → 关系表。

### B. 团队权限管理是否支撑

**需求**：
- 哪些团队管哪些层（Global→platform-ops, Middleware→dba|middleware, Application→business）
- catalog 项对哪些团队可见（visibility）
- 团队在哪些层可申请资源（team_cloud_grants.allowed_layers）

**当前表结构**：
- `teams(kind CHECK(platform|dba|middleware|business))` — 团队类型
- `layer_rule_set_versions.layers_json.owning_team_pattern` — 层↔团队匹配（字符串/正则）
- `catalog_items.visibility_json` — 团队可见性（team_ids 数组）
- `team_cloud_grants.allowed_layers_json`（B10 非 MVP）— 团队层授权

**审计**：
- ⚠️ **断裂 B（已知）**：`teams.kind` 枚举与 `owning_team_pattern` 字符串无结构化映射。如果加 `security` 层，owning_team_pattern 写 `security`，但 teams.kind 枚举没有 `security` 值——**teams.kind 应该去掉 CHECK 约束或改为可扩展**
- ⚠️ **断裂 C（已知）**：`team_cloud_grants.allowed_layers_json` 用硬编码层名 `[Global|Middleware|Application]`，应改 `allowed_layer_logical_ids_json`（存 layer_logical_id）
- ✅ `catalog_items.visibility_json` + GIN 索引支撑"按团队过滤目录"
- ❌ **缺：catalog_items 到 layer 的绑定**——catalog 项有 `layer_logical_id`（资源属哪层），但**没有"这个 catalog 项对哪些层的团队开放"的约束**。当前设计是 visibility_json 控制团队可见性，layer_logical_id 控制资源归属——但这两者交叉（"DBA 团队只能看到 Middleware 层的 catalog 项"）需要应用层 JOIN 逻辑，DB 无约束

**建议修复**：
1. `teams.kind` 去掉 CHECK（改为 TEXT 无约束，或加 `security` 等值）——因为层可自定义，kind 枚举也应可扩展
2. catalog_items 的层↔团队绑定靠应用层（visibility_json + layer_logical_id 交叉过滤），DB 不加约束——这是正确的（绑定规则在应用层灵活组合）

### C. catalog 如何映射到层 + 团队

**需求**：catalog 项绑定到某个层（如 RDS 模块属 Middleware 层），对该层的团队开放

**当前表结构**：
- `catalog_items.layer_logical_id FK→layer_logical_refs` — catalog 项属哪层
- `catalog_items.visibility_json` — 对哪些团队可见
- `catalog_items.owner_team_id FK→teams` — 谁负责这个 catalog 项

**审计**：
- ✅ `layer_logical_id` 绑定 catalog 项到层
- ✅ `visibility_json` 控制团队可见性
- ✅ `owner_team_id` 标记负责人
- ⚠️ **但缺一个约束**：catalog 项的 `layer_logical_id` 应该跟模块的层一致——`modules.layer` 字段存在但与 `catalog_items.layer_logical_id` 冗余且无校验（断裂 D，已知）
- ❌ **真实三层架构的映射问题**：在 terraform-alicloud-modules 里，atomic 模块没有"层"属性（vpc/ecs/rds 都是平铺的），"层"是 control 层的组合决定的（network-topology 调 vpc/vswitch/nat → 这些原子模块属于 Global 层）。**但平台 DB 里 `modules.layer` 给每个原子模块标了层**——这与真实架构不符。真实的层归属应该在 **catalog_items 层面**（发布目录项时决定"这个 RDS 服务属于 Middleware 层"），而不是模块层面

**建议修复**：
- `modules.layer` 改为 nullable + 注释"信息性字段，权威层归属看 catalog_items.layer_logical_id"
- 或直接删 `modules.layer`（让 catalog_items 单独决定层归属）

### D. 原子模块注册——是否支撑 aliyun 模块的结构

**需求**：注册 atomic 层的 62 个模块，提取变量契约 + 输出契约 + 跨层依赖

**当前表结构**：
- `modules(name, git_source, module_path, provider, layer, owner_team_id, status)`
- `module_versions(version, commit_sha, providers_json, variables_contract_json, is_current)`
- `module_dependencies(module_version_id, variable_name, depends_on_layer, depends_on_module, output_key, required)`

**审计**：
- ✅ `module_path` 支撑子目录（atomic/ecs, atomic/rds）
- ✅ `variables_contract_json` 存变量契约（S1 管道输入）
- ✅ `module_dependencies` 存跨层依赖（variable_name + depends_on_layer + depends_on_module + output_key）
- ❌ **缺：outputs 契约**——`module_versions` 只有 `variables_contract_json`（输入），**没有 `outputs_contract_json`（输出）**。真实模块的 outputs.tf 是跨层依赖的关键（RDS 输出 connection_string，下游 Application 层引用）。`module_dependencies.output_key` 引用的是上游模块的 output，但上游模块的 output 契约无处存储——**无法校验 output_key 是否存在**
- ❌ **缺：control 层模块的注册**——当前 modules 表设计只考虑 atomic 层模块注册。但 control 层（network-topology/web-cluster/database-cluster）也是模块，也需要注册——它们调 N 个 atomic 模块，有自己的 variables（编排参数）和 outputs（聚合输出）。**modules 表应该能注册三层所有模块**，加一个 `module_type` 字段（atomic/control/declarative）
- ⚠️ `providers_json` 字段名歧义——在 aliyun 模块里 atomic 层没有 provider 配置（provider 在 declarative 层），这个字段存的是什么？应该是 `required_providers_json`（如 `alicloud`/`aws`/`null`/`random`）

**建议修复**：
1. `module_versions` 加 `outputs_contract_json`（存 outputs.tf 提取的输出契约）
2. `modules` 加 `module_type TEXT CHECK(module_type IN ('atomic','control','declarative'))`
3. `module_versions.providers_json` 改名 `required_providers_json`（语义更准）

### E. 依赖关系上下游 + status

**需求**：
- atomic 模块间的依赖（RDS 依赖 VPC 的 vswitch_id）
- control 层调 atomic 的关系（web-cluster 调 ecs/alb/slb）
- stack 实例间的依赖（Application stack 引用 Middleware stack 的 RDS connection_string）
- 模块/版本的 status 生命周期

**当前表结构**：
- `module_dependencies(module_version_id, variable_name, depends_on_layer, depends_on_module, output_key, required)` — 模块级依赖
- `stack_dependencies(from_stack_id, to_stack_id, kind, output_key, inject_as, status)` — stack 级依赖（B2 非 MVP）
- `modules.status CHECK(pending_validation|validated|validation_failed|deprecated)` — 模块状态
- `module_versions.is_current` — 当前版本标记

**审计**：
- ✅ `module_dependencies` 支撑模块级跨层依赖（RDS 的 `vswitch_id` 变量依赖 Global 层 vpc 模块的 `vswitch_id` output）
- ✅ `stack_dependencies` 支撑 stack 实例级依赖（remote_state/data_source/watch_only）
- ❌ **缺：control→atomic 的调用关系**——control 层模块调 N 个 atomic 模块（如 web-cluster 调 15 个），这个"组合关系"无处存储。`module_dependencies` 是"变量级依赖"（RDS 依赖 VPC 的 output），不是"组合级调用"（web-cluster 包含 ecs+alb+slb）。需要加 `module_compositions` 表（control_module_id → atomic_module_id + count/map 配置）
- ⚠️ `modules.status` 只有 4 值——真实模块注册流程可能需要更多状态（如 `draft` 草稿、`testing` 测试中）。但 MVP 够用
- ⚠️ `module_versions.is_current` 是单布尔——如果一个模块有多个 active 版本（如 v1.0 和 v2.0 并行），is_current 只能标一个。应该用 `module_versions.status` 替代（active/superseded）

### F. control 层自动生成 vs declarative 层表单提交

**需求（用户明确提到）**：
- control 层是平台**自动生成**的依赖关系编排
- declarative 层等价于**前端表单或 webhook yaml 提交**

**当前表结构支撑**：
- control 层自动生成 → codegen 生成 control 模块的 main.tf（调 atomic 模块 + for_each）。DB 侧需要 `module_compositions` 表存"control 模块调了哪些 atomic 模块"——**当前缺失**
- declarative 层表单提交 → `requests.form_values_json` 存用户填的表单值（等价于 tfvars），`catalog_items.form_schema_json` 定义表单结构。✅ 支撑
- declarative 层 webhook yaml → `cicd_triggers` 表（B13 非 MVP）存 yaml 提交。✅ 支撑

**审计**：
- ✅ declarative 层（表单/yaml → requests.form_values_json + cicd_triggers）有表支撑
- ❌ control 层自动生成的"组合关系"（control 模块调哪些 atomic）无表支撑——需要新增 `module_compositions` 表（非 MVP，Wave 2 codegen 时落）

## 发现汇总

### BLOCKER（影响 MVP 或近 Wave）

| # | 问题 | 修复 | 优先级 |
|---|---|---|---|
| **F1** | `module_versions` 缺 `outputs_contract_json` | 加列——跨层依赖校验需要（module_dependencies.output_key 引用上游 output，但 output 契约无处存） | MVP（模块注册时就要提取） |
| **F2** | `modules` 缺 `module_type`（atomic/control/declarative） | 加列——区分三层模块，control 层注册需要 | MVP（atomic 先注册，control Wave 2） |

### RISK（Wave 2 前修）

| # | 问题 | 修复 | 优先级 |
|---|---|---|---|
| **R1** | `modules.layer` 与 `catalog_items.layer_logical_id` 冗余 | modules.layer 改 nullable 或删除 | Wave 2 |
| **R2** | `teams.kind` CHECK 限制可扩展性 | 去掉 CHECK 或加更多值 | Wave 2（自定义层时） |
| **R3** | `module_versions.providers_json` 命名歧义 | 改名 `required_providers_json` | Wave 2 |
| **R4** | `module_versions.is_current` 单布尔不够 | 改 `status` 列（active/superseded） | Wave 2 |
| **R5** | 缺 `module_compositions` 表（control→atomic 组合关系） | 新增表（非 MVP，Wave 2 codegen 时落） | Wave 2 |
| **R6** | `layers_json` JSONB 可能需拆关系表 | Wave 2 层管理 UI 时评估 | Wave 2 |

### OK（已支撑）

- 自定义层级（layer_logical_refs + layer_rule_set_versions）✅
- catalog↔层绑定（catalog_items.layer_logical_id）✅
- catalog↔团队可见性（visibility_json + GIN）✅
- declarative 层表单（form_values_json + form_schema_json）✅
- declarative 层 yaml（cicd_triggers B13）✅
- 模块级跨层依赖（module_dependencies）✅
- stack 级依赖（stack_dependencies B2）✅
- 版本管理（module_versions）✅
- 状态机（modules.status 4 值）✅ MVP 够用
