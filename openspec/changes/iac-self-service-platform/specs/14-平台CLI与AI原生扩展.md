# 16-平台CLI与AI原生扩展（platform-cli-ai）

能力 ID：`platform-cli-ai`
覆盖：平台命令行工具（`tm` CLI）、机器身份（AK/SK + service account）、MCP server 集成、AI 原生能力（声明式 skills / 自然语言入口 / LLM-friendly 输出）。

## ADDED Requirements

### Requirement: 统一平台 CLI（`tm`）
平台 SHALL 提供单一二进制 CLI `tm`，覆盖人/脚本/agent 三类调用方，能力与 HTTP API 对齐（catalog/request/stack/drift/approval/cost）；D16 gate 能力 SHALL 以 `tm gate ...` 子命令组为权威入口，历史 `tm-gate` MAY 作为兼容 shim，MUST NOT 维护双 CLI。

#### Scenario: 脚本批量操作
- **WHEN** 运维用 shell 批量查询所有漂移
- **THEN** `tm drift list --status drifted --output json` 一次性返回，退出码反映成功/失败，可管道处理

#### Scenario: 与历史 tm-gate 统一
- **WHEN** CICD 需要 gate
- **THEN** 使用 `tm gate wait <request-id>` / `tm gate release <request-id>`，无需另装独立 `tm-gate`

### Requirement: 机器身份与 AK/SK
平台 SHALL 为非交互场景（CLI/agent/CICD）提供机器身份：service account + AK/SK（HMAC 签名，类 AWS SigV4）；AK MUST 绑定 service account 并继承其 role_bindings，所有操作入审计且 `actor = sa:xxx`。

#### Scenario: agent 调用
- **WHEN** agent 持 AK/SK 调用 `tm request list`
- **THEN** 平台用 HMAC 校验签名，按 service account 的 RBAC 过滤结果，记录审计

#### Scenario: AK 轮换
- **WHEN** 管理员轮换 AK
- **THEN** 支持新旧 AK 共存期，旧 AK 失效前所有调用方平滑切换

#### Scenario: AK scope 最小权限
- **WHEN** 管理员签发只读 AK
- **THEN** 该 AK 仅可 catalog 读 + request 创建，不能 approve/apply，越权操作被拒

### Requirement: MCP server 集成
平台 CLI SHALL 内置 MCP（Model Context Protocol）server（`tm mcp serve`），把所有命令、skills 暴露为 MCP tools（schema 自描述），LLM agent 通过标准 MCP client 连接，MUST NOT 为每个 agent 写专有适配。

#### Scenario: Claude Code 接入
- **WHEN** 工程师在 Claude Code 中配置 `tm mcp serve` 作为 MCP server
- **THEN** Claude 可直接 list catalogs / create request / explain drift，agent 不需要学平台专有协议

#### Scenario: 跨 agent 通用
- **WHEN** 同一 MCP server 接入 Cursor / Continue / 自研 agent
- **THEN** 无需平台侧改动，所有支持 MCP 的 agent 即插即用

### Requirement: AI 原生 skills（声明式编排）
平台 SHALL 提供可声明、可版本化的 skills（YAML：trigger 自然语言模式 / steps CLI+LLM 调用序列 / output contract），agent 按 skill 调用保证输出可预期；MUST 支持平台内置 skills + 团队自定义 skills。

#### Scenario: 内置 skill 起新 RDS
- **WHEN** 用户对 agent 说"给 order-service 起一台 4C8G 的 mysql"
- **THEN** agent 匹配 `new-rds` skill：LLM 抽参 → `tm catalog get rds` → `tm request create --yaml` → `tm request wait`，全程可审计

#### Scenario: 团队自定义 skill
- **WHEN** 业务团队注册"起新微服务"skill（组合 vpc 查询 + ecs 申请 + slb 规则）
- **THEN** 该团队 agent 可调用此 skill 跨多底层命令编排，skill 可见性受 RBAC 约束

### Requirement: LLM-friendly 输出
所有 CLI 命令 SHALL 支持 `--output {table|json|yaml|llm}`；`llm` 输出为语义化 markdown（字段名自描述 + 结构稳定），MUST NOT 让 LLM 依赖表格解析。

#### Scenario: drift 解释
- **WHEN** 执行 `tm drift explain <id> --output llm`
- **THEN** 输出语义化 markdown：期望状态 / 实际状态 / 差异 / 建议动作，LLM 可直接转述用户

### Requirement: 自然语言入口
平台 SHALL 提供 `tm ai "<意图>"` 自然语言入口，LLM 放平台后端做意图路由（匹配 skill 或分解为 CLI 步骤）；CLI MUST NOT 把治理决策权交给本地 LLM。

#### Scenario: 自然语言起新资源
- **WHEN** 执行 `tm ai "给 order-service 起一台 4C8G mysql"`
- **THEN** 平台后端 LLM 路由到 new-rds skill 执行，返回结构化结果

### Requirement: AI 不豁免治理（安全边界）
AI 原生能力 MUST NOT 绕过平台治理：LLM 生成的 yaml 申请单 SHALL 经 OPA 策略校验 + 审批流（与人提交等价）；高危操作（destroy / 跨层依赖变更）即使 agent 发起也 MUST 强制人工审批。

#### Scenario: LLM 申请单等同人工
- **WHEN** agent 用 LLM 生成 RDS 申请单提交
- **THEN** 走与表单相同的 OPA 校验与审批流，不因来源是 agent 而跳过

#### Scenario: 高危拦截
- **WHEN** agent 发起 destroy 类操作
- **THEN** 强制人工审批，agent 不能自审自批，service account 也不豁免

#### Scenario: 审计可追溯
- **WHEN** agent 通过 skill 执行多步操作
- **THEN** 每步（LLM 抽参 / CLI 调用 / 申请提交）入审计，actor = sa:xxx，可回溯整条链路
