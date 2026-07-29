## ADDED Requirements

### Requirement: React + Ant Design 前端应用

平台 MUST 在 `web/` 目录实现 React 前端应用（Vite + React 19 + TypeScript + Ant Design 6）。通过 Connect-ES + TanStack Query 调用后端 API，RJSF 动态渲染表单，go:embed 单二进制部署。

#### Scenario: 服务目录页面

- **WHEN** 用户打开服务目录页面
- **THEN** TanStack Query 调 CatalogService.ListItems 获取资源列表
- **AND** Ant Design Table 渲染（名称/分类/层级/团队/状态）
- **AND** 点"申请"打开 RJSF 动态表单弹窗

#### Scenario: RJSF 动态表单

- **WHEN** 用户点击"申请 RDS MySQL"
- **THEN** 获取 catalog_item.form_schema_json
- **AND** RJSF 自动渲染表单（instance_type/ttl 暴露，secret_key/vswitch_id 隐藏）
- **AND** 用户填写后提交 → LifecycleService.CreateRequest

#### Scenario: 工单列表轮询

- **WHEN** 用户在"我的工单"页面
- **THEN** TanStack Query 每 5 秒轮询 LifecycleService.ListRequests
- **AND** 状态变化自动更新（pending_approval → applying 等）

#### Scenario: 审批操作

- **WHEN** 审批人在审批中心点击"批准"
- **THEN** 调用 LifecycleService.DecideApproval（decision=approved）
- **AND** 工单状态从 pending_approval → applying

### Requirement: go:embed 单二进制部署

平台 MUST 通过 go:embed 将前端构建产物（web/dist/）嵌入 Go 二进制。单一二进制同时 serve API + 前端。

#### Scenario: 单二进制部署

- **WHEN** `cd web && npm run build` → `go build`
- **THEN** 生成的二进制包含前端静态资源
- **AND** 浏览器访问 / 返回前端 SPA
- **AND** 浏览器访问 /api/ 走 Connect-RPC API
