## Why

W2 codegen 是 MVP 主干的核心引擎——把"用户申请"变成"可执行的 Terraform 代码"。它是 W1 全部模块的直接消费者：PathGenerator（W1-04）出路径、ContractExtractor（W1-03）出契约、RegistryService（W1-03）出 module source、DependencyGraph（W1-04）出跨层依赖。没有 codegen，W1 的模块都是"空转"——有数据但没有产物。

**影响层级**：业务核心层（`server/core/codegen/`），不改 DB schema / proto 契约。

**为什么现在做**：W1 的 4 个模块（适配器/数据/注册/分层）全部完成，codegen 的输入依赖全部就位。

## What Changes

### 1. core/codegen/ 主入口（task 5.1）

新建 `server/core/codegen/`：
- **Generator**：主入口，输入 CodegenInput → 输出 stack 目录树
- **输入**：表单值（form_values_json）+ 模块契约（variables_contract_json）+ catalog defaults + 依赖图 + PathGenerator 结果
- **输出**：每个 stack 目录含 stack.tm.hcl + main.tf + backend.tf + cross-layer.tf（可选）
- **调用 PathGenerator**：从 layer_rule_set_versions 取 path_template → PathGenerator.Generate → repo_path/state_key/stack_id/tags
- **module source 构造**：git 源 = `git::url//path?ref=commit_sha`；registry 源 = `ns/name/cloud` + version（doc 09 §6.1）

### 2. CardinalityInjector（task 5.2）

- 按 catalog_items.cardinality（single/list/map）在 main.tf 注入：
  - single：直接 `module "x" {...}`
  - list：`count = N`（同质）
  - map：`for_each = tomap({...})`（异构，key = 实例标识）
- per_instance 字段从 each.value 取，shared 字段直接注入
- 模块 variables.tf 全 scalar，零侵入（D25）

### 3. 模板渲染（task 5.1 的 main.tf.tmpl + backend.tf.tmpl + cross-layer.tf.tmpl）

- main.tf.tmpl：module 块 + 参数注入（含 for_each/count）
- backend.tf.tmpl：远程 state 配置（bucket/region/key 从 state_backends 表读）
- cross-layer.tf.tmpl：terraform_remote_state data 块（依赖图驱动）
- stack.tm.hcl.tmpl：Terramate stack 定义（id/tags/after/watch）

### 4. outputs.tf 自动聚合（task 5.4）

- cardinality=map 时生成 `output "x" { value = { for k,m in module.y : k => m.z } }`

### 5. terraform fmt 校验（task 5.5）

- 生成后强制 `terraform fmt` 校验（确定性路径，D19）
- 禁止 local backend（强制远程 state）

### 6. Phase 1 参数管道简化（5 阶段，D19/D28）

Phase 1 用 5 阶段简化管道（可映射到 D28 的 S1-S9）：
1. contract（模块契约兜底）
2. defaults（catalog defaults）
3. governance（平台强制 tag + state key）
4. user（用户表单值）
5. dependency（跨层依赖注入）

### 不做（本次范围外）

- 9 阶段完整管道（Phase 2）
- provenance 审计完整版（Phase 2）
- catalog 项 cardinality CRUD 表单前端（W3 前端）
- batch 批量注册脚本

## Capabilities

### New Capabilities

- `code-generation`: 表单→Terramate stack + Terraform 代码生成引擎。PathGenerator 驱动路径 + 契约驱动参数 + cardinality 驱动多实例 + 依赖图驱动跨层注入。Phase 1 五阶段简化管道。

### Modified Capabilities

（无）

## Impact

- **代码**：新建 `core/codegen/`（generator + cardinality + templates + pipeline）。不改现有代码。
- **API**：不新增 proto RPC（codegen 是编排引擎的内部调用，不是用户直接 API）
- **依赖**：无新外部依赖（text/template 标准库 + W1 的 PathGenerator/ContractExtractor）
- **DB**：不改 schema（读 module_versions/catalog_items/layer_rule_set_versions/state_backends）
- **测试**：golden file 测试（固定输入→固定输出），覆盖 single/list/map + 有/无 space + 三层
