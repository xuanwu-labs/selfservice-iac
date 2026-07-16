# 17-平台CLI与AI原生扩展

> 对应 design `D17`、spec `specs/14-平台CLI与AI原生扩展`。回应：如何让 agent 按平台 skills 调用 CLI，平台如何具备 AI 原生扩展能力。

## 1. 为什么需要平台 CLI（不只是 HTTP API + Web）

| 场景 | 为什么 CLI 优于 curl |
|------|----------------------|
| CICD 脚本 | shell 里拼 curl 参数/解析 json 痛苦，CLI 一行搞定 |
| LLM agent tool call | CLI 是天然 tool boundary（exit code + stdout 契约），比 HTTP 更稳定 |
| 本地调试 | 工程师不想为查个 drift 开浏览器 |
| 幂等批量 | shell 管道友好 |

CLI 与 HTTP API 同源（共用 `server/internal/api` 的 service 层），CLI 是薄封装，不重复业务逻辑。

## 2. 双身份模型：人 vs 机器

```
人登录:    浏览器 → OIDC(dex) → 会话 cookie   → Web / 个人 CLI (`aether auth login`)
机器调用:  CLI/agent → AK/SK HMAC 签名         → service account → 受 RBAC 约束
```

- **OIDC 会话**：给人用，token 过期、MFA、SSO 体验。
- **AK/SK + service account**：给机器用，长期凭证，绑 role_bindings，全审计。

两者 RBAC 引擎统一（同一 `role_bindings` 表），actor 字段区分 `user:zhangsan` / `sa:cicd-deploy`。

## 3. AK/SK 设计

- **签发**：管理员在 Web 创建 service account，平台生成 AK（公开标识）+ SK（HMAC 密钥，只展示一次）。
- **签名**：类 [AWS SigV4](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html) / 阿里云：用 SK 对 `method + path + body + timestamp` 做 HMAC-SHA256，请求头携带；timestamp 防重放（5 分钟窗口）。
- **存储**：SK 仅以 hash 存 DB（不可逆）；AK 明文。
- **轮换**：支持双 AK 共存（rotation window），新旧都可签，旧 AK 撤销前所有调用方切换完毕再删旧。
- **作用域**：AK 可限定 scope（只读 / 仅某 Space / 仅 catalog 读 + request 创建），最小权限。
- **审计**：所有 AK 操作入 `audit_logs`，`actor = sa:xxx`。

## 4. CLI 命令结构（`aether`）

```
tm auth     login | whoami | token | logout
tm catalog  list | get <item> | versions <item>
tm request  create --yaml|--file | list | get | wait | approve | reject | cancel
tm stack    list | show <id> | drift <id>
tm drift    list | show <id> | explain <id>          # explain 走 LLM
tm cost     estimate --yaml | show <request-id>
tm approval list | decide <id> --approve|--reject
aether gate     wait | release | reject | status         # 复用 D16
aether mcp      serve | tools                            # MCP server
tm skills   list | show <name> | run <name> [...]
aether ai       "<natural language intent>"              # 自然语言入口
tm config   set-profile | get | switch
```

- 单二进制（Go，cobra），与平台同语言，复用 `server/internal/api` service 层。
- 配置：`~/.tm/config.yaml`（profile / ak-sk 引用 / endpoint），SK 不进文件，走 env / vault / OS keychain。
- 输出：`--output {table|json|yaml|llm}`，默认 table（人），agent 用 json/llm。

## 5. MCP server 集成（关键：让 agent 标准化接入）

[MCP（Model Context Protocol）](https://modelcontextprotocol.io) 是 LLM↔工具标准协议。平台 CLI 内置 MCP server：

```bash
# 一次性启动，agent 通过 stdio 或 SSE 连接
aether mcp serve
```

MCP server 把以下暴露为 MCP tools（schema 自描述）：
- 每个 `aether <cmd>` → 一个 tool（参数 schema 自动从 cobra flags 生成）
- 每个 skill → 一个 tool
- 资源（catalog / stacks / drifts）→ MCP resources（可被 agent 读取上下文）

**接入示例（Claude Code）**：在项目 `.mcp.json` 或 Claude Code 设置里加 `aether mcp serve` 作为 stdio server，Claude 自动发现所有平台 tool，用户用自然语言驱动。

**为什么 MCP 而不是让 agent 硬编码 CLI**：协议标准化 → 一次接入，所有支持 MCP 的 agent（Claude Code / Cursor / Continue / 自研 agent）通用；schema 自描述 → LLM 不靠猜；版本协商 → 平台升级不破坏 agent。

## 6. AI 原生 skills（声明式 agent 编排）

Skill = 可复用的「自然语言触发 + 编排步骤 + 输出契约」YAML：

```yaml
# skills/new-rds.yaml
name: new-rds
description: 申请一台新的 RDS 实例
trigger:
  patterns: ["起.*mysql", "申请.*rds", "新建.*数据库"]
  examples:
    - "给 order-service 起一台 4C8G 的 mysql"
steps:
  - llm.extract:                    # LLM 抽取参数
      prompt: "从用户输入提取 spec/storage/engine/team"
      output: params
  - cli.run: ["tm", "catalog", "get", "rds-managed"]
  - cli.run: ["tm", "request", "create", "--yaml", "${render(request.template, params)}"]
  - cli.run: ["tm", "request", "wait", "${last.id}"]
output:
  contract: |
    ## 已提交 RDS 申请
    - 申请单: #{request_id}
    - 规格: {spec}
    - 当前状态: {status}
    - 审批人: {approvers}
```

两层 skills：
- **平台内置**：随平台发布（new-rds / drift-explain / cost-estimate / bulk-import）。
- **团队自定义**：团队注册自己的 skill（如业务团队的"起新微服务"，组合多底层命令），存 DB 或 git。

Agent 调用 skill 两种方式：
- **MCP 模式**：`skills.new-rds` 是一个 MCP tool，agent 直接调。
- **CLI 模式**：`aether skills run new-rds "给 order-service 起一台 4C8G mysql"`。

## 7. LLM-friendly 输出（`--output llm`）

给人看的 table / json 对 LLM 不友好（字段名缩写、表格需重新解析）。`--output llm` 输出语义化 markdown：

```
## drift: rds-orders-prod
- stack: rds-orders-prod (Middleware 层, owner=DBA)
- 期望: instance_class=db.r5.xlarge, storage=200GB
- 实际: instance_class=db.r5.2xlarge, storage=200GB
- 差异: instance_class 被外部改为更大规格
- 建议: adopt-cloud（采用云上现状）或 restore-desired（回滚）
- 上次检测: 2026-06-15 03:00
```

字段名稳定、自描述，LLM 可直接转述或决策。

## 8. 自然语言入口（`aether ai`）

`aether ai "<意图>"` 把 LLM 放平台后端：
1. 平台收到自然语言意图。
2. 平台 LLM 路由 → 匹配 skill 或分解为 CLI 步骤。
3. 平台执行（生成 yaml → OPA 校验 → 审批 → apply）。
4. 返回结构化结果。

**为什么 LLM 放后端而不是 CLI 本地**：平台侧能控制 LLM 版本、prompt、上下文（catalog/stack/drift），保证治理边界；CLI 本地 LLM 难以统一行为，且无法做 OPA/审批联动。

## 9. 安全边界（关键！AI 不能绕治理）

| 治理项 | agent / LLM 是否豁免 |
|--------|----------------------|
| OPA 策略校验 | **否**，LLM 生成的 yaml 走相同校验 |
| 审批流 | **否**，agent 发起的申请走相同审批 |
| RBAC | **否**，service account 的 role_bindings 约束可见与可操作 |
| 高危操作（destroy/跨层依赖变更） | **强制人工审批**，agent 不能自审 |
| 审计 | **全记录**，actor = sa:xxx，含 skill 编排全链路 |

核心原则：**LLM 是意图翻译器 + 编排器，不是绕过治理的捷径**。LLM 把"起 RDS"翻译成 yaml，但 yaml 是否能 apply 仍由平台治理决定。

### 9.1 高危操作判定标准（具体化，否则治理形同虚设）

平台在 codegen 后 / plan 前 MUST 按以下维度判定高危，触发"强制人工审批"分支（即使申请来自 AI actor）：

| 维度 | 触发条件 | 处置 |
|------|---------|------|
| **action 类型** | `destroy` / `replace`（不可逆） / `depose` | 强制 pre-plan + pre-apply 双门审批，禁 OPA 自动放行 |
| **资源类型敏感** | 涉及 `rds`/`redis`/`kafka`/`vpc`/`iam`/`ack` 等有状态/共享/全局资源 | 强制审批 + 通知归属团队所有 owner |
| **跨层影响** | Application 层变更会反推 Middleware/Global 层依赖（违反单向）| 阻断，要求人工架构评审 |
| **批量阈值** | 单工单影响资源数 > 阈值（默认 50）或 stack > 5 | 强制审批 + 走灰度（先 plan 一批，验证再 apply 剩余）|
| **敏感字段写** | 改动 `sensitive=true` 字段（db_password/access_key 等） | 强制审批 + 双签（运维 + 资源 owner）|
| **生产环境** | env=prod 且 action=destroy/replace | 强制审批 + 时间窗（仅维护窗口）|
| **预算越限** | Infracost 预估超 team/space 预算阈值 | 升级到成本审批节点（D18）|

**强制人工审批的执行机制**：
- AI actor（`actor_type=ai`）的工单**自动跳过 OPA 自动放行逻辑**（即使 OPA 判低危，AI 触发的高危场景也走人工）。
- 审批节点绑 RBAC 角色（如 `dba-approver`/`platform-ops`），**不允许 AI actor 的 service account 自审**（role_bindings 校验时排除 actor 自己）。
- 高危工单的 plan 产物保留期延长（默认 90 天 vs 普通 30 天）。

### 9.2 AI 操作审计字段（精确化，便于追溯）

每条 AI 触发的 `audit_logs` 记录 MUST 含：

```
audit_logs(
  actor_id,                          -- sa:tm-ai-xxx
  actor_type='ai',                   -- 区分 human/system
  action, target_type, target_id,
  correlation_id,                    -- 串起同一工单的 codegen/plan/apply/审批
  ai_metadata_json,                  -- 仅 actor_type=ai 时填：
    ├── prompt_hash,                 -- 触发操作的 prompt 哈希（不存原文，存 sha256 + 长度）
    ├── prompt_length,
    ├── skill_name,                  -- 调用的 skill（如 "request-rds"）
    ├── skill_version,
    ├── request_id,                  -- 关联工单 ID
    ├── llm_model,                   -- 用的 LLM 模型（如 glm-5.1/claude-opus）
    ├── tool_calls_json,             -- MCP tool 调用链（catalog.search/request.create/...）
    └── confidence_score             -- LLM 自评置信度（<阈值时强制人工复核）
  before_json, after_json, occurred_at
)
```

**审计脱敏**：prompt 原文不入审计（可能含业务敏感信息），只存 sha256 + 长度；如需重放，从单独的 `ai_prompts` 表（加密存储 + 限访问）按 hash 查。tool_calls 完整记录（便于复盘 AI 的操作路径）。

## 10. 选型

| 用途 | 选型 | 说明 |
|------|------|------|
| CLI 框架 | [cobra](https://github.com/spf13/cobra) | Go 事实标准 |
| 配置 | viper + `~/.tm/config.yaml` | profile 切换 |
| HMAC 签名 | crypto/hmac + 自定义 SigV4-like | 参考 AWS SigV4 |
| MCP server | [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) 或自实现 | Go MCP SDK |
| LLM（后端） | OpenAI 兼容 API（可接内部网关） | 意图路由 / 参数抽取 / drift 解释 |
| Skill 模板 | Go text/template + YAML | 声明式编排 |

## 11. 与现有设计的关系

- **D16 gate CLI**：以 `aether gate` 子命令组为权威入口，不维护双 CLI；历史脚本中的 `aether-gate ...` 只作为兼容 shim 过渡到 `aether gate ...`。
- **D11 审批引擎**：agent 发起的申请走相同 ApprovalEngine；service account 可作为审批人（自动化审批场景，如非生产环境自动批）。
- **specs/08 平台 API**：CLI 是 HTTP API 的薄封装，不重复业务逻辑；MCP server 也基于同一 service 层。
- **specs/09 身份同步**：service account 与自然人身份分开存储，不参与目录同步，但 RBAC 统一。

## 12. 关键链接

- [MCP（Model Context Protocol）](https://modelcontextprotocol.io) · [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- [AWS SigV4](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)
- [cobra](https://github.com/spf13/cobra) · [Claude Code MCP 配置](https://docs.claude.com/en/docs/claude-code/mcp)
