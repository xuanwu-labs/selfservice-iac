# Aether 架构概览

> 本文档是用户文档(区别于 `openspec/` 设计过程文档)。精简版架构说明,完整设计决策见 `openspec/`。

## Monorepo 结构

```
selfservice-iac/
├── server/         Go 后端(HTTP + CLI + Connect-RPC)
├── web/            React 前端(规划中)
├── contracts/      API 契约源(proto-first,buf 管理)
├── deploy/         部署清单(Dockerfile / k8s / compose)
├── docs/           用户文档
└── openspec/       设计过程文档(spec-driven 工作区)
```

## server/ 分层

```
cmd/server/main.go          极简启动器(OTel → wire → Server.Run → Shutdown)
cmd/server/wire.go          纯 ProviderSet 聚合(无业务逻辑)
internal/server/            统一启动层
  ├── server.go             NewServer(Option 模式)+ Run + Shutdown
  ├── http.go               gin HTTP server(中间件由 Option 注入)
  └── connect.go            Connect-RPC 组装(拦截器 + handler 注册)
internal/middleware/        横切关注点(gin 中间件 + Connect 拦截器 + Option)
api/connect/                Connect-RPC handler(CatalogService 等)
api/http/                   gin handler(healthz/metrics)
core/<domain>/              领域核心(package-by-feature)
data/                       数据访问层(pgxpool + dbset)
pkg/db/                     sqlc 输入输出
internal/{config,otel,auth}  基建
```

## 传输层:一个端口服务所有协议

```
:8080 (一个 http.Server, TCP 端口)
  ├── /healthz /ready /metrics → gin handler (HTTP/1.1, 运维端点)
  └── /api/...                  → Connect handler (gRPC 线协议)
       ├── grpc-go 客户端      → 能连
       ├── gRPC-Web 浏览器     → 能连(Connect 原生,不需 Envoy)
       ├── Connect-JSON curl   → 能连
       └── connect-es 前端     → 能连
```

Connect-RPC 路由统一以 `/api/` 前缀挂载(如 `/api/aether.platform.v1.CatalogService/ListItems`),和 gin 运维端点(`/healthz`、`/metrics`)层级分离。

- Connect 可通过 `AETHER_CONNECT_ENABLED=false` 关闭(纯 HTTP 网关模式)
- 没有 `grpc.NewServer` / 独立 gRPC 端口 — Connect 在 HTTP 上说标准 gRPC 线协议

## 设计模式选择

| 场景 | 模式 | 理由 |
|---|---|---|
| server 启动装配 | DI (wire) | 依赖静态已知,编译期检查 |
| CLI 依赖构造 | Factory (lazy) | --help 不触网 |
| 中间件/拦截器 | Option 模式 | WithGinMiddleware/WithConnectInterceptor 可插拔 |
| 无依赖简单构造 | Factory 返回接口 | clock.New() → Clock,测试换 Fake |
| 异步耗时作业 | Job Queue (River) | 持久化 + 重试 + trace 贯穿 |
| 进程内事件通知 | 直接函数调用 | Phase 1 单进程,不引入 EventBus |

## OTel 端到端 trace

```
HTTP 请求 → otelgin(server span)
  → handler → otelpgx(query span, 同 trace_id)
  → handler → otelhttp(出站 span, 同 trace_id)
  → handler → TracedInsert(写入 River job metadata)
    → worker → TracedWorker(提取 traceparent, worker span 同 trace_id)
Connect-RPC → otelconnect(server/client span, 同 trace_id)
```

logger 全链路使用 `*otelzap.Logger`(wire 注入,带 trace_id)。

## 测试基础设施

- `make test` — 短测试(无需 Docker)
- `make test-db` — 全量测试(testcontainers PG,远程 Docker via DOCKER_HOST)
- pgtestdb 模板克隆:每测隔离 DB,毫秒级
- Clock 注入:时间相关逻辑可确定性测试(FakeClock)

## 配置加载

四层优先级(业界标准):

```
flag (-config, -env)          ← 最高:配置文件路径 + 环境
    ↓
环境变量 (AETHER_*)            ← 敏感值 + 部署覆盖
    ↓
config.yaml                   ← 非敏感默认值
    ↓
内置默认值                     ← 最低
```

敏感值(密码、JWT secret)可在 yaml 或 env 里设置;env 优先(推荐生产用)。

## 文档体系

| 文档 | 职责 | 面向 |
|---|---|---|
| `server/AGENTS.md` | 工程实践(代码规范、DI/Factory/Option、config 加载、Connect 扩展、优雅关闭) | 写代码时读 |
| `openspec/config.yaml` | 流程规则(分支策略、commit 格式、proposal/specs 格式) | 做提案时读 |
| `docs/architecture.md` | 架构概览(本文件) | 理解项目时读 |
| `docs/` | 用户文档(快速上手、CLI、运维) | 使用平台时读 |
| `openspec/` | 设计过程文档(proposal/design/tasks/specs) | 做设计决策时读 |
