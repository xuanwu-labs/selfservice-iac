# Aether — IaC 自服务平台

[![Status](https://img.shields.io/badge/status-early%20development-orange)]()
[![License](https://img.shields.io/badge/license-TBD-lightgrey)]()

> Aether 是一个构建在 [Terramate](https://github.com/terramate-io/terramate) 之上的基础设施即代码(IaC)自服务平台,提供模块注册表、服务目录、代码生成、审批流、漂移检测、CMDB/FinOps 与多云凭证管理等能力。

> ⚠️ **本项目处于早期开发阶段,API 与结构可能随时变动,暂不可用于生产。**

## 仓库结构

本仓库是 monorepo,包含平台的全部组件:

| 目录 | 说明 | 状态 |
|---|---|---|
| [`server/`](server/) | Go 后端(HTTP API + `aether` CLI + migrate) | 🚧 脚手架阶段 |
| [`web/`](web/) | React 前端 | 📋 规划中 |
| [`contracts/`](contracts/) | API 契约源(proto-first,前后端共享) | 📋 规划中 |
| [`deploy/`](deploy/) | 部署清单(Dockerfile / k8s / compose) | 📋 规划中 |
| [`docs/`](docs/) | 用户文档 | 📋 规划中 |
| [`openspec/`](openspec/) | 设计过程文档(spec-driven 工作区) | ✅ 完整 |

## 快速开始

### 后端

```bash
cd server
make build          # 编译所有二进制(server / aether / migrate)
make test           # 运行测试(需 PostgreSQL)
make migrate-up     # 应用 DB 迁移
./bin/server        # 启动 HTTP API
./bin/aether        # CLI
```

详见 [`server/README.md`](server/README.md) 与 [`server/AGENTS.md`](server/AGENTS.md)。

## 设计与架构

本项目采用 **OpenSpec** 规范驱动开发。完整的设计决策(D1–D45)、能力规格与架构文档见 [`openspec/`](openspec/)。

关键架构决策:

- **D1 边界**:平台通过 `exec` 调用 Terramate CLI,**不 import 其内部包**(由 depguard CI 硬门强制)。
- **技术栈**:Go + gin + sqlc/pgx + cobra + wire + Connect-RPC + OpenTelemetry(详见 `openspec/changes/platform-tech-stack-and-scaffold/`)。

## 贡献

本项目使用 OpenSpec 流程管理变更。新需求请先通过 `/opsx:propose` 创建 change,勿直接提交代码。详见 [`AGENTS.md`](AGENTS.md)。

## License

**待定(TBD)。** 在正式发布前确定。
