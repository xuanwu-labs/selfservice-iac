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

## Git 提交语言（开源规范，强制）

**所有 commit message 必须全英文**（subject + body）。这是开源项目的基本要求。中文文档内容（docs/*.md、migration SQL 的 PG COMMENT、代码注释引用中文 doc）不在此限制内——限制的是 git commit message。详见 `server/AGENTS.md` 的 "Git 提交规范" 段。

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

| 动作 | 谁发起 | agent 能做什么 |
|---|---|---|
| **propose**(新建 change) | 🔒 维护者 | 起草 proposal/design/tasks **内容**;新建 change 目录/分支由维护者指令触发 |
| **apply**(实现 tasks) | 🔒 维护者 | 写代码、跑测试、改文档(在维护者明确指令"开始实现 X"之后) |
| **archive**(归档 change) | 🔒 维护者 | 验证 task 完成度、报告状态;归档操作由维护者指令触发 |
| **sync**(specs 并入主线) | 🔒 维护者 | 跟随 archive 一起做 |

**红线**:agent 不得擅自新建 change、不得擅自归档、不得擅自把 change 标记完成或重新定义验收范围。这三类动作改变仓库结构语义,必须反映维护者真实意图。若 agent 判断某 change 应推进,需说明依据并等待指令。
