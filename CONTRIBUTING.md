# 贡献指南

感谢你对 Aether 的兴趣!本项目使用 **OpenSpec** 规范驱动开发,所有变更(新功能、重构、修复)都需走 propose → apply → archive 流程。

## 开发环境要求

| 工具 | 版本 | 说明 |
|---|---|---|
| Go | **1.25+** | 见 `server/go.mod`(对齐 Terramate 上游) |
| sqlc | 最新 | SQL → Go 代码生成 |
| wire | v0.6+ | 编译时依赖注入 |
| protoc / buf | 最新 | proto 契约生成(Connect-RPC) |
| golangci-lint | v2.x | 代码检查 |
| Docker | 20+ | 集成测试(testcontainers 起临时 PG 容器;本地或远程均可) |

## 快速上手

```bash
git clone <repo-url>
cd selfservice-iac/server

# 验证环境
go build ./...      # 编译全部
make test           # 短测试(无需 Docker,跳过 DB 测试)
make lint           # 代码检查
```

## 测试(testcontainers)

DB 相关测试用 [testcontainers-go](https://golang.testcontainers.org/) 起独立的 PG 容器,
**不依赖任何开发数据库**——每个测试拿到全新、隔离、已迁移的 DB。

- `make test`(默认)只跑短测试,**无需 Docker**。
- `make test-db` 跑全量测试(含 DB 集成测试),**需要 Docker**。

### 本地 Docker

如果 Docker 装在本机,直接:

```bash
make test-db
```

### 远程 Docker(无本地 Docker 时)

testcontainers-go 的 Go client 只认 `tcp://` 和 `unix://`,**不支持 `ssh://`**。
若 Docker daemon 在远程机器上且只暴露了 unix socket,用一个 socat 代理容器把
socket 转成 TCP(testcontainers 经此 TCP 连远程 Docker):

```bash
# 在远程机器上执行一次(持久,--restart unless-stopped):
docker run -d --name docker-api-proxy --restart unless-stopped \
  -p 23750:2375 -v /var/run/docker.sock:/var/run/docker.sock \
  alpine/socat -d -d TCP-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

# 在本地:
export DOCKER_HOST=tcp://<remote-host>:23750
export TESTCONTAINERS_RYUK_DISABLED=true   # 若远程拉不到 ryuk 镜像
make test-db
```

> **已知环境坑(CentOS 7 + Docker 26 + Enforcing SELinux / 严格 seccomp)**:
> PG 16 容器的 initdb 会因 seccomp 拦截报 `Operation not permitted`。
> `pkg/db/testdb.go` 已为容器加 `seccomp=unconfined` 解决。若你遇到类似错误,
> 确认 testdb.go 里的 `WithHostConfigModifier` 未被移除。


## 变更流程(OpenSpec)

**不要直接提交代码。** 所有变更先创建 change:

1. **探索**:`/opsx:explore [话题]` — 想法调研、方案对比
2. **提案**:`/opsx:propose [name]` — 生成 proposal/design/tasks/specs
3. **实现**:`/opsx:apply [name]` — 按 tasks 产出代码
4. **同步**:`/opsx:sync [name]` — delta specs 并入主线
5. **归档**:`/opsx:archive [name]` — change 完成

> **协作边界(重要)**:步骤 2(propose)、3(apply)、4(sync)、5(archive)这四个**改变 change 状态的动作,必须由项目维护者显式发起**。AI agent 协作时,不得擅自新建 change、不得擅自归档、不得擅自把 change 标记完成或重新定义验收范围。agent 可以起草内容、写代码、跑验证、报告状态,但推进生命周期需维护者指令。详见 [`AGENTS.md`](AGENTS.md) "协作边界"段与 [`openspec/config.yaml`](openspec/config.yaml) rules.lifecycle-ownership。

## 目录约定(必读)

Aether server 采用 **领域核心顶层分层**(`core/` 为一等公民,`internal/` 只装私有基建)。贡献者须遵守以下边界:

| 目录 | 放什么 | 不放什么 |
|---|---|---|
| `core/<domain>/` | 领域业务逻辑(package-by-feature) | ❌ 基建、❌ handler |
| `api/{http,grpc}/` | 传输层 handler | ❌ 业务逻辑 |
| `data/` | pgxpool provider + dbset 表分组 | ❌ SQL 源(在 pkg/db) |
| `pkg/db/` | sqlc 输入(`queries/`)与输出(`generated/`) | ❌ 手写查询逻辑 |
| `internal/` | 私有基建(config/otel/errors/cli/cmdutil/...) | ❌ 业务逻辑 |

**数据访问路径**（混合范式：ferret Repo struct × DIP 可演进 × sqlc SQL-as-truth）：`core/<domain>/`（业务层，通过 wire 注入 Repo struct）→ `data/repo/`（Repo struct 薄包装 `*generated.Queries` + 跨表事务 WithTx + 动态查询 query_wrapper）→ `pkg/db/generated`（sqlc `*Queries`）→ `data/`（pgxpool 池）。core 不直接 import `pkg/db`（DIP 依赖方向正确）；需要测试/mock 时在 core 提取小 interface（Go 隐式 interface，无需改 data 层）。

## D1 边界(关键架构守护)

**禁止 `server/**` import `github.com/terramate-io/terramate/<任何子包>`。**

平台通过 `exec` 调用 terramate CLI,不直接 import 其内部包。此规则由 `.golangci.yml` 的 depguard **模块前缀 deny** 强制,CI 会拒绝违规 PR。

## 代码规范

遵循 [Google Go Style Guide](https://google.github.io/styleguide/go/):

- 包名短小写单词(`registry`、`catalog`);导出标识符驼峰(`CatalogService`)
- 错误:`fmt.Errorf("context: %w", err)` 包装;定义 sentinel error;handler 层 `errors.Is/As`
- 接口:消费侧定义小接口(1-3 方法),不预定义大接口
- 测试:表驱动 + `t.Run` + testify assert/require(不用 mock);`t.Helper`/`t.Cleanup`/`t.TempDir`
- Context:跨边界函数首参 `ctx context.Context`

## 常用命令

```bash
make build          # 编译所有二进制
make test           # 短测试(无需 Docker,跳过 DB 集成测试)
make test-db        # 全量测试(含 DB 集成测试,需 Docker)
make lint           # golangci-lint
make migrate-up     # 应用 DB 迁移
make sqlc-gen       # 生成 sqlc 代码(在 pkg/db/ 下执行)
make wire-gen       # 生成 wire 装配代码
make proto-gen      # 生成 proto/Connect-RPC 代码
```

## 提交规范

- 一个 change 对应一个 PR(或多个聚焦 PR)
- commit message 中文/英文均可,建议:`<scope>: <描述>`
- PR 必须通过 `make build` + `make test` + `make lint`(CI 自动检查)
- 涉及架构决策的改动,需先更新 `openspec/` 对应 design.md

## 问题与讨论

- Bug / 功能需求:先 `/opsx:explore` 或开 issue 讨论,再 propose
- 架构问题:参考 `openspec/changes/iac-self-service-platform/design.md`(D1–D30 决策)
