# 06a-云账号 Bootstrap 手册

> 本文解决 day-0 / day-1 问题：平台如何从“没有任何云账号执行能力”走到“首个标准 catalog item 可真实 plan/apply”。这是 Phase 0 的实施前置。

## 1. Bootstrap 目标

首个云账号接入完成后，平台必须具备：

- 一个 active `cloud_accounts` 记录。
- 一个受控 secret store 引用，保存 bootstrap 凭据。
- 平台 OIDC issuer / JWKS 可被云账号信任。
- 至少一个 per-team execution role。
- 至少一条 `team_cloud_grants`。
- 一个可执行标准 catalog item 的最小权限链路。

## 2. 角色边界

| 角色 | 职责 |
|------|------|
| Platform Admin | 录入云账号、配置 issuer、发起 bootstrap |
| Cloud Account Guardian | 双人持有 bootstrap 凭据授权 |
| Team Owner | 接受 execution grant，确认预算和范围 |
| Security Reviewer | 审查 trust policy 与 IAM policy |

## 3. Day-0 准备

| 前置项 | 要求 |
|--------|------|
| Secret Store | Vault/KMS/平台内嵌 secret store 至少一种可用 |
| Platform Issuer | 平台能暴露 OIDC issuer URL 与 JWKS endpoint |
| Clock Sync | 平台与 Executor 节点时钟偏差小于 500ms |
| Provider Mirror | Terraform provider 可下载，离线环境需 mirror |
| Audit Storage | `audit_logs` 和对象存储可用 |

## 4. Bootstrap 流程

### Step 1：登记云账号

在平台创建 `cloud_accounts`：

| 字段 | 示例 |
|------|------|
| `provider` | `alicloud` |
| `account_id` | `1234567890` |
| `alias` | `corp-prod-1` |
| `default_region` | `cn-hangzhou` |
| `bootstrap_status` | `none` |

### Step 2：写入 bootstrap 凭据引用

Cloud Account Guardian 双人确认后，把 bootstrap AK/SK 写入 secret store，DB 只存 `secret_ref`。

要求：

- 密文不得进入 DB。
- 写入动作必须写 `audit_logs`。
- bootstrap 凭据默认只允许执行 IAM/STS/billing/tag policy 相关初始化。

### Step 3：发布平台 OIDC issuer

平台生成并暴露：

| 项 | 要求 |
|----|------|
| `issuer_url` | 稳定 HTTPS URL |
| `jwks_uri` | 云账号可访问 |
| `audience` | `iac-platform` 或 per-cloud account audience |
| `subject` | `team:<team_id>:catalog:<capability>` 或等价模式 |

### Step 4：配置云账号 OIDC trust

使用 bootstrap 凭据在云账号内创建 trust policy，使云账号信任平台 issuer。

输出：

- `cloud_accounts.oidc_trust_configured = true`
- `cloud_accounts.bootstrap_status = ok`
- 写入 `outbox_events(event_type=cloud.oidc_trust.configured)`

### Step 5：生成首个 execution role

平台从 catalog item 的 `required_permissions` 聚合 IAM policy，生成 `iam_role_templates`。

首个建议角色：

| 团队 | 角色 | 用途 |
|------|------|------|
| DBA | `TmExec-DBA-RDS` | RDS 标准申请 |
| Platform Ops | `TmExec-Platform-VPCAdmin` | Global VPC bootstrap |

要求：

- OPA 校验 IAM policy，禁止 `Action:*` 和 `Resource:*`，除非 platform-admin break-glass。
- 角色创建成功后写 `iam_role_templates.version`。

### Step 6：创建 team_cloud_grants

授权 team 使用 cloud account、layer、env、role：

| 字段 | 示例 |
|------|------|
| `team_id` | `dba` |
| `cloud_account_id` | `corp-prod-1` |
| `allowed_layers` | `["middleware"]` |
| `env_scope_json` | `["dev","staging"]` for Phase 1 |
| `iam_role_template` | `TmExec-DBA-RDS` |
| `budget_quota_cents` | 初始预算 |

### Step 7：首个 dry-run 验证

用标准 catalog item 创建 request，只执行到 plan：

- 解析目标云账号。
- OIDC assume role 成功。
- Terraform init 可下载 provider。
- plan artifact 生成并写 sha256。
- 日志无 AK/SK 明文。

### Step 8：首个 apply 验证

审批后执行 apply：

- apply 使用同一 `pinned_commit` 与 plan artifact。
- state 写入远程 backend。
- CMDB index 成功或进入 `reconcile-pending`。
- request 进入 `succeeded`。

## 5. Day-1 资源栈 Bootstrap（首个业务申请前）

云账号 bootstrap 只解决“平台能否拿到执行身份”。首个 Application/Middleware 资源还需要先解决网络与跨层依赖，否则 codegen 会缺 `vpc_stack_id`、`subnet_ids` 或 remote state。

### 5.1 推荐顺序

| 顺序 | 动作 | 结果 |
|------|------|------|
| 1 | 创建或导入 Global VPC seed stack | 形成 `global/vpc-platform-default-dev` 或等价 stack |
| 2 | 跑 VPC stack plan/apply | state backend 中存在 VPC outputs |
| 3 | 校验 outputs | 至少含 `vpc_id`、`subnet_ids` / `vswitch_ids`、`security_group_base_id` |
| 4 | 写入 Phase 1 固定 dependency seed | 将 VPC stack 记录到 `stack_dependencies` 或固定 dependency 配置 |
| 5 | 若启用 env/tenant binding | 写入 `environment_tenant_bindings(env=dev, tenant=platform-default, layer=Application → vpc_stack_id)` |
| 6 | 注册首个 Application/Middleware catalog item | codegen dependency 阶段可解析网络上下文 |
| 7 | 执行 golden request | 验证 Request → Codegen → Git → Plan → Approval → Apply → Reconcile |

### 5.2 Phase 1 简化策略

Phase 1 可不完整启用 `docs/07` 的 env/tenant binding 引擎，但必须提供等价的固定依赖 seed：

- 默认 tenant：`platform-default`。
- 默认 env：`dev` 或 `staging`。
- 默认 Global VPC stack：由平台管理员 seed。
- Application/Middleware stack 的网络上下文只从该 seed 读取。
- seed 缺失时，申请必须在 `generating` 或 `planning` 前阻断，错误码为 `dependency_seed_missing`，不能进入 apply。

### 5.3 验收

- 首个 Global VPC stack 已有 state。
- VPC outputs 可被 `terraform_remote_state` 读取。
- `stack_dependencies` 或 binding 记录可追溯到 owner。
- 首个业务 catalog item 的 `ResolvedParams.provenance` 中能看到网络上下文来源。

## 6. 失败处理

| 失败点 | 状态 | 处理 |
|--------|------|------|
| secret store 不可用 | `blocked-policy` | 禁止创建 bootstrap 凭据 |
| JWKS 不可访问 | `failed-retryable` | 修复网络/DNS 后重试 trust 配置 |
| IAM policy 过宽 | `blocked-policy` | OPA 拒绝，要求缩小 `required_permissions` |
| AssumeRoleWithOIDC 失败 | `waiting-manual` | 生成任务，检查 issuer/audience/subject |
| Provider mirror 不可用 | `blocked-state-health` | 阻断 plan，通知 platform-ops |
| 日志发现 secret | `failed-terminal` | 阻断，触发 credential rotation |
| Global VPC seed 缺失 | `blocked-policy` | 阻断业务申请，要求先完成资源栈 bootstrap |
| VPC outputs 缺字段 | `blocked-policy` | 修复 Global VPC module outputs 后重跑 |

## 7. 验收清单

- `cloud_accounts.oidc_trust_configured = true`
- `cloud_credentials` 无个人执行凭据
- `team_cloud_grants` 至少一条 active 记录
- `iam_role_templates` 至少一条 active 记录
- 首个 Global VPC seed stack 已可读
- Application/Middleware 的 dependency seed 或 binding 已创建
- 首个 plan 成功生成 artifact
- 首个 apply 成功写 state
- 日志和 git 无明文凭据
- 审计可追溯 guardian、admin、team owner 的全部动作
