# design.md — platform-tech-stack-and-scaffold

> 配套 change：`iac-self-service-platform`。本 design 仅记录**技术栈选型决策（D31–D45）**与**目录骨架方案**，不重复大方向架构（见原 change `design.md §03`、`docs/01-总体架构.md`）。详细目录树与选型论证分别拆到 `docs/01-目录骨架设计.md`、`docs/02-技术栈选型论证.md`。

## 01-背景与现状

`iac-self-service-platform` 已完成 4/4 规格 artifacts（proposal/design/specs/tasks），含 30 个大方向决策（D1–D30）、22 份能力规格、23 份设计深潜、209 个实现 task。但其 `design.md §04` 的决策是**架构层**的（做什么、为什么、怎么分层），未涉及**用什么 Go 库实现**。

进入编码前（即 `tasks.md ## 00c-Phase 0 Contract Freeze` 与 `## 01-平台骨架` 之前），必须冻结一组实现级选型。否则 209 个 task 会在不一致的栈上反复返工，且这些选型一旦写深就难改：

| 选型悬案 | 影响范围 | 写深后回退成本 |
|---|---|---|
| DB 访问层 | `core/store` 全部 + D28 provenance 审计 | 极高（每个 query 重写）|
| CLI 框架 | `cmd/aether` 全部子命令 | 中（骨架可换，但生态特性难补）|
| 迁移工具 | `db/migrations` 全部 + 审计版本历史 | 高（迁移文件格式不可逆）|
| HTTP 路由 | `api/http` 全部中间件 | 中 |
| 日志 | 全局 | 低 |
| 代码规范 | `AGENTS.md` + `.golangci.yml` | 低（但规范缺失导致代码风格漂移）|

参考实证：`multica-ai/multica` 是一个成熟的 Go + PostgreSQL 平台（agent 管理平台），其 `server/` 子树用 **sqlc + pgx + goose + cobra + chi + log/slog** —— 与本平台的技术需求高度对口（都是平台、都有多租户、都用 PostgreSQL、都需要审计）。本 change 在此基础上做适配调整。

## 02-目标与非目标

### 目标

1. 固化 15 个实现级技术栈决策（D31–D45），每项含决策/理由/备选/影响，可追溯。
2. 落地最小可编译的 `server/` 目录骨架（go.mod + cmd + internal + pkg + db + Makefile）。
3. 落地工程契约（`server/AGENTS.md`）与 lint 配置（`.golangci.yml`，含 D1 边界守护）。
4. 落地 sqlc 配置 + 一个示例 query、goose 迁移骨架 + 一个示例迁移、cobra CLI 骨架 + 9 子命令、gin 路由骨架 + 健康检查 —— 每项配契约测试。
5. `make build && make test` 全绿，证明骨架可用。

### 非目标

- ❌ 不实现任何业务功能（catalog/审批/编排等留给 `iac-self-service-platform` 的 task）。
- ❌ 不批量建 18 张表的迁移（只建 teams 表作为迁移示例）。
- ❌ 不写前端。
- ❌ 不重复 D1–D30 的架构决策。
- ❌ 不修改 `iac-self-service-platform` 任何文件。

## 03-总体方案

### 3.1 技术栈矩阵

| 层 | 选定（D-id）| 备选（拒绝理由）| 关键依赖 |
|---|---|---|---|
| DB 驱动 | **pgx/v5**（D31）| lib/pq（已停维护）| `github.com/jackc/pgx/v5` |
| Query 层 | **sqlc**（D31）| GORM（反射+审计不透明）、ent（学习曲线+迁移耦合）| sqlc 工具 + `github.com/Masterminds/squirrel`（仅按需）|
| 迁移 | **goose**（`//go:embed` + 完整历史 + advisory lock）（D33）| Atlas（声明式但依赖最重）、golang-migrate（只记最后版本）、tern（PG 专用）| `github.com/pressly/goose/v3` |
| CLI | **cobra**（D32）| kong（虽 Terramate 用，但平台是独立工具）、urfave/cli | `github.com/spf13/cobra` + `pflag` |
| HTTP 路由 | **gin**（D34）| chi（更轻量但中间件生态小）、echo、stdlib net/http | `github.com/gin-gonic/gin` |
| 日志 | **zap 原生 Logger**（D35）| slog（stdlib 零依赖但性能低）、zerolog（生态小）、SugaredLogger（printf 反射）| `go.uber.org/zap` |
| 代码规范 | **Google Go Style Guide + golangci-lint**（D36）| ECC golang-patterns（不引用谷歌规范）| `.golangci.yml` + `AGENTS.md` |
| 配置 | **viper**（多源合并 + 热更新）（D37）| env+struct（零依赖但无文件源）、koanf（轻量替代）、caarlos0/env（纯 env）| `github.com/spf13/viper` |
| 依赖注入 | **google/wire**（编译期 codegen）（D38）| 手动（冗长）、fx/dig（运行时反射反审计）、samber/do（采用度小）| `github.com/google/wire` |
| 任务队列 | **river**（D39）| asynq（Redis，失原子性）、machinery（老旧）、PGMQ（无 worker）| `github.com/riverqueue/river` |
| 事件通知 | **直接调用处理函数**（Phase 1 单进程）（D39）| EventBus 接口/outbox（过度设计）、NATS/Kafka（拆服务才需要）| 无（函数调用 + River 重试）|
| JSON Schema | **santhosh-tekuri/jsonschema/v6**（D40）| gojsonschema（废弃在 Draft-07）| `github.com/santhosh-tekuri/jsonschema/v6` |
| 可观测 | **OpenTelemetry + otelzap 注入 trace_id**（D41）| 仅 zap 无 trace（失全链路关联）| `go.opentelemetry.io/otel` + `prometheus/client_golang` + `otelzap` |
| 测试 DB | **testcontainers-go + pgtestdb**（D42）| SQLite（方言不符）、dockertest（API 较旧）| `github.com/testcontainers/testcontainers-go` + `pgtestdb` |
| 安全 | **argon2id + golang-jwt/v5 + zitadel/oidc**（D43）| bcrypt（次选）、coreos/go-oidc（已放缓）| `golang.org/x/crypto/argon2` + `golang-jwt/jwt/v5` + `github.com/zitadel/oidc` |
| 时间 | **Clock 接口注入**（D44）| benbjohnson/clock（次选）、直接 time.Now（不可测）| 自写 6 行 interface |
| RPC 框架 | **Connect-RPC**（D45）| grpc-go+Envoy（重+200-on-error 陷阱）、grpc-gateway（5 插件+无 bidi）、手写两套（违背 proto 唯一真相源）| `connectrpc.com/connect` + buf |
| 测试断言 | **testify assert/require**（D36）| 纯 stdlib（冗长）| `github.com/stretchr/testify` |
| 测试 mock | **手写 fake + uber-go/mock**（D36）| testify/mock（耦合高）| `go.uber.org/mock` |
| 校验 | **go-playground/validator/v10**（gin 自带）| — | gin binding 内置 |
| HTTP 客户端 | **stdlib net/http + x/oauth2** | resty（引入依赖）| stdlib |
| Git | **go-git/v5**（pin 补丁版）| shell out（CVE 同样存在）| `github.com/go-git/go-git/v5` |
| 限流 | **x/time/rate**（MVP）| ulule/limiter（多实例再上）| `golang.org/x/time/rate` |
| UUID | **google/uuid** | — | `github.com/google/uuid` |

### 3.2 目录骨架（详见 `docs/01-目录骨架设计.md`）

**采用领域核心顶层分层**（package-by-feature）：领域核心 `core/` 顶层一等公民，`api/` 放传输 handler，`internal/` 只装私有基建。

```
server/
├── go.mod / sqlc.yaml / .golangci.yml / AGENTS.md / Makefile
├── cmd/{server,aether,migrate}/      # 入口；server/aether 含 wire.go + wire_gen.go（D38）
├── api/                              # ★ 传输层 handler（非契约目录；proto 源在仓库根 contracts/）
│   ├── grpc/                         #   gRPC handler（按业务域分包，后续）
│   └── http/                         #   gin handler + 中间件链
├── core/                             # ★★ 领域核心（顶层，package-by-feature）
│   ├── registry/ catalog/ tenancy/ stackmodel/ codegen/ orchestrator/
│   ├── approval/ executor/ terramate/ drift/ workspace/ envtenant/
│   ├── cmdb/ finops/ cloudcreds/ identity/ importer/ gate/ ai/
│   ├── events/ audit/ clock/ store/ queue/ crypto/ + core.go（wire ProviderSet）
├── data/                             # 数据访问层（data.go pgxpool + dbset/ 按表分）
├── internal/                         # ★ 只装私有基建（不外露；单体直接展开，不套 micro/）
│   ├── config/  otel/  server/       # 配置(viper) + 可观测(OTel) + server 启动装配
│   ├── model/entity/  proto/  mapping/   # Entity(DB表) + proto生成(自己+外部服务) + 转换
│   ├── errors/  event/  auth/  httpclient/  asset/  utils/   # 错误/事件/安全/HTTP客户端/资源/工具
├── pkg/db/{queries,generated}/       # sqlc 输入输出
└── db/migrations/                    # goose .up/.down.sql
```

设计原则（实践验证 + 守护 D28/D1）：
- **SQL 即真相源**：sqlc 输入（`pkg/db/queries/*.sql`）与 goose 迁移（`db/migrations/*.sql`）共用 SQL，审计员能把任何 schema 变更追溯到被 PR review 的迁移文件（守护 D28）。
- **领域核心顶层**：`core/` 是系统心脏，不该埋在 `internal/`（业务演进时 package-by-feature 比 package-by-layer 友好）。
- **wire 编译期装配**：各层 `ProviderSet`，`cmd/wire.go` 触发 `wire gen`，生成代码无反射可审计（D38）。
- **消费侧接口**：`core/` 内定义业务接口（如 `codegen.PathGenerator`），实现在同包或 `data/`。
- **D1 边界守护**：`.golangci.yml` depguard 模块前缀 deny `github.com/terramate-io/terramate`，allow-list 默认空。
- 与 `iac-self-service-server/...s/03` 的关系：原 docs/03 的业务包（registry/codegen/orchestrator 等）**位置从 `internal/` 提升到 `core/`**，原 docs/03 作为权威业务清单保留参考。


## 04-关键决策

### D31 — 数据访问层：sqlc + pgx/v5

- **决策**：数据访问层用 `sqlc`（codegen from hand-written SQL）+ `pgx/v5`（PostgreSQL 驱动）。`core/store` 包装 `*db.Queries`（sqlc 生成）。
- **理由**：
  1. **审计透明（守护 D28）**：sqlc 从手写 `.sql` 生成代码，审计员能把任何 `audit_logs`/`request_events` 的**行级写入**追溯到一行被 PR review 过的 SQL（D28 §3 的 audit_logs 审计维度）。注意：D28 的**值级 provenance**（`resolved_params_json` 里每个值的 source/rank/rule_id）由 codegen 的 `paramresolver`（Go 逻辑）负责，sqlc 只持久化结果 JSON —— sqlc 提供 SQL 行级可追溯，不替代值级 provenance。
  2. **类型安全**：编译期类型检查，无运行时反射开销。
  3. **2025-2026 生产共识**：社区对新项目已从 GORM 转向 sqlc（Bytebase、Brandur 实证）。
  4. **multica 实证**：multica-ai/multica 的 `server/pkg/db/generated` 用 sqlc + pgx/v5 跑 142 个迁移、~700 query 稳定运行。
- **备选**：
  - GORM：反射 + N+1 风险 + 审计不透明 → **拒绝**。
  - ent：schema-as-code 学习曲线高 + 与迁移工具耦合 → **拒绝**。
  - sqlx/pgx 裸写：失去类型安全，scan 代码重复 → 作为 sqlc 不可用时的兜底。
- **影响**：`core/store` 全部、`pkg/db/queries`、`pkg/db/generated`。动态查询（如 catalog 列表多过滤）需配 `Masterminds/squirrel`。
- **跨引用**：守护 D28 §3 的 audit_logs 行级审计可追溯；D28 §(3) 的值级 provenance 由 `core/codegen/provenance` 负责（本 change 不实现）。D6（state key 隔离）、D18（CMDB 资源索引）受益。

### D32 — CLI 框架：cobra

- **决策**：`cmd/aether` 用 `spf13/cobra`（+ `pflag`）。
- **理由**：
  1. **事实标准**：kubectl/gh/hugo 都用 cobra，贡献者熟悉度最高。
  2. **生态最大**：cobra-cli 脚手架、自动 bash/zsh/fish 补全、命令分组（GroupID）。
  3. **9 子命令适配**：catalog/request/stack/drift/approval/cost/gate/mcp/ai 用 cobra 的 `AddGroup` + `AddCommand` 自然组织。
- **备选**：
  - kong：Terramate 用（已验证 go.mod 直依赖），struct-tag 声明式适配配置驱动；但生态小、补全弱 → **次选**。
  - urfave/cli：更轻但无显著优势 → **拒绝**。
- **影响**：`cmd/aether/` 全部。与 Terramate 的 kong 不一致是有意识的取舍（平台 CLI 是独立工具）。
- **跨引用**：对应 D17（平台 CLI 与 AI 原生扩展）。

### D33 — 数据库迁移：goose（`//go:embed` + 完整版本历史 + advisory lock）

- **决策**：迁移用 `pressly/goose/v3`，迁移文件 `db/migrations/NN_name.{up,down}.sql`，通过原生 `embed.FS` 编入 `cmd/migrate` 二进制。goose 提供 advisory lock（并发启动安全）+ 完整应用版本历史表。
- **理由**：
  1. **原生 `//go:embed` 最佳**：goose 在所有迁移库里 `embed.FS` 支持最干净（`goose.Up(db, "migrations", goose.WithEmbedFS(fs))`），`aether` 作为单一产物分发零摩擦。
  2. **advisory lock（关键）**：goose 内置锁，多实例/并发启动安全。**反例**：multica 手写迁移器因无锁导致 issue #3647 并发冲突 bug —— 这是不该手写的实证。
  3. **完整版本历史**：goose 记录每个已应用版本（golang-migrate 只记最后版本），满足审计场景。
  4. **依赖最轻**：单一库，依赖树小，符合"最小依赖"偏好。
  5. **SQL + Go func 迁移**：数据回填场景可写 Go migration。
- **备选**：
  - Atlas（声明式 schema-as-code）：方法论与 Terraform 一致是吸引力，但**依赖树最重** + 学习曲线（diff/plan/apply + atlas.hcl + atlas.sum）+ 两个参考项目（Terramate/multica）都没用 → **次选**（若未来想要声明式 schema drift 检测再评估）。
  - golang-migrate：只记最后版本 + 失败留 dirty 状态需手动修 → **拒绝**。
  - tern：PG 专用，pgx 同源，但社区比 goose 小 + 锁定 PG → **次选**。
  - 手写（multica 式）：无锁（#3647 bug）+ 无篡改检测 → **拒绝**。
  - gorm AutoMigrate：生产反模式 → **拒绝**。
- **影响**：`db/migrations/`、`cmd/migrate/`。
- **跨引用**：守护 D28（迁移文件 PR review + 完整版本历史审计）、对应 D2（platform 代码归属）、D4（元数据驱动恢复）。

### D34 — HTTP 路由：gin

- **决策**：HTTP 传输层 `api/http` 用 `gin-gonic/gin`。
- **理由**：
  1. **生态最大**：gin 是 Go 最流行的 web 框架，中间件生态丰富（RBAC/限流/审计/CORS/JWT 都有现成）。
  2. **中间件链成熟**：本平台有大量中间件需求（OIDC 鉴权、RBAC、限流、审计日志、correlation ID、幂等键），gin 的 `r.Use(mw...)` 模式天然适配。
  3. **性能**：基于 httprouter，路由性能好。
  4. **绑定/校验**：`c.ShouldBindJSON` + validator tags 对表单 schema 驱动（`catalog_items.form_schema_json`）友好。
- **备选**：
  - chi：更贴近 stdlib net/http、更轻量；但中间件生态比 gin 小 → **次选**（若团队偏好 stdlib 风格）。
  - echo：与 gin 同类，无明显优势 → **拒绝**。
  - stdlib net/http（Go 1.22+ 路由增强）：最轻但中间件需自写 → **拒绝**（中间件需求大）。
- **影响**：`api/http/`、`cmd/server/main.go`。
- **跨引用**：对应 D8（表单触发与长任务）、D21（双门禁审批挂起）、specs/08（平台 API 与网关）。

### D35 — 日志：zap（原生 Logger API）

- **决策**：日志用 `go.uber.org/zap` 的**原生 `zap.Logger` API**（非 SugaredLogger），通过 `zap.L()` 全局 logger 调用，生产用 `zap.NewProduction()`、开发用 `zap.NewDevelopment()`。
- **理由**：
  1. **性能**：zap 是 Go 结构化日志性能标杆（比 slog 快数倍，高并发下优势显著），用原生 API 性能最高。
  2. **类型安全**：原生 API 强制 `zap.String("k", v)`/`zap.Int("n", n)` 显式字段类型，编译期检查（SugaredLogger 的 printf 风格是运行时反射）。
  3. **生态成熟**：Uber 生产验证，与 OpenTelemetry（D41）桥接成熟（`zap-otel` 把 trace_id 注入日志）。
  4. **用户决策**：用户明确选 zap 原生 API（高性能优先）。
- **备选**：
  - slog（stdlib）：零依赖但性能低，zap 在结构化日志领域更久经考验 → **次选**（若未来想降级到 stdlib）。
  - zerolog：与 zap 同级，生态略小 → **拒绝**。
  - SugaredLogger：易用但 printf 风格运行时反射，性能低 → **拒绝**（性能优先选原生）。
- **影响**：全局（所有包的 `zap.L()` 调用）、`cmd/*/main.go`（logger 初始化 + OTel handler 桥接）。
- **跨引用**：对应 specs/22（平台运营指标）、D30（安全合规证据链）、D41（OTel trace_id 注入日志）。

### D36 — 代码规范：Google Go Style Guide + golangci-lint

- **决策**：代码规范遵循 [Google Go Style Guide](https://google.github.io/styleguide/go/) + Effective Go，由 `.golangci.yml`（gofmt/goimports/vet/staticcheck/depguard）强制，工程契约写入 `server/AGENTS.md`。测试断言用 `stretchr/testify`（`require`+`assert`），mock 用手写 fake 优先 + `go.uber.org/mock`（**不用** `testify/mock`）。
- **理由**：
  1. **权威性**：Google Go Style Guide 是 Go 社区公认的最完整风格指南（含 StyleDecisions、Best Practices）。
  2. **testify 实测纠错**：Google Go Style Guide **不禁止** testify（Google 内部用私有 assert 基建，但公开规范对第三方断言库沉默）→ `testify/assert`/`require` 可接受且社区普遍使用；仅 `testify/mock` 该避免（用手写 fake 或 uber-go/mock，耦合更低）。
  3. **ECC 不覆盖**：ECC 的 `golang-patterns` skill 与 `rules/golang/` 基于社区谚语，不引用谷歌规范 → 需独立固化。
  4. **工具强制**：golangci-lint 把规范变成 CI 硬门（gofmt 不过 = CI 红）。
  5. **depguard 守护 D1**：depguard 用**模块前缀 deny** `github.com/terramate-io/terramate`（根 + 全部子包），默认空 allow-list —— 比"枚举 6 个包"更完整，不会漏 `cloud`/`ls`/`cmd`/`cloudsync` 等内部包。
- **D1 内嵌 carve-out**：D1 保留了"未来可选 import Terramate Go 包做内嵌实现"。为避免 D1 的未来路径与本决策冲突，depguard 配置一个**命名 allow-list**（默认空），仅当未来启用内嵌适配器时，在 `core/terramate/embedded`（带 `//go:build terramate_embedded` 标签）显式 allow。本 change 不实现内嵌，allow-list 保持空。
- **备选**：
  - 仅用 ECC golang-patterns：不引用谷歌规范，风格漂移风险 → **作为补充而非替代**。
  - 仅社区谚语：不够系统 → **拒绝**。
  - 纯 stdlib testing 无 testify：更贴 multica 风格但断言冗长，且谷歌规范不禁止 testify → **拒绝**（采纳 testify）。
- **影响**：`server/AGENTS.md`、`server/.golangci.yml`、CI、测试代码风格。
- **跨引用**：守护 D1（exec 边界，前缀 deny + 命名 allow-list 兼容 D1 的未来内嵌路径）、对应 `iac-self-service-server/...s/00-工程契约.md`。

### D37 — 配置管理：viper（多源合并 + 热更新）

- **决策**：配置用 `spf13/viper`，支持 yaml/json/toml 配置文件 + 环境变量 + 命令行 flag **多源合并**（优先级 flag > env > file > default），并支持文件 watch 热更新。
- **理由**：
  1. **多源合并**：你的平台需支持"配置文件（部署默认）+ env（K8s ConfigMap/Secret 注入）+ flag（运维临时覆盖）"三层优先级（D15），viper 是 Go 生态多源合并的事实标准。
  2. **热更新**：viper 的 `WatchConfig` + `OnConfigChange` 支持 DB 连接池/日志级别等热调，对 D15（Web 可改配置）友好。
  3. **生态最大**：Go 配置库采用度最高，文档/示例最多，贡献者熟悉度高。
  4. **用户决策**：用户明确选 viper。
- **取舍（如实记录）**：
  - viper 用 `interface{}` 取值（`viper.GetString("db.dsn")`），需类型断言或 Unmarshal 到 struct，配置错误运行时才报（不像 caarlos0/env 编译期检查）。
  - 比"env+struct"重（反射 + 运行时魔法），但灵活性收益对多环境部署平台值得。
- **使用模式**：定义强类型 `Config` struct，启动时 `viper.Unmarshal(&cfg)` 一次填充；热更新字段用 viper 监听回调单独处理。
- **备选**：
  - env+struct（手写 helper）：零依赖、最显式，但不支持文件/flag 多源 → **次选**（multica 实证，若未来想最小化依赖可降级）。
  - koanf：viper 的轻量替代，模块化无反射，但生态小 → **次选**。
  - caarlos0/env：纯 env → struct，极轻但不支持配置文件 → **拒绝**（需要文件源）。
- **影响**：`internal/config/`、`cmd/*/main.go`（viper 初始化 + Unmarshal + WatchConfig）。
- **跨引用**：对应 D15（配置管理：启动配置 DB+Web 可改）、D41（OTel 日志级别热调）。

### D38 — 依赖注入：google/wire（编译期 codegen）

- **决策**：依赖注入用 `google/wire`（编译期 codegen），各层用 `wire.NewSet(...)` 定义 `ProviderSet`（`api/`、`core/`、`data/`、`internal/` 各子包（config/otel/server 等）各自聚合），在 `cmd/server/wire.go` 用 `//go:build wireinject` 触发 `wire gen` 生成 `wire_gen.go`。（注：`cmd/aether` CLI 不用 wire，用 factory 模式，见 §08 D-cli-factory。）
- **理由**：
  1. **编译期检查**：缺失/歧义依赖在 `wire gen` 时失败，不是启动时才报 —— 对 14 个核心层（registry/codegen/orchestrator/...）的中大型平台，依赖图会变复杂，wire 的编译期验证收益大。
  2. **生成的代码可读**：`wire_gen.go` 是纯手写风格 Go（无反射、无运行时容器），reviewer 可直接读，审计员可追溯依赖链。
  3. **各层解耦**：`ProviderSet` 按层聚合，`cmd/wire.go` 只组装，符合分层架构，新增业务包只改对应层 `ProviderSet`。
- **接线模式**：
  - 每层 `xxx.go` 顶部 `var ProviderSet = wire.NewSet(NewXxxService, ...)` 聚合本层 provider
  - 消费侧小接口（如 `core/codegen` 的 `PathGenerator` 接口在 `core/` 定义，实现在同包）
  - `cmd/server/wire.go` 用 `wire.Build(api.ProviderSet, core.ProviderSet, data.ProviderSet, internal.ProviderSet)` 装配
- **备选**：
  - 手动构造器：最显式但接线超一文件后冗长 → **次选**（降级路径）。
  - uber-go/fx：运行时容器 + 生命周期钩子，反射与 D28 审计理念冲突 + 过重 → **拒绝**。
  - samber/do：generics 运行时容器，采用度小 → **拒绝**。
- **影响**：`cmd/server/wire.go` + `wire_gen.go`（checked in）、各层 `ProviderSet`、构造器签名（接收接口）。
- **跨引用**：守护 D28（生成的 wire_gen.go 无反射、可追溯）。

### D39 — 异步作业用 River；事件通知直接调用处理函数（Phase 1 不引入 broker）

- **决策**：
  - **异步作业**（codegen/plan/apply/drift）用 `riverqueue/river`（PG-backed，`river.InsertTx` 入队与业务 DB 写原子）。
  - **事件通知**（drift.detected/approval.*/migration.*）Phase 1 **直接调用处理函数**，不引入 EventBus 接口、outbox、broker。需要重试的走 River，需要异步的用 `go func()`。
  - **未来如果拆多服务/需跨进程事件分发**，再评估引入 broker（**首选 RabbitMQ**：Go 官方客户端 `amqp091-go` + 死信队列原生 + Erlang 稳定性；Kafka 次选：海量日志流场景）。记录在 assess，不在本 change 预留代码。

- **理由（River 选定，作业队列）**：
  1. **PG-only 契合**：codegen/plan/apply/drift 是 DB 状态绑定的作业流水线（D8 工单状态机）。
  2. **原子性（决定性）**：`river.InsertTx(tx, job)` 让"工单状态迁移 + 入队 apply"在同一 PG 事务，避免幽灵工单；asynq+Redis 拆两步需 outbox 补。
  3. **维护活跃（2026 实测）**：River v0.40（2026-07-02）月度发布；asynq v0.26（2026-02）14-18 月间隔 + asynqmon UI 2024-05 停更。
  4. **无新基础设施**：没有 Redis，asynq 纯增负担。

- **理由（事件通知直接调用，不引入 broker/EventBus 接口）**：
  1. **Phase 1 是单进程**：事件的生产者和消费者在同一程序内（如 orchestrator 发现漂移 → 直接调 audit.Log/notify.Send/webhook.Send），函数直接调用即可，消息不会丢。
  2. **YAGNI**：通知/审计类网关服务的常见实践是直接函数调用 + DB 记状态，无需引入 broker；过早抽象 EventBus 接口会增加无消费者时的复杂度。
  3. **EventBus 接口/outbox 是过度设计**：单进程下 Go channel 或直接函数调用足够，提前抽象接口 + outbox 表 + Relay goroutine 增加复杂度无收益。YAGNI。
  4. **重试用 River**：如发 webhook 失败需重试，用 River 入队一个"发 webhook"作业即可（它有 retry/backoff/DLQ）。
  5. **审计用数据库**：事件同时写一条 audit_logs 记录，审计回溯走数据库，不需要 broker 重放。

- **代码形态（说人话）**：
  ```go
  // core/orchestrator/drift.go
  func (o *Orchestrator) OnDriftDetected(ctx context.Context, d DriftResult) {
      o.audit.Log(ctx, "drift.detected", d)        // 直接调，写审计
      o.notifier.NotifyTeam(ctx, d.TeamID, d.Summary) // 直接调，发通知
      // webhook 可能失败需重试 → 入队 River 作业
      o.queue.InsertTx(ctx, tx, river.NewJob("send-webhook", webhookPayload))
  }
  ```

- **备选**：
  - asynq（Redis，作业队列）：失去 PG 原子性 + 增 Redis + 维护放缓 → **拒绝**。
  - machinery：老旧 + worker-hang + 245 issues → **拒绝**。
  - PGMQ：只是队列表无 worker → **拒绝**。
  - EventBus 接口 + InProcess/NATS/Kafka 多实现：单进程下过度设计 → **Phase 1 不做，未来拆服务再评估**。
  - outbox 表 + Relay goroutine：单进程下过度设计 → **Phase 1 不做**。

- **影响**：`core/queue/`（River 封装）、`core/orchestrator/`（作业入队 + 事件直接调用处理函数）、worker 进程模型（同进程或独立 `cmd/server-worker`）。
- **跨引用**：对应 D8（工单状态机 + 异步队列）、specs/06（编排引擎流水线）、specs/07（漂移检测调度）、specs/22（平台运营指标事件）。broker 全景调研存档见 `assess/event-broker-deep-research-20260707.md`（未来拆服务时参考）。

### D40 — JSON Schema 校验：santhosh-tekuri/jsonschema/v6

- **决策**：JSON Schema 校验用 `santhosh-tekuri/jsonschema/v6`（Draft 2020-12），覆盖 `catalog_items.form_schema_json`、`requests.resolved_params_json`、`RequestCreate` 等 schema 校验。
- **理由**：
  1. **catalog 核心特性**：`form_schema_json` 是服务目录契约（specs/02），用户表单由它驱动渲染，必须校验。
  2. **Draft 2020-12**：xeipuuv/gojsonschema 卡在 Draft-07 废弃；qri-io 只到 2019-09；santhosh-tekuri v6 支持 2020-12。
  3. **速度 + 正确性双优（vearutop benchmark 实测）**：是唯一**同时**最快且最准的库（通过 Bowtie 官方测试套件）。benchmark 作者原话："要快的选 santhosh-tekuri 或 qri-io；要准的选 santhosh-tekuri"。
  4. **Google 2026-01 jsonschema-go 不换**：为 LLM/Gemini schema 推断优化（从 Go struct 生成 schema），非通用校验；6 个月新，无 Bowtie 记录，12 个月后观望。
- **实现要点（4 条硬约定）**：
  1. **编译一次复用**：启动时 `jsonschema.NewCompiler()` 编译平台 meta-schema 一次，复用 `*Schema` 对象（不要每次校验重编译）。
  2. **注册自定义 format**：module path / ARN / semver 等平台标识符用 `compiler.AddFormat(...)` 注册。
  3. **禁用远程 `$ref` loader**：用户 schema 必须自包含，不允许 fetch 任意 URL（安全 + 确定性）。santhosh-tekuri 默认可禁用。
  4. **两级校验**：先校验用户提交的 schema 合法（对平台 meta-schema），再校验实例（对已信任的用户 schema）—— form_schema_json 是用户定义的，必须先验证它自身结构合法。
- **备选**：
  - xeipuuv/gojsonschema：废弃在 Draft-07 → **拒绝**。
  - qri-io/jsonschema：只到 2019-09 + 测试失败多 → **拒绝**。
  - google/jsonschema-go：为 LLM 优化 + 太新 → **观望**（12 个月）。
  - kaptinlin/jsonschema：新，direct struct 校验有趣但无基准 → **观望**。
  - quicktype 代码生成：你的 schema 是运行时用户定义，不能 build-time codegen → **拒绝**。
- **影响**：`core/catalog/`（form_schema 校验）、`core/orchestrator/`（resolved_params 校验）、API 入参校验链。
- **跨引用**：对应 D28（resolved_params_json 契约）、specs/02（服务目录 form_schema_json）、specs/19（API 与 Schema 契约）。

### D41 — 可观测性：OpenTelemetry 端到端 trace 贯穿 + otelzap Mode 1 + Prometheus exporter

- **决策**：可观测用 OpenTelemetry（`go.opentelemetry.io/otel`）。**trace 端到端贯穿**（入口 → RPC → DB → Redis → 第三方 HTTP → 日志，单一 trace_id 全链路传递）。zap 桥接**明确选 otelzap Mode 1**（trace_id 作为结构化字段注入 zap 日志，不依赖 Beta Logs SDK）。**metrics 走 OTel SDK + Prometheus exporter**（单独 `/metrics` 端点对接 Prometheus/VictoriaMetrics，不混入 trace 管线）。trace 后端地址**可配置**（env `OTEL_EXPORTER_OTLP_ENDPOINT` → Collector → Jaeger/Tempo/Datadog）。
- **端到端 trace 链路（核心要求）**：
  ```
  入口（gin/otelgin 或 Connect/otelconnect）
    ↓ W3C traceparent 提取，创建 server span，存入 ctx
  业务逻辑（ctx 贯穿）
    ↓ pgx 查询（otelpgx 自动建子 span，SQL 文本入属性）
    ↓ Redis（redisotel 自动建子 span，若引入 Redis）
    ↓ 出站 HTTP 调第三方（otelhttp 注入 traceparent 头）
    ↓ 出站 Connect/gRPC 调微服务（otelconnect 注入 metadata）
    ↓ zap 日志（otelzap 从 ctx 读 trace_id 注入字段）
  ```
  **黄金法则**：Go 没有 thread-local "当前 span"，trace 只靠 `context.Context` 传递。**任一 hop 丢了 ctx → trace 断**。
- **各层 instrumentation 库（2026 实测，全部官方/contrib 维护）**：
  | 层 | 库 | import | 自动？ |
  |---|---|---|---|
  | 传播器（启动设一次）| TraceContext + Baggage | `go.opentelemetry.io/otel/propagation` | ✅ 全局生效 |
  | gin 入站 | otelgin | `…/gin-gonic/gin/otelgin` | ✅ 提取+建 span |
  | Connect 入站/出站 | **otelconnect**（非 `connectrpc.com/otel`）| `connectrpc.com/otelconnect` | ✅ 需 `WithTrustRemote()` 内部互信 |
  | grpc-go 出站 | otelgrpc（StatsHandler API）| `…/google.golang.org/grpc/otelgrpc` | ✅ ⚠️ 旧拦截器有 CVE，用 NewClientHandler |
  | HTTP 出站（云 API/OIDC/SCM）| otelhttp | `…/net/http/otelhttp` | ✅ 注入 traceparent |
  | pgx | **exaring/otelpgx**（非 contrib）| `github.com/exaring/otelpgx` | ✅ QueryTracer |
  | Redis（若引入）| redisotel | `github.com/redis/go-redis/extra/redisotel/v9` | ✅ Hook |
  | zap↔trace | otelzap Mode 1（Uptrace 版，trace_id 入字段）| `github.com/uptrace/opentelemetry-go-extra/otelzap` | ⚠️ 需传 ctx |
  | metrics | Prometheus exporter | `go.opentelemetry.io/otel/exporters/prometheus` | `/metrics` 抓取 |
  | trace 导出 | OTLP/HTTP + env 配置 | `…/otlp/otlptrace/otlptracehttp` | env `OTEL_EXPORTER_OTLP_ENDPOINT` |
- **3 个重要纠错（实测，避免踩坑）**：
  1. **`go.opentelemetry.io/contrib/.../pgx/v5/otelpgx` 不存在**（contrib 所有 pgx 路径 404）→ 用第三方 `github.com/exaring/otelpgx`（事实标准）。
  2. **Connect 包是 `connectrpc.com/otelconnect`，不是 `connectrpc.com/otel`**。
  3. **Connect 默认不信任远端 trace**（建新 root span 只 Link）→ 内部服务间必须 `otelconnect.WithTrustRemote()`，否则 trace 在 RPC 边界断裂成多个独立 trace。
- **otelzap 两种模式（明确选 Mode 1）**：
  - **Mode 1（选定）：trace_id 字段注入** —— Uptrace otelzap `WithTraceIDField(true)`，把 trace_id/span_id 作为 zap 字段注入。仅依赖稳定 Traces SDK。**必须传 ctx**（`otelzap.L().Ctx(ctx).Info(...)`），裸 `zap.L()` 读不到 trace（#6114 全零症状）。
  - **Mode 2（不选）：OTLP 日志导出** —— 依赖 Beta Logs SDK，推迟。
- **理由**：
  1. **OTel 是 2026 无可争议标准**（CNCF，已替代厂商 SDK）。
  2. **端到端 trace 是 D8/D28 审计的硬要求**：工单全链路（表单→codegen→plan→apply→reconcile）必须能按单一 trace_id 串联日志/事件/审计/DB 查询/外部调用。
  3. **metrics 单独走 Prometheus exporter**：与 trace 管线解耦，Prometheus 原生抓取，不经过 OTel Collector（你明确要分离）。
  4. **trace 后端可配置**：env `OTEL_EXPORTER_OTLP_ENDPOINT` 指向 Collector，Collector 再 fan-out 到 Jaeger/Tempo/Datadog（解耦 app 与后端）。
- **备选**：
  - prometheus/client_golang 直连（不经 OTel SDK）：极简但与 trace 仪表盘分裂 → **次选**。
  - otelzap Mode 2（OTLP 日志）：Beta 依赖 → **推迟**。
  - 弃用 OTel 自建：重复造轮子 → **拒绝**。
- **影响**：全局（启动设 propagator + TracerProvider + MeterProvider；所有层 instrumentation；ctx 贯穿所有调用；otelzap 替换全局 logger）、`cmd/server/main.go`（OTel SDK 初始化 + Prometheus exporter + tp.Shutdown）、`internal/otel/`（封装）、`internal/otel/`（SDK 装配）。
- **跨引用**：对应 specs/22（平台运营指标）、D30（安全合规证据链）、D8（工单全链路可观测）、D34（gin otelgin）、D45（Connect otelconnect）、D31（pgx otelpgx）。完整集成设计见 `docs/03-可观测性集成设计.md`。

### D42 — 测试数据库：testcontainers-go + pgtestdb（PG 专用，禁 SQLite）

- **决策**：测试用真实 PostgreSQL，由 `testcontainers-go`（Postgres 模块）+ `peterldowns/pgtestdb`（迁移一次 + 模板克隆加速）提供。**严禁**用 in-memory SQLite 替代。
- **理由**：
  1. **PG 方言专属性**：你的 schema 用 PG 特性（JSONB、partial index、enum、CONCURRENTLY），SQLite 方言不同 → SQLite 测试给"假自信"。
  2. **sqlc 生成代码绑 pgx**：SQLite 跑不了 sqlc 生成的 pgx 代码。
  3. **pgtestdb 解决慢**：迁移一次建模板，每测试克隆，毫秒级。
- **备选**：
  - SQLite in-memory：方言不符 + 跑不了 pgx → **拒绝**。
  - dockertest：testcontainers-go 更现代、API 更好 → **拒绝**。
  - 共享 dev PG：测试隔离差、并发冲突 → **拒绝**。
- **影响**：`pkg/db/`/`core/store/` 测试、CI（需 Docker）。
- **跨引用**：对应 D31（sqlc+pgx 测试链路）、D28（审计测试需真 PG）。

### D43 — 安全基础：argon2id + golang-jwt/v5 + zitadel/oidc

- **决策**：
  - 密码哈希：`golang.org/x/crypto/argon2`（argon2id，OWASP 2025 首选）
  - JWT：`golang-jwt/jwt/v5`（v4 是 legacy）
  - OIDC 客户端：`zitadel/oidc`（活跃维护，client+server；coreos/go-oidc 已放缓且仅 client）
- **理由**：argon2id > bcrypt（2025 OWASP）；zitadel/oidc 比 coreos/go-oidc 活跃且功能全。
- **备选**：
  - bcrypt：可用但 argon2id 更现代 → **次选**。
  - coreos/go-oidc：已放缓 → **拒绝**。
- **影响**：`internal/auth/`、`internal/identity/`（后续 change 实现具体逻辑，本 change 仅锁定库选择）。
- **跨引用**：对应 D10（OIDC 认证）、D23（云凭据）、D30（break-glass + 证据链）。

### D44 — 时间抽象：Clock 接口注入（可测试性）

- **决策**：定义最小 `Clock` interface（`Now() time.Time`），在业务逻辑中注入，所有 `time.Now()` 走 Clock。时间戳统一存 UTC `timestamptz`。
- **理由**：
  1. **drift/scheduling/TTL 可测试**：漂移检测（D5）、任务调度（D39 River）、审批超时（D21）、break-glass TTL（D30）依赖时间，注入 Clock 才能确定性测试。
  2. **6 行接口 vs 依赖**：自写比 `benbjohnson/clock` 更轻，符合"少依赖"原则。
- **备选**：
  - benbjohnson/clock：现成但引入依赖，6 行接口够用 → **次选**。
  - 直接 time.Now()：不可测 → **拒绝**。
- **影响**：`core/clock/`（接口 + 真实实现 + FakeClock）、所有业务包构造器加 clock 参数。
- **跨引用**：守护 D5（drift）、D21（审批超时）、D30（break-glass TTL）、D39（任务调度）。

### D45 — RPC 框架：Connect-RPC（proto 唯一真相源 + gRPC-Web 原生 + 与 gin 共存）

- **决策**：业务 RPC 用 `connectrpc.com/connect`（Connect-RPC）。`.proto` 是**唯一真相源**，一个 Connect handler 同时服务 gRPC（服务间）、gRPC-Web（前端 React）、Connect/JSON（aether CLI + curl）。Webhook 回调 + 外部系统对接 + 运维端点（/healthz、/metrics）用 gin（D34），两者挂在同一 `http.ServeMux`（一进程一端口）。
- **理由**：
  1. **proto 唯一真相源（决定性）**：用户明确要求 `.proto` 是唯一真相源。Connect 从一个 `.proto` 生成一个 handler，同时说三种协议 —— 避免手写 HTTP/gRPC 两套 handler（两个真相源会漂移）。
  2. **前端 gRPC-Web 无 Envoy**：用户要前端用 gRPC-Web。标准 gRPC-Web 需 Envoy 代理转换 + 有"失败返回 HTTP 200"的观测陷阱（trailer 丢失）。Connect **原生说 gRPC-Web 协议**，无 Envoy，EOS metadata 嵌入 JSON 消息避免 trailer 问题。
  3. **与 gin 共存（D34）**：Connect handler 是标准 `http.Handler`，和 gin 挂同一 ServeMux —— gin 服务 webhook/运维（HTTP-only），Connect 服务全部业务 RPC。无两端口、无独立 gateway 进程。
  4. **最小依赖**：1 核心库（connect-go）+ 2 代码生成插件（protoc-gen-connect-go + protoc-gen-connect-openapi）+ 0 基础设施。比 grpc-gateway（5 插件 + 可能 Envoy）轻得多。
  5. **wire-compatible 与 grpc-go**：Connect handler 内部用 gRPC 线协议，服务间调用可用任何 gRPC 客户端（含 grpc-go）连。
  6. **CNCF 项目 + Anthropic 生产用**：成熟度足够。
- **协议分工**（明确边界）：
  - **业务 RPC（全部）**：Connect handler —— Request/Catalog/Planning/Apply/Approval/Drift/CMDB/FinOps 等所有 specs/19 列的 proto service
  - **前端**：gRPC-Web（Connect 原生，React 用 connect-es 客户端）
  - **aether CLI**：Connect/JSON（HTTP/1.1，curl 可调）或 gRPC
  - **服务间**：gRPC（orchestrator→executor，HTTP/2 + protobuf）
  - **Webhook 回调**：gin（外部系统 HTTP/JSON 固定 schema，非 RPC）
  - **运维端点**：gin（/healthz、/ready、/metrics）
- **Connect interceptor 层（关键）**：业务 RPC 的横切关注点（auth/RBAC/审计/限流）MUST 用 **Connect interceptor** 实现（`connect.WithInterceptors(...)`），不依赖 gin 中间件（gin 中间件只覆盖 webhook/运维，不经 Connect handler）。定义在 `api/grpc/interceptor/`：auth_interceptor（JWT/OIDC + AK/SK）、rbac_interceptor（角色绑定）、audit_interceptor（请求审计）、ratelimit_interceptor（per-actor 限流）。共享逻辑抽纯函数，Connect interceptor 和 gin middleware 各写薄适配器调用。
- **架构图**：
  ```
  ┌─────────────────────────────────────────┐
  │      .proto（唯一真相源，specs/19）       │
  │   services + google.api.http annotations │
  └──────────────────┬──────────────────────┘
                     │ buf generate
     ┌───────────────┼──────────────────┐
     ▼               ▼                  ▼
  Go Connect     TS client         Go client
  handlers       (connect-es)      (aether CLI)
  (connect-go)                     (connect-go)
     │
  ┌──┴──────────────────────┐
  │ http.ServeMux（一进程一端口）│
  ├─────────────────────────┤
  │ /api/  → Connect handler │ ← gRPC（服务间）+ gRPC-Web（前端）+ Connect/JSON（CLI/curl）
  │ /webhooks/ → gin         │ ← Webhook 回调（HTTP/JSON）
  │ /healthz, /metrics → gin │ ← 运维端点
  └─────────────────────────┘
  ```
- **备选**：
  - grpc-go + Envoy（gRPC-Web）：成熟但需 Envoy 基础设施 + 200-on-error 陷阱 → **拒绝**（违背最小依赖 + 观测有坑）。
  - grpc-gateway：proto 生成 REST，成熟但 5 插件 + OpenAPI 只 v2 + 无 bidi → **拒绝**（重 + 前端要 REST 而非 gRPC-Web）。
  - 手写 gin + grpc-go 两套 handler：违背 proto 唯一真相源（两套会漂移）→ **拒绝**。
- **影响**：`api/grpc/`（Connect handler 实现）、`cmd/server/main.go`（ServeMux 装配 Connect + gin）、`buf.gen.yaml`（connect-go + connect-es + connect-openapi 插件）、前端用 connect-es。
- **跨引用**：对应 specs/19（API 与 Schema 契约，proto 唯一真相源）、specs/08（平台 API 与网关）、D34（gin 共存做 webhook/运维）、D17（aether CLI 用 Connect 客户端）。

## 05-风险与权衡

| 风险 | 缓解 |
|---|---|
| sqlc 动态查询短板（运行时 WHERE） | 仅在有真实动态查询需求的包（catalog 列表过滤）引入 squirrel；其余全静态 SQL |
| cobra 与 Terramate 的 kong 不一致 | 平台 CLI 是独立工具；cobra 生态/补全长期收益 > 一致性成本 |
| gin 与 Terramate 主仓的 web 工具不一致 | 平台是独立 module，物理隔离；不共享 web 代码 |
| Go 版本对齐 | `server/go.mod` 声明 `go 1.25`（与 Terramate 对齐），CI 锁 toolchain |
| Google Go Style Guide 与社区惯用法的细微差异 | AGENTS.md 明确"谷歌规范优先，社区谚语补充"，golangci-lint 兜底 |

## 06-迁移与上线

本 change 自身就是 `iac-self-service-platform` 进入编码的前置。无分阶段 —— 一次性落地全部骨架 + 决策。

完成标准：
1. `openspec validate platform-tech-stack-and-scaffold` 通过。
2. 落地后 `server/` 目录 `make build && make test` 全绿。
3. `.golangci.yml` 的 depguard 对故意违规 probe（import terramate 内部包）报错。

## 07-测试策略

每个骨架组件配契约测试（对齐 `iac-self-service-platform` 的 `tasks.md` 测试约定）：

| 组件 | 测试 | 包路径 |
|---|---|---|
| 目录骨架 | `go build ./...` 通过 | `server/...` |
| sqlc 生成 | testdb 跑 List/Create 断言 | `server/pkg/db/...` |
| goose 迁移 | Up→Down→Up 幂等 | `server/cmd/migrate/...` |
| cobra CLI | Execute 断言 help 输出 | `server/cmd/aether/...` |
| gin 路由 | httptest 断言 /healthz | `server/api/http/...` |
| golangci-lint | `make lint` 通过 + depguard 对 probe 报错 | `server/...` |
| 整体 | `make build && make test` | `server/...` |

测试风格遵循 Google Go Style Guide + D36：表驱动、`t.Run` subtest、`t.Helper`/`t.Cleanup`/`t.TempDir`、遵循 D36：testify assert/require + 表驱动 + t.Run subtest，不用 testify/mock。

## 08-演进记录：开源 monorepo 整改（2026-07）

本 change 原始方案把后端放在 `platform/` 目录（对齐 `iac-self-service-platform` D2 的"在 Terramate 主仓库新增 platform/"）。落地后为适配开源与"前后端整套系统"形态（参照 multica-ai/multica），做以下三处偏离修正：

### D-rename — `platform/` → `server/`（monorepo 后端目录）

- **变更**：后端目录从 `platform/` 改名为 `server/`，go module 路径从 `github.com/xuanwu-labs/selfservice-iac/platform` 改为 `github.com/xuanwu-labs/selfservice-iac/server`。`cmd/platform` 二进制改名 `cmd/server`，`cmd/tm` 改名 `cmd/aether`。
- **理由**：`platform/` 在 monorepo 中语义模糊（"平台"是整个系统，不是某个子目录）；`server/` 明确是后端服务，与未来的 `web/`（前端）对级。对齐 multica 等 monorepo 惯例。
- **D1 影响**：depguard 守护范围从 `platform/**` 调整为 `server/**`，规则语义不变。

### D-monorepo — 确立 monorepo 骨架

- **变更**：在 `selfservice-iac/` 仓库根建立 monorepo 骨架：`server/`（Go 后端）、`web/`（React 前端，规划中）、`contracts/`（proto/OpenAPI 契约源，前后端共享）、`deploy/`（部署清单）、`docs/`（用户文档）、`openspec/`（设计文档）。
- **理由**：本项目是 IaC 一整套系统（前后端 + 平台），不是单独工具。monorepo 形态对开源贡献者最直观。

### D-pkg-db — 恢复 `pkg/db/`（对齐 §3.2 原设计）

- **变更**：sqlc 的输入（`queries/`）与输出（`generated/`）从 `data/` 迁回 `pkg/db/`，对齐本 design.md §3.2 与 D31/D42 的原始设计。`data/` 只保留 `data.go`（pgxpool provider + wire ProviderSet）与 `dbset/`（表分组薄包装）。
- **理由**：先前实现把 sqlc 悄悄合并进 `data/` 且未更新文档，造成"文档骗人"。`pkg/db/` 是可复用数据层的常见位置（multica 等项目亦如此），且与"数据访问三件套"（core/store → data/ → pkg/db/generated）边界更清晰。

> 上述三处修正均已同步至本文档及 `tasks.md`、`docs/01`、`specs/`、`proposal.md`。`iac-self-service-platform` 的 D2 决策以演进注记形式说明此变更。

### D-cli-factory — CLI 不用 wire，保持 factory 模式（task 8.3 落地时决定）

- **变更**：`cmd/aether` 不引入 wire，继续用 `cmdutil.NewFactory()`（gh/cli 式 lazy factory）装配 CLI 依赖。task 8.3 原"写 cmd/aether/wire.go"设想放弃。
- **理由**：CLI 依赖（Stdin/Stdout、config、API client）需 **lazy 构造**——`--help`/`--version` 不能触碰网络或加载 config。wire 是编译期全装配，会强制在启动时构造所有依赖，破坏 lazy 特性。CLI 与 server 各取所长：
  - **server**：依赖图静态（pgxpool/logger/router），启动期全装配合理 → wire
  - **CLI**：依赖按命令路径 lazy 展开（`aether --help` 不应连数据库）→ factory 模式
- **对照**：此选择与 gh（cli/cli）、multica 的 CLI 实践一致——它们也不用 wire/DI 框架装配 CLI。
- **影响**：`internal/cmdutil/factory.go` 保持现状（已是 lazy factory），task 8.3 重定义为"记录此决定"而非"写 wire.go"。
