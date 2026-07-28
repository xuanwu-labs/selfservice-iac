## Context

W2 codegen 是 MVP 主干核心——把用户申请变成 Terraform 代码。

**现有架构约束**（W1 已落地）：
- PathGenerator（W1-04）：StackMeta → PathResult（repo_path/state_key/stack_id/tags）
- ContractExtractor（W1-03）：variables_contract_json（纯 scalar 契约）
- RegistryService（W1-03）：module source（git_source + module_path + commit_sha）
- DependencyGraph（W1-04）：TopoSort（跨层依赖排序）
- layer_rule_set_versions（migration 010 seed v1）：path_template
- state_backends（migration 011）：bucket/region
- catalog_items：defaults_json + cardinality + form_schema_json
- doc 09：模板示例 + 5 阶段简化管道 + module source 构造规则

**Phase 1 简化策略**（doc 00b）：5 阶段管道（contract→defaults→governance→user→dependency），可映射到 D28 的 S1-S9。

## Goals / Non-Goals

**Goals:**
- Generator 主入口：CodegenInput → stack 目录树（文件集）
- main.tf 生成（module 块 + 参数 + cardinality 注入）
- backend.tf 生成（远程 state）
- stack.tm.hcl 生成（Terramate stack 定义）
- cross-layer.tf 生成（跨层 remote_state）
- Phase 1 五阶段参数管道
- golden file 测试

**Non-Goals:**
- 不做 9 阶段完整管道 + provenance（Phase 2）
- 不做 catalog cardinality 前端表单（W3）
- 不做 terraform fmt 真实执行（W2 编排模块调 Executor；codegen 只生成文本）
- 不改 DB schema / proto

## Decisions

### D1：Generator 输出 FileSet（内存文件集，不直接写磁盘）

**决策**：Generator 返回 `map[string][]byte`（路径→内容），不直接写文件系统。

```go
type FileSet map[string][]byte // "middleware/platform-default/rds-prod/main.tf" → 内容
```

**理由**：
- 解耦生成与持久化（workspace manager W2 task 7 负责 git add/commit）
- 可测试（golden file 对比 FileSet，不需临时目录）
- 确定性（同输入→同 FileSet，D19）

### D2：模板用 Go text/template（不从代码拼接 HCL）

**决策**：main.tf/backend.tf/stack.tm.hcl 用 `text/template` 模板文件，不拼接字符串。

**理由**：doc 09 §6 已定义模板格式；模板可审计（git 可见）；确定性。

**模板位置**：`core/codegen/templates/*.tmpl`

### D3：参数管道 5 阶段（Phase 1 简化）

**决策**：Phase 1 实现 5 阶段管道（doc 00b）：

```
Stage 1 contract:  模块契约兜底（variables_contract_json 的 default）
Stage 2 defaults:  catalog defaults（catalog_items.defaults_json）
Stage 3 governance: 平台强制（tag + state_key + ownership）
Stage 4 user:      用户表单（form_values_json）
Stage 5 dependency: 跨层依赖（remote_state data source 变量绑定）
```

**优先级**：governance(3) > user(4) > dependency(5) > defaults(2) > contract(1)。治理永远赢。

**输出**：`map[string]any`（参数名→最终值），传给 main.tf 模板渲染。

### D4：CardinalityInjector 在模板层处理（不在 Go 代码分支）

**决策**：cardinality 注入用模板条件（`{{if eq .Cardinality "list"}}`），不在 Go 代码里 if/else。

**理由**：模板可审计；single/list/map 只是 main.tf 结构差异，不是业务逻辑差异。

### D5：module source 构造（git vs registry，doc 09 §6.1）

**决策**：按 modules.source_type（MVP 默认 git）构造 source URL：
- git：`git::{{git_source}}//{{module_path}}?ref={{commit_sha}}`
- registry（Phase 2）：`{{registry_source}}` + `version = "{{version}}"`

### D6：backend.tf 从 state_backends 表读（不硬编码 bucket）

**决策**：backend.tf 的 bucket/region 从 state_backends 表读（doc 09 §6 修订）。

```hcl
terraform {
  backend "s3" {
    bucket = "{{.Backend.Bucket}}"  ← state_backends.bucket
    region = "{{.Backend.Region}}"  ← state_backends.region
    key    = "{{.StateKey}}"        ← PathGenerator.StateKey
  }
}
```

## Risks / Trade-offs

- **[模板灵活性 vs 确定性] → text/template 固定模板**：模板不可配（Phase 2 才开放），但保证确定性（D19）。
- **[cardinality 复杂度] → MVP 只 single + map**：list（count）推迟（map for_each 已覆盖多实例场景）。
- **[参数管道 5 阶段 vs 9 阶段] → Phase 1 可映射**：字段名/source 名与 D28 S1-S9 兼容，Phase 2 无缝升级。
- **[terraform fmt] → codegen 只生成文本，fmt 由 Executor 执行**：codegen 输出的 HCL 可能格式不完美，但语义正确。Executor（W2 task 6）调 `terraform fmt` 校正。

## Migration Plan

1. 实现 core/codegen/generator.go（主入口 + FileSet）
2. 实现 core/codegen/pipeline.go（5 阶段参数管道）
3. 实现 core/codegen/cardinality.go（cardinality 评估 + 注入数据准备）
4. 创建模板文件（main.tf.tmpl + backend.tf.tmpl + stack.tm.hcl.tmpl + cross-layer.tf.tmpl）
5. 测试（golden file）
6. `go build ./... && go vet ./... && go test ./server/core/codegen/...`

## Open Questions

- **cross-layer.tf 的 remote_state key 从哪取？** 从 stacks.state_key（上游 stack 的 PathGenerator 输出）。MVP 需要查 DB 获取上游 stack 的 state_key。
- **outputs.tf 生成规则？** MVP 从 module_versions.outputs 契约自动生成（Phase 1 只 single + map 聚合）。
