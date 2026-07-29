## Why

W5 前端是 MVP 的用户界面——把 W1+W2+W3 的后端 API 通过 React Web 暴露给用户。没有前端，用户只能通过 API/CLI 操作（Phase 2 的 aether CLI）。前端是 MVP"自助"核心——用户在浏览器里浏览目录、填表申请、审批、查看漂移。

**影响层级**：前端层（`web/`），不改后端代码。

**为什么现在做**：W1-W3+W5-e2e 全部归档（10 个模块），后端 API 全部就位（CatalogService/LifecycleService/RegistryAdminService）。前端选型已确认（Vite + React 19 + Ant Design 6 + RJSF + Connect-ES + TanStack Query）。

## What Changes

### 1. 前端脚手架（web/ 目录）

新建 `web/` 目录（镜像 server/ 命名）：
- **Vite + React 19 + TypeScript** 脚手架
- **Ant Design 6** UI 组件库
- **@connectrpc/connect-web + @connectrpc/connect-query**（proto → TS 类型安全）
- **TanStack Query v5**（服务端状态 + 轮询）
- **Zustand**（客户端 UI 状态）
- **React Router v7**（路由）
- **react-jsonschema-form (RJSF) + @rjsf/antd**（JSON Schema 表单渲染）

### 2. Connect-ES 类型生成

- buf 生成 TypeScript 类型（从 contracts/platform/v1/ proto）
- Connect-ES 客户端连接后端 Connect-RPC API

### 3. 7 个页面（MVP 界面已确认）

| 页面 | 功能 | API 调用 |
|------|------|---------|
| 📦 服务目录 | 浏览 + 分类筛选 + 申请按钮 | CatalogService.ListItems |
| 🗂️ 分层目录 | 按 Global/Middleware/Application 分层展示 | CatalogService.ListItems（按 layer 过滤） |
| 🔌 模块注册 | Admin 注册模块 + 列表 + 契约展示 | RegistryAdminService.RegisterModule/ListModules |
| 📋 我的工单 | 工单列表 + Tab 过滤 + 状态标签 | LifecycleService.ListRequests |
| ✅ 审批中心 | 待审批工单 + 批准/拒绝 | LifecycleService.ListPendingApprovals/DecideApproval |
| 🔍 工单详情 | 步骤条 + 时间线 + Plan 差异 | LifecycleService.GetRequest/ListRequestEvents |
| 📡 漂移检测 | 漂移记录 + 变更数 + 操作 | (Phase 2 drift API) |

### 4. 申请表单（RJSF 动态渲染）

- 从 catalog_items.form_schema_json 动态渲染表单
- RJSF + @rjsf/antd 主题
- 用户填写 → CreateRequest API

### 5. go:embed 集成

- Vite build → `web/dist/` 静态资源
- Go server `go:embed` 嵌入 → 单一二进制部署

### 不做

- 复杂图表/Dashboard（Phase 2）
- 国际化 i18n（Phase 2）
- WebSocket/SSE 实时推送（Phase 2，Phase 1 用轮询）
- 移动端适配（Phase 2）
- 暗黑模式（Phase 2）

## Capabilities

### New Capabilities

- `web-frontend`: React + Ant Design 前端应用。7 个 MVP 页面 + RJSF 动态表单 + Connect-ES 类型安全 API + TanStack Query 轮询。go:embed 单二进制部署。

## Impact

- **代码**：新建 `web/` 目录（React + TS + Vite）。后端加 go:embed 静态资源服务。
- **依赖**：Node.js + npm（前端构建）；@connectrpc/connect-web、antd、@rjsf/*、@tanstack/react-query、zustand、react-router-dom
- **构建**：`cd web && npm install && npm run build` → `web/dist/` → Go embed
- **proto**：buf 生成 TS 类型（从现有 contracts/）
