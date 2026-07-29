## 1. 测试 fixture（null/random provider）

- [ ] 1.1 创建 `test-fixtures/atomic-null/variables.tf`：含 scalar required（instance_name）+ scalar default（ttl=300）+ sensitive（secret_key）+ platform-inferred（vswitch_id）
- [ ] 1.2 创建 `test-fixtures/atomic-null/main.tf`：null_resource + random_id（Terraform 内置 provider，无需下载）
- [ ] 1.3 创建 `test-fixtures/atomic-null/outputs.tf`：random_id hex + null_resource id
- [ ] 1.4 创建 `test-fixtures/atomic-null/versions.tf`：required_providers null + random + required_version >= 1.4

## 2. E2E 测试（Go test）

- [ ] 2.1 创建 `server/e2e/helpers.go`：E2E 测试辅助——初始化临时 git 仓库（Terramate 需要 git 上下文）+ 检查 terramate/terraform CLI 可用 + -short skip
- [ ] 2.2 实现 `TestE2E_RegisterAndPublish`：注册 atomic-null 模块（ContractExtractor）→ 发布 catalog_item（FormSchemaGenerator）→ 断言契约 + form_schema 正确
- [ ] 2.3 实现 `TestE2E_CodegenAndCommit`：codegen.Generate（FileSet 含 main.tf/backend.tf/stack.tm.hcl）→ workspace.WriteFiles → 断言 commit SHA 返回 + 文件存在
- [ ] 2.4 实现 `TestE2E_PlanAndApply`：TerramateAdapter.Run（terramate run -- terraform plan -detailed-exitcode）→ exit code 验证 → apply → 断言 terraform.tfstate 文件存在 + null_resource 在 state 中
- [ ] 2.5 实现 `TestE2E_FullLifecycle`：完整链路串联——注册→发布→工单→codegen→plan→审批→apply→state。使用 local backend + null provider

## 3. walking skeleton 脚本

- [ ] 3.1 创建 `scripts/walking-skeleton/run.sh`：shell 脚本引导完整流程（初始化环境→注册模块→发布→建工单→plan→apply→验证 state）。用于 demo/演示

## 4. 验证

- [ ] 4.1 `go build ./... && go vet ./...` 通过
- [ ] 4.2 `go test ./server/e2e/... -short` 通过（无 terramate/terraform 时 skip）
- [ ] 4.3 `go test ./server/e2e/... -v`（有 terramate + terraform 时全跑）
- [ ] 4.4 `gofmt -l server/` 无输出
- [ ] 4.5 提交到 `feat/w5-e2e-validation` 分支
