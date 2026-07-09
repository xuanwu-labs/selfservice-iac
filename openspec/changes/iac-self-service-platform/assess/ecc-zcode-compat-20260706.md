# ECC → ZCode 兼容性验证报告

> 日期：2026-07-06
> 验证者：ZCode agent（builtin:bigmodel-coding-plan/GLM-5.2）
> 关联：回应"是否引入 ECC 作为 selfservice-iac 的多 agent 评审执行层"
> 状态：**实证完成，结论可用**

## 0. TL;DR

| 问题 | 结论 | 证据强度 |
|---|---|---|
| ECC 的 `agents/*.md` 能否被 ZCode 派发？ | **能**（格式高度兼容） | ✅ 反编译确认 + 文件格式验证 |
| 当前 session 能否立即派发自定义 agent？ | **不能**（需重启会话） | ⚠️ 实测 `not found`，根因是注册表快照 |
| systemPrompt 内容是否有效？ | **有效**（3 个 agent 全部命中靶子问题） | ✅ 用 general-purpose 携带 systemPrompt 实测 |
| ECC 的 hooks（GateGuard 等）能否迁移？ | **不能**（schema 不兼容 + 依赖 Claude 运行时） | ✅ 双方 schema 对照 |
| 弱化版 ECC（只 agents 不 hooks）够用吗？ | **够用，但有 3 个明确缺口需补救** | ✅ 见 §4 |

---

## 1. agents 兼容性（正面结果）

### 1.1 格式对照（反编译 ZCode `zcode.cjs` 函数 `qbe` 得出）

| 字段 | ECC 格式 | ZCode 要求 | 兼容性 |
|---|---|---|---|
| 文件位置 | `.claude/agents/<name>.md` | `<repo>/.zcode/agents/<name>.md` 或 `~/.zcode/agents/` | ✅ 放对目录即可 |
| `name`（必需）| ✅ | ✅ | ✅ 直接兼容 |
| `description`（必需）| ✅ | ✅ | ✅ 直接兼容 |
| body → systemPrompt | ✅ | ✅ | ✅ 直接兼容 |
| `tools` | inline `[a,b]` | YAML list `- a`（loose parser 也能吃 inline）| ⚠️ 改格式更稳 |
| `model` | `sonnet` | 认 `inherit`/`sonnet`/`lite` | ✅ 直接兼容 |
| `color`/`maxTurns`/`permissionMode` | 部分有 | 都可选 | ✅ 不影响 |

### 1.2 三个 agent 文件已创建（selfservice-iac 项目定制版）

`E:/Iac/selfservice-iac/.zcode/agents/`：
- `code-reviewer.md` — 质量审查（P0 正确性 / P1 工程契约 / P2 Go 习惯 / P3 可测性）
- `security-reviewer.md` — 安全审查（针对项目威胁模型：凭据泄漏/OIDC 边界/Executor 逃逸/break-glass）
- `architect.md` — 架构审查（守护 D1–D30，每个违规必须引用决策原文）

**注意**：这些不是 ECC 原版的逐字拷贝，而是融合了 ECC 方法论 + 本项目 design.md/specs 的定制版。ECC 原版是通用 Go reviewer，本项目版本强化了对 D1/D20/D27/D28/D29/D30 的守护。

### 1.3 frontmatter 格式验证（python 解析）

```
code-reviewer.md:  name ✓  description ✓  model: sonnet ✓  color: cyan ✓  tools ✓
security-reviewer.md: name ✓  description ✓  model: sonnet ✓  color: red ✓   tools ✓
architect.md:      name ✓  description ✓  model: sonnet ✓  color: purple ✓ tools ✓
```

无 BOM，首行 `---`，frontmatter 闭合，全部可解析。

---

## 2. 派发机制的关键限制（需重启会话）

### 2.1 实测结果

用 `Agent` 工具派发 `subagent_type: "code-reviewer"` / `"security-reviewer"` / `"architect"`，**三个全部返回**：

```
Agent type '<name>' not found. Available agents: general-purpose, Explore
```

### 2.2 根因分析

反编译确认 ZCode 加载 agent 的函数 `I9r`（`loadZCodeAgentProfiles`）在**会话启动时**扫描 `.zcode/agents/` 生成注册表。运行中新建的 agent 文件**不热加载** —— 当前 session 启动时该目录不存在，所以注册表里只有内置的 `general-purpose` 和 `Explore`。

### 2.3 解决方案

**重启 ZCode 会话**即可。重启后 `.zcode/agents/*.md` 会被扫描注册，`subagent_type: "code-reviewer"` 即可派发。

⚠️ **本 session 无法验证重启后的派发**（重启会丢上下文），但格式和路径的正确性已通过反编译 + 解析双重确认，重启后可用的确定性很高。

---

## 3. systemPrompt 有效性验证（核心正面证据）

为绕过 §2 的会话级限制，用 `general-purpose` agent 携带每个 agent 的 systemPrompt 做了等效验证。靶子是 `probe.go`（含 12 个标注问题）。

### 3.1 code-reviewer 命中情况

| 埋的问题 | 抓到 | 备注 |
|---|---|---|
| `stack.Run` 未检查 error（P0）| ✅ | 还指出"silent failure"影响 |
| off-by-one `i <= len(vals)` panic（P0）| ✅ | 给出 `range vals` 修复 |
| `cmd.Run` 未检查 error（P0）| ✅ | |
| 死代码 `FindRequest`（P2）| ✅ | grep 确认无 caller |
| 无用 `strings.TrimSpace`（P2）| ✅ | |
| **routing**：识别出安全/架构问题并标注 "→ route to" | ✅ | systemPrompt 的 routing 逻辑生效 |

### 3.2 security-reviewer 命中情况

| 埋的问题 | 抓到 | 备注 |
|---|---|---|
| 硬编码 AWS 密钥（P0）| ✅ | **正确 redact 为 ****，未输出明文** |
| SQL 注入 `fmt.Sprintf`（P0）| ✅ | 给出 `$1` 参数化修复 |
| shell 注入 `sh -c`（P0）| ✅ | 指出这是最高危，跨信任边界 RCE |
| 密钥写入日志（P0）| ✅ | |
| break-glass 无审计（P1）| ✅ | 还补了一个 advisory AuthZ 观察 |

### 3.3 architect 命中情况（最强证据）

| 决策违规 | 抓到 | 是否引用 design.md 原文 |
|---|---|---|
| D1（import Terramate 内部包）| ✅ | ✅ 引用"默认实现通过 exec 调用 terramate CLI" |
| D20（硬编码 executor 模式）| ✅ | ✅ 引用"MUST NOT 硬编码任何运行时" |
| D27（路径缺 env/layer 维度）| ✅ | ✅ 引用"(env × tenant × layer) 三元组" |
| D28（无 provenance）| ✅ | ✅ 引用"MUST 写入 requests.resolved_params_json" |
| D29（非 layer-first，无 stack.tm.hcl）| ✅ | ✅ 引用"分层根目录拓扑" |
| D30（break-glass 无双人/录屏/补审批）| ✅ | ✅ 引用"自动全程录屏…24h 内必须补审批回填" |

**关键**：architect 真的去读了 `design.md §04` 的决策原文并逐条引用，证明 systemPrompt 里"先读决策再评审"的指令完全生效。

---

## 4. hooks 不可迁移的缺口分析（负面结果 + 补救）

### 4.1 ECC hooks vs ZCode hooks schema 对照

| 维度 | ECC（Claude Code 格式）| ZCode | 兼容性 |
|---|---|---|---|
| 配置文件 | `hooks/hooks.json`（Claude schema）| `<repo>/.zcode/config.json` → `hooks` 或 `hooks.json`（ZCode schema）| ❌ 结构不同 |
| 事件名 | PreToolUse / PostToolUse / PreCompact / SessionStart / SessionEnd / Stop / PostToolUseFailure | SessionStart / UserPromptSubmit / PreToolUse / PermissionRequest / PostToolUse / PostToolUseFailure / Stop | ⚠️ 部分同名但 PreCompact/SessionEnd 缺失 |
| 脚本实现 | Node.js（`scripts/hooks/*.js`）| 任意可执行（shell/script）| ⚠️ Node 脚本依赖 Claude 运行时环境变量 |
| GateGuard 机制 | PreToolUse + `permissionDecision: deny` | ZCode PreToolUse 可返回决策但 schema 不同 | ❌ 需重写 |

### 4.2 丢失的能力清单（弱化版 ECC 相比完整 ECC）

| ECC 自动门禁 | 作用 | 弱化版状态 | 风险 |
|---|---|---|---|
| **GateGuard fact-force** | 第一次 Edit/Write 硬拦截，逼列 importers/影响面/用户原话 | ❌ 丢失 | 主 agent 可能不调研就改文件 |
| **config-protection** | 阻止弱化 linter/formatter 配置 | ❌ 丢失 | 配置被悄悄改坏 |
| **quality-gate (PostToolUse)** | Edit 后自动 lint/format/typecheck | ❌ 丢失 | 低级错误漏网 |
| **design-quality-check** | 前端代码漂移到模板 UI 时告警 | ❌ 丢失 | 不适用本项目（无前端代码） |
| **continuous-learning / instincts** | 从操作中自动学习并注入 | ❌ 丢失 | 无自学习，每次从头 |

### 4.3 补救策略（按可行性排序）

**策略 A（推荐）：用 AGENTS.md 硬规则 + 主动派发，替代 GateGuard 的"强制调研"**

在 `selfservice-iac/AGENTS.md` 追加规则：
```markdown
## 强制评审纪律（替代 GateGuard）
- 每次 Write/Edit 一个 .go 文件后，MUST 派发 code-reviewer + security-reviewer 评审
- 涉及 D1/D20/D27/D28/D29/D30 模块时，MUST 额外派发 architect
- 派发前 MUST 先 grep 出该文件的所有 caller（替代 GateGuard 的"列 importers"）
```
**优点**：零基础设施，立即可用。
**缺点**：依赖主 agent 自觉（软纪律，不是硬拦截）。

**策略 B：写一个 ZCode PostToolUse hook 替代 quality-gate**

ZCode 支持 PostToolUse hook，可以写一个 shell 脚本在 Edit/Write 后跑 `gofmt -l` + `go vet`。比 GateGuard 弱（不能 deny Edit），但能自动抓低级错误。
**优点**：硬触发，不依赖主 agent 自觉。
**缺点**：只覆盖静态检查，覆盖不了 GateGuard 的"调研强制"。

**策略 C（最接近完整 ECC）：ZCode PreToolUse hook 模拟 GateGuard**

ZCode 的 PreToolUse 可以返回决策。写一个 hook 在 Edit/Write 前检查"是否已 grep 过 callers"，若否则提示主 agent 先调研。这最接近 ECC，但开发成本高。

**建议**：Phase 1 用策略 A（零成本启动），发现软纪律不够再加策略 B。

---

## 5. 对 selfservice-iac 209 task 实现的最终判断

### 5.1 可用性结论

**弱化版 ECC（3 个 agent + AGENTS.md 软规则）足以支撑 selfservice-iac 的实现阶段**，理由：

1. ✅ 三个 agent 覆盖了平台代码最关键的三个维度：质量（code-reviewer）、安全（security-reviewer，针对凭据/OIDC/break-glass）、架构（architect，守护 D1–D30）
2. ✅ 实测命中率高（靶子 12 个问题全中），且 routing 逻辑正确（各司其职，不越界）
3. ✅ 格式确认兼容，重启会话即可派发
4. ⚠️ hooks 缺口有明确补救路径（策略 A 立即可用）

### 5.2 推荐使用方式

```
/opsx:apply iac-self-service-platform  (跑 W1 的某个 task)
    ↓ Write 代码
    ↓ 按 AGENTS.md 规则，主 agent 派发：
    ├── Agent(subagent_type="code-reviewer")      ← 重启后可用
    ├── Agent(subagent_type="security-reviewer")   ← 凭据/注入/边界
    └── Agent(subagent_type="architect")           ← D1/D20/D28 守护（关键模块才派）
    ↓ 汇总评审，修正，标记 task 完成
```

### 5.3 待验证项（重启会话后）

- [ ] 重启 ZCode 后，`Agent` 工具的 `subagent_type: "code-reviewer"` 是否真被识别（本 session 因快照限制无法验证）
- [ ] 若仍 not found，检查是否需要 `~/.zcode/v2/agents-state.json` 显式启用（反编译提到 disable state 文件，但未确认是否需要显式 enable）
- [ ] `model: sonnet` 在 ZCode 是否生效（反编译认这个值，但实际派发时是否真的切到 sonnet 模型需观察）

---

## 附录：验证产物

- `E:/Iac/selfservice-iac/.zcode/agents/code-reviewer.md`
- `E:/Iac/selfservice-iac/.zcode/agents/security-reviewer.md`
- `E:/Iac/selfservice-iac/.zcode/agents/architect.md`
- `E:/Iac/selfservice-iac/.zcode/agents/_probe/probe.go`（评审靶子，测试用，勿提交）

## 附录：未引入的 ECC 能力（记录用，非推荐）

ECC 另有 ~260 skills / 64 agents，本验证只迁移了 3 个最关键 agent。其余未迁移的领域 skill（如 `search-first` 选型、`hexagonal-architecture`、`postgres-patterns`、`production-audit`）可按需单独评估迁移，但每个都要单独做 ZCode 兼容性验证 —— 本报告的结论只适用于 `agents/*.md` 这一类（subagent 定义），不适用于 `skills/` 和 `hooks/`。
