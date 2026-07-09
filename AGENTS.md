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

## 边界

- ❌ 不要修改 `../terramate/` 仓库(那是上游开源引擎,独立项目)。
- ❌ 不要让 `server/**` import `github.com/terramate-io/terramate/<任何子包>`(D1 边界,由 depguard 强制)。
- ❌ 不要覆盖 `openspec/config.yaml` 的定制 context/rules。
- ✅ 新增 change 用 `/opsx:propose`,不要手建 `changes/<name>/` 目录。
- ✅ 实现任务前先 `/opsx:explore` 或读 design.md,确保理解决策 D1–D45。
- ✅ `docs/` 是用户文档,`openspec/` 是设计文档,两者职责不同,勿混淆。
