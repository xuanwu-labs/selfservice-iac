# Aether MVP 整体操作流程（Phase 1 填锻指南）

> 本文档供你（维护者）按照步骤操作，验证 MVP 端到端。

## 环境准备

```bash
# 1. 确认工具链
node --version          # >= 18
go version              # >= 1.22
terramate --version     # >= 0.17（可选，e2e 测试用）
terraform --version     # >= 1.4（可选，e2e 测试用）

# 2. 构建前端
cd web
npm install
npm run build           # → server/internal/web/dist/

# 3. 构建后端
cd ../server
go build -o ../aether ./cmd/server

# 4. 准备 PostgreSQL（需要 Docker 或本地 PG）
# 方式 A：Docker
docker run -d --name aether-pg -e POSTGRES_PASSWORD=dev -p 5432:5432 postgres:16

# 方式 B：本地已安装的 PG
# 确保 localhost:5432 可连接
```

## 启动平台

```bash
# 1. 跑 migration（建表）
cd server
go run ./cmd/migrate up

# 2. 启动 Go server（API + 前端）
go run ./cmd/server

# 或者用编译好的二进制
cd ..
./aether

# 3. 浏览器打开
# http://localhost:8080 → 看到 Aether 前端界面
# http://localhost:8080/healthz → 健康检查
# http://localhost:8080/api/... → Connect-RPC API
```

## 开发模式（前后端分离）

```bash
# 终端 1：后端
cd server
go run ./cmd/server          # :8080

# 终端 2：前端（热更新）
cd web
npm run dev                  # :5173（自动 proxy /api → :8080）

# 浏览器打开 http://localhost:5173
```

## MVP 操作流程（6 步）

### Step 1：注册原子模块（Admin）

```
页面：模块注册 → 注册新模块

填写：
  Git 源：    本地 file:// 路径或 git URL
  模块路径：   atomic/rds（或 atomic-null）
  版本：      v1.0.0
  Provider：  alicloud（或 null）
  显示名：    RDS MySQL
  归属团队：   DBA Team

点击"注册模块"：
  → 平台 clone git 仓库
  → ContractExtractor 解析 variables.tf
  → 提取 variables_contract_json（变量名/类型/默认值）
  → 提取 providers_json（provider 版本）
  → 存入 modules + module_versions 表
  → 状态：validated ✅
```

### Step 2：发布到服务目录（Admin）

```
页面：模块注册 → 找到刚注册的模块 → 点"发布目录"

填写：
  显示名：    RDS MySQL
  分类：      数据库
  层级：      middleware
  归属团队：   DBA Team
  可见性：     全局 / 特定团队

平台自动：
  → FormSchemaGenerator 从契约裁剪 form_schema_json
  → 隐藏 sensitive + platform-inferred 字段
  → 注入 defaults_json（最佳实践默认值）
  → D40 校验 form_schema_json
  → 存入 catalog_items 表
  → 服务目录页面出现"RDS MySQL"
```

### Step 3：用户申请资源

```
页面：服务目录 → 找到"RDS MySQL" → 点"申请"

表单（由 form_schema_json 自动渲染）：
  环境：       [prod ▾]
  实例规格：    [mysql.n2.large ▾]
  存储大小：    [100 GB ▾]
  高可用：      [高可用 ▾]
  标签：       [owner=zhangsan]

点击"提交申请"：
  → CreateRequest API（含 idempotency_key）
  → 工单创建（status=submitted）
  → Pipeline.Execute 自动推进
```

### Step 4：平台自动执行（后台）

```
工单状态自动流转：
  submitted → generating（codegen 生成 stack 代码）
  generating → planning（terramate plan）
  planning → plan_ready（plan 成功）
  plan_ready → pending_approval（等待审批）

在"我的工单"页面可以看到状态实时更新（5s 轮询）
```

### Step 5：审批

```
页面：审批中心 → 找到待审批工单

查看 Plan 差异 → 点击"批准"
  → DecideApproval（decision=approved）
  → 工单状态 pending_approval → applying

平台自动：
  applying → terramate run terraform apply
  applying → reconciling（apply 成功）
  reconciling → succeeded ✅

代码 merge 到 main 分支
state 写入 backend
```

### Step 6：验证结果

```
页面：我的工单 → 工单详情

  步骤条：提交→生成→Plan→审批→Apply→完成 全绿 ✅
  时间线：每步的时间 + commit SHA
  Plan 差异：资源创建摘要
  状态：succeeded ✅

infra-repo main 分支：
  middleware/platform-default/rds-prod/
    ├── stack.tm.hcl
    ├── main.tf（module "rds" {...}）
    └── backend.tf

云上：
  RDS 实例已创建 ✅
```

## API 调试（curl）

```bash
# 健康检查
curl http://localhost:8080/healthz

# Connect-RPC（需要 Connect 协议）
curl -X POST http://localhost:8080/api/aether.platform.v1.catalog.CatalogService/ListItems \
  -H "Content-Type: application/json" \
  -d '{}'

# 注册模块（Admin）
curl -X POST http://localhost:8080/api/aether.platform.v1.registry.RegistryAdminService/RegisterModule \
  -H "Content-Type: application/json" \
  -d '{"gitSource":"file:///path/to/modules","modulePath":"atomic-null","version":"v1.0.0","provider":"null","name":"null-demo","ownerTeamId":"1"}'

# 创建工单
curl -X POST http://localhost:8080/api/aether.platform.v1.lifecycle.LifecycleService/CreateRequest \
  -H "Content-Type: application/json" \
  -d '{"catalogItemId":"1","envId":"dev","formValues":"{}"}'
```

## E2E 测试

```bash
# 快速测试（无 terramate/terraform）
cd server
go test ./e2e/... -short -v

# 完整测试（需要 terramate + terraform CLI）
go test ./e2e/... -v

# Walking skeleton 脚本（手动 demo）
bash scripts/walking-skeleton/run.sh
```

## 故障排查

| 问题 | 原因 | 解决 |
|------|------|------|
| 前端空白 | Vite dev 未启动 | `cd web && npm run dev` |
| API 404 | Connect 未启用 | 检查 config.yaml connect.enabled |
| migration 失败 | PG 未启动 | `docker start aether-pg` |
| 注册失败 | git clone 错误 | 检查 git_source 路径/权限 |
| plan 失败 | terraform/provider 未装 | `terraform init` 或用 null provider |
| apply 失败 | 云凭据缺失 | 检查 cloud_accounts 或用 null provider |
