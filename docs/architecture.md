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
  ├── HTTP/1.1 → gin handler (healthz/metrics/webhook/curl)
  └── HTTP/2   → Connect handler (gRPC 线协议)
       ├── grpc-go 客户端      → 能连
       ├── gRPC-Web 浏览器     → 能连(Connect 原生,不需 Envoy)
       ├── Connect-JSON curl   → 能连
       └── connect-es 前端     → 能连
```

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
