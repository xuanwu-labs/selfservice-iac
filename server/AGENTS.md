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

## Service 三层模型(Server / Admin / Internal)

所有 proto service 归入三层之一,决定认证、授权和网络暴露。

| 层 | 谁调 | 认证 | proto 位置 |
|---|---|---|---|
| **Server**(用户) | 终端用户(Web/CLI/AI) | OIDC JWT + RBAC | request/catalog/planning/approval/entitlement/dependency |
| **Admin**(管理员) | 平台管理员 | OIDC JWT + **admin 角色** | admin.proto(ModuleRegistry + CatalogAdmin) |
| **Internal**(内部) | 平台内部逻辑(codegen/executor/CMDB/drift) | **无 RPC**——进程内直接函数调用 | 不需要 proto |

**为什么没有 InternalServer proto**:内部操作(codegen/executor/CMDB ingest/drift scan)是同一 Go 进程内的**函数调用**(D39 直接调用原则),不暴露 Connect-RPC。

**Admin 权限拦截**:Connect 拦截器按 service 名前缀校验 admin 角色:
- `ModuleRegistryService.*` → 需要 admin 角色
- `CatalogAdminService.*` → 需要 admin 角色

**判断标准**:
- 终端用户调的 → Server
- 管理员配置的 → Admin
- 平台内部自己调的 → Internal(函数调用,不写 proto)

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
| `TESTCONTAINERS_RYUK_DISABLED` | 禁用 ryuk reaper(远程 Docker 必设) |

## 远程 Docker 测试环境操作流程

测试依赖 testcontainers-go 启动 PG 容器，需要 Docker daemon。本地无 Docker 时，
通过远程 Docker daemon（socat TCP proxy）运行。远程主机：`192.168.31.33`（CentOS 7）。

### 架构

```
本机 (Windows)                          远程主机 192.168.31.33 (CentOS 7)
┌─────────────────┐                    ┌──────────────────────────────────┐
│ go test         │  DOCKER_HOST ──→   │ socat proxy (容器)               │
│ testcontainers  │  tcp:23750         │   0.0.0.0:23750 → /var/run/      │
│                 │                    │                     docker.sock   │
│                 │                    │ Docker daemon (主机服务)          │
│                 │                    │   ├─ aether-postgres (dev PG)     │
│                 │                    │   └─ postgres:16-alpine (测试 PG) │
└─────────────────┘                    └──────────────────────────────────┘
```

- **socat proxy**：远程 Docker daemon 只监听 Unix socket，socat 容器把它转成 TCP 端口 23750
- **dev PG**（aether-postgres）：开发用的持久 PG（端口 5433），config.yaml 指向它
- **测试 PG**（testcontainers 临时创建）：每个测试隔离的 PG 实例，测试结束自动销毁

### 环境变量（在 `server/.env` 或 shell 中设置）

```bash
# 远程 Docker daemon 的 socat TCP proxy 地址
export DOCKER_HOST=tcp://192.168.31.33:23750
# 远程 Docker 无法拉取 ryuk 镜像时必须禁用（ryuk 是 testcontainers 的清理 sidecar）
export TESTCONTAINERS_RYUK_DISABLED=true
```

`.env.example` 已包含这两个变量的模板。`.env` 文件被 gitignore，需手动创建：
```bash
cp server/.env.example server/.env  # 然后按需修改
```

### 首次部署（在远程主机上执行一次）

通过 SSH 连到远程主机后执行：

```bash
ssh root@192.168.31.33

# 1. 确保 Docker 服务运行
sudo systemctl start docker
sudo systemctl enable docker  # 开机自启

# 2. 部署 socat proxy（Docker API TCP 桥）
docker run -d --name docker-api-proxy --restart unless-stopped \
  -p 23750:2375 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  alpine/socat -d -d TCP-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

# 3. 部署 dev PG（CentOS 7 必须 seccomp=unconfined，否则 PG 16 initdb 失败）
docker run -d --name aether-postgres --restart unless-stopped \
  -p 5433:5432 \
  --security-opt seccomp=unconfined \
  -e POSTGRES_USER=aether \
  -e POSTGRES_PASSWORD=aether_dev_2026 \
  -e POSTGRES_DB=aether_dev \
  postgres:16-alpine
```

- `--restart unless-stopped`：主机重启后自动恢复（两个容器都设了）
- 端口 23750 是约定（避免与 Docker TLS 2376 冲突）
- `seccomp=unconfined`：CentOS 7 默认 seccomp 拒绝 PG 16 initdb 的 syscall

### 日常验证连通性

```bash
# 从本机检查 Docker proxy 可达
curl -s http://192.168.31.33:23750/_ping  # 应返回 OK

# 从本机检查 dev PG 可达
# (无 psql 时用 Go 的 /dev/tcp 探测)
timeout 2 bash -c "echo > /dev/tcp/192.168.31.33/5433" && echo "PG OPEN" || echo "PG CLOSED"
```

### 跑测试

```bash
cd server

# 短测试（不依赖 Docker，快速验证逻辑）
make test
# 等同: go test -race -short ./...

# 全量测试（含 testcontainers PG，需 Docker proxy 可达）
make test-db
# 等同: go test -race -p 1 ./...

# 只跑迁移测试
go test -v -count=1 -timeout=180s ./cmd/migrate/...
```

`make test-db` 需要 `DOCKER_HOST` + `TESTCONTAINERS_RYUK_DISABLED` 已在环境中
（`.env` 或 shell export）。Makefile 不自动加载 `.env`——在 shell 里 `source .env`
或直接 export。

### 故障排查

**症状：端口 23750 或 5433 不可达**

```bash
# 1. SSH 到远程主机检查
ssh root@192.168.31.33

# 2. 检查 Docker 服务
systemctl is-active docker  # 应为 active

# 3. 检查容器状态
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
# 应看到 docker-api-proxy (Up) + aether-postgres (Up)

# 4. 如果容器没了（被误删或机器重启后没恢复），重建：
docker run -d --name docker-api-proxy --restart unless-stopped \
  -p 23750:2375 -v /var/run/docker.sock:/var/run/docker.sock \
  alpine/socat -d -d TCP-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

docker run -d --name aether-postgres --restart unless-stopped \
  -p 5433:5432 --security-opt seccomp=unconfined \
  -e POSTGRES_USER=aether -e POSTGRES_PASSWORD=aether_dev_2026 \
  -e POSTGRES_DB=aether_dev postgres:16-alpine
```

**症状：testcontainers 报 "no space left on device"**

测试容器积累占满了远程主机磁盘：
```bash
ssh root@192.168.31.33 "docker container prune -f && docker image prune -f"
```

**症状：testcontainers 报 ryuk 超时**

远程 Docker 拉不到 ryuk 镜像。确认 `TESTCONTAINERS_RYUK_DISABLED=true` 已设置。

### 注意事项

1. **不要误删 socat proxy 容器**：清理测试容器时**只删 postgres 测试容器**，不删
   `docker-api-proxy` 和 `aether-postgres`。testcontainers 创建的容器名含 `testdb_` 前缀，
   可据此区分。
2. **磁盘空间**：远程主机磁盘有限（CentOS 7 虚拟机），testcontainers 每次测试创建
   临时 PG 容器，正常会自动销毁。如果 ryuk 被禁用（`RYUK_DISABLED=true`），临时容器
   不会自动清理——定期手动 prune。
3. **CentOS 7 seccomp**：PG 16 的 initdb 用了 CentOS 7 默认 seccomp 拒绝的 syscall。
   `testdb.go` 的 `startContainer()` 已设 `seccomp=unconfined`（针对 testcontainers
   创建的 PG）；dev PG 容器也需手动加 `--security-opt seccomp=unconfined`。
4. **Makefile 不自动加载 .env**：`make test-db` 不会读 `.env` 文件。需要在 shell 里
   `export DOCKER_HOST=...` 或 `source .env` 后再跑。

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
