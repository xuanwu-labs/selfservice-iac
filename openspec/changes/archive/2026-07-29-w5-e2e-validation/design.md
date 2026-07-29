## Context

W5 是 MVP 闭环验证——证明 W1+W2+W3 后端能端到端跑通。

**调研结论**（2026-07-29，参考 Atlantis + Terramate 官方测试）：
- **null/random provider + local backend** 是业界标准 IaC e2e 测试方案
- Atlantis 自己的 e2e fixture 明确规定"只用 null_resource/random/local_file"
- Terramate 自己的仓库也用 `_test_mock.tf`（null provider）
- 零云凭证、零容器依赖、CI 友好

**分层策略**：
| Tier | 验证什么 | 工具 | 依赖 |
|------|---------|------|------|
| Unit | codegen 输出 | golden file diff | 无（已有） |
| Static | TF 语法 | terraform validate + fmt | terraform CLI |
| **E2E** | **完整链路** | **null provider + local backend** | **terramate + terraform CLI** |

## Goals / Non-Goals

**Goals:**
- null/random 原子模块 fixture
- E2E 测试（注册→发布→工单→codegen→plan→审批→apply→state）
- walking skeleton 脚本

**Non-Goals:**
- 前端实现（后续 change）
- 真实云测试（Phase 2）
- LocalStack/MinIO（不需要）
- 多 stack 跨层依赖（Phase 2）

## Decisions

### D1：null_resource + random_id 替代真实云模块

**决策**：用 `null_resource` + `random_id` 创建测试原子模块。

```hcl
# test-fixtures/atomic-null/main.tf
resource "random_id" "this" { byte_length = 8 }
resource "null_resource" "demo" {
  triggers = { value = var.instance_name }
}
```

**理由**：
- null/random 是 Terraform 内置 provider（不需要下载）
- 完整支持 plan/apply/destroy/state 生命周期
- Atlantis + Terramate 官方都用这个方案
- 零云凭证

### D2：local backend 替代 S3 backend

**决策**：E2E 测试用 `backend "local"`（terraform.tfstate 写本地磁盘）。

```hcl
terraform { backend "local" { path = "terraform.tfstate" } }
```

**理由**：
- 不需要 S3/OSS/MinIO
- Terramate 是 backend 无关的（只 shim terraform）
- state 写入断言 = 检查 terraform.tfstate 文件存在

### D3：E2E 测试用 Go test + 真实组件（不 mock 后端）

**决策**：E2E 测试用真实的 Go 组件（codegen.Generator + workspace.Manager + TerramateAdapter），不 mock。

**理由**：
- E2E 的意义是"验证真实组件能串通"
- Unit test 已经 mock 测过各组件
- 需要 terramate + terraform CLI 安装（CI 预装，-short 跳过本地无 CLI 的环境）

### D4：E2E 测试需要 terramate + terraform CLI

**决策**：E2E 测试标记 `testing.Short() skip`——本地无 terramate/terraform 时跳过，CI 跑全量。

**理由**：
- 和 DB 测试一样（Docker 不可用时 skip）
- CI 环境预装 terramate + terraform

## Risks / Trade-offs

- **[null provider 不测试真实云逻辑] → 测试的是编排逻辑**：Aether 的核心价值是编排（codegen→plan→审批→apply），不是云 API。null provider 完整覆盖编排链路。
- **[terramate CLI 版本依赖] → CI pin 版本**：CI 环境安装固定版本 terramate + terraform。
- **[local backend 不测试 S3 lock] → Phase 2 补**：MinIO smoke test 单独跑。

## Migration Plan

1. 创建 test-fixtures/atomic-null/ 模块
2. 写 E2E 测试（8 个用例）
3. 写 walking skeleton 脚本
4. `go test ./server/e2e/... -v`（需要 terramate + terraform）

## Open Questions

- **terramate 是否需要 git 仓库上下文？** 是的——Terramate 依赖 git diff 做 change detection。E2E 测试需要在临时 git 仓库里运行。
