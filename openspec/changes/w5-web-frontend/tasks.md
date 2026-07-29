## 1. 前端脚手架 + 依赖

- [ ] 1.1 `npm create vite@latest web -- --template react-ts`（React 19 + TypeScript）
- [ ] 1.2 安装依赖：antd@5 @connectrpc/connect-web @connectrpc/connect-query @tanstack/react-query @rjsf/core @rjsf/antd@5 @rjsf/validator-ajv8 zustand react-router-dom（P1-1：用 antd 5 而非 6，@rjsf/antd 兼容 antd 5）
- [ ] 1.3 配置 buf.gen.yaml：加 @connectrpc/protoc-gen-connect-es + @bufbuild/protoc-gen-es 插件（P1-3：connect-query 需要 connect-es 插件生成 typed hooks）+ `buf generate`
- [ ] 1.4 配置 Vite proxy（dev 时 /api → Go :8080）+ build.outDir: '../server/internal/web/dist'（P0-1：embed 路径在 module 内）

## 2. App Shell + 路由 + Auth

- [ ] 2.1 实现 `web/src/App.tsx`：Layout（Sider + Header + Content）+ React Router（6 个路由，P0-2：删除 DriftPage）
- [ ] 2.2 实现 `web/src/main.tsx`：Ant Design 5 ConfigProvider + TanStack QueryClientProvider
- [ ] 2.3 实现 `web/src/lib/transport.ts`：Connect-ES transport（P1-2：从 localStorage 读 Bearer token + 注入 Authorization header。Phase 1 无登录页，用户手动设置 token）

## 3. 6 个页面（P0-2：移除 DriftPage）

- [ ] 3.1 实现 CatalogPage（服务目录）：CatalogService.ListItems + Table + 统计 + 筛选 + 申请
- [ ] 3.2 实现 LayerCatalogPage（分层目录）：按 3 层分组 + 路径模板 + DAG
- [ ] 3.3 实现 ModulesPage（模块注册）：RegistryAdminService.RegisterModule/ListModules + 契约展示
- [ ] 3.4 实现 RequestsPage（我的工单）：LifecycleService.ListRequests + Tab + 状态 Tag
- [ ] 3.5 实现 ApprovalPage（审批中心）：ListPendingApprovals + DecideApproval
- [ ] 3.6 实现 RequestDetailPage（工单详情）：Steps + Timeline + Plan 差异

## 4. RJSF 动态表单

- [ ] 4.1 实现 `web/src/components/RequestForm.tsx`：RJSF（从 form_schema_json string → JSON.parse → Form schema）+ 提交 CreateRequest。注意：proto form_schema_json 是 string 类型，需 JSON.parse 转 object（P2）
- [ ] 4.2 实现 `web/src/components/RequestFormModal.tsx`：Modal 包裹 RequestForm

## 5. go:embed 集成（P0-1：embed 在 server/ 内部）

- [ ] 5.1 创建 `server/internal/web/embed.go`：`//go:embed dist/*` + Assets embed.FS + SPA handler（fallback index.html）
- [ ] 5.2 在 server 路由注册静态资源 handler（/ → SPA，/api/ → Connect-RPC）
- [ ] 5.3 验证 `cd web && npm run build` → 输出到 server/internal/web/dist/ → `go build` 成功

## 6. 验证

- [ ] 6.1 `cd web && npm run build` 通过
- [ ] 6.2 `go build ./...` 通过（go:embed 成功）
- [ ] 6.3 浏览器打开验证 6 个页面渲染
- [ ] 6.4 提交到 `feat/w5-web-frontend` 分支
