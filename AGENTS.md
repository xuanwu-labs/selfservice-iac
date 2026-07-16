# AGENTS.md — selfservice-iac

本项目是 **Aether**——一个构建在 Terramate 之上的 IaC 自服务平台。本仓库是 **monorepo**,包含后端(Go)、前端(React,规划中)、API 契约、部署清单与设计文档。

## 项目背景

- **上游引擎**:`../terramate/`(开源 Terramate CLI)。平台通过 `exec` 调用 `terramate` CLI 进行编排执行(决策 D1),**不 import terramate 内部包**。
- **方法论**:OpenSpec v1.5.0(`schema: spec-driven`),所有变更走 `openspec/changes/<change>/` 流程。
- **当前 change**:
  - `openspec/changes/iac-self-service-platform/`(主设计:决策 D1–D30,22 份能力规格,23 份设计文档,6 份架构评审)
  - `openspec/changes/platform-tech-stack-and-scaffold/`(技术栈与脚手架:决策 D31–D45)

## Monorepo 目录布局

```
selfservice-iac/
├── server/              # ★ Go 后端(HTTP + CLI + migrate,工程契约见 server/AGENTS.md)
├── web/                 # ★ React 前端(规划中,尚未实现)
├── contracts/           # ★ API 契约源(proto-first,前后端共享)
├── deploy/              # ★ 部署清单(Dockerfile / k8s / compose)
├── docs/                # ★ 用户文档(快速上手 / 架构 / CLI / 运维)
├── openspec/            # 设计过程文档(spec-driven 工作区,勿与 docs/ 混淆)
│   ├── config.yaml      # schema + context + rules(已定制,勿覆盖)
│   └── changes/         # 各 change(proposal/design/tasks/specs/docs)
└── .zcode/              # ZCode 工作区级配置(OpenSpec 集成)
    ├── commands/opsx/   # /opsx:* 命令
    └── skills/          # 自动触发 skill
```

**关于 server/**:其内部工程契约(代码规范、D1 边界、目录分层、wire DI、常用命令)见 [`server/AGENTS.md`](server/AGENTS.md)。

## OpenSpec 工作流(本项目的核心)

所有新需求、设计、实现都走 OpenSpec 的 propose → apply → archive 流程。使用以下 `/opsx:*` 命令驱动:

| 命令 | 用途 | 何时用 |
|---|---|---|
| `/opsx:explore [话题]` | 探索/思考/调研,不写代码 | 想法还没成型、需要对比方案、澄清需求 |
| `/opsx:propose [name]` | 新建 change 并一次性生成 proposal/design/tasks | 决定要做某个变更 |
| `/opsx:apply [name]` | 实现 change 的 tasks | 规格就绪,开始产出代码/脚本 |
| `/opsx:sync [name]` | 把 delta specs 同步到主 specs | change 的能力规格需要并入主线 |
| `/opsx:archive [name]` | 归档完成的 change | change 全部完成 |

## 工作约定(来自 openspec/config.yaml 的 rules)

- **proposal** 必须标注影响层级(CLI | LSP | HCL | config | stack | generate | cloud | 平台层),并包含兼容性说明。
- **specs** 按能力(Capability)组织,命名 `NN-中文名.md`,用 ADDED/MODIFIED 区分,场景用 **WHEN**/**THEN**。
- **design** 章节用 `## 01-标题` 数字前缀;详细架构/分层/库表/时序拆到 docs/。
- **tasks** 按 `## 01-功能模块` 分组,按依赖排序,每个 task 必须配测试任务,最后一个 task 跑 `make build` + `make test`。
- 产物目录:`changes/<change>/{proposal.md, design.md, tasks.md, specs/, docs/, scripts/}`。

## 语言规范（开源项目，权威定义在 config.yaml rules）

| 产出类型 | 语言 | 原因 |
|---------|------|------|
| **OpenSpec 文档**（proposal/design/tasks/specs 内容） | **中文** | 维护者要读、要审，中文更高效 |
| **spec.md 结构标记**（ADDED Requirements / Requirement / Scenario / WHEN / THEN） | **英文** | OpenSpec CLI parser 要求，改了不认 |
| **Scenario 标题 + WHEN/THEN 后的描述内容** | **中文** | 结构标记英文，内容中文（如 `#### Scenario: 未配置时返回错误` + `**WHEN** GitProvider 是 noop stub`） |
| **代码注释**（Go 注释） | **英文** | 开源标准，面向全球贡献者 |
| **commit message** | **英文** | 开源标准 |
| **proto 注释** | **英文** | 开源标准 |

详见 `openspec/config.yaml` 的 `rules.git` → "语言规范（开源项目）" 段。

## Git 提交语言

**所有 commit message 必须全英文**（subject + body）。详见 `server/AGENTS.md` 的 "Git 提交规范" 段。

## 分支策略（一个功能一个分支——最高优先级）

权威定义在 `openspec/config.yaml` 的 `rules.git` 段。关键规则：

- **一个 OpenSpec change = 一个 feature 分支**：`feat/<change-name>`（如 `feat/platform-db-schema`）。
- **同功能的实现 + review 修复 + 衍生改进全在同一个分支上累积**，不开 `fix/xxx` 子分支（会导致分支爆炸）。
- **合并用 `--no-ff`**（保留 merge commit，历史可追溯）；合并后删分支（保持分支列表清爽）。
- **不在 main 直接提交**；所有改动走 feature 分支。
- **只有跨功能的独立紧急修复**（main 上的生产 bug，不属于任何进行中的功能）才用 `fix/<描述>` 短命分支。
- **分支命名**：`<type>/<简短描述>`，type = feat/fix/chore/docs/refactor/test。

## OpenSpec 生命周期纪律（最高优先级，agent MUST 遵守）

**agent 开始工作前 MUST 先读 `openspec/config.yaml` 的 `rules` 段**——特别是 `lifecycle-ownership` 规则。关键纪律：

### 必须使用 `/opsx:` 命令操作 OpenSpec 生命周期

本项目有 5 个 slash command 封装了 OpenSpec 工作流（`.zcode/commands/opsx/`）。**agent MUST 使用这些命令，不得手动建目录/手写文件替代。**

| 命令 | 用途 | 何时用 |
|------|------|--------|
| `/opsx:propose <name>` | 新建 change + 用 CLI 脚手架生成全部 artifacts | 维护者说"新建提案/new change" |
| `/opsx:apply <name>` | 实现 tasks（写代码 + 勾 `[x]`）| 维护者说"apply/开始实现" |
| `/opsx:archive <name>` | 归档完成的 change（specs 并入主线 + 移到 archive/）| 维护者说"归档/archive" |
| `/opsx:sync <name>` | delta specs 同步到主 specs（通常跟 archive 一起）| 维护者说"sync" |
| `/opsx:explore <topic>` | 探索模式（思考/调研，不写代码）| 维护者说"explore/思考" |

**禁止的做法**：
- ❌ 手动 `mkdir changes/<name>/` + 手写 proposal/design/tasks —— 必须用 `/opsx:propose`（它调 `openspec new change` 脚手架 + `openspec instructions` 拿 template）
- ❌ 手动勾 tasks `[x]` —— 必须在 `/opsx:apply` 流程中完成
- ❌ 手动 `mv` 归档 —— 必须用 `/opsx:archive`

**允许的做法（微调不需要命令）**：
- ✅ `/opsx:propose` 后微调 artifacts 内容（改 proposal/design/tasks/specs）→ **直接编辑文件 + git commit**，不需要任何 `/opsx:` 命令
- ✅ `/opsx:apply` 过程中发现设计问题需要改 artifacts → **直接编辑文件 + git commit**，apply 文档明确允许 "suggest updating artifacts"
- ✅ `/opsx:explore` 讨论后想更新 artifacts → **直接编辑文件 + git commit**

> **关键区分**：`/opsx:` 命令管**生命周期阶段切换**（propose→apply→sync→archive）；artifact **内容编辑**随时可直接改文件 + commit。需要命令的阶段切换有 4 个：
> - **新建 change** → `/opsx:propose`（CLI 脚手架 + instructions template）
> - **开始实现** → `/opsx:apply`（写代码 + 勾 tasks）
> - **同步 specs** → `/opsx:sync`（delta specs 合并到主 specs，通常跟 archive 一起）
> - **归档 change** → `/opsx:archive`（change 移到 archive/）
>
> 中间的微调、修正、补充都**直接编辑文件 + commit**，不需要命令。

### 生命周期发起权

1. **propose / apply / archive / sync 四个生命周期动作必须由维护者显式发起**。agent 不得擅自执行。
   - **propose**：维护者说"新建提案" → agent 用 `/opsx:propose` 命令。**不能自己写代码**——代码是 apply 阶段的事。
   - **apply**：维护者说"apply/开始实现" → agent 用 `/opsx:apply` 命令开始写代码。
   - **archive**：维护者确认"做完了" → agent 用 `/opsx:archive` 命令归档。
2. **agent 不得在 propose 阶段写实现代码**。propose 只产文档。代码在 apply 后才写。
3. **openspec validate MUST 通过**才能推进。
4. 若 agent 认为某 change 应推进，需向维护者说明依据并**等待明确指令**，而非直接执行。

> **反面教材 1**：agent 在维护者只说"新建提案"时，不仅建了提案还直接写了实现代码 + 标记 tasks 完成——违反了 propose→apply 的先后顺序。
>
> **反面教材 2**：agent 手动 `mkdir` + 手写 proposal/design/tasks/specs 文件，没用 `/opsx:propose` 命令——导致 spec 目录结构错误（`specs/01-xxx.md` 而非 `specs/<capability>/spec.md`）、template 格式没遵循。正确做法是用 `/opsx:propose`，它会调 `openspec new change` + `openspec instructions` 拿正确的 template。

## 边界

- ❌ 不要修改 `../terramate/` 仓库(那是上游开源引擎,独立项目)。
- ❌ 不要让 `server/**` import `github.com/terramate-io/terramate/<任何子包>`(D1 边界,由 depguard 强制)。
- ❌ 不要覆盖 `openspec/config.yaml` 的定制 context/rules。
- ❌ **不要擅自执行 OpenSpec 生命周期动作**:新建 change(`/opsx:propose`)、apply、archive、sync **必须由维护者显式发起**。agent 不得自行判断"做完了"就归档,不得擅自新建第二个 change,不得通过重新定义验收范围来凑归档条件。详见下方"协作边界"与 `openspec/config.yaml` rules.lifecycle-ownership。
- ❌ **不要为同一功能开多个分支**:一个功能(如 error 注入、模块注册)= 一个分支。该功能的实现 + review 修复 + 衍生改进全在同一个分支上累积提交,不开 `fix/xxx` 子分支。只有跨功能的独立紧急修复才用 `fix/`。详见 `openspec/config.yaml` rules.git。
- ✅ 实现任务前先 `/opsx:explore` 或读 design.md,确保理解决策 D1–D45。
- ✅ `docs/` 是用户文档,`openspec/` 是设计文档,两者职责不同,勿混淆。
- ✅ 若 agent 认为 change 应推进到 apply/archive,需说明依据并等待维护者指令,而非直接执行。

## 协作边界:OpenSpec 生命周期动作的发起权

OpenSpec 流程 `propose → apply → sync → archive` 中,**改变 change 状态的动作必须由维护者(人类)发起**,agent 只能在维护者指令下执行具体工作。

| 动作 | 谁发起 | agent 能做什么 | 用的命令 |
|---|---|---|---|
| **propose**(新建 change) | 🔒 维护者 | 用 `/opsx:propose` 命令调 CLI 脚手架生成 artifacts | `/opsx:propose <name>` |
| **apply**(实现 tasks) | 🔒 维护者 | 用 `/opsx:apply` 命令写代码、跑测试、勾 tasks | `/opsx:apply <name>` |
| **archive**(归档 change) | 🔒 维护者 | 用 `/opsx:archive` 命令归档 + 同步 specs | `/opsx:archive <name>` |
| **sync**(specs 并入主线) | 🔒 维护者 | 用 `/opsx:sync` 命令（通常跟 archive 一起）| `/opsx:sync <name>` |

**红线**:agent 不得擅自新建 change、不得擅自归档、不得擅自把 change 标记完成或重新定义验收范围。这三类动作改变仓库结构语义,必须反映维护者真实意图。若 agent 判断某 change 应推进,需说明依据并等待指令。
