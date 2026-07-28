## 1. core/tenancy/（task 4.1）

- [ ] 1.1 实现 `server/core/tenancy/service.go`：TenancyService 注入 TeamRepo/ProjectRepo/SpaceRepo，实现 team/project/space CRUD（Create/Get/List/Update/SoftDelete 薄包装 Repo）
- [ ] 1.2 实现 `server/core/tenancy/ownership.go`：资源归属规则（layer→default_owner_team 硬编码映射：global→platform-ops，middleware→dba/middleware 按 component，application→request.team_id）。MVP 用硬编码，Phase 2 走 team_cloud_grants
- [ ] 1.3 实现 `server/core/tenancy/service_test.go`：归属规则判定（给定 layer+component → 预期 owner_team_kind）

## 2. core/stackmodel/pathgenerator（task 4.3）

> D29 layer-first Path Contract。PathGenerator 是 codegen（W2）的身份契约基础。

- [ ] 2.1 实现 `server/core/stackmodel/pathgenerator/generator.go`：PathGenerator 接口 + 实现。输入 StackMeta(layer, tenant, team, space, component, env) → 输出 PathResult(RepoPath, StateKey, StackID, TerramateTags)。从 layer_rule_set_versions.layers_json 读 path_template（Go text/template 渲染）
- [ ] 2.2 实现 `server/core/stackmodel/pathgenerator/generator_test.go`：三层路径渲染断言（global/middleware/application 各一个用例 + space 可选 + stack_id 格式 + tags 格式）

## 3. core/stackmodel/granularity（task 4.4）

- [ ] 3.1 实现 `server/core/stackmodel/granularity/evaluator.go`：StackGranularity 评估器。读 catalog_items.stack_grouping → MVP 只返回 per-component（默认）；Phase 2 扩展 per-space/per-team/custom
- [ ] 3.2 实现 `server/core/stackmodel/granularity/evaluator_test.go`：per-component 默认评估

## 4. core/stackmodel/dependency（task 4.5）

- [ ] 4.1 实现 `server/core/stackmodel/dependency/graph.go`：DependencyGraph 构建（从 stack_dependencies + layer.depends_on 构建 DAG）+ 拓扑排序（Kahn 算法 BFS）+ 无环校验（有环报结构化错误）
- [ ] 4.2 实现 `server/core/stackmodel/dependency/graph_test.go`：拓扑排序（global→middleware→application 顺序）+ 环检测报错

## 5. core/stackmodel/layer（task 4.2）

- [ ] 5.1 实现 `server/core/stackmodel/layer/service.go`：LayerService 注入 LayerLogicalRefRepo/LayerRuleSetVersionRepo。Phase 1 只读（ListLayers + GetActiveRuleSet + GetRuleSetByID），不开放管理员改
- [ ] 5.2 实现 `server/core/stackmodel/layer/service_test.go`：读 seed 数据（3 层 + v1 active）

## 6. wire 装配 + 验证

- [ ] 6.1 实现 `server/core/tenancy/provider.go` + `server/core/stackmodel/provider.go`：wire ProviderSet 注册各 service
- [ ] 6.2 更新 `server/core/core.go`：加 tenancy.ProviderSet + stackmodel.ProviderSet
- [ ] 6.3 `go build ./... && go vet ./...` 通过
- [ ] 6.4 `go test ./server/core/tenancy/... ./server/core/stackmodel/... -short` 通过
- [ ] 6.5 `gofmt -l server/` 无输出
- [ ] 6.6 提交到 `feat/w1-tenancy-layer` 分支，commit message 英文
