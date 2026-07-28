## 1. Generator 主入口（task 5.1）

- [ ] 1.1 实现 `server/core/codegen/generator.go`：Generator 结构体 + Generate 方法。输入 CodegenInput（StackMeta + 契约 + defaults + form_values + 依赖 + backend 配置）→ 输出 FileSet（map[string][]byte）。调用 PathGenerator 渲染路径。调用模板渲染各文件
- [ ] 1.2 实现 module source 构造（buildModuleSource）：git 源 = `git::url//path?ref=sha`；registry 源 = `ns/name/cloud` + version（doc 09 §6.1）。MVP 只 git

## 2. 参数管道（task 5.1 + Phase 1 五阶段）

- [ ] 2.1 实现 `server/core/codegen/pipeline.go`：5 阶段合并（contract→defaults→governance→user→dependency）。输入各来源 → 输出 map[string]any（最终参数值）。优先级：governance > user > dependency > defaults > contract
- [ ] 2.2 实现 HCL 值渲染（hclRender）：按类型输出 HCL 语法（string 加引号、number/bool 原样、list/map HCL 语法）

## 3. CardinalityInjector（task 5.2）

- [ ] 3.1 实现 `server/core/codegen/cardinality.go`：cardinality 评估（single/list/map）。准备模板数据（.Cardinality + .Instances + .InstanceKey + .Vars）
- [ ] 3.2 MVP 支持 single + map（list/count 推迟）。map 的 instances 从 form_values 的 per_instance_fields 构造

## 4. 模板文件（task 5.1）

- [ ] 4.1 创建 `server/core/codegen/templates/main.tf.tmpl`：module 块 + source + version + for_each/count 条件 + 参数注入（hclRender）
- [ ] 4.2 创建 `server/core/codegen/templates/backend.tf.tmpl`：terraform backend 块（bucket/region/key 从 state_backends + PathGenerator.StateKey）
- [ ] 4.3 创建 `server/core/codegen/templates/stack.tm.hcl.tmpl`：Terramate stack 定义（stack_id + tags + after/watch 可选）
- [ ] 4.4 创建 `server/core/codegen/templates/cross-layer.tf.tmpl`：terraform_remote_state data 块（依赖图驱动，{{range .Dependencies}}）
- [ ] 4.5 创建 `server/core/codegen/templates/outputs.tf.tmpl`：outputs 自动聚合（cardinality=map 时 for 循环聚合）

## 5. outputs.tf 自动聚合（task 5.4）

- [ ] 5.1 实现 outputs 生成逻辑：从 module_versions 契约的 outputs 列表 → 生成 outputs.tf。single = 直接 output；map = `output { value = { for k,m in module.y : k => m.z } }`

## 6. wire + 验证

- [ ] 6.1 实现 `server/core/codegen/provider.go`：wire ProviderSet（Generator 构造，注入 PathGenerator）
- [ ] 6.2 更新 `server/core/core.go`：加 codegen.ProviderSet
- [ ] 6.3 `go build ./... && go vet ./...` 通过
- [ ] 6.4 `go test ./server/core/codegen/... -short` 通过（golden file 测试）
- [ ] 6.5 `gofmt -l server/` 无输出
- [ ] 6.6 提交到 `feat/w2-codegen` 分支

## 7. golden file 测试（task 5.6）

- [ ] 7.1 创建 testdata/golden/rds-middleware-single/：single cardinality 的 RDS middleware stack 完整文件集（main.tf + backend.tf + stack.tm.hcl + cross-layer.tf + outputs.tf）
- [ ] 7.2 创建 testdata/golden/ecs-application-map/：map cardinality 的 ECS application stack（for_each 多实例）
- [ ] 7.3 创建 testdata/golden/vpc-global-single/：global 层 VPC（无 cross-layer）
- [ ] 7.4 实现 `server/core/codegen/generator_test.go`：固定输入 → 对比 golden file（路径 + 内容完全一致）
