# Aether Server

Aether 平台的 Go 后端 —— 提供 HTTP API、`aether` CLI 与数据库迁移。

> 工程契约(代码规范、D1 边界、目录分层)见 [`AGENTS.md`](AGENTS.md)。

## 二进制

| 二进制 | 入口 | 用途 |
|---|---|---|
| `server` | `cmd/server/` | HTTP API 服务(gin) |
| `aether` | `cmd/aether/` | 平台 CLI(cobra) |
| `migrate` | `cmd/migrate/` | 数据库迁移(goose,embed SQL) |

## 环境要求

- **Go 1.25+**(对齐 Terramate 上游)
- **PostgreSQL 14+**
- 开发工具:sqlc、wire、buf(可选,改契约时)、golangci-lint v2

## 快速开始

```bash
# 1. 编译
make build              # 产出 bin/server, bin/aether, bin/migrate

# 2. 准备数据库(默认 DSN 见 Makefile,或设 AETHER_DB_DSN)
export AETHER_DB_DSN="postgres://user:pass@host:5432/aether_dev"
make migrate-up         # 应用迁移

# 3. 运行
./bin/server            # 启动 HTTP API(默认 :8080,/healthz /ready)
./bin/aether            # CLI

# 4. 测试(需 PG)
make test
```

## 代码生成

本项目用代码生成管理三处:

```bash
make sqlc-gen           # pkg/db/queries/*.sql → pkg/db/generated/(SQL 即真相源)
make wire-gen           # cmd/*/wire.go → wire_gen.go(编译时 DI)
make proto-gen          # contracts/ → Connect-RPC stub(契约源在仓库根 contracts/)
```

**修改 SQL 后必须 `make sqlc-gen`**;修改 wire ProviderSet 后必须 `make wire-gen`。

## 目录速览

```
server/
├── cmd/{server,aether,migrate}/   # 二进制入口
├── api/{http,connect}/           # 传输层 handler
├── core/<domain>/                 # 领域核心(顶层一等公民,规划中)
├── data/                          # 数据访问层(pgxpool + repo/ Repo struct + dbset/ + query_wrapper)
├── internal/                      # 私有基建(config/cli/cmdutil/...)
└── pkg/db/{queries,generated}/    # sqlc 输入输出
```

完整目录约定与 `core/` vs `internal/` vs `data/` vs `pkg/db/` 的边界规则,见 [`AGENTS.md`](AGENTS.md)。

## 相关文档

- 工程契约:[`AGENTS.md`](AGENTS.md)
- 贡献指南:[`../CONTRIBUTING.md`](../CONTRIBUTING.md)
- 完整设计与决策(D1–D45):[`../openspec/`](../openspec/)
