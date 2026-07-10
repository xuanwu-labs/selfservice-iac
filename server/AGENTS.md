# AGENTS.md — Aether Server

> 本文件是 Aether 后端(`server/`)的工程契约,适用于所有在此目录下工作的 AI agent 和人类贡献者。

## 代码规范

遵循 [Google Go Style Guide](https://google.github.io/styleguide/go/) + [Effective Go](https://golang.org/doc/effective_go)。关键约定：

- **命名**：包名短小写单词（`registry`、`catalog`、`codegen`），导出标识符用驼峰（`CatalogService`）
- **错误处理**：`fmt.Errorf("context: %w", err)` 包装错误；定义 sentinel error（`ErrNotFound`）；在 handler 层用 `errors.Is/As` 判断
- **接口**：消费侧定义小接口（1-3 方法），在调用方包内定义，不预定义大接口
- **测试**：表驱动 + `t.Run` subtest + testify assert/require（不用 testify/mock）；用 `t.Helper`/`t.Cleanup`/`t.TempDir`
- **Context**：所有可能跨边界的函数第一个参数是 `ctx context.Context`

## D1 边界（关键架构守护）

**禁止 `server/**` import `github.com/terramate-io/terramate/<任何子包>`。**

平台通过 exec 调用 terramate CLI 二进制(`core/terramate` 适配器),**不直接 import terramate 内部包**。

此规则由 `.golangci.yml` 的 depguard **模块前缀 deny** 强制。

> **已知局限(task 6.5 未解决)**:depguard 仅在 `terramate-io/terramate` 存在于 go.mod 时才可执行——若 terramate 不在依赖里,Go typechecker 在 depguard 运行前就报错(`no required module provides package`),golangci-lint 会静默丢弃 depguard 的发现。由于 D1 禁止 terramate 入 go.mod,当前 depguard 规则**实际未生效**。完整修复(待 CI):写一个独立 lint 测试(如 `internal/audit/d1_guard_test.go`),在临时模块 `go mod download` terramate 后跑 `golangci-lint` 子进程 + 断言 exit code。task 6.5/7.2/16.2 中"depguard D1 守护生效"的表述以此局限为准。

**未来内嵌 carve-out**(D1 保留):如果未来需要内嵌实现,在 `core/terramate/embedded`(带 `//go:build terramate_embedded` 标签)显式 allow,depguard allow-list 默认空。

## 目录结构(领域核心顶层 / package-by-feature)

```
server/
├── cmd/{server,aether,migrate}/     # 二进制入口(HTTP server / CLI / 迁移)
├── api/{http,connect}/              # 传输层 handler(契约在仓库根 contracts/)
├── core/<domain>/                   # 领域核心(顶层一等公民,package-by-feature)
├── data/                            # 数据访问层(pgxpool provider + dbset 表分组包装)
├── internal/                        # 只装私有基建(config/otel/model/proto/cli/cmdutil/...)
├── pkg/db/{queries,generated}/      # sqlc 输入输出(可复用数据层)
└── cmd/migrate/migrations/          # goose 迁移(embed)
```

### core/ vs internal/ vs data/ vs pkg/db/ 边界

| 目录 | 职责 | 内容 |
|---|---|---|
| `core/<domain>/` | **领域核心**(业务逻辑) | registry/catalog/codegen/orchestrator/drift 等领域包,顶层一等公民 |
| `data/` | **数据访问层** | `data.go`(pgxpool provider + wire ProviderSet)、`dbset/`(按表分组的薄包装) |
| `pkg/db/` | **sqlc 生成层**(可复用) | `queries/`(SQL 源)、`generated/`(sqlc 输出) |
| `internal/` | **私有基建**(不外露) | config/otel/model/proto/mapping/errors/event/auth/httpclient/asset/utils/**cli/cmdutil** |

**数据访问三件套**:`core/store`(薄包装,调用方)→ `data/`(pgxpool 池)→ `pkg/db/generated`(sqlc 生成的 `*Queries`)。

## 依赖注入（wire）

各层用 `wire.NewSet(...)` 定义 ProviderSet。`cmd/*/wire.go` 用 `//go:build wireinject` 触发 `wire gen`。

- **DI 用于启动期装配**:server 的依赖图(config→data→core→api→connect→server)在启动时全部已知,用 wire 编译期检查 + 无反射装配。
- **不加空 ProviderSet**:如果包当前无 provider 可注入(auth/httpclient 等待 config 字段就绪),不要加空 `wire.NewSet()`——空集是仪式主义,等真正有依赖时再加。

## 设计模式与原则

本项目因地制宜选择模式,不为模式而模式。

| 场景 | 模式 | 理由 |
|---|---|---|
| server 启动装配 | **DI (wire)** | 依赖静态已知,编译期检查,无反射可审计(D28) |
| CLI 依赖构造 | **Factory (lazy)** | `--help` 不触网;Factory 按需构造,不到运行时不初始化 |
| 中间件/拦截器组合 | **Option 模式** | `WithGinMiddleware()` / `WithConnectInterceptor()` 可插拔 |
| 无依赖简单构造 | **Factory 返回接口** | `clock.New() → Clock` 接口,测试换 FakeClock |
| 异步耗时作业 | **Job Queue (River)** | 持久化 + 重试 + trace 贯穿;InsertTx 与 DB 事务原子 |
| 进程内事件通知 | **直接函数调用** | Phase 1 单进程,不引入 EventBus/Observer(过度设计) |

**判断标准**:
- 依赖在启动时全部已知 → DI(wire)
- 依赖需要 lazy 或运行时选择 → Factory
- 中间件/拦截器可增减 → Option
- 持久化异步任务 → Job Queue

**Go 接口原则**:Go 的接口是隐式的(结构化类型),不需要 Java 式工厂层级。`NewXxx() Interface` 就够。消费侧定义小接口(1-3 方法),不预定义大接口。

## 数据层架构

四层模型分离(DDD 规范):

| 层 | 位置 | 职责 | 当前状态 |
|---|---|---|---|
| sqlc 生成 model | `pkg/db/generated/models.go` | DB 表的 struct(sqlc 自动) | ✅ teams 表 |
| 手写 entity | `internal/model/entity/` | 额外字段/校验标签/表名方法 | 空(teams 不需要额外字段) |
| dbset 薄包装 | `data/dbset/` | 组合 sqlc Queries + entity | 空(等 core/store 落地) |
| mapping 转换 | `internal/mapping/` | entity ↔ proto message | 空(等 handler 接 DB) |

**数据流**(未来完整):
```
Connect 请求(proto) → handler → core 业务逻辑 → dbset → pkg/db generated → PostgreSQL
                                                              ↓
                                            entity → mapping → proto 响应
```

## 配置加载策略

四层优先级(业界标准, 12-Factor / Spring Boot / viper):

```
flag (-config, -env)          ← 最高: 基础设施参数(配置文件路径, 环境)
    ↓ 覆盖
环境变量 (AETHER_*)            ← 敏感值(密码, JWT secret) + 部署覆盖
    ↓ 覆盖
config.yaml                   ← 非敏感默认值(端口, 日志级别, OTel 名)
    ↓ 兜底
内置默认值 (viper SetDefault)  ← 最低
```

**关键规则**:
- 敏感值(密码, JWT secret, API key) **只从环境变量来**, 不入 yaml/git
- config.yaml 只放非敏感默认值
- Config struct 只存数据, 不含行为(无 DSN() 方法等)
- `internal/config/config.go` 是唯一加载入口, server/migrate 都走 `config.Load()`

**常用配置项**:
| 环境变量 | 说明 |
|---|---|
| `AETHER_DATA_DATABASE_PASSWORD` | PG 密码(必填) |
| `AETHER_AUTH_JWT_SECRET` | JWT 签名密钥 |
| `AETHER_OTEL_ENDPOINT` | OTLP collector 地址(空=noop) |
| `AETHER_CONNECT_ENABLED` | Connect 开关(false=纯 HTTP) |
| `DOCKER_HOST` | testcontainers Docker 地址(测试用) |

## Connect 服务扩展指南

新增一个 Connect-RPC 服务(如 RequestService)的步骤:

1. **写 proto**: `contracts/platform/v1/request.proto`
2. **生成代码**: `cd contracts && buf generate`
3. **写 handler**: `api/connect/request.go` — 实现 `RequestServiceHandler` 接口
4. **注册到 ProviderSet**: `api/connect/provider.go` 加 `NewRequestHandler`
5. **注册到 mux**: `internal/server/connect.go` 的 `ProvideServerConfig` 加 `NewRequestServiceHandler` 调用 + `WithConnectHandler`
6. **重新生成 wire**: `make wire-gen`

不需要改 `internal/server/server.go` 或 `cmd/server/wire.go` — Option 模式自动处理。

## 优雅关闭

关闭顺序(必须按序, 反向初始化):
1. `httpServer.Shutdown(ctx)` — 停止接受新连接, 等 in-flight 请求完成
2. `otelSDK.Shutdown(ctx)` — flush trace spans + meter metrics
3. `pool.Close()` — 关闭 pgxpool(通过 wire cleanup defer)

超时: 10 秒(硬编码在 main.go, 未来可从 config 读)。

## 常用命令

```bash
make build          # 编译所有二进制
make test           # 运行测试（含 race detector）
make lint           # golangci-lint
make migrate-up     # 应用 DB 迁移
make sqlc-gen       # 生成 sqlc 代码
make wire-gen       # 生成 wire 装配代码
```
