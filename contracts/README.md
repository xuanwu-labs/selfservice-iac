# contracts/

Aether 平台的 **API 契约唯一真相源**(proto-first,前后端共享)。

> **Status**: 规划中,`.proto` 尚未填充。

## ★ 单一真相源原则

本目录是 `.proto` / OpenAPI 定义的**唯一来源**。**禁止**在后端 `server/` 或前端 `web/` 内手写或修改契约——所有契约变更必须在此发起。

## 数据流(生成方向,单向不可逆)

```
contracts/                        ← 唯一 .proto 源(人编辑这里)
  └─ platform/v1/*.proto
        │
        │  buf generate(buf.yaml + buf.gen.yaml 驱动)
        ▼
  ┌─────────────────────────────────────────────┐
  │                                             │
  ▼                  ▼                          ▼
server/internal/proto/   server/api/grpc/      web/(TS 客户端)
  *.pb.go(messages)      Connect 服务接口       *.ts(由 connect-es 生成)
```

| 产物 | 位置 | 内容 | 谁消费 |
|---|---|---|---|
| Go messages | `server/internal/proto/*.pb.go` | protobuf message 定义 | server 内部 |
| Go Connect 接口 | `server/api/grpc/` | Connect service handler 接口 | server 的 gRPC handler |
| TS 客户端 | `web/`(未来) | connect-es 客户端 | 前端 |

**为什么 `server/api/proto/` 不存在**:proto 源只此一处(contracts/)。后端不再保留 proto 源副本,避免双源漂移。

## 目录结构

```
contracts/
├── buf.yaml                 # ⏳ buf 模块配置(待 task 15 建立)
├── buf.gen.yaml             # ⏳ 生成插件配置(connect-go + connect-es,待 task 15)
├── platform/
│   └── v1/
│       └── *.proto          # ⏳ 首批服务定义(待 task 15.2)
└── README.md
```

## 版本演进

按 `platform/v1`、`platform/v2` ... 目录做 API 版本演进。新版本不破坏旧版本,buf 的 breaking-change 检测(BREAKING lint)在 CI 强制。

## 工具链(待 task 15 落地)

```bash
# 在仓库根目录
buf lint contracts/                    # 检查 proto 规范
buf breaking contracts/ --against=.git  # 检查向后兼容
buf generate                           # 生成 Go + TS 代码到 server/ 与 web/
```

`server/Makefile` 的 `proto-gen` 目标调用 `buf generate`。

## 相关文档

- Connect-RPC 决策:`openspec/changes/platform-tech-stack-and-scaffold/`(D45)
- 脚手架 task 15:`.../tasks.md`(proto 源 + Connect handler + 拦截器链)
