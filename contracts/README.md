# contracts/

Aether 平台的 **API 契约唯一真相源**（proto-first, Connect-native）。

## Connect-native 原则

proto 文件是唯一契约源。不维护 openapi.yaml / schemas/*.json / protocol-mapping.md。
Connect-RPC 天然支持 gRPC / gRPC-Web / Connect-JSON 三种协议，一个 handler 全覆盖。

```
contracts/platform/v1/*.proto    ← 唯一真相源（人只维护这个）
       │
       │  buf generate（2 个插件）
       ▼
  server/internal/proto/*.pb.go          ← Go message 类型
  server/internal/proto/.../*.connect.go ← Connect service 接口（handler + client）
       │
       │  前端（未来）
       ▼
  web/src/gen/*.ts                       ← connect-es TypeScript client
```

## 目录结构

```
contracts/
├── platform/v1/          proto service 定义（唯一契约源）
│   ├── request.proto     RequestService（创建/查询工单/事件）
│   ├── planning.proto    PlanningService + ArtifactService（plan 生成/artifact 查询）
│   ├── approval.proto    ApprovalService + ApplyService（审批/apply 执行）
│   ├── catalog.proto     CatalogService（服务目录查询）
│   └── entitlement.proto EntitlementService（云账号查询）
├── error-codes.yaml      错误码注册表（proto 表达不了 remediation/owner）
├── fixtures/             测试数据（状态机/适配器/骨架 seed）
├── buf.yaml              buf 模块配置
├── buf.gen.yaml          buf 代码生成配置
└── README.md             本文件
```

## 为什么没有 openapi.yaml？

Connect-RPC 原生支持 curl / Postman（Connect/JSON 协议），不需要 REST 文档。
接口文档 = proto 文件本身（最精确的接口定义）。

如果未来需要 REST 文档给非 Connect 客户端，可以启用 buf.gen.yaml 里的
protoc-gen-openapi 插件从 proto 自动生成——但不手写。

## 工具链

```bash
cd contracts
buf lint                # 检查 proto 规范
buf generate            # 生成 Go 代码到 server/internal/proto/
# 未来：buf generate 也生成 TS client 到 web/src/gen/
```
