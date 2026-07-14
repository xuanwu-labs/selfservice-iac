# 01-平台数据库

## ADDED Requirements

### Requirement: 平台控制面数据库 schema

平台 MUST 使用 PostgreSQL 作为控制面元数据库，schema 遵循统一的命名规范（snake_case、复数表名、`pk_/fk_/uq_/ck_/ix_` 约束前缀）和审计规范（created_at/updated_at + trigger、软删除 deleted_at）。

#### Scenario: MVP 主链路表创建

- **WHEN** 运行 `goose up`
- **THEN** 20 张 MVP 表全部创建成功（teams/projects/bundles/modules/module_versions/module_dependencies/catalog_items/requests/request_events/plan_artifacts/gate_results/approval_flows/approval_runs/approval_node_runs/approval_decisions/cloud_accounts/audit_logs/outbox_events/layer_logical_refs/layer_rule_set_versions）
- **AND** 3 条 layer seed（global/middleware/application）+ 1 条 layer_rule_set_versions（v1 active）落地
- **AND** set_updated_at() trigger 函数创建

#### Scenario: 迁移幂等

- **WHEN** 依次执行 `goose up` → `goose down` → `goose up`
- **THEN** 全部成功，无错误
- **AND** 最终状态与首次 up 一致

### Requirement: 雪花 ID 统一主键

所有表的主键 MUST 使用应用层生成的雪花 ID（BIGINT），不使用 DB 自增（serial/identity）。ID 由 `server/internal/utils/snowflake.go` 的 `GenerateID()` 生成。

#### Scenario: 应用层生成 ID

- **WHEN** INSERT 一条记录
- **THEN** 应用层调 `utils.GenerateID()` 生成 BIGINT ID
- **AND** DB 列无 DEFAULT 自增

### Requirement: proto 枚举与 DB CHECK 对齐

DB CHECK 约束的取值域 MUST 与 proto enum（`contracts/platform/v1/common/enum.proto`）对齐——剥前缀后 lowercase + snake_case。proto 是冻结的契约唯一源（D45），DB 对齐 proto 而非 docs。

#### Scenario: 枚举值一致性

- **WHEN** proto RequestStatus 有 19 个非零值
- **THEN** requests.status CHECK 包含对应的 19 个 snake_case 值
- **AND** 无 docs 残留的 kebab-case 值（如 `pending-admission` → `pending_admission`）

### Requirement: 非 MVP 表设计定稿

非 MVP 表（B1-B15，约 48 张）的字段/类型/约束/FK/索引 MUST 在 design.md §03 全部推导定稿，但不落迁移 SQL——等对应 Wave 实现功能时按定稿直接建表。

#### Scenario: 非 MVP 表定稿可用

- **WHEN** Wave 2 实现 stacks 表
- **THEN** 按 design.md §03 B2 定稿的字段/约束/FK 直接写迁移 SQL
- **AND** 不需要回头重新设计表结构
