# 审计报告：platform-db-schema × postgresql-table-design skill

> 用 `wshobson/agents` 的 `plugins/database-design/skills/postgresql/SKILL.md`（已安装到 `.zcode/skills/postgresql-table-design/`）作为对账清单，逐条审 `design.md`（当前在 `feat/platform-db-schema` 分支，未合并 main）。
>
> 约定：
> - **P0 = 违反 skill 明确的 DO/DO NOT 硬规则**，必须改（否则 skill 直接判不合格）。
> - **P1 = skill 强烈推荐、本设计遗漏**，强烈建议改。
> - **P2 = skill 提及、本设计有改进空间**，可选。
> - **OK = skill 要求、本设计已满足**。

---

## 汇总

| 等级 | 数量 | 说明 |
|---|---|---|
| P0 | 5 | 违反 skill 硬规则（VARCHAR、serial 痕迹、FK 无索引、UNIQUE NULLs、TIMESTAMPTZ 无精度说明但 OK）|
| P1 | 4 | skill 强烈推荐但缺失 |
| P2 | 3 | 可选优化 |
| OK | 7 | 已合规 |

---

## P0 — 违反 skill 硬规则

### P0-1 ❌ `VARCHAR(n)` 应改 `TEXT` + `CHECK(LENGTH(col) <= n)`
**skill 原文（"Do not use"）：** `DO NOT use char(n) or varchar(n); DO use text instead.`
**skill 原文（Data Types > Strings）：** "if length limits needed, use `CHECK (LENGTH(col) <= n)` instead of `VARCHAR(n)`"。

**design.md 当前违规处（7 处）：**
- §1.5 总则：`status VARCHAR(32) NOT NULL CHECK (status IN (...))`
- §03 A7 `layer_logical_refs.logical_id VARCHAR(64) PK`
- §04 A2 `cost_currency VARCHAR(8)`
- §04 A4 `correlation_id VARCHAR(64)`
- §04 A5 `source VARCHAR(32) CHECK`
- §03 B4 `oidc_providers` 等（多处隐式 VARCHAR）

**修复：** 全部改 `TEXT`；如确需长度上限，加 `CHECK (LENGTH(col) <= 32)`。枚举列改成 `TEXT NOT NULL CHECK (...)`，长度由枚举域本身保证，无需 VARCHAR。

---

### P0-2 ❌ FK 列未强制手动索引（skill 红线）
**skill 原文（Core Rules #4）：** "Create indexes for access paths you actually query: PK/unique (auto), **FK columns (manual!)**"
**skill 原文（Gotchas）：** "**FK indexes**: PostgreSQL **does not** auto-index FK columns. Add them."
**skill 原文（Constraints > FK）：** "Add explicit index on referencing column—speeds up joins and prevents locking issues on parent deletes/updates."

**design.md 当前：** §1.4 约束命名表有 `ix_<table>_<col>`，但**全文未规定"每个 FK 必须配索引"**。`grep "INDEX.*_id"` 命中 0。

**后果（skill 明示）：** 父表（如 teams）删除/更新时，子表无索引会全表锁。本设计 `ON DELETE RESTRICT`，但 RESTRICT 也要扫子表找引用——无索引全表扫 + 锁。

**修复：** 在 §1.3 外键策略后加一条硬规则：
> **每个 FK 列必须建索引**（`CREATE INDEX ix_<table>_<fk_col> ON <table>(<fk_col>)`）。无例外。多 FK 列按访问路径考虑复合索引。

---

### P0-3 ❌ UNIQUE 未用 `NULLS NOT DISTINCT`（PG15+）
**skill 原文（Gotchas）：** "UNIQUE allows multiple NULLs. Use `UNIQUE (...) NULLS NOT DISTINCT` (PG15+) to restrict to one NULL."
**skill 原文（Constraints > UNIQUE）：** "Prefer `NULLS NOT DISTINCT` unless you specifically need duplicate NULLs."

**design.md 当前：**
- §2.1 软删除：`CONSTRAINT uq_stacks_path UNIQUE (bundle_id, component, env, deleted_at)` — 这里 `deleted_at` 是 NULL（未删）时，PG 默认允许多行同 `(b, c, e, NULL)`，**软删语义实际不成立**：两条未删 stack 可以共存。

  这其实是软删 + UNIQUE 的经典坑：用 `deleted_at` 进 UNIQUE 当"未删时不约束，删了才约束"是**反过来**的——未删时多行 NULL 被放行（不约束），删了时不同时间戳才约束（已晚）。

**正确做法（二选一）：**
1. **`NULLS NOT DISTINCT`**：未删时（`deleted_at NULL`）也只允许一行——但这与"软删后可重建同名"冲突。
2. **partial unique index（skill 间接支持，Upsert-Friendly 提到 partial index 限制）**：
   ```sql
   CREATE UNIQUE INDEX uq_stacks_path_active
       ON stacks (bundle_id, component, env)
       WHERE deleted_at IS NULL;
   ```
   这是软删 + 唯一性的业界标准解（GitHub/Linear 都这么干）——**只约束未删行**，删了可重建。

**修复：** §2.1 软删除规则改为"未删行唯一用 partial unique index (`WHERE deleted_at IS NULL`)；不用 `deleted_at` 进 UNIQUE 列"。并把 §03 所有含 `deleted_at` 的 UNIQUE 改写。

---

### P0-4 ❌ `BIGSERIAL` 痕迹与 snowflake 决策的 skill 张力
**skill 原文（Core Rules #1）：** "prefer `BIGINT GENERATED ALWAYS AS IDENTITY`; use `UUID` only when global uniqueness/opacity is needed."
**skill 原文（Do not use）：** "DO NOT use `serial` type; DO use `generated always as identity` instead."
**skill 原文（Insert-Heavy）：** "Prefer `BIGINT GENERATED ALWAYS AS IDENTITY` over `UUID`."

**张力分析：**
- 现有 `001_init.sql` 的 teams 用的是 `BIGSERIAL`——**直接违反 skill 的 DO NOT use serial**。
- design.md §1.2 决策是**应用层 snowflake，DB 列 `BIGINT PRIMARY KEY` 无 DEFAULT/IDENTITY**。

这跟 skill 的"`GENERATED ALWAYS AS IDENTITY` 优先"**不是直接冲突**，但是两种不同的 ID 来源哲学：
- skill 的 IDENTITY = DB 生成（集中、单源、无应用协调）。
- snowflake = 应用生成（分布式、时间有序、需 machineID 协调）。

**关键：** skill 并没有禁止应用生成 ID，但它把 `BIGINT GENERATED ALWAYS AS IDENTITY` 列为**默认首选**，snowflake 属于"分布式部署需要"的例外场景。本平台 MVP 单实例，分布式是 Wave 之后的事。

**修复（两选一，需用户决策）：**
- **方案 A（贴合 skill 默认，MVP 最简）：** MVP 用 `BIGINT GENERATED ALWAYS AS IDENTITY`（DB 生成），snowflake 工具类留 `server/internal/utils/snowflake.go` 作为占位，到多实例 Wave 再切。优点：贴合 skill 默认、零应用协调、DB 单源。缺点：未来切 snowflake 要迁移现有行（但 MVP 无业务数据，零成本）。
- **方案 B（坚持 snowflake，需在 design.md 显式论证例外）：** 在 §1.2 加一段"为何偏离 skill 默认"：明确写"本平台从 day 1 就为多实例设计，snowflake ID 提前到位避免后期迁移；int64 + 时间有序对 B-Tree 友好"。skill 接受这种论证（"use UUID only when..."的同类例外逻辑）。

**当前 design.md 缺这段论证**——读起来像"我们就是要 snowflake"，但没解释为何不用 skill 默认的 IDENTITY。无论选 A/B，**现有 001_init.sql 的 `BIGSERIAL` 必须改**（P0，违反 DO NOT use serial）。

---

### P0-5 ❌ TIMESTAMPTZ 无统一精度声明（轻微，但 skill 有硬规则）
**skill 原文（Do not use）：** "DO NOT use `timestamptz(0)` or any other precision specification; DO use `timestamptz` instead."

**design.md 当前：** §2.1 写 `TIMESTAMPTZ NOT NULL DEFAULT now()`——**已合规**（无精度后缀）。✅

**但 §03 表清单里有几处时间列只写列名未标类型**（如 `registered_at`, `decided_at`, `expires_at`, `evaluated_at`, `occurred_at`, `last_synced_at`, `rotated_at`, `first_seen_at`）——需在落迁移时**全部确认是 `TIMESTAMPTZ` 不是 `TIMESTAMP`**。

**修复：** §2.1 加一句硬规则："所有时间列一律 `TIMESTAMPTZ`，禁用 `TIMESTAMP`（without tz）和 `timestamptz(n)`（带精度）。"

---

## P1 — skill 强烈推荐但缺失

### P1-1 ⚠️ 缺 money/NUMERIC 规范
**skill 原文：** "**NUMERIC** for money"; "DO NOT use `money` type; DO use `numeric` instead."

**design.md 当前：** §03 用 `cost_estimate_cents BIGINT` + `cost_currency`（存分，整数）——这其实是**比 NUMERIC 更好的选择**（避免浮点，cents 整数存储是业界共识，Stripe/Shopify 都这么干）。但 design.md **没写这条规范**，读者会困惑"为何不用 NUMERIC"。

**修复：** §1 加一条数据类型规范："**金额一律存整数分 `BIGINT`（`*_cents` 后缀），不用 NUMERIC/money**——避免浮点，对齐 Stripe 等业界惯例。币种独立列 `currency TEXT`。"这是对 skill 的合理偏离，但要写明理由。

---

### P1-2 ⚠️ JSONB 缺统一索引规范
**skill 原文（JSONB Guidance）：** "Prefer `JSONB` with **GIN** index." + 区分默认 GIN vs `jsonb_path_ops`。

**design.md 当前：** §03 多处 `*_json`（tags_json, policy_json, form_schema_json, defaults_json, visibility_json, regions_json, payload_json...），§A3 提到"visibility_json 建 GIN 索引"——但**没立统一规范**：哪些 `*_json` 该建 GIN、哪些不用。

**修复：** §1 加一条："JSONB 索引规范：**仅对需要按内容过滤的 JSONB 列建 GIN**（如 visibility_json 按 team 过滤、tags_json 按标签查）。schema 文档（form_schema_json）、只读快照（before_json/after_json）不建索引。GIN 默认 opclass，仅当确认只做 `@>` 不做 `?` 查询时改 `jsonb_path_ops`。"

---

### P1-3 ⚠️ 缺 partial index 规范（除软删外）
**skill 原文（Indexing > Partial）：** "for hot subsets (`WHERE status = 'active'` → `CREATE INDEX ON tbl (user_id) WHERE status = 'active'`)."

**design.md 当前：** 仅 §2.1 软删除提到 partial index（还是 P0-3 要补的）。但本设计有大量 `status` 列（requests/approval/cloud_accounts/...）——**热查询通常只查 active 行**。

**修复：** §1 加一条可选规范："高频 `status` 过滤（如只查 active requests）考虑 partial index `WHERE status IN ('submitted','generating',...)`，缩小索引体积。"标为可选，按实际查询模式决定。

---

### P1-4 ⚠️ 缺 `fillfactor` / 更新模式规范（update-heavy 表）
**skill 原文（Update-Heavy Tables）：** "Use `fillfactor=90` to leave space for HOT updates." + "Avoid updating indexed columns."

**design.md 当前：** 无。本平台 `requests` 表 status 频繁更新（状态机），`updated_at` 每次 UPDATE 都动——是典型 update-heavy 表。

**修复：** §1 加一条（标可选）："update-heavy 表（requests, approval_runs, executor_runs）考虑 `fillfactor=90` 提升 HOT update 命中。"这是性能调优项，MVP 可不做，但写明供后续。

---

## P2 — 可选优化

### P2-1 ℹ️ 枚举类型选择：CREATE TYPE AS ENUM vs TEXT+CHECK
**skill 原文：** "`CREATE TYPE ... AS ENUM` for small, stable sets (US states, days of week). For business-logic-driven and evolving values (order statuses) → use TEXT (or INT) + CHECK or lookup table."

**design.md §1.5 决策：** `TEXT + CHECK`（不用 PG ENUM，理由：加值要 ALTER TYPE 迁移麻烦）。

**评估：** 这跟 skill **完全一致**——skill 明确说"evolving values → TEXT + CHECK"。本平台 status 取值会随业务演进（plan/apply/drift/import phase、approval status），属 evolving。✅ **已合规**，无需改。但 design.md 没引用 skill 这条作为依据，可补一句"对齐 skill：evolving values 用 TEXT+CHECK"。

---

### P2-2 ℹ️ `tags_json` vs 数组类型
**skill 原文（Arrays）：** "`TEXT[]`, `INTEGER[]`... Good for tags, categories."
**skill 原文（JSONB）：** "Arrays inside JSONB: use GIN + `@>` for containment (e.g. tags)."

**design.md 当前：** teams/catalog_items 的 `tags_json` 用 JSONB 存标签数组。

**评估：** skill 两种都可（数组类型 vs JSONB 内数组）。JSONB 更灵活（标签可能带值），但纯字符串标签用 `TEXT[]` + GIN 更轻。**可选**，不改也行，但可在 §1 注明"纯字符串标签集未来可优化为 `TEXT[]` + GIN"。

---

### P2-3 ℹ️ range 类型机会（environments / leases）
**skill 原文（Range types）：** "`tstzrange` for intervals... Good for scheduling, versioning."

**design.md 当前：** `workspace_checkouts.leased_until TIMESTAMPTZ`（点时间，不防重叠）、`sessions.expires_at`、`approval_runs.expires_at`。

**评估：** 如未来要查"哪些 checkout 租约现在有效且不重叠"，`tstzrange [leased_at, leased_until)` + GiST + EXCLUDE 约束更优雅（skill 的 exclusion constraint 例子就是防重叠）。**MVP 不必**，但记一笔供 Wave 2-3 考虑。

---

## OK — 已合规项

| # | skill 要求 | design.md 状态 |
|---|---|---|
| OK-1 | snake_case 命名 | §1.1 ✅ |
| OK-2 | 复数表名 | §1.1 ✅ |
| OK-3 | TIMESTAMPTZ（不带精度） | §2.1 ✅（需在落迁移时全员确认，见 P0-5）|
| OK-4 | 3NF 优先 | §03 表结构是规范化的 ✅ |
| OK-5 | NOT NULL + DEFAULT | §2.1 created_at/updated_at 都 NOT NULL DEFAULT now() ✅ |
| OK-6 | evolving enum 用 TEXT+CHECK | §1.5 ✅（与 skill 一致）|
| OK-7 | PK 显式定义 | §1.2 ✅ |

---

## 整改清单（按优先级排）

落迁移 SQL 前**必须**处理 P0（违反 skill 硬规则 = 不合格）：

- [ ] **P0-1**：全表 `VARCHAR(n)` → `TEXT`（+ `CHECK(LENGTH())` 如需上限）。改 §1.5、§03 所有表、§04 修复条目。
- [ ] **P0-2**：§1.3 加硬规则"每 FK 必须配 `ix_<table>_<fk_col>` 索引"，并在 §03 每张表的 FK 列后标注索引。
- [ ] **P0-3**：§2.1 软删除 UNIQUE 改为 partial unique index `WHERE deleted_at IS NULL`；§03 含 deleted_at 的 UNIQUE 全改。
- [ ] **P0-4**：用户决策 snowflake（方案 B 加论证）vs IDENTITY（方案 A MVP 默认）；无论如何 `001_init.sql` 的 `BIGSERIAL` 必改。
- [ ] **P0-5**：§2.1 加"时间列一律 TIMESTAMPTZ，禁 TIMESTAMP/timestamptz(n)"硬规则。

P1 强烈建议（提升规范性，可本轮或下轮）：

- [ ] **P1-1**：§1 加金额规范（整数分 BIGINT，不用 NUMERIC/money）+ 偏离理由。
- [ ] **P1-2**：§1 加 JSONB GIN 索引规范（哪些列建、哪些不建）。
- [ ] **P1-3**：§1 加 partial index 规范（可选，按查询模式）。
- [ ] **P1-4**：§1 加 fillfactor/update-heavy 规范（可选，MVP 可不做）。

P2 可选（记一笔供后续 Wave）：

- [ ] **P2-1**：§1.5 补"对齐 skill：evolving enum 用 TEXT+CHECK"一句（文档完善）。
- [ ] **P2-2**：§1 注明 tags_json 未来可优化为 `TEXT[]`（可选）。
- [ ] **P2-3**：§03 B1 checkout 租约未来可考虑 `tstzrange` + EXCLUDE（Wave 2-3 考虑）。

---

## 对 tasks.md 的影响

P0 整改需要在 `tasks.md` 加任务（否则实现时会漏）：

- **01-规范基线**段需加：
  - 1.4 §1 加数据类型规范（TEXT/TIMESTAMPTZ/BIGINT cents/JSONB GIN/FK 索引/partial index/fillfactor）
  - 1.5 §2.1 软删除改 partial unique index
  - 1.6 §1.2 snowflake vs IDENTITY 决策落地（含论证）
- **02-组织归属表**及之后每段：实现时强制每 FK 配索引、枚举用 TEXT+CHECK、金额用 BIGINT cents。

## 决策点（需用户拍板）

**P0-4 是唯一需要用户决策的项**：MVP 用 DB `IDENTITY`（贴合 skill 默认）还是应用层 snowflake（提前为多实例准备）。其余 P0/P1/P2 我可按报告直接改 design.md + tasks.md，不需要决策。
