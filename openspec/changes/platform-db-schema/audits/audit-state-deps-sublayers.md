# 深度审计：远程状态存储 + 依赖关系 + 子层级 + 初始化配置

> 三 agent 并行研究：(1) Terramate 源码的远程状态/依赖能力 (2) 业界平台（Spacelift/Env0/TFC/Atlantis）的状态+依赖设计 (3) docs 的分层/子层级/初始化设计。综合审计当前表结构是否合理。

---

## 一、远程状态存储：业界怎么做 vs 我们怎么做

### 业界共识（4 个平台一致）

| 平台 | state JSON 存哪 | 控制面 DB 存什么 |
|---|---|---|
| TFC | 平台自己就是 backend（托管对象存储）| state 版本历史 + **outputs 缓存**（单独存储，不读全量 state） |
| Spacelift | 可托管或自带 S3 | state 版本 + outputs 缓存 + **依赖图（持久化 DAG）** |
| Env0 | 可托管或自带 | state 版本 + **deployment correlation**（state 版本↔部署关联） |
| Atlantis | 永远自带（平台不存） | 几乎不存（只有 project lock） |

**核心共识**：
1. **state JSON 永远不存关系型 DB**（太大、太频繁变更、需要锁）→ 存对象存储（S3/OSS）
2. **控制面 DB 存 metadata + outputs 缓存**（不是 state 全文）→ outputs 单独存，查快、可权限隔离
3. **依赖图**：Spacelift 持久化到 DB（显式配置），TFC/Atlantis 运行时从代码推导（不持久化）

### Terramate 的能力（源码级证据）

- **Terramate 不管理 state**——零 `.tfstate` 读写，零 backend 配置。State 100% 是 Terraform 的责任。
- **跨 stack 输出共享**：通过 `input.from_stack_id` + `sharing_backend.command`（通常 `terraform output -json`）在运行时 shell 出去读上游 outputs。是实验特性（`outputs-sharing` experiment）。
- **不生成 `terraform_remote_state`**——用户手写或 codegen 生成。Terramate 的模型是 `outputs → variable`，不是 `data "terraform_remote_state"`。
- **依赖图**：`engine/dependencies.go` 从 `input.from_stack_id` 提取真正的数据依赖图（可查询传递依赖、检测环）——这是平台可复用的最有价值能力。

### 我们当前的设计

- **D6**：每 stack 独立 state key + 远程 backend（S3/OSS + DynamoDB 锁）+ 禁止 local backend ✅
- **state_key = repo_path**（确定性推导）✅
- **codegen 写 backend.tf**（注入 state key）✅
- **state 锁**：DynamoDB 原生锁 + PG 行级锁（双锁）✅
- **stacks 表存 state_key + repo_path**（metadata，不存 state JSON）✅
- **StateBackend 可插拔适配器**（D7）✅

### 审计结论

**当前设计完全符合业界共识**：
- ✅ state JSON 在对象存储（不在 DB）
- ✅ 控制面 DB 只存 metadata（state_key + repo_path + version pin）
- ✅ 远程 backend + 锁（不自己造锁）
- ✅ 跨层依赖走 `terraform_remote_state`（codegen 生成）

**但缺一个关键东西**：

### ❌ 缺：stack outputs 缓存表

TFC 和 Spacelift 都把 **outputs 单独缓存**到控制面 DB——这样：
1. 查"stack X 的 vpc_id 是什么"不需要下载全量 state JSON（可能几十 MB）
2. 可以做权限隔离（outputs 单独授权，不暴露全量 state）
3. 审批 UI 展示 plan diff 时可以直接读缓存 outputs

当前 `stacks` 表只有 `state_key`（指向 backend 的 key），没有 `outputs_cache_json`。每次需要上游 outputs 都要 shell 出去 `terraform output -json`——这在 Terramate 的 `sharing_backend` 里可行，但平台控制面（如审批 UI 展示"这个工单依赖的 VPC ID"）需要一个更快的路径。

**建议**：非 MVP，但 Wave 2（codegen 跨层依赖落地时）需要加 `stack_outputs` 表或 `stacks.outputs_cache_json` 列。更新策略：apply 成功后 codegen/executor 从 `terraform output -json` 提取 outputs 写入缓存。

---

## 二、依赖关系：DB 层依赖 VPC 子网——怎么存

### 你的问题

> DB 层依赖 VPC 子网。ID 保存在表里，还是表里存储 VPC 的状态以及什么？

### 业界做法

| 平台 | 依赖图存哪 | 依赖类型 |
|---|---|---|
| Spacelift | **持久化到 DB**（显式配置 `spacelift_stack_dependency`）| stack→stack + output→input 映射 |
| TFC | **不持久化**（运行时从 `terraform_remote_state` 代码推导）| data source 引用 |
| Atlantis | **不持久化**（`depends_on` 在 yaml 里每次读）| repo 内排序 |

**最优解（Spacelift 式）**：持久化依赖图到 DB。理由：
1. 可以触发下游 run（上游 apply 成功→自动 plan 下游）
2. 可以画拓扑图
3. 可以阻断删除（"这个 stack 有下游依赖，不能删"）
4. 不需要每次跑 `terraform plan` 才发现依赖

### 我们当前的设计

**两层依赖模型**：

1. **模块级依赖**（`module_dependencies` 表，MVP）：
   - RDS 模块的 `vswitch_id` 变量依赖 Global 层 vpc 模块的 `vswitch_id` output
   - `module_dependencies(module_version_id, variable_name, depends_on_layer, depends_on_module, output_key, required)`
   - 这是**声明性依赖**——注册模块时提取，告诉 codegen"RDS 需要 VPC 的 vswitch_id"

2. **stack 实例级依赖**（`stack_dependencies` 表，B2 非 MVP）：
   - `stack_dependencies(from_stack_id, to_stack_id, kind[remote_state|data_source|watch_only], output_key, inject_as, status)`
   - 这是**运行时依赖**——实际创建的 stack 之间的引用关系

### 审计结论

**当前设计是正确的**——不存 VPC 的 state 或 ID 在依赖表里，而是存**引用关系**（哪个 stack 依赖哪个 stack 的哪个 output）：

```
module_dependencies: "RDS 模块的 vswitch_id 变量 → 依赖 vpc 模块的 vswitch_id output"
                      （声明性，注册时提取）

stack_dependencies:  "rds-orders-prod stack → 依赖 vpc-platform-prod stack 的 vswitch_ids output"
                      （运行时，codegen 根据 module_dependencies + 实际 stack 创建时生成）
```

**VPC 的实际 ID（如 vsw-bp1xxx）存在哪？** → 存在 Terraform state JSON 里（对象存储）。平台 DB 不存资源 ID——资源 ID 是 state 的事实，不是控制面的 metadata。平台只存**引用关系**（哪个 stack 依赖哪个 stack），运行时 codegen 生成 `terraform_remote_state` data source，Terraform 自己从 state 读 ID。

**这跟业界完全一致**：Spacelift 也不存资源 ID，只存 stack→stack 的依赖关系 + output key 映射。

---

## 三、子层级：业务层有子层级对应业务团队

### 你的问题

> 业务层的话是有子层级的哦，子层级对应业务团队

### 当前设计

**当前设计明确不支持子层级**——layers 是扁平有序列表，没有 `parent_id`。业务团队的分组通过 `bundle`（路径分组）和 `owning_team_pattern`（正则匹配）实现，不是层级的嵌套。

docs/02 §1 的设计哲学：
- Application 层 → `owning_team_pattern: business` → 多个业务团队在同一层
- 业务团队内的项目组 → `bundle`（路径分组，不是子层）
- 路径：`application/{tenant}/{team}/{bundle?}/{component}-{env}`

### 这合理吗？

**对照业界**：
- Spacelift：stack 是扁平的，用 space/project 分组，没有子层概念
- TFC：workspace 是扁平的，用 project/organization 分组
- Atlantis：扁平，用目录分

**对照阿里云模块**：
- declarative 层用阶段号分组（01-network → 02-security → 03-compute...），不是嵌套层
- control 层是平铺的 7 个组合模块

**结论**：**扁平层 + 多维度分组（team/bundle/env/tenant）是业界共识**。子层级会增加复杂性（嵌套的 order/depends_on/path_template 解析）而没有实际收益。当前设计**合理**。

但有一个真实需求没覆盖：**"业务A 团队只能看到自己的 catalog 项"**。当前靠 `visibility_json`（team_ids 数组）控制，这是正确的——不靠层级隔离，靠 RBAC + visibility 隔离。

---

## 四、初始化配置（DB/中间件/运维/业务）

### 你的问题

> 自由化配置，以及初始化配置（db 中间件 运维 业务）

### 当前 seed

`010_layers.sql` seed 3 层：
- `global`（运维/platform-ops）
- `middleware`（DBA + 中间件并列）
- `application`（业务团队）

`owning_team_pattern`：
- global → `platform`
- middleware → `dba|middleware`
- application → `business`

### 对照阿里云模块

阿里云 declarative 层的 7 个阶段对应：
- 00-shared（公共）→ 对应 global 的 provider/region 配置
- 01-network → global 层（VPC/VSwitch/NAT/CEN）
- 02-security → global 层（安全组/KMS/WAF）
- 03-compute → application 层（ACK/ECS）
- 04-database → middleware 层（RDS/Redis/Kafka）
- 05-middleware → middleware 层（MSE/RocketMQ/ES）
- 06-web → application 层（ALB/SLB/NLB + ECS）
- 07-bigdata → application 层（EMR/Hologres）

**映射关系**：阿里云的 7 个阶段可以映射到我们的 3 层——但**不是 1:1**。阿里云把 database 和 middleware 分成不同阶段，但在我们的模型里它们同属 Middleware 层（DBA 和中间件团队并列）。

### 审计结论

**当前 3 层 seed 合理**——覆盖了运维/DB/中间件/业务 4 个角色（DB 和中间件合并到 middleware 层，靠 owning_team_pattern 正则区分）。如果未来需要更细粒度（如拆 `data` 和 `messaging` 为独立层），D24 的可配置机制支持加层——改 `layer_rule_set_versions` bump v2 即可。

---

## 五、control 层自动生成 vs declarative 层表单提交

### 你的思考

> control 是我们平台的依赖关系自动生成的。declarative 相当于前端表单或者 webhook 的 yaml 提交

### 对照阿里云模块

完全吻合：
- **control 层**（7 个组合模块）：编排逻辑（`for_each` 调 atomic）→ **平台 codegen 自动生成**（根据 module_compositions + module_dependencies 生成 main.tf）
- **declarative 层**（tfvars）：用户填的参数 → **等价于平台的前端表单**（`form_values_json`）或 **webhook yaml 提交**（`cicd_triggers`）

### 表结构支撑

| 概念 | 阿里云对应 | DB 表 | 状态 |
|---|---|---|---|
| atomic 模块注册 | atomic/ecs, atomic/rds | `modules(module_type='atomic')` | ✅ |
| control 模块（自动生成） | control/web-cluster | `modules(module_type='control')` | ✅（字段已加） |
| control→atomic 组合关系 | web-cluster 调 15 个 atomic | `module_compositions`（**缺，Wave 2**）| ❌ RISK |
| declarative 表单 | tfvars | `requests.form_values_json` + `catalog_items.form_schema_json` | ✅ |
| declarative yaml | webhook 提交 | `cicd_triggers`（B13）| ✅ |
| 跨层依赖 | terraform_remote_state | `stack_dependencies`（B2）| ✅ |
| 模块级依赖 | RDS 的 vswitch_id ← VPC | `module_dependencies` | ✅ |

---

## 六、发现汇总

### 需要修复的

| # | 问题 | 严重度 | 修复 | 时机 |
|---|---|---|---|---|
| **S1** | 缺 stack outputs 缓存 | RISK | Wave 2 加 `stack_outputs` 表或 `stacks.outputs_cache_json`（apply 后从 `terraform output -json` 提取写入）| Wave 2 |
| **S2** | 缺 `module_compositions` 表 | RISK | Wave 2 加（control→atomic 组合关系，codegen 生成 control main.tf 的输入）| Wave 2 |

### 确认合理的（不需要改）

| 设计决策 | 业界对照 | 结论 |
|---|---|---|
| state JSON 不存 DB，存对象存储 | TFC/Spacelift/Env0/Atlantis 全一致 | ✅ 正确 |
| DB 只存 state metadata（state_key + repo_path）| 全一致 | ✅ 正确 |
| 依赖图持久化到 DB（stack_dependencies）| Spacelift 式（最优解）| ✅ 正确 |
| 依赖关系存引用（不存资源 ID）| Spacelift 一致 | ✅ 正确 |
| 扁平层 + 多维度分组（不嵌套子层）| Spacelift/TFC/Atlantis 全扁平 | ✅ 正确 |
| 3 层 seed（运维/DB+中间件/业务）| 覆盖 4 角色，DB+中间件合并合理 | ✅ 正确 |
| control 自动生成 + declarative 表单提交 | 与阿里云三层架构完全对应 | ✅ 正确 |
| owning_team_pattern 正则匹配（N:M）| Spacelift 用 space + RBAC，我们用 pattern | ✅ 合理（但 Wave 2 可考虑加 `layer_team_bindings` 关系表替代正则） |

### 设计方向确认

当前表结构设计**在状态存储、依赖关系、层级模型三个维度上都符合业界最优实践**。不需要推翻重设计。Wave 2 需要补 `stack_outputs` 缓存 + `module_compositions` 组合关系表——这是增量演进，不是架构调整。
