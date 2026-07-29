## 1. 前端脚手架 + 依赖

- [ ] 1.1 `npm create vite@latest web -- --template react-ts`（React 19 + TypeScript）
- [ ] 1.2 安装依赖：antd @connectrpc/connect-web @connectrpc/connect-query @tanstack/react-query @rjsf/core @rjsf/antd @rjsf/validator-ajv8 zustand react-router-dom
- [ ] 1.3 配置 buf.gen.yaml 加 Connect-ES TypeScript 生成插件 + `buf generate`
- [ ] 1.4 配置 Vite proxy（开发时代理 /api → Go 后端 :8080）

## 2. App Shell + 路由

- [ ] 2.1 实现 `web/src/App.tsx`：Layout（Sider + Header + Content）+ React Router（7 个路由）
- [ ] 2.2 实现 `web/src/main.tsx`：Ant Design ConfigProvider + TanStack QueryClientProvider + Connect transport 初始化
- [ ] 2.3 实现 `web/src/lib/transport.ts`：Connect-ES transport（Bearer token 注入 + 基地址配置）

## 3. 7 个页面

- [ ] 3.1 实现 CatalogPage（服务目录）：TanStack Query 调 CatalogService.ListItems + Ant Design Table + 统计卡片 + 分类筛选 + 申请按钮
- [ ] 3.2 实现 LayerCatalogPage（分层目录）：按 Global/Middleware/Application 分组展示 + 路径模板 + DAG 图
- [ ] 3.3 实现 ModulesPage（模块注册）：注册表单 + ListModules Table + 模块契约展示（variables_contract_json）
- [ ] 3.4 实现 RequestsPage（我的工单）：ListRequests Table + Tab 过滤 + 状态 Tag + 操作按钮
- [ ] 3.5 实现 ApprovalPage（审批中心）：ListPendingApprovals + 批准/拒绝按钮 → DecideApproval
- [ ] 3.6 实现 RequestDetailPage（工单详情）：Steps + Timeline + Plan 差异代码块
- [ ] 3.7 实现 DriftPage（漂移检测）：统计卡片 + 漂移记录 Table + Badge

## 4. RJSF 动态表单

- [ ] 4.1 实现 `web/src/components/RequestForm.tsx`：RJSF Form（从 GetCatalogItem.form_schema_json 动态渲染）+ 提交 → CreateRequest
- [ ] 4.2 实现 `web/src/components/RequestFormModal.tsx`：Ant Design Modal 包裹 RequestForm

## 5. go:embed 集成

- [ ] 5.1 在 Go server 加 `//go:embed web/dist` 静态资源服务（SPA fallback 到 index.html）
- [ ] 5.2 验证 `cd web && npm run build` → `go build` → 单二进制 serve 前端 + API

## 6. 验证

- [ ] 6.1 `cd web && npm run build` 通过（Vite build 无错误）
- [ ] 6.2 `go build ./...` 通过（go:embed 成功）
- [ ] 6.3 浏览器打开验证 7 个页面渲染正确
- [ ] 6.4 提交到 `feat/w5-web-frontend` 分支
