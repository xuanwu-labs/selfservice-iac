# Proposal: platform-db-schema

## What

构建平台控制面 PostgreSQL 表结构，围绕 MVP 主链路推导初版 schema，对齐已冻结的 proto 契约，并建立全库统一的命名/审计/约束规范基线。这是 W1 模块 02（元数据存储）的前置依赖——schema 先冻结，再落迁移。

**为什么独立成 change**：docs/04-数据库设计.md 是设计草稿（约 52 张表），审计发现 12 项 A 级断裂（proto 字段在表里缺列、实体归属三处矛盾、动态分层链路在 stack 端断开）、12 项 B 级不一致（命名/枚举取值域对不齐）、8 项 C 级规范瑕疵（无 FK/无 CHECK/无索引/审计字段不一）。不能直接拿来写迁移 SQL。本 change 先规范基线 + 落 MVP 最小集，后续随实现逐步打磨完善。

## Why

1. **proto 是唯一契约源（D1 Connect-native），表结构必须对齐 proto**——审计发现 requests 表缺 team_id/source/correlation_id/cost_estimate，CatalogItem proto 暴露字段过少，module_versions 缺 dependencies/providers。这些是业务功能（审批 DSL 成本门禁、ListRequests 团队过滤、前端 cardinality 表单）的硬依赖。
2. **命名规范缺失导致系统性不一致**——docs/04 出现 name↔namespace、git_source↔source_url、version↔ref 等命名漂移。需对标业界（PostgreSQL snake_case 共识 + proto 字段名 = DB 列名的零映射原则）统一。
3. **审计属性/约束规范未定**——主键类型四种混用、无 FK 约束、无 CHECK、无 GIN 索引、deleted_at 缺失、审计字段逐表不一。需建立与 sqlc/pgx 集成的统一基线。

## MVP 范围（主链路最小表集）

MVP 主链路：管理员注册模块 → 发布目录项 → 用户申请 → plan → 审批 → apply。最小表集按域划分：

| 域 | proto 实体 | MVP 表 | 用途 |
|---|---|---|---|
| **组织** | Actor（common） | teams, projects, bundles | 团队/项目组/bundle 归属（bundle 可选） |
| **registry** | Module, ModuleVersion, ModuleDependency | modules, module_versions, module_dependencies | 模块注册/版本/跨层依赖契约 |
| **catalog** | CatalogItem | catalog_items | 服务目录发布项（含 layer_logical_id/cardinality/visibility） |
| **lifecycle** | LifecycleRequest, LifecycleEvent, PlanArtifact, ApprovalRun, ApprovalNodeRun, ApprovalDecisionRecord, GateResult | requests, request_events, plan_artifacts, approval_runs, approval_node_runs, approval_decisions, gate_results | 工单全生命周期 + 审批节点链 |
| **cloud** | CloudAccount | cloud_accounts | 云账号纳管（ListRequestableCloudAccounts 数据源） |
| **分层** | （PathGenerator 内部） | layer_logical_refs, layer_rule_set_versions | 固定三层 seed（Phase 1 只读） |
| **审计** | — | audit_logs, outbox_events | 审计 + 事件 outbox（Saga） |

**MVP 不含**（推迟）：stacks/stack_dependencies（Wave 2 codegen 后）、drift_runs/drift_records（Wave 4）、CMDB/FinOps 表（Wave 4）、CICD/gate 表（Wave 5）、import 表（Wave 5）、stack_grouping_rules/layer_migrations（Phase 2/3）。

**MVP 表数**：约 18-20 张（初版，随实现打磨）。

## Impact

- **影响层级**：数据层（server/data + server/pkg/db + server/cmd/migrate/migrations）
- **兼容性**：不破坏现有——当前仅 teams 1 张表落地，本 change 重写 teams 对齐新规范 + 新增 MVP 表
- **依赖**：proto 契约已冻结（contracts/platform/v1/）；sqlc/pgx 基建已就位（server/pkg/db/）
- **被消费方**：W1 的 02/03/04 模块按此 schema 落迁移 + sqlc 查询；后续 Wave 按规范增量加表

## Deliverables

| 产物 | 位置 | 状态 |
|---|---|---|
| 命名/审计/约束规范基线 | `design.md` | ⏳ 本 change 产出 |
| MVP schema 迁移 SQL（重写 teams + 新增 ~18 表） | `server/cmd/migrate/migrations/` | ⏳ apply 阶段产出 |
| sqlc 查询 + 重新生成 | `server/pkg/db/{queries,generated}/` | ⏳ apply 阶段产出 |
| docs/04 标注修订（断裂点标记 + 指向本 change） | `docs/04-数据库设计.md` | ⏳ apply 阶段产出 |

## 原则

1. **proto 字段名 = DB 列名**（零映射），proto 没有的内部字段用 `_json`/`_at` 后缀明确区分。
2. **命名对标业界**：snake_case、复数表名、单数列名、`pk_/fk_/uq_/ck_/ix_` 约束前缀。
3. **MVP 初版可推导，后续随实现打磨**——初版覆盖主链路最小集，不追求一步到位落 52 张表。
4. **sqlc/pgx 集成**：审计字段用 PG `DEFAULT now()` + trigger 自动维护（sqlc 不自动管审计列，需 DB 侧 trigger）。
