## Context

W1-02 为 W1-03（registry/catalog）和 W1-04（tenancy/envtenant/stackmodel）准备元数据访问底座。

**现有架构约束**（脚手架已落地）：
- `server/data/data.go`：pgxpool provider + wire ProviderSet + `NewQueries(pool) → *generated.Queries`（otelpgx instrumented，D41）
- `server/pkg/db/queries/`：sqlc SQL 真相源（`schema.sql` 全表投影 + `teams.sql` 范例）
- `server/pkg/db/generated/`：sqlc 生成（`models.go` 24 个 model + `querier.go` + `teams.sql.go`）
- `server/pkg/db/testdb.go`：testcontainers + migrate up/down + clone
- `server/data/teams_test.go`：测试范例（snowflake Init + testdb.New + generated.Queries CRUD 断言）
- snowflake ID：`internal/utils.GenerateID()` 应用层生成，调用方传入
- 软删除：`deleted_at TIMESTAMPTZ NULL`，active 过滤 `WHERE deleted_at IS NULL`

**工程参考**：`E:\GolandProjects\ferret`（kratos + GORM 工程，已建立成熟的 data 层范式）：
- `data/data.go`：ProviderSet 注册所有 `NewXxxRepo`
- `data/query_wrapper.go`：MyBatis-Plus 风格查询构造器（动态条件 + 排序 + 分页）
- `data/<entity>.go`：每实体一个 Repo struct（`EmailRepo{log, *GormDB}`）+ 业务语义 CRUD
- `data/dbset/`：多表组合 struct（`User{entity.User, Address}`）
- `internal/model/entity/`：实体定义（GORM tag + TableName）
- `core/<domain>/<service>.go`：注入 Repo，调 `s.repo.FindByUser(...)`

**已有表**（25 张，migration 001-012）；**缺失表**（W1 必需）：environments/tenants/environment_tenant_bindings/tag_policies。

**server/AGENTS.md 内部矛盾**（本 change 修正）：
- line 68 "数据访问三件套：core/store(薄包装)→ data/→ pkg/db/generated"（Version B，core/store 违反 DIP）
- line 132 数据流 "core → dbset → pkg/db generated"（Version A，dbset 在 data 层）
- line 127 表格 "dbset 薄包装 data/dbset/ ... 空(等 core/store 落地)"（自相矛盾）

## Goals / Non-Goals

**Goals:**
- 落地 4 张 W1 必需表（env/tenant/binding/tag_policy）的 migration
- 为 15 个核心实体写 sqlc query + `sqlc generate` 重生成
- 建立 `data/repo/` 层（ferret 风 Repo struct，薄包装 generated.Queries + 事务 + 动态查询）
- 建立 `data/query_wrapper.go`（动态查询构造器，适配 pgx）
- 修正 server/AGENTS.md 矛盾 + data.go 注释，统一为混合范式
- testdb 测试模式扩展到 Repo 层

**Non-Goals:**
- 不建 `core/store/`（违反 DIP，由 `data/repo/` 取代）
- 不强求每表建 entity（opt-in，仅在 sqlc model 不够时）
- 不做 core/<domain>/ 业务逻辑（W1-03/04）
- 不落 W2 归属表的 query/Repo（身份/编排/漂移/审批）
- 不改 proto 契约
- 不改 DB schema 已落地表结构（纯增量）

## Decisions

### D1：混合范式架构（ferret Repo struct × DIP 可演进 × sqlc SQL-as-truth）

**决策**：数据访问层采用三层混合范式：

```
core/<domain>/  ──注入──→  data/repo/<entity>.go  ──薄包装──→  pkg/db/generated  ──→  PG
   │                          │                                        │
   │ 业务逻辑                   │ Repo struct                            │ sqlc model
   │ （需要测试时提取           │ （ferret 风，具体类型）                  │ （DB struct）
   │  小 interface，           │ + 业务语义 CRUD（GetByID/Create/List）
   │  Go 隐式，不改 data）     │ + 跨表事务（WithTx）
   │                          │ + 动态查询（query_wrapper）
   ↓                          ↓
DIP 可演进                    ferret 工程范式
```

**理由**（三源交叉验证）：
- **ferret 实证**：`data/email.go` 的 `EmailRepo{log, *GormDB}` + `FindByUser/Create/Creates`，core 直接注入 `repo *data.EmailRepo`，wire 在 `data.ProviderSet` 注册 `NewEmailRepo`。成熟落地，简单直接。
- **DIP 可演进**：Go interface 隐式（结构化类型）。core 需要测试时，在 core 定义 `type ModuleStore interface { GetModule(ctx, id) (Module, error) }`，`*data.repo.ModuleRepo` 自动满足——**无需改 data 层**。默认不提取 interface（YAGNI），需要时才提。
- **sqlc SQL-as-truth**：CRUD 的 SQL 在 `queries/*.sql`（编译期类型安全），Repo 方法是 `generated.Queries` 的薄包装。动态查询用 `query_wrapper + squirrel`。

**备选**：①纯 DDD（core 定义 interface，data 实现）——过度抽象，违背 Go 简单性；②core 直接用 generated.Queries（我之前的错案）——违背 ferret 范式 + 无事务/动态查询封装；③纯 ferret（core/store 持有 Repo）——违反 DIP，core 依赖 pkg/db。

### D2：Repo struct 在 data/repo/，不在 core/store/

**决策**：Repo 实现放在 `server/data/repo/<entity>.go`（数据层），**不**放 `server/core/store/`（领域层）。

**理由**（DIP 依赖方向）：
- Repo struct 必须 import `pkg/db/generated`（基础设施）
- 若放 `core/store/`，则 core 层 import pkg/db → **domain 依赖 infrastructure**（DIP 违反）
- 放 `data/repo/`：data 层 import pkg/db（合理，data 本就是基础设施层），core 通过 wire 注入 Repo struct（core 不知道 data 的存在细节，仅依赖注入的具体类型）

**依赖方向**（正确）：
```
core → （wire 注入 *repo.ModuleRepo）→ data/repo → pkg/db/generated → PG
                                          ↑
                              data 层持有 generated 依赖（合理）
                              core 不直接 import pkg/db
```

**备选**：core/store/ 持有 Repo（违反 DIP）；core 直接用 generated（无封装层）。

**影响**：修正 server/AGENTS.md line 68 的 "core/store(薄包装,调用方)" 表述 → 改为 "core 注入 data/repo Repo struct"。

### D3：Repo 方法分三类（薄包装 + 事务 + 动态查询）

**决策**：每个 Repo struct 的方法分三类：

| 类型 | 示例 | 实现 |
|------|------|------|
| **薄包装**（多数）| `GetByID(ctx, id)` / `Create(ctx, params)` / `List(ctx)` | 直接调 `r.queries.GetModule(ctx, id)` |
| **跨表事务**（少数）| `CreateWithVersion(ctx, module, version)` | `pool.Begin` + `queries.WithTx(tx)` + 多个 generated 调用 + Commit |
| **动态查询**（sqlc 不擅长）| `ListByFilter(ctx, wrapper)`（IN-list / ad-hoc 过滤）| `query_wrapper.BuildSQL()` + `pool.Query` |

**理由**：
- ferret 的 `EmailRepo` 已有此分类（`FindByUser` 薄包装 + `Creates` 事务 + 动态 `Where` 链）
- sqlc 的 `WithTx` 处理事务（官方文档），query_wrapper 处理动态（squirrel 标准库）
- 多数方法（80%）是薄包装，保持简单；少数（20%）事务/动态才复杂

**范例**（ModuleRepo）：
```go
// 薄包装
func (r *ModuleRepo) GetByID(ctx context.Context, id int64) (generated.Module, error) {
    return r.queries.GetModule(ctx, id)
}

// 跨表事务
func (r *ModuleRepo) CreateWithVersion(ctx context.Context, m CreateModuleWithVersionParams) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil { return err }
    defer tx.Rollback(ctx)
    q := r.queries.WithTx(tx)
    module, err := q.CreateModule(ctx, m.Module)
    if err != nil { return err }
    _, err = q.CreateModuleVersion(ctx, withModuleID(m.Version, module.ID))
    if err != nil { return err }
    return tx.Commit(ctx)
}

// 动态查询
func (r *ModuleRepo) ListByFilter(ctx context.Context, w *QueryWrapper) ([]generated.Module, error) {
    sql, args := w.BuildSQL("SELECT * FROM modules WHERE deleted_at IS NULL")
    rows, err := r.pool.Query(ctx, sql, args...)
    // scan rows → []generated.Module
}
```

### D4：query_wrapper.go 适配 pgx（非 GORM）

**决策**：`data/query_wrapper.go` 实现动态查询构造器，**适配 pgx**（非 ferret 的 GORM）。用 `Masterminds/squirrel` 标准 SQL builder + 自定义 Wrapper 封装常用模式。

**理由**：
- ferret 的 query_wrapper 绑定 GORM（`ApplyToGorm(db)`），我们不能直接复用
- `Masterminds/squirrel` 是 Go 标准 SQL builder（无 ORM 魔法，生成 SQL 字符串 + args）
- pgx 原生接受 `(sql string, args ...any)`，与 squirrel 天然契合

**接口设计**：
```go
// data/query_wrapper.go
type QueryWrapper struct {
    conditions []Condition  // AND/OR 嵌套
    orderBy    []OrderClause
    page       *Pagination  // nil = 不分页
}

func New() *QueryWrapper { ... }
func (w *QueryWrapper) Eq(col string, val any) *QueryWrapper { ... }
func (w *QueryWrapper) In(col string, vals ...any) *QueryWrapper { ... }
func (w *QueryWrapper) Like(col string, pattern string) *QueryWrapper { ... }
func (w *QueryWrapper) Or(fn func(*QueryWrapper)) *QueryWrapper { ... }  // 嵌套 OR
func (w *QueryWrapper) OrderBy(col string, desc bool) *QueryWrapper { ... }
func (w *QueryWrapper) Page(page, size int) *QueryWrapper { ... }

// BuildSQL 生成 SQL + args（用 squirrel 底层）
func (w *QueryWrapper) BuildSQL(base string) (sql string, args []any) { ... }
```

**使用场景**（仅 sqlc 不擅长的）：
- 动态 IN-list（`WHERE id IN (?, ?, ?)` 数量运行时确定）
- ad-hoc 过滤组合（catalog 列表按 layer/owner/status/tag 多维可选过滤）
- 分页（`LIMIT ? OFFSET ?`）

**不用于**：固定查询（这些走 sqlc .sql 文件）。

### D5：dbset/ 多表组合（opt-in）

**决策**：`data/dbset/<name>.go` 用于跨表聚合 struct（如 `StackWithSpace{Stack, Space, Layer}`），**仅在 core 需要聚合视图时建**。不强制每实体建 dbset。

**理由**：
- ferret 的 `data/dbset/user.go` 是 `User{entity.User, Address}` 跨表组合（opt-in）
- 多数 core 操作只需单表（用 Repo 足够）；少数列表视图需要 JOIN 聚合（用 dbset）
- sqlc 的 `sqlc.embed()` 也能处理 JOIN 嵌套，dbset 是 Go 层的组合补充

**W1 范围**：仅建 `StackWithSpace`（W1-04 stackmodel 列表需要）。其余 opt-in。

### D6：entity 层 opt-in（非每表必建）

**决策**：`internal/model/entity/` 仅在 sqlc model 不够时建（需要聚合字段/计算字段/值对象/隐藏内部列）。MVP 大多数表不需要 entity，直接用 sqlc model。

**理由**：
- ferret 的 entity 带 GORM tag（因为 GORM 需要映射）；我们用 sqlc（自动生成 model），entity 的映射职责消失
- 业界共识（rednafi/sqlc #944）：sqlc model 直接用，仅在 domain 逻辑真正偏离表结构时才建 entity
- opt-in 而非 project-wide rule（避免每表 boilerplate）

**W1 范围**：可能需要 entity 的——`StackContext`（PathGenerator 输出聚合，非 DB 表）。其余直接用 sqlc model。

### D7：W1 表范围 = 15 个核心实体 + 4 张新表

（同前版，不变）覆盖 teams/projects/spaces/modules/module_versions/module_dependencies/catalog_items/stacks/stack_dependencies/layer_logical_refs/layer_rule_set_versions/environments/tenants/environment_tenant_bindings/tag_policies。推迟 W2 表的身份/编排/漂移/审批 query/Repo。

## Risks / Trade-offs

- **[15 个 Repo 文件 + query_wrapper + dbset 工作量大] → 模板化 + 优先级**：Repo 80% 是薄包装（模板化生成），query_wrapper 一次性投入复用，dbset 仅 StackWithSpace 一个。按依赖顺序：先 query/Repo 骨架 → 再业务方法 → 再测试。
- **[query_wrapper 学习成本] → 仅用于动态场景，固定查询走 sqlc**：明确边界——sqlc 处理 80% 固定查询，query_wrapper 处理 20% 动态。不滥用。
- **[squirrel 新依赖] → 标准 SQL builder，无魔法**：squirrel 是 Go 社区标准（无 ORM 反射），生成纯 SQL 字符串，与 pgx 契合。比自研 wrapper 更可维护。
- **[core 注入 Repo struct 不够 DIP] → Go 隐式 interface 兜底**：默认 struct（简单），需要 mock 时在 core 提取 interface（无需改 data）。渐进式 DIP。
- **[server/AGENTS.md 修正影响其他 wave 认知] → 本 change 内同步修正**：修正后 line 68/127/132 三处统一为混合范式，避免后续 wave confusion。
- **[sqlc generate 重生成可能影响 teams_test.go] → 重跑测试验证**：sqlc generate 幂等，teams.sql 不改则 teams.sql.go 不变。

## Migration Plan

1. 写 `013_env_tenant_tagpolicy.sql`（4 表 + COMMENT + 索引 + seed platform-default tenant + dev/staging/prod/dr envs + layer_rule_set v1 seed）
2. 更新 `schema.sql` 追加 4 表
3. 写 14 个 query .sql 文件（teams 已有）
4. 跑 `sqlc generate` 重生成
5. 建 `data/repo/` 15 个 Repo struct + `data/query_wrapper.go` + `data/dbset/stack_with_space.go`
6. 扩展 `data.ProviderSet` 注册所有 `NewXxxRepo`
7. 修正 `data.go` 注释 + `server/AGENTS.md` 三处矛盾
8. 写测试（Repo CRUD + 事务 + 动态查询 + FK + migration 幂等）
9. `go build ./... && go vet ./... && go test ./server/data/... -short`

**回滚**：migration 013 down 路径 DROP 4 表；data/repo/ 删除；query.sql 删除后重 generate。无数据迁移。

## Open Questions

- **squirrel vs 自研 query_wrapper？** 倾向 squirrel（标准库，可维护），但若团队偏好自研（ferret 风），可在 apply 阶段决定。proposal 暂定 squirrel。
- **layer_rule_set_versions v1 seed 是否本 change 补？** 倾向补（否则 stacks.layer_rule_set_version_id 无默认值）。tasks 阶段确认。
- **entity 层 W1 是否需要 StackContext？** 倾向 W1-04 stackmodel 实现时再决定，W1-02 先不建（YAGNI）。
