# Proposal: platform-db-schema

## What

构建平台控制面 PostgreSQL 表结构，围绕 MVP 主链路推导初版 schema，对齐已冻结的 proto 契约，并建立全库统一的命名/审计/约束规范基线。这是 W1 模块 02（元数据存储）的前置依赖——schema 先冻结，再落迁移。

**生命周期（长存 change）**：本 change **不是 MVP 落完即归档**——它是平台 DB 结构的活文档，随每个 Wave 演进：MVP 表先落迁移，非 MVP 表随对应 Wave 实现按定稿建表，任何字段调整都回写到 design.md。直到所有功能落地、DB 结构稳定不再改、docs/04 标注完成对齐，**才由 maintainer 发起 archive**。归档条件 = DB 结构冻结（不是 MVP 完成）。

**为什么独立成 change**：docs/04-数据库设计.md 是设计草稿（约 52 张表），审计发现 12 项 A 级断裂（proto 字段在表里缺列、实体归属三处矛盾、动态分层链路在 stack 端断开）、12 项 B 级不一致（命名/枚举取值域对不齐）、8 项 C 级规范瑕疵（无 FK/无 CHECK/无索引/审计字段不一）。不能直接拿来写迁移 SQL。本 change 先规范基线 + 落 MVP 最小集，后续随实现逐步打磨完善。

**规范基线已对账 `postgresql-table-design` skill**（wshobson/agents，安装于 `.zcode/skills/postgresql-table-design/`），审计报告见 `audit-postgresql-skill.md`（5 P0 / 4 P1 / 3 P2 / 7 OK）。整改已合入 §01-02。

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

**MVP 不含（但已设计定稿）**：stacks/stack_dependencies、drift、auth/identity、adapters_config、Saga 补偿、Run Hooks/运营、分层迁移、CMDB/FinOps、云凭据、环境租户、标签策略、CICD、存量导入——这些表（约 33 张）的**字段/类型/约束/FK 在本 change 全部推导定稿**（design.md §03 B1-B14），但不落迁移 SQL，等对应 Wave 实现功能时按定稿直接建表。这样后续 Wave 不用回头重新设计表结构。

**表数**：全量初版设计 ~52 张（design.md §03 定稿）；本 change 落迁移的是 MVP 19 张（优先级 A）。

## Impact

- **影响层级**：数据层（server/data + server/pkg/db + server/cmd/migrate/migrations）
- **兼容性**：不破坏现有——当前仅 teams 1 张表落地，本 change 重写 teams 对齐新规范 + 新增 MVP 表
- **依赖**：proto 契约已冻结（contracts/platform/v1/）；sqlc/pgx 基建已就位（server/pkg/db/）
- **被消费方**：W1 的 02/03/04 模块按此 schema 落迁移 + sqlc 查询；后续 Wave 按规范增量加表

## Deliverables

| 产物 | 位置 | 状态 |
|---|---|---|
| 命名/审计/约束规范基线 | `design.md` §01-02 | ⏳ 本 change 产出 |
| **全量 ~52 张表初版设计定稿** | `design.md` §03 A+B | ⏳ 本 change 产出 |
| MVP schema 迁移 SQL（重写 teams + 新增 ~18 表） | `server/cmd/migrate/migrations/` | ⏳ apply 阶段产出 |
| sqlc 查询 + 重新生成 | `server/pkg/db/{queries,generated}/` | ⏳ apply 阶段产出 |
| docs/04 标注修订（断裂点标记 + 指向本 change） | `docs/04-数据库设计.md` | ⏳ apply 阶段产出 |

## 原则

1. **proto 字段名 = DB 列名**（零映射），proto 没有的内部字段用 `_json`/`_at` 后缀明确区分。
2. **命名对标业界**：snake_case、复数表名、单数列名、`pk_/fk_/uq_/ck_/ix_` 约束前缀。
3. **MVP 初版可推导，后续随实现打磨**——初版覆盖主链路最小集，不追求一步到位落 52 张表。
4. **sqlc/pgx 集成**：审计字段用 PG `DEFAULT now()` + trigger 自动维护（sqlc 不自动管审计列，需 DB 侧 trigger）。
5. **雪花 ID 统一主键**：全库 `id BIGINT`，应用层 `internal/utils/snowflake.go` 工具类生成（参考 ferret + bwmarrin/snowflake）。所有 INSERT 调 `utils.GenerateID()`，DB 不自增。proto ID 字段保持 string（雪花 int64 ↔ string，避免 JSON 精度丢失）。
6. **对账 postgresql-table-design skill**：数据类型/约束/索引/审计规范以 skill 硬规则为准（详见 design.md §01-02 + audit-postgresql-skill.md）。skill 的硬 DO/DO NOT（如禁 `VARCHAR(n)`/`serial`/`money`、FK 必须手动索引、TIMESTAMPTZ 不带精度）作为本 change 的红线。
