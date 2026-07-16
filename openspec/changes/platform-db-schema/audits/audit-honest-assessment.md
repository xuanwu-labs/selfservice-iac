# 诚实评估：表结构设计 vs skill 规范 vs proto 契约 vs 职责划分

> 读完 proto 契约（5 域 / 11 文件 / 29 RPC + 66 message）+ 全部设计文档后的真实评估。不报喜不报忧，只列事实。

---

## 第 1 点：远程 Docker 声明在哪

**已找到**：`server/.env.example` 行 28-31 + `server/AGENTS.md` 行 145。
```
DOCKER_HOST=tcp://192.168.31.33:23750
TESTCONTAINERS_RYUK_DISABLED=true
```
我之前只查 `env | grep docker`（shell 没 export .env）就放弃，是错的。`.env.example` 是模板，实际 `.env`（gitignored）才有真值。

**当前状态**：socat proxy 容器被我误删（清理时没排除 `docker-api-proxy`），需你在 192.168.31.33 重建。重建后迁移测试可跑（3 个 bug 已修：PK 重复/trailing comma/goose v0）。

---

## 第 2 点：PG skill 规范遵从度 + 职责划分

### skill 规范遵从（迁移 SQL grep 验证）

| skill 硬规则 | 状态 | 证据 |
|---|---|---|
| TEXT 替代 VARCHAR(n) | ✅ | 迁移 0 列用 VARCHAR(n) |
| TIMESTAMPTZ 不带精度 | ✅ | 全迁移 TIMESTAMPTZ |
| BIGINT（非 serial）| ✅ | 0 BIGSERIAL；snowflake 应用层生成 |
| FK 必须手动索引 | ✅ | 23 REFERENCES / 65 ix_ |
| 软删 partial unique | ✅ | `WHERE deleted_at IS NULL` |
| JSONB GIN | ✅ | visibility_json + user_allowed_tag_keys_json |
| TEXT+CHECK 枚举 | ✅ | 全枚举列有 CHECK |
| 禁 money/char(n)/timetz | ✅ | grep 0 命中 |

**结论**：skill 硬规则全遵从。

### 职责划分（按域）

| 域 | 表 | 职责 | 清晰度 |
|---|---|---|---|
| 组织(A1) | teams/projects/bundles | 团队/项目组/bundle 归属 | ✅ 清晰 |
| registry(A2) | modules/versions/dependencies | 模块注册/版本/跨层依赖 | ✅ 清晰 |
| catalog(A3) | catalog_items | 服务目录发布项 | ✅ 清晰 |
| lifecycle(A4-A5) | requests/events/plan_artifacts/gate_results/approval×4 | 工单全生命周期+审批 | ✅ 清晰 |
| cloud(A6) | cloud_accounts | 云账号纳管 | ✅ 清晰 |
| layer(A7) | layer_logical_refs/rule_set_versions | 分层配置 | ⚠️ 见下断裂 |
| 审计(A8) | audit_logs/outbox_events | 审计+事件 | ✅ 清晰 |

**职责不清/断裂（真实问题，不是凑数）**：

### 断裂 A：proto 枚举 vs DB CHECK 不一致（最严重，影响 MVP 能否跑通）

读 proto `common/enum.proto` 后发现 **7 处枚举值域不一致**：

| 字段 | proto enum 值 | DB CHECK 值 | 差异 |
|---|---|---|---|
| RequestStatus | 16 值（无 blocked-policy/blocked-state-health/paused-drift）| 19 值（含这 3 个）| **DB 多 3 个 proto 没有的** — 这些是 doc 12a 健康状态，proto 还没加 |
| ApprovalRunStatus | UNSPECIFIED/PENDING/APPROVED/REJECTED/EXPIRED | pending/approved/rejected/timeout/cancelled | **proto 是 EXPIRED，DB 是 timeout；proto 无 cancelled** |
| ArtifactStatus | UNSPECIFIED/READY/EXPIRED/CONSUMED | active/superseded/expired/orphan | **完全不同的命名体系** — proto READY≠DB active |
| GateSeverity | UNSPECIFIED/ERROR/WARNING | info/warning/error/critical | **DB 多 info/critical** |
| ApprovalNodeMode | UNSPECIFIED/SINGLE/COUNTERSIGN/CONDITIONAL | any/all/majority/count>=N | **完全不同** — proto 用 doc 04 的值，DB 用 doc 12 的值 |
| CloudProvider | UNSPECIFIED/AWS/ALIYUN/AZURE/GCP | alicloud/aws/azure | **proto 有 GCP，DB 没有** |
| CatalogItemStatus | UNSPECIFIED/ACTIVE/DEPRECATED | draft/active/deprecated/archived/blocked | **DB 多 draft/archived/blocked** — proto 只有 2 个业务态 |

**影响**：这是 MVP 能否跑通的**硬阻塞**。handler 从 proto enum 映射到 DB CHECK 时，值对不上会直接拒绝写入。比如 `ApprovalNodeMode` proto 是 `SINGLE/COUNTERSIGN/CONDITIONAL`，DB CHECK 是 `any/all/majority/count>=N` — 完全无法映射。

**根因**：我 design.md 的枚举值取自 docs（doc 12 §2.3 说 any/all/majority），但 proto enum 用的是 doc 04 §2.10 的值（single/countersign/conditional）。**proto 是契约冻结的唯一源（D45），DB 必须对齐 proto，不是对齐 docs**。

**修复方向**：DB CHECK 值域改为对齐 proto enum（剥前缀后 lowercase）。proto 没有的值（如 blocked-policy）要么 proto 加，要么 DB 不放主 status（放子表）。这是 apply 阶段必须先解决的。

### 断裂 B：layer ↔ team 绑定无结构化映射（上轮已发现，确认仍在）

- `teams.kind` = `[platform|dba|middleware|business]`
- `layer_rule_set_versions.layers_json.owning_team_pattern` = 字符串/正则（`dba|middleware`）
- 二者靠字符串匹配，无 FK 约束。MVP 固定三层能跑，Wave 2 层版本化会断。

### 断裂 C：`team_cloud_grants.allowed_layers` 用硬编码层名

- docs/04 行 376：`allowed_layers -- [Global|Middleware|Application] 子集`
- D26 让 catalog_items 用 `layer_logical_id`（跨版本稳定身份），但 allowed_layers 还用硬编码名 → 层改名后失效。

### 断裂 D：`modules.layer` 与 `catalog_items.layer_logical_id` 冗余

- modules 表有 `layer` 字段，catalog_items 有 `layer_logical_id`
- 无一致性校验。一个模块被多 catalog 项以不同 layer 注册 → modules.layer 单值冲突。

---

## 第 3 点：MVP 表能否跑核心逻辑 + 短路 + 非 MVP 改动

### MVP 核心逻辑跑通评估

主链路：注册模块 → 发布目录 → 用户申请 → plan → 审批 → apply → 审计

| 步骤 | 涉及表 | 能跑？ | 短路点 |
|---|---|---|---|
| 注册模块 | modules + module_versions + module_dependencies | ✅ | — |
| 发布目录项 | catalog_items | ✅ | layer_logical_id FK 由 010 backfill ✓ |
| 用户申请 | requests + request_events | ⚠️ | **断裂 A：status/kind/source 的 CHECK 值域与 proto enum 不一致** |
| plan | plan_artifacts | ⚠️ | **断裂 A：status CHECK 值（active/superseded/expired/orphan）与 proto ArtifactStatus（READY/EXPIRED/CONSUMED）不一致** |
| 审批 | approval_flows/runs/node_runs/decisions | ❌ | **断裂 A：node_runs.mode CHECK（any/all/majority）与 proto（SINGLE/COUNTERSIGN/CONDITIONAL）完全不同；runs.status CHECK（timeout/cancelled）与 proto（EXPIRED）不同** |
| gate | gate_results | ⚠️ | **断裂 A：severity CHECK 多 info/critical** |
| apply | (复用 requests + plan_artifacts) | ✅ | — |
| 审计 | audit_logs + outbox_events | ✅ | — |
| 云账号 | cloud_accounts | ⚠️ | **断裂 A：provider CHECK 缺 GCP** |

**结论**：**MVP 不能直接跑通**——审批模块的枚举值域与 proto 完全对不上，handler 无法映射。这是我的设计错误：枚举值取自 docs 而非 proto（proto 是冻结的契约唯一源）。

### 非 MVP 改动

非 MVP 表（B1-B15）只在 design.md 定稿，没落迁移。本轮 docs 通读后已补全（11 张丢失表 + 字段对齐）。**非 MVP 没有需要再改的**——它们的定稿是设计层面的，落迁移时按定稿走即可。但断裂 B/C/D（layer 绑定）会影响 Wave 2 层版本化，需在 Wave 2 落迁移前修。

---

## 必须修的（按优先级）

### P0（MVP 跑不通，必须先修）：proto 枚举对齐

DB CHECK 值域改为对齐 proto enum。7 处不一致，每处要么改 DB CHECK 值，要么 proto 加值（后者需你发起 proto change）。

具体方案（推荐 DB 对齐 proto，因为 proto 已冻结）：
1. RequestStatus：DB 改 16 值（去掉 blocked-policy/blocked-state-health/paused-drift，这些放 request_events 或子表）
2. ApprovalRunStatus：DB 改 pending/approved/rejected/expired（去掉 timeout/cancelled）
3. ArtifactStatus：DB 改 ready/expired/consumed（去掉 active/superseded/orphan）
4. GateSeverity：DB 改 error/warning（去掉 info/critical）
5. ApprovalNodeMode：DB 改 single/countersign/conditional（去掉 any/all/majority/count>=N）
6. CloudProvider：DB 加 gcp
7. CatalogItemStatus：DB 改 active/deprecated（去掉 draft/archived/blocked——这些是 doc 19 的运营状态，proto 没冻结，MVP 不放 CHECK，或单独加 proto 值）

### P1（Wave 2 前修）：layer 绑定断裂 B/C/D

- teams.kind ↔ owning_team_pattern 加映射规则或文档约束
- team_cloud_grants.allowed_layers 改用 layer_logical_id
- modules.layer 去掉或改 layer_logical_id

### P2（已在 design.md 定稿，非 MVP 无需改）

B1-B15 表设计已完成，落迁移时按定稿走。
