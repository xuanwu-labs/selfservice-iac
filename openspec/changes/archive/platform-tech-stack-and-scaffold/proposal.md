# platform-tech-stack-and-scaffold

> 配套 change：`iac-self-service-platform`（D1–D30 大方向决策）。本 change 落地"用什么库实现 + 目录骨架 + 工程契约"，不重复大方向决策。

## Why

`iac-self-service-platform` 的 30 个关键决策（D1–D30）定了**做什么、为什么、架构怎么分层**，但没有定**用什么 Go 库实现**。这留下了一组在实现期会反复争论、且一旦写深就难改的选型悬案：

- 数据访问层用 **GORM / sqlc / ent / sqlx** 中的哪个？这直接决定 `server/core/store` 的设计形态，且影响 D28 的 provenance 审计是否可追溯（ORM 的魔法 SQL 与"每行审计可追溯到一行被 PR review 的 SQL"互相矛盾）。
- CLI 框架用 **cobra / kong / urfave/cli** 中的哪个？这决定 `cmd/aether` 的骨架形态。Terramate 本身用 kong，但平台 CLI 是独立工具，不必强对齐。
- 迁移工具用 **golang-migrate / goose / atlas**？这决定 `db/migrations` 的格式与版本历史粒度（审计场景要求完整版本历史）。
- 代码规范遵循 **Google Go Style Guide** 还是社区惯用法？ECC 的 `golang-patterns` skill 不引用谷歌规范，需要一个独立的工程契约固化。

这些选型**必须在写第一行代码前冻结**，否则 209 个 task 的实现会在不一致的栈上反复返工。本 change 把这些选型固化成可追溯的决策（D31–D45，续 D1–D30 编号），并落地最小可编译的目录骨架 + 工程契约 + lint 配置，使后续 `/opsx:apply iac-self-service-platform` 有一个确定的地基。

## What Changes

- **固化技术栈决策（D31–D45）**：数据访问层 sqlc+pgx、CLI cobra、迁移 goose、HTTP 路由 gin（webhook/运维）、RPC 框架 Connect-RPC（业务 RPC，proto 唯一真相源）、日志 zap 原生 Logger、代码规范 Google Go Style Guide、配置 viper 多源合并、依赖注入 wire 编译期 codegen、任务队列 river、JSON Schema santhosh-tekuri、可观测 OpenTelemetry、测试 testcontainers+testify、安全 argon2id+jwt+oidc、时间 Clock 接口 —— 每项给出决策/理由/备选/影响，续 D1–D30 编号，跨引用原决策。
- **落地 `server/` 目录骨架**（领域核心顶层分层，package-by-feature）：建 `cmd/{server,aether,migrate}/`、`api/{http,grpc}/`（传输 handler；proto 源在仓库根 `contracts/`）、`core/`（领域核心顶层）、`data/`（pgxpool + dbset）、`internal/{config,errors,...}/`（只装私有基建，不套微服务封装）、`pkg/db/{queries,generated}/`（sqlc）、`cmd/migrate/migrations/`（goose）、`Makefile`、`go.mod`。详见 `docs/01-目录骨架设计.md`。
- **工程契约 `server/AGENTS.md`**：固化 Google Go Style Guide 引用、D1 边界（禁止 import terramate 内部包）、错误处理约定、包命名约定。
- **golangci-lint 配置 `.golangci.yml`**：启用 gofmt/goimports/vet/staticcheck + 自定义 depguard 规则禁止 `server/**` import `github.com/terramate-io/terramate/<protected>` 内部包。
- **sqlc 配置 `sqlc.yaml`**：engine postgresql、sql_package pgx/v5、emit_json_tags、emit_empty_slices —— 锁定 SQL 即真相源。
- **cobra CLI 骨架 `cmd/aether/`**：rootCmd + 9 个子命令骨架（catalog/request/stack/drift/approval/cost/gate/mcp/ai）+ version 注入。
- **goose 迁移骨架 `db/migrations/`**：`001_init.up/down.sql`（teams 表，对齐 docs/04）+ `cmd/migrate/main.go`（`//go:embed` + Up/Down，advisory lock）。

## Capabilities

### New Capabilities

<!-- 平台是全新层；以下每项对应一个 specs/NN-中文名.md -->

- `platform-scaffold`（→ `specs/01-平台脚手架.md`）：最小可编译的目录骨架 + Makefile + go.mod + 一个端到端可跑的"build → test → migrate Up/Down"链路。
- `tech-stack-decisions`（→ `specs/02-技术栈决策.md`）：15 个技术栈决策（D31–D45）固化，每项可追溯。

### Modified Capabilities

<!-- 无：本 change 纯新增，不修改 iac-self-service-platform 的任何 capability -->

（无。本 change 不触碰 `iac-self-service-platform` 的 specs/docs/tasks；它是后者的"实现地基层石"。）

## Impact

**影响层级**：
- **平台层（新增）**：本 change 全部影响落在 `server/` 子树，是新增层。
- **Terramate CLI / LSP / HCL / config / stack / generate / cloud**：均不触碰。本 change 不 import 也不修改 Terramate 任何内部包（D1 边界由 `.golangci.yml` 的 depguard 强制）。

**兼容性**：不破坏任何已有 Terramate 配置或 CLI 用法。本 change 产出的 `server/` 是独立 Go module（`server/go.mod`），与 Terramate 主 module 物理隔离；CI lint 规则禁止反向依赖。

**HCL 语法 / config schema 变更**：**无**。本 change 不涉及 HCL 或 config schema。

**新增依赖**（`server/go.mod`，均不污染 Terramate 的 go.mod）：
- `github.com/jackc/pgx/v5`（PostgreSQL 驱动）
- `github.com/pressly/goose/v3`（迁移）
- `github.com/spf13/cobra`（CLI）
- `github.com/gin-gonic/gin`（HTTP 路由）+ `github.com/go-playground/validator/v10`（gin 自带校验）
- `github.com/spf13/viper`（配置多源合并 + 热更新）
- `go.uber.org/zap`（结构化日志，原生 Logger API）
- `github.com/Masterminds/squirrel`（动态查询，仅按需）
- `github.com/google/wire`（依赖注入，编译期 codegen）
- `connectrpc.com/connect` + buf（RPC 框架，proto 唯一真相源，业务 RPC 走 gRPC/gRPC-Web/Connect-JSON）
- `github.com/riverqueue/river`（任务队列，PostgreSQL-backed）
- `github.com/santhosh-tekuri/jsonschema/v6`（JSON Schema 校验）
- `go.opentelemetry.io/otel` + `github.com/prometheus/client_golang`（可观测）+ `github.com/exaring/otelpgx`（pgx trace）+ `github.com/uptrace/opentelemetry-go-extra/otelzap`（zap↔trace 桥接）+ `connectrpc.com/otelconnect`（Connect trace）+ `go.opentelemetry.io/contrib/instrumentation/...`（otelgin/otelhttp/otelgrpc）
- `github.com/stretchr/testify`（测试断言）+ `github.com/testcontainers/testcontainers-go` + pgtestdb（测试 DB）
- `go.uber.org/mock`（测试 mock，按需）
- `golang.org/x/crypto/argon2`（密码哈希）+ `github.com/golang-jwt/jwt/v5`（JWT）+ `github.com/zitadel/oidc`（OIDC）
- `github.com/go-git/go-git/v5`（Git 操作，D4，pin 补丁版）
- `github.com/google/uuid`（request ID）
- `golang.org/x/time/rate`（限流）+ `golang.org/x/oauth2`（HTTP 客户端 OIDC）
- `github.com/rs/cors`（CORS 中间件）
- 工具链：`sqlc`（codegen，CI 调用）、`golangci-lint`（lint）

**复用 Terramate 的方式**：本 change 不直接复用 Terramate 代码。后续 `iac-self-service-platform` 的 D1（TerramateAdapter 通过 exec 调用 terramate CLI）在实现期才落地；本 change 只在 `.golangci.yml` 预置"禁止 import terramate 内部包"的规则以守护 D1。

**关键风险**：
1. **sqlc 的动态查询短板**：运行时 WHERE/filter 构造需配 squirrel，增加一处心智负担。缓解：仅在有真实动态查询需求的包（如 catalog 列表过滤）引入，其余全部静态 SQL。
2. **cobra 与 Terramate 的 kong 不一致**：贡献者跨两个 CLI 框架工作有认知成本。缓解：平台 CLI 是独立工具，与 Terramate 解耦；cobra 生态更大、自动补全更好，长期收益大于一致性成本。
3. **Go 版本对齐**：Terramate `go.mod` 声明 `go 1.25`，本 change 需对齐或更高，避免贡献者切仓时 Go toolchain 混乱。
