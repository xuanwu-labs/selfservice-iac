## Context

W5 前端是 MVP 的用户界面。

**技术选型已确认**（2026-07-29 调研）：
- Vite + React 19 + TypeScript（SPA，不需要 SSR）
- Ant Design 6（中国企业 B2B 标准）
- RJSF + @rjsf/antd（JSON Schema → React 表单）
- TanStack Query v5 + @connectrpc/connect-query（proto 类型安全 + 轮询）
- Zustand（轻量 UI 状态）
- React Router v7
- go:embed（单二进制部署）

**后端 API 已就位**：
- CatalogService（ListItems/GetCatalogItem）
- CatalogAdminService（PublishCatalogItem）
- RegistryAdminService（RegisterModule/ListModules）
- LifecycleService（CreateRequest/ListRequests/DecideApproval/...）

## Goals / Non-Goals

**Goals:**
- Vite 脚手架 + Ant Design + Connect-ES 集成
- 7 个 MVP 页面
- RJSF 动态表单
- go:embed 部署

**Non-Goals:**
- 不做 i18n / 暗黑模式 / 移动端（Phase 2）
- 不做 WebSocket 实时推送（Phase 1 轮询）
- 不做复杂 Dashboard 图表（Phase 2）

## Decisions

### D1：web/ 目录在 selfservice-iac/ 内（同仓 monorepo）

**决策**：前端放 `selfservice-iac/web/`，和 `server/` 同仓。

**理由**：
- proto 契约共享（contracts/ → Connect-ES 生成 TS）
- 单仓 git 版本同步
- 部署简单（web/dist → go:embed → 单二进制）

### D2：Connect-ES + TanStack Query 类型安全端到端

**决策**：用 @connectrpc/connect-web + @connectrpc/connect-query。

**数据流**：
```
Go proto 定义 → buf generate → TypeScript 类型 → connect-query hooks → React 组件
全程类型安全，零手写 API 类型。
```

### D3：RJSF 从 form_schema_json 动态渲染

**决策**：用 react-jsonschema-form + @rjsf/antd。

```tsx
<Form schema={catalogItem.formSchemaJson} validator={validator} onSubmit={handleSubmit} />
```

**理由**：零手写表单代码——加新资源自动出表单。

### D4：TanStack Query 轮询替代 WebSocket

**决策**：Phase 1 用 `refetchInterval: 5000` 轮询工单状态。

**理由**：
- Phase 1 单进程，轮询够用
- Phase 2 改 WebSocket/SSE 只需换 transport（TanStack Query 组件不变）

### D5：go:embed 单二进制部署（P0-1 修正：embed 路径在 module 内部）

**决策**：Vite build → `server/internal/web/dist/` → Go `//go:embed` 嵌入 → server 同时 serve API + 前端。

**P0-1 修正**：Go module root 是 `server/`，`//go:embed` 不支持 `..` 路径。Vite build 输出到 `server/internal/web/dist/`（在 module 内部）。

```go
// server/internal/web/embed.go
package web

import "embed"

//go:embed dist/*
var Assets embed.FS
```

Vite 配置 `build.outDir: '../server/internal/web/dist'`。

### D6：Ant Design 5（P1-1 修正：@rjsf/antd 兼容性）

**决策**：用 **Ant Design 5**（不是 6）。@rjsf/antd 历史上绑定 antd 5，antd 6 支持未验证。antd 5 稳定且企业级组件齐全。

### D7：漂移页移除（P0-2 修正）

**决策**：本 change **删除 DriftPage**。漂移没有 proto RPC（无 DriftService srv.proto）。Phase 2 补漂移 API + 前端页面。

### D8：登录 + Token（P1-2 修正）

**决策**：Phase 1 简化 token 获取：
- 用户在浏览器 DevTools 或 config 设置 Bearer token（OIDC 或 bootstrap admin token）
- 不做登录页面（Phase 2 加 OIDC authorization-code flow）
- Connect transport 从 localStorage 读 token + 注入 Authorization header

**理由**：自托管 OSS 平台最简部署（一个二进制 = 完整平台）。

## Risks / Trade-offs

- **[Node.js 构建依赖] → CI 加 npm install + npm run build**：Go 构建前先跑前端构建。
- **[Connect-ES 生成] → buf generate 加 TypeScript 插件**：buf.gen.yaml 加 connect-es 插件。
- **[Ant Design 6 体积] → 按需加载**：Vite tree-shaking + antd 自动按需。

## Migration Plan

1. `npm create vite@latest web -- --template react-ts`
2. 安装依赖（antd/connect-web/connect-query/rjsf/tanstack-query/zustand/react-router）
3. 配置 buf.gen.yaml（Connect-ES TypeScript 生成）
4. 实现 7 个页面
5. 实现 RJSF 申请表单
6. Vite build → go:embed
7. 验证端到端（前端调后端 API）

## Open Questions

- **Ant Design 6 还是 5？** Ant Design 6 已发布（React 19 兼容）。如果 6 不稳定用 5。
- **Connect-ES 需要后端 CORS 配置？** Phase 1 go:embed 同源（不需要 CORS）。Phase 2 分离部署才需要。
