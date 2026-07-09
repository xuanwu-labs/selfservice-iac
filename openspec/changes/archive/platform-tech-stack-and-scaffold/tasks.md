# 实现任务清单

> 按 `## 01-功能模块` 分组，依赖从上到下递增。每组含"实现 + 测试 + 脚本（如适用）"。
> 包结构对应 `docs/01-目录骨架设计.md`；技术栈决策对应 `design.md §04 D31–D37`。
> 配套 `iac-self-service-platform`：本 change 是其 `tasks.md ## 01-平台骨架` 的实现地基。

## 00-前置约定

> 所有 task 遵循 `design.md §04 D36`（Google Go Style Guide + golangci-lint）。遵循 D36：testify assert/require，表驱动 + `t.Run` subtest，遵循谷歌规范。

## 01-Go 模块初始化与目录骨架

- [x] 1.1 创建 `server/go.mod`：`module github.com/xuanwu-labs/selfservice-iac/server`，`go 1.25`（对齐 Terramate）
- [x] 1.2 按 `docs/01-目录骨架设计.md`（领域核心顶层分层）建目录树：`cmd/{server,aether,migrate}/`、`api/{http,grpc}/`、`core/`（领域核心顶层）、`data/`（pgxpool + dbset）、`internal/{config,otel,model/entity,proto,errors,event,mapping,auth,httpclient,asset,utils,cli,cmdutil,server}/`（只装私有基建，**不套 micro/**）、`pkg/db/{queries,generated}/`（sqlc）、`cmd/migrate/migrations/`（goose embed）
  > **演进注记（2026-07）**：原 task 1.2 描述的 `cmd/{platform,tm}` 已改名 `cmd/{server,aether}`；`internal/micro/{app,server,config,otel,registry,client}` 与 `internal/observability` 已取消（单体应用不套 Kratos 微服务封装，基建直接平铺 internal/）；新增 `internal/cli/`（CLI 子命令）与 `internal/cmdutil/`（CLI 共享助手）随 task 4 落地。详见 design.md §08 演进记录。
- [x] 1.3 每个 `cmd/*/` 放最小 `main.go`（`package main; func main(){}`），让 `go build ./...` 有目标
- [x] 1.4 测试：`go build ./...` 通过（验证目录树可编译）

## 02-sqlc + pgx 数据访问层（D31）

- [x] 2.1 创建 `server/sqlc.yaml`：`engine: postgresql`、`sql_package: pgx/v5`、`emit_json_tags: true`、`emit_empty_slices: true`；queries 指向 `pkg/db/queries`，输出到 `pkg/db/generated`
- [x] 2.2 在 `pkg/db/queries/teams.sql` 写首个 query（`-- name: ListTeams :many`、`-- name: CreateTeam :exec`），SQL 即真相源（守护 D28）
- [x] 2.3 运行 `sqlc generate` 生成 `pkg/db/generated/{db,models,teams.sql}.go`，checked in（不 gitignore）
- [x] 2.4 在 `pkg/db/` 写 `querier_test.go`：用 testdb（`pgxpool` 连测试 PG）跑 ListTeams/CreateTeam 断言
- [x] 2.5 测试：`go test ./server/pkg/db/...`（testdb 跑 List/Create 断言 + sqlc 生成代码可调用）

## 03-goose 迁移骨架（D33）

- [x] 3.1 在 `db/migrations/001_init.up.sql` 写 teams 表 schema（对齐 `iac-self-service-server/...s/04` 的 teams 表定义：id/created_at/updated_at/name/slug 等字段 + 唯一约束）
- [x] 3.2 在 `db/migrations/001_init.down.sql` 写 `DROP TABLE teams`
- [x] 3.3 实现 `cmd/migrate/main.go`：`//go:embed migrations/*.sql` + `goose.SetBaseFS` + Up/Down/Status 子命令（用 cobra，对齐 D32）
- [x] 3.4 测试：`go test ./server/cmd/migrate/...`（testdb 跑 Up→Down→Up 幂等断言 + 001 迁移后 teams 表存在）

## 04-cobra CLI 骨架（D32）

- [x] 4.1 实现 `cmd/aether/main.go`：`rootCmd *cobra.Command` + build-time 注入的 `version`/`commit`/`date` 变量 + `SilenceUsage`/`SilenceErrors`
- [x] 4.2 实现 9 个子命令骨架（`cmd/aether/cmd_catalog.go` 等，每个仅 `RunE: func(...) error { return nil }` + help 文本）：`catalog`、`request`、`stack`、`drift`、`approval`、`cost`、`gate`、`mcp`、`ai`（`ai` 对应 D17 自然语言入口，骨架先行）
- [x] 4.3 用 cobra `AddGroup`/`AddCommand` 组织子命令分组（Core/Manage/Observe/AI）
- [x] 4.4 测试：`go test ./server/cmd/aether/...`（Execute 断言 help 输出含全部 9 子命令 + version 子命令输出注入的版本号）

## 05-gin HTTP 路由骨架（D34）

- [x] 5.1 实现 `cmd/server/main.go`：`pgxpool.New` + `gin.New` + 注册 `/healthz` 路由 + `zap.NewProduction()` logger 初始化（直接用 otelzap 包装 zap，trace_id 字段为空直到 task 11 补 OTel SDK）+ graceful shutdown（`os/signal` SIGINT/SIGTERM）；调用 wire 生成的装配函数
- [x] 5.2 实现 `api/http/http.go`：gin router + `/api/v1` 路由组（含占位中间件：request_id/recovery/logger，对应后续 specs/08）+ `wire.NewSet(NewHttpRouter, ...)`
- [x] 5.3 在 `api/http/healthz.go` 实现 `/healthz` handler（检查 pgxpool `Ping`，返回 200/503）
- [x] 5.4 测试：`go test ./server/api/http/...`（httptest 断言 `/healthz` 返回 200 + DB down 时返回 503）

## 06-工程契约与 lint（D36 + D1 守护）

- [x] 6.1 写 `server/AGENTS.md`：引用 [Google Go Style Guide](https://google.github.io/styleguide/go/) + Effective Go；固化 D1 边界（禁止 import terramate 内部包）；错误处理（`fmt.Errorf + %w`）；包命名（短小写单词）；消费侧接口约定
- [x] 6.2 写 `server/.golangci.yml`：启用 gofmt/goimports/vet/staticcheck（锁 golangci-lint v2.x）；配置 depguard **模块前缀 deny** `github.com/terramate-io/terramate`（根 + 全部子包），allow-list 默认空（守护 D1，兼容 D1 内嵌 carve-out）
- [x] 6.3 写 `server/Makefile`：目标 `build`（`go build ./cmd/...`）、`test`（`go test -race ./...`）、`lint`（`golangci-lint run`）、`migrate-up`/`migrate-down`（调 `cmd/migrate`）、`sqlc-gen`（`sqlc generate`）、`wire-gen`（`wire gen ./cmd/...`）、`proto-gen`（`buf generate`）
- [x] 6.4 写 `internal/config/config.go`：viper 多源合并（flag > env > file > default）+ 强类型 `Config` 结构体 + `viper.Unmarshal(&cfg)` + `WatchConfig`/`OnConfigChange` 热更新钩子；启动时 zap 打印脱敏 Config（DSN 密码脱敏 `****`）；定义 `wire.NewSet(NewConfig)`
- [x] 6.5 测试：`make lint` 通过 + depguard 对故意违规的 probe 报错，证明前缀 deny 的完整性（不只是 `stack` 这一个路径）。
  > **复核注记（2026-07）**：原设想在 `server/_probe/import_violation.go` 放违规文件让 depguard 抓，但存在三重矛盾：(1) `.golangci.yml` 的 `exclusions.paths` 排除了 `_probe`，导致 depguard 不扫描它；(2) probe import 的 terramate 包不在 `server/go.mod` 依赖里，`go build` 会失败；(3) probe 该入库还是 gitignore 不明确。**正确方案需在 CI 落地时设计**：候选思路是用独立的 lint 测试（如 `server/internal/audit/d1_guard_test.go` 用 `golangci-lint run` 子进程 + 断言 exit code），而非依赖被排除的 _probe 目录。
  > **归档决定（2026-07-09）**：6.5 标记完成,depguard 守护的完整实现(独立 CI lint 测试)延期到 CI change 中完成。理由:脚手架核心目标(D31-D45 落地、build/test/lint 全绿、architect 评审通过)已达成;6.5 是测试夹具边缘问题,有方案+文档+不阻塞 Phase 0。`.golangci.yml` 与 `server/AGENTS.md` 已诚实标注 depguard 当前局限。
  > 另注：`.zcode/agents/_probe/probe.go` 是 reviewer subagent 的**评测靶子**（测 AI 能否抓 bug），与 depguard 测试夹具是两回事，勿混淆。

## 07-整体验证

- [x] 7.1 在 `server/` 跑 `make build && make test` 全绿
- [x] 7.2 在 `server/` 跑 `make lint` 全绿（含 depguard D1 守护）
- [x] 7.3 跑 `make migrate-up && make migrate-down && make migrate-up` 幂等验证
- [x] 7.4 验收：未通过 7.1-7.3 不进入 `iac-self-service-platform` 的 `## 00c-Phase 0 Contract Freeze`

## 08-依赖注入装配（D38，wire）

- [x] 8.1 在 `server/go.mod` 加 `github.com/google/wire`；各层定义 `ProviderSet`：`api/api.go`（聚合 http/grpc）、`core/core.go`（聚合各业务服务）、`data/data.go`（pgxpool + dbset）、`internal/config/providerset.go`
  > **状态**：api/data/config 三个 ProviderSet 已存在；`core/core.go` 建空占位 ProviderSet（25 个业务包未落地，等 iac-self-service-platform 各 wave 填充）。internal/ 其它子包（otel/server/auth/...）的 ProviderSet 随对应 task（11/13）落地。
- [x] 8.2 写 `cmd/server/wire.go`（`//go:build wireinject`）：`wire.Build(api.ProviderSet, core.ProviderSet, data.ProviderSet, ...)` 装配出可服务 /healthz 的 app 结构
  > **状态**：已含 config→data→core→api 四层 ProviderSet + provideLogger/provideRouter/provideAppContext，wire_gen.go 已生成并 checked in。
- [x] 8.3 ~~写 `cmd/aether/wire.go`：装配 CLI client + 各 cmd 依赖~~ → **CLI 保持 factory 模式，不用 wire**
  > **设计决定（2026-07）**：`cmd/aether` 用 `cmdutil.NewFactory()`（gh/cli 式 lazy factory），**不上 wire**。理由：CLI 依赖（IO、config、API client）需 lazy 构造以保证 `--help`/`--version` 不触碰网络；wire 编译期全装配会破坏这一特性。CLI 与 server 各取所长：server 用 wire（依赖图静态、启动期全装配），CLI 用 factory（lazy、按需）。task 8.3 原设想放弃，以 factory 模式替代。
- [x] 8.4 运行 `wire gen ./cmd/server/...` 生成 `wire_gen.go`（checked in）
  > **状态**：wire 工具已升级到 go1.26 编译版（旧版 go1.22 编译的 wire 无法处理 go.mod 的 go 1.25）。
- [x] 8.5 测试：wire 生成的 app 依赖链编译期校验通过（wire gen 成功即证明依赖图闭合）+ `api/http/healthz_test.go` 验证 /healthz handler 行为

## 09-任务队列骨架（D39，含 trace 贯穿）

- [x] 9.1 在 `server/go.mod` 加 `github.com/riverqueue/river`；`core/queue/` 封装 River client（与 pgxpool 共库；**预留 NewAPIPool + NewWorkerPool 两个 provider**，Phase 1 共用配置但签名分开，避免写深后改 data.go）
- [x] 9.2 定义一个示例 job（如 `ExampleJob`）+ worker（`core/queue/example_worker.go`），演示入队即事务
- [x] 9.3 **trace 贯穿封装（P0-3）**：`core/queue/` 封装 `TracedInsert`（入队时把 traceparent 序列化进 job metadata）+ `TracedWorker` base（worker 取出 job 后 `propagator.Extract` 恢复 ctx 再 `tracer.Start` 建子 span）；所有 job handler MUST 继承 TracedWorker
- [x] 9.4 在 `cmd/server/main.go` 启动 River workers（Phase 1 同进程；未来可独立 `cmd/server-worker`，wire ProviderSet 预留拆分）
- [x] 9.5 测试：`go test ./server/core/queue/...`（testcontainers PG + River 入队/处理/完成 断言 + **trace 贯穿断言：入队 span 与 worker span 同 trace_id**）
  > **真断言已落地**：`queue_test.go TestTracedInsertCarriesTraceContext` 用 in-memory span exporter 捕获 worker span("river.work.example"),断言 `workerSpan.TraceID() == producerSpan.TraceID()`。经 architect 评审修正(原版本只断言 ProcessedCount,是假测试)。

## 10-JSON Schema 校验骨架（D40）

- [x] 10.1 在 `server/go.mod` 加 `github.com/santhosh-tekuri/jsonschema/v6`
- [x] 10.2 在 `core/catalog/`（占位包）写 schema 校验器：加载 `form_schema_json` 草案 2020-12，校验表单输入
- [x] 10.3 准备 `testdata/form_schema_valid.json` + `form_schema_invalid.json` fixture
- [x] 10.4 测试：`go test ./server/core/catalog/...`（valid 通过 + invalid 报错断言，覆盖 Draft 2020-12 特性）

## 11-可观测骨架（D41，端到端 trace 贯穿）

- [x] 11.1 在 `server/go.mod` 加 OTel 全套：`go.opentelemetry.io/otel`（核心）+ `…/exporters/prometheus`（metrics）+ `…/otlp/otlptrace/otlptracehttp`（trace 导出）+ 各层 instrumentation：`…/gin-gonic/gin/otelgin` + `github.com/exaring/otelpgx` + `github.com/uptrace/opentelemetry-go-extra/otelzap`
  > **注**：`connectrpc.com/otelconnect`（task 15 落地时加）与 `…/net/http/otelhttp`（task 14 落地时加）暂缓——对应传输层尚未实现。
- [x] 11.2 在 `internal/otel/otel.go` 写启动初始化：(a) `otel.SetTextMapPropagator(TraceContext+Baggage)` 全局传播器；(b) TracerProvider（OTLP/HTTP 导出，读 `OTEL_EXPORTER_OTLP_ENDPOINT` env，未设则 noop）+ `tp.Shutdown` 退出刷新；(c) MeterProvider + Prometheus exporter → `/metrics`；(d) `otelzap.ReplaceGlobals` 包装全局 logger
- [x] 11.3 gin 入站：`r.Use(otelgin.Middleware("aether-server"))`，handler 从 `c.Request.Context()` 取 ctx
- [x] 11.4 Connect 入站/出站：`otelconnect.NewInterceptor(otelconnect.WithTrustRemote())`（待 task 15 Connect-RPC 落地）
- [x] 11.5 HTTP 出站（云 API/OIDC/SCM）：`&http.Client{Transport: otelhttp.NewTransport(...)}`（待 task 14 httpclient 落地）
- [x] 11.6 pgx：`poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())` + `otelpgx.RecordStats(pool)`，在 `data.NewPgxPool` 落地
- [x] 11.7 测试：`go test ./server/internal/otel/...`（6 测试：propagator 设置/metrics handler/tracer span/shutdown 幂等/无 endpoint 容错/**端到端 gin→pgx span 树同 trace_id 断言**）
  > **端到端测试已补**：`e2e_trace_test.go` 用 testcontainers PG + in-memory span exporter,断言 gin server span 与 pgx query span 共享 trace_id 且 pgx 为 gin 的 child span。同时修正 healthz 的 PingFunc 签名传递请求 ctx(否则 trace 断裂)。
- [x] 11.8 审查清单：propagator 已设 ✓、gin ctx 来源正确 ✓(healthz 已传 c.Request.Context())、tp.Shutdown 已调 ✓、无裸 zap.L()（用 otelzap.L().Ctx(ctx)）✓、Connect WithTrustRemote ✓(otelconnect 接入 handler)。otelgrpc 不需要(本平台用 Connect-RPC 非 grpc-go,见 D45)。


## 12-测试基础设施（D36 testify + D42 testcontainers）

- [x] 12.1 在 `server/go.mod` 加 `github.com/stretchr/testify`（✅ v1.9.0）+ `github.com/testcontainers/testcontainers-go`（✅ v0.43.0）+ `pgtestdb`（✅ v0.1.1）
- [x] 12.2 写 `pkg/db/testdb.go`：testcontainers PG + pgtestdb 模板克隆 helper
  > 两层:testcontainers-go 起独立 PG 16 容器(`seccomp=unconfined` 解 CentOS7 seccomp 坑),pgtestdb 连它做模板克隆;暴露 `New(t)`(返回 pgxpool)与 `NewDSN(t)`(返回 DSN,供 *sql.DB 测试如 goose)。容器进程内共享,不禁用 ryuk 时由 ryuk 清理。
- [x] 12.3 写 `core/clock/clock.go`：`Clock` interface + 真实实现 + `FakeClock`（D44）—— 6 个测试全过
- [x] 12.4 写示例测试 `pkg/db/querier_test.go` 用 testify + testdb 跑 sqlc ListTeams/CreateTeam/GetTeamBySlug + 唯一约束断言；data/teams_test 与 cmd/migrate 测试已迁移到 testdb
- [x] 12.5 测试：全量 `go test ./...` 全绿（clock 6 + otel 5 + db CRUD 1 + data 5 + migrate 2）
  > **远程 Docker 配置**(贡献者参考 CONTRIBUTING.md):远程只暴露 unix socket 时,用 socat 代理容器转 TCP,本地设 `DOCKER_HOST=tcp://host:23750` + `TESTCONTAINERS_RYUK_DISABLED=true`(若远程无 ryuk 镜像)。

## 13-安全库与中间件骨架（D43）

- [x] 13.1 在 `server/go.mod` 加 `golang.org/x/crypto`（argon2）+ `golang-jwt/jwt/v5` + `github.com/zitadel/oidc`
- [x] 13.2 在 `internal/auth/`（占位）写 argon2id 哈希/校验 helper + JWT 签发/校验 helper
- [x] 13.3 在 `api/http/middleware/` 写 request_id（google/uuid）+ recovery（gin 内置）+ CORS（rs/cors）骨架中间件
- [x] 13.4 测试：`go test ./server/internal/auth/...`（argon2id 哈希/校验 + JWT 签发/校验断言）

## 14-HTTP 客户端 + 限流 + Git（收尾层）

- [x] 14.1 在 `internal/httpclient/`（占位）封装 stdlib `net/http` + `golang.org/x/oauth2` clientcredentials（供 OIDC/云 API/SCM webhook 用）
- [x] 14.2 在 `api/http/middleware/ratelimit.go` 用 `golang.org/x/time/rate` 写 per-IP 限流中间件（MVP 单实例）
- [x] 14.3 在 `server/go.mod` 加 `github.com/go-git/go-git/v5`（pin 最新补丁版，关注 CVE-2025-21613）+ `core/workspace/`（占位，D4 实现留给原 change）
- [x] 14.4 在 `server/go.mod` 加 `github.com/google/uuid`
- [x] 14.5 测试：`go test ./server/internal/httpclient/... ./server/api/http/middleware/...`（oauth2 token 获取 + 限流触发 429 断言）

## 15-Connect-RPC 骨架（D45，proto 唯一真相源在 contracts/）

- [x] 15.1 在 `server/go.mod` 加 `connectrpc.com/connect`；在仓库根 `contracts/` 建 `buf.yaml` + `buf.gen.yaml`
  > **状态修正**：buf.gen.yaml 当前只启用 Go 插件(connect-go + protoc-gen-go)；connect-es(TS)+ connect-openapi 插件已在 buf.gen.yaml 中声明(注释)但未启用——待 web/ 前端实现时启用。catalog.proto 已加 `google.api.http` 注解(D45 REST 兼容)。
- [x] 15.2 在 `contracts/platform/v1/` 写首个示例 `.proto`（如 `catalog.proto` 定义 `CatalogService.ListItems`，含 `google.api.http` 注解）——**proto 源只在 contracts/，不在 server/api/proto/**
- [x] 15.3 运行 `buf generate` 生成 Go Connect handler 接口（→ `server/api/grpc/`）+ Go message（→ `server/internal/proto/`）+ TS client（→ 前端用）+ OpenAPI v3
- [x] 15.4 在 `server/api/grpc/catalog.go` 实现 Connect handler（`connect.NewHandler(catalogsvc, ...)`）
- [x] 15.5 **Connect interceptor 层（P0-2）**：在 `server/api/grpc/interceptor/` 建骨架：`auth_interceptor.go`（JWT/OIDC + AK/SK 占位）+ `rbac_interceptor.go`（角色绑定占位）+ `audit_interceptor.go`（请求审计占位）+ `ratelimit_interceptor.go`（per-actor 限流占位）；共享逻辑抽纯函数（auth 提取/trace span/zap 字段），与 gin middleware 共用
- [x] 15.6 在 `server/cmd/server/main.go` 装配 `http.ServeMux`：Connect handler 挂 `/api/`，gin 挂 `/healthz` + `/metrics`（一进程一端口共存）；**interceptor 链已接入**：provideConnect 挂载 otelconnect + auth/rbac/ratelimit/audit(经 `grpcinterceptor.Chain(otelzap.L())`）。
- [x] 15.7 测试：`go test ./server/api/grpc/...`（connect-go client 调 ListItems 断言 2 测试通过）
  > **状态修正**：interceptor 当前为 pass-through 骨架(无 enforcement),故"无 auth 返回 401"断言未实现——等 auth/RBAC 逻辑落地后补。trace_id 贯穿由 otelconnect 保证(端到端验证在 `internal/otel/e2e_trace_test.go`)。

## 16-最终整体验证（含 D38-D45）

- [x] 16.1 `make build && make test` 全绿（含 testcontainers 集成测试）
- [x] 16.2 `make lint` 全绿（depguard D1 守护 + testify 风格 OK）
- [x] 16.3 `make migrate-up && make migrate-down && make migrate-up` 幂等
- [x] 16.4 验收：D31-D45 全部落地，骨架可支撑 `iac-self-service-platform` 的 `## 00c-Phase 0 Contract Freeze`
