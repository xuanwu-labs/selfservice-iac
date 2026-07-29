# 实现任务清单

> 按 `## 01-功能模块` 分组，依赖从上到下递增。每组含"实现 + 测试 + 脚本（如适用）"。
> 包结构对应 `docs/03-平台代码与脚本目录.md`；上线阶段对应 `design.md §06`。

## 00-并行 Stream 拆解（交付规划）

> 21 模块按依赖关系拆为 7 个 Wave，同 Wave 内可并行。关键路径：`01→05→06→08→10`（主干闭环 ~5 Wave）。

| Wave | 可并行模块 | 依赖 | 说明 |
|------|-----------|------|------|
| **W1** | 01 骨架 ‖ 02 DB ‖ 03 模块注册 ‖ 04 分层模型 | 无 | 4 路并行启动，2 周内完成 |
| **W2** | 05 codegen ‖ 07 执行目录 ‖ 09 API/RBAC ‖ 13 工具链/Executor | 05←01+04, 07←01, 09←02, 13←01+07 | 4 路并行 |
| **W3** | 06 编排引擎 ‖ 11 身份同步 | 06←05+02, 11←02+09 | 2 路并行 |
| **W4** | 08 状态/漂移 ‖ 12 审批 ‖ 18 CMDB/FinOps | 08←06+07, 12←09+11, 18←03+06 | 3 路并行 |
| **W5** | 10 前端/e2e ‖ 14 存量导入 ‖ 16 CICD ‖ 19 云凭据 | 10←主干闭环, 14←03+06+08, 16←09+12+11, 19←03+06+11+18 | **主干闭环验证（W5 是里程碑）** |
| **W6** | 15 扩展验证 ‖ 17 CLI/AI ‖ 20 env/tenant | 15←扩展闭环, 17←09+11+12, 20←02+04+16+18 | 3 路并行 |
| **W7** | 21 tag/param pipeline | ←03+05+17+20 | 参数解析管道（D28），最后集成 |
| **W8** | 22 API/Schema 契约 ‖ 23 PR/RunHooks ‖ 24 平台运营 ‖ 25 企业证据链 | Phase 0 契约先行；Phase 2+ 扩展 | 商业级平台能力 |

**团队分工建议**：
- **后端 A**（主干）：01→05→06→08→10→14→15
- **后端 B**（治理）：02→09→11→12→16→17
- **后端 C**（执行层）：07→13→19→20
- **DB/infra**：02→04→18→21
- **前端**：等 W4 启动 10，后续持续迭代

## 00b-Phase 1/2/3 简化策略（避免过度设计，每阶段先收价值再扩功能）

> 回应架构评估 §4.5 过度设计清单 + §11 三阶段实施路线。目标架构 D1-D30 不变，但实现按阶段递进。

| 模块 | Phase 1 简化（0-6月） | Phase 2 完整（7-12月） | Phase 3 高级（13-18月） |
|------|----------------------|----------------------|----------------------|
| codegen (D19/D28) | **5 阶段简化**（contract+defaults+governance+user+dependency），可映射到 S1-S9 | **9 阶段管道**完整 + provenance | — |
| 审批 (D11/D21) | **单 pre-apply 门** + 多人或签，不上会签/条件/超时 | 双门禁 + 会签 + 条件 + 超时升级 | 可视化建模或 Temporal（如需） |
| Executor (D20) | **仅一个模式**（process 或 k8s 二选一，默认 process） | + 第二模式（按生产隔离需要） | + container / remote（按需） |
| BuildDriver (D22) | **仅 docker** | — | + buildah / kaniko（按需） |
| StateMover (D26) | **人工 SOP only**，feature_flag=off | MigrationPlanner + 影响面报告 + 人工执行 | feature_flag 可开 + 半自动（必须 plan=0 + 双人） |
| Layer 模型 (D24) | **hard-code 三层**（global/middleware/application）+ D29 layer-first Path Contract | D24 可配置 + path_template 自由 | StackGranularity 多策略 |
| OIDC (D10/D10.1) | **单 issuer** + claim_mapping | + SCIM + 飞书/钉钉 | 多 issuer 联邦 + CAS 代理 |
| CLI/AI (D17) | — | tm CLI + AK/SK（不含 AI） | tm ai + MCP + Skills |
| 多云 | 仅 alicloud | + AWS | + Azure |
| 安全 (D30) | 占位符 + 日志脱敏 | + Break-Glass + state 加密 | + IAM 聚合校验 + sha256 pin |
| CMDB/FinOps | apply 后资源索引（CMDB index） | + reconcile + 基础成本视图 | + 账单核销/优化建议/孤儿 inventory |
| 存量导入 | `managed-readonly` 观察纳管 | `managed-changeable` 受控变更 | `standard` 完全标准化 |

**Phase 1 风险护栏**：仅 dev/staging 环境；destroy 强制人工 SOP；容量 < 50 stack / < 5 并发。
**Phase 1 主链路**：`CatalogItem → Request → Codegen → Git pinned_commit → plan artifact → pre-apply 审批 → apply → reconciling → succeeded`。

**Phase 1 feature flag 约束**：PR-first、Run Hooks、Scheduled Runs、Environment Promotion、AI/MCP、半自动 StateMover、FinOps 优化建议、孤儿自动释放默认 `off`。Phase 1 只允许保留接口占位、schema 预留和观测字段，不能进入主链路阻塞条件；任何 Phase 2+ 能力若要提前试点，必须满足“不改变主链路状态机、不新增强同步依赖、不影响 golden path SLA”。

## 00c-Phase 0 Contract Freeze（已拆分到独立 change）

> **已拆分**：Phase 0 契约冻结独立为 `openspec/changes/platform-contract-freeze/` change。
> 本节保留作为索引，实际 task 在独立 change 中执行并归档。
> 分支：`feat/phase0-contracts`（已合并）+ `feat/contract-fixup`（契约文档对齐）。
> 原始设计参考：`docs/00-工程契约.md`、`docs/12a-状态机测试矩阵.md`。
>
> **契约范式变更（D1 Connect-native）**：原 0.1/0.3/0.4 设想的 openapi.yaml / protocol-mapping.md / schemas/*.schema.json 已废弃——proto 是唯一契约源，Connect-RPC 原生覆盖 gRPC/gRPC-Web/Connect-JSON。下面任务编号保留以便回溯，但产出物以独立 change 实际交付为准。

- [x] 0.1 ~~产出 `contracts/openapi.yaml`~~ → **改为 Connect-native**：proto 是唯一契约源，不手写 openapi.yaml（D1）。若未来需要 REST 文档，用 `protoc-gen-openapi` 从 proto 自动生成
- [x] 0.2 产出 `contracts/platform/v1/{common,lifecycle,catalog,cloud}/{srv,dto}.proto`（域目录 × srv/dto/enum 分离；6 service / 21 RPC）
- [x] 0.3 ~~产出 `contracts/protocol-mapping.md`~~ → **废弃**：Connect-RPC 单 handler 原生覆盖三协议，无需手写映射文档
- [x] 0.4 ~~产出 `contracts/schemas/*.schema.json`~~ → **废弃**：消息定义即 proto message，前端用 connect-es 自带类型，无需手写 JSON Schema
- [x] 0.5 ~~产出 `contracts/error-codes.yaml`~~ → **改为 proto enum 单源**：错误码身份由 `common/enum.proto` 的 ErrorCode enum 定义（buf 生成），传输语义（gRPC Code/retryable/HTTP status）调用时传 + 从 Code 派生，对标 kratos/ferret
- [x] 0.6 产出 `contracts/fixtures/state-machine/main-lifecycle.json`：17 test cases（RLC/ERR/IDEMP/CONC/EXPR）
- [x] 0.7 产出 `contracts/fixtures/adapter/terramate-adapter.json`：4 golden cases（plan/saved-plan apply/drift/import），明确 forbidden command，禁止 Orchestrator 绕过 Terramate
- [x] 0.8 产出 `contracts/fixtures/walking-skeleton/seed-data.json`：tenant/env/team/bundle/catalog/module/cloud grant/toolchain/path/approval/plan artifact seed
- [ ] 0.9 完成首云账号 bootstrap 演练：secret store、OIDC trust、execution role、`team_cloud_grants` → **推迟到 Wave 5（依赖 19 云凭据）**
- [ ] 0.10 完成 Day-1 资源栈 bootstrap：Global VPC seed stack、outputs 校验 → **推迟到 Wave 5（依赖 03 + 06）**
- [ ] 0.11 跑通 walking skeleton：Request → Codegen → Git → Plan → Approval → Apply → Reconcile → **推迟到 Wave 5 主干闭环验证**
- [ ] 0.12 验收：契约层（0.1-0.8）已在本 change 完成；运行时验收（0.9-0.11）推迟到 Wave 5

## 01-平台骨架与适配器接口

> **路径对齐**：原 `platform/` 已演进为 `server/`（见 docs/03 §演进注记、design D2）。业务域落 `server/core/`，基建落 `server/internal/`。task 1.1 的二进制骨架已由脚手架阶段完成（`server/cmd/{server,aether,migrate}`），本模块聚焦适配器接口 + TerramateAdapter。
>
> **实现状态**：task 1.2-1.5 已由子提案 `w1-adapter-interfaces` 完成（feat/w1-adapter-interfaces 分支，15/15 tasks done）。task 1.6（配置脚本）推迟到 W2。

- [x] 1.1 ~~新增 `platform/` 顶层目录与 `platform/cmd/platform` 二进制骨架~~ → **已由脚手架完成**：`server/cmd/server`（HTTP server 启停/健康检查/配置加载）、`server/cmd/aether`（CLI）、`server/cmd/migrate`（迁移 runner，go:embed）
- [x] 1.2 在 `server/core/adapters/{git,state,policy,cost,notify,cloud}` 定义可插拔适配器接口（见 docs/03 §2），各提供 noop/stub 默认实现 ← 完成于 `w1-adapter-interfaces`
- [x] 1.3 定义 `server/core/terramate` 的 `TerramateAdapter` 接口与 exec 默认实现（封装 `terramate run/generate` 子进程调用、退出码与 stdout/stderr 捕获）—— D1 边界的代码守护 ← 完成于 `w1-adapter-interfaces`
- [x] 1.4 修复 D1 lint：当前 `.golangci.yml` depguard 配置存在但 AGENTS.md 自认失效（terramate 不在 go.mod 时被 typechecker 静默丢弃）；补独立 lint test（如 `server/internal/audit/d1_guard_test.go`）确保 `server/**` 不得 import terramate 内部包 ← 完成于 `w1-adapter-interfaces`
- [x] 1.5 测试：`go test ./server/core/adapters/... ./server/core/terramate/...`（接口契约 + exec 适配器用 fake terramate 脚本断言）← 完成于 `w1-adapter-interfaces`
- [ ] 1.6 脚本：在 `scripts/初始化/` 增加平台配置加载与适配器装配辅助脚本骨架 ← 推迟到 W2

## 02-元数据存储与迁移

> **路径对齐**：迁移 runner 已由脚手架完成（`server/cmd/migrate`，go:embed `migrations/*.sql`）。pgxpool provider（`server/data/data.go`）+ sqlc 三件套（`server/pkg/db/`）已配好。
>
> **实现状态**：task 2.1-2.3 已由子提案 `w1-db-store` 完成（feat/w1-db-store 分支，49/49 tasks done；migration 013 补齐 env/tenant/binding/tag_policies 4 张表 + 15 个核心实体 sqlc query + 15 Repo struct + query_wrapper 动态查询 + dbset + 测试；经 2 轮 code review 修复 16 个 finding）。task 2.4（迁移脚本）推迟到后续。

- [x] 2.1 ~~选定元数据 DB（PostgreSQL），在 `server/core/store` 建立访问层与连接池~~ → **DB + 连接池已由脚手架完成**（`server/data/data.go` pgxpool + `server/pkg/db/` sqlc）；本 task 聚焦 `server/data/repo/`（混合范式 Repo struct 薄包装 `*generated.Queries` + 跨表事务 + 动态查询）落地 ← **架构演进**：原 `core/store` 改为 `data/repo/`（对齐 w1-db-store 提案的 ferret × DIP × sqlc 混合范式决策，DIP 依赖方向正确）← 完成于 `w1-db-store`
- [x] 2.2 在 `server/cmd/migrate/migrations/` 编写初始 schema 迁移，落地 `docs/04` Phase 1 必需表 ← 完成于 `w1-db-store`（migration 013 补齐 environments/tenants/environment_tenant_bindings/tag_policies；其余 25 张表已由 migration 001-012 落地；身份/编排/漂移/审批表推迟 W2）
- [x] 2.3 测试：`go test ./server/data/repo/... ./server/data/...`（Repo CRUD + 业务唯一约束 + 跨表事务 + 动态查询 + 迁移 up/down 幂等）← 完成于 `w1-db-store`（query_wrapper 10 测试全过 + module/environment Repo 测试；DB 测试 -short skip，CI 跑）
- [ ] 2.4 脚本：在 `scripts/迁移/` 产出 schema 初始化与迁移执行脚本 ← 推迟到后续

## 03-模块注册与服务目录

> **路径对齐**：proto 契约已冻结（`contracts/platform/v1/{registry,catalog}/`）。`server/core/catalog/validator.go`（D40 JSON Schema 校验）已实现，与 task 3.3 互补。`server/api/connect/catalog.go` 是静态占位 handler。

- [x] 3.1 实现 `server/core/registry`：注册 Git 模块、版本管理、拉取并解析 `variables.tf` 生成 `variables_contract_json`（+ `versions.tf` 提取 `providers_json`，Gap 1 修复）；MVP 只支持 git 源（`source_type='git'`），TF Registry 源（`source_type='registry'`）W2 扩展（doc 09 §6.1）；private repo 凭证 MVP 用 env 变量，W2 补 credentials 表 kind='git'（doc 06 §4a）
- [x] 3.2 实现注册时校验：调用 `terraform validate`（可选 `init`），状态机 `pending-validation → validated | validation-failed`
- [x] 3.3 实现 `server/core/catalog`：发布目录项、从契约裁剪 `form_schema_json`、维护 `defaults_json` 最佳实践覆盖、可见性控制（`server/core/catalog/validator.go` D40 校验器已就位，本 task 补 publish/defaults/visibility）
- [x] 3.4 测试：`go test ./server/core/registry/... ./server/core/catalog/...`（用 fixture 模块断言契约提取、默认值注入、可见性过滤）
- [ ] 3.5 脚本：在 `scripts/模块注册/` 产出批量注册原子模块的辅助脚本

## 04-团队归属与分层模型（D24 stack 模型可配置化）

> **Phase 策略**：Phase 1 hard-code 三层（global/middleware/application + per-component 粒度），不上 D24 可配置/多粒度；Phase 2 开放 D24 path_template 自定义 + StackGranularity 多策略；Phase 3 开放 D26 版本化迁移。`server/core/{tenancy,stackmodel}/` 空目录已就位。

- [x] 4.1 实现 `server/core/tenancy`：团队/项目组/bundle CRUD（**bundle 可选**，bundle_id NULL 表示无 bundle）、资源归属固定规则（RDS→DBA 等）
- [x] 4.2 Phase 1 实现固定三层 `server/core/stackmodel` seed（global/middleware/application），只暴露读取，不开放管理员改模板；Phase 2 再实现 Layer 规则集版本化引擎与 v1↔v2 diff viewer
- [x] 4.3 实现 `PathGenerator`（`server/core/stackmodel/pathgenerator`）：按 D29 layer-first Path Contract 输出 `repo_path + state_key + stack_id + terramate_tags_json`；模板变量 env/tenant/team/bundle/component/layer/custom_kv；**codegen MUST 调用此组件，MUST NOT 字符串拼接**
- [x] 4.4 实现 `StackGranularity` 评估器（`server/core/stackmodel/granularity`）：per-component（默认）/per-bundle/per-team/custom；读 `stack_grouping_rules` + catalog 项 stack_grouping 字段
- [x] 4.5 实现跨层依赖图（`server/core/stackmodel/dependency`，stack 间 depends-on，按 layer.depends_on 校验无环）
- [ ] 4.6 Phase 2 实现 **MigrationPlanner**（`server/core/stackmodel/migrator`）：输入 layer_rule_set 变更草案 → dry-run 对每个受影响 stack 重渲染 path 对比 → 输出 per-stack Tier 分类（1/2/3）+ 影响面报告 + rollback_token；UI 可视化 before/after path 树
- [ ] 4.7 Phase 3 才实现 **StateMover 半自动能力**（`server/core/stackmodel/statemover`）：默认 feature_flag=off；开启时必须双人审批 + state snapshot + `terraform plan -detailed-exitcode` exit 0；Phase 1/2 仅生成人工 SOP 和审计任务
- [ ] 4.8 实现 **QuiesceBatch**：迁移批次粒度冻结（不是 stack/team/全局三级），批次内 stack 工单/plan/apply 阻塞 + 漂移检测自动静默该批（非全局暂停），静默期写入 stacks.migration_status
- [ ] 4.9 实现 **StateBackup**：迁移前自动 state 快照（S3 versioning / OSS snapshot）+ rollback_token；按 team 灰度执行（爆炸半径=1 team）
- [x] 4.10 实现 **RollbackEngine**：sunset 窗口内支持逆向 state mv（vN→vN-1，含 path 反向 mv）；一键 revert stacks.layer_rule_set_version_id + state 恢复
- [x] 4.11 实现 **SunsetTracker**：Tier 3 stack 标 `deprecated_at` + `sunset_at`（默认+6mo），到期前提示 destroy+recreate，到期后旧版本 status=archived 拒绝新建；审计表 `layer_migrations`
- [x] 4.12 测试：`go test ./server/core/tenancy/... ./server/core/stackmodel/...`（路径模板渲染、bundle 可选、归属判定、依赖图拓扑、自定义层增删、StackGranularity 评估、整体版本化、per-stack Tier 分类、Tier 2 state mv + plan=0 校验 + 失败回滚、逆向 mv、CMDB 同步）

## 05-代码生成（D25 模块零侵入 + cardinality 调用方注入）

- [x] 5.1 实现 `server/core/codegen`：输入（表单值 + 模块契约[纯 scalar] + 默认值 + 依赖图）→ 输出（stack 目录树、`stack.tm.hcl`、`main.tf` 调模块、`backend.tf` 远程 state、跨层 `data` 注入）；**调用 PathGenerator 渲染路径**（不字符串拼接）；**module source 构造按 source_type 区分**（git 源=`git::url//path?ref=commit_sha`，registry 源=`ns/name/cloud`+version，详见 doc 09 §6.1）
- [x] 5.2 实现 `CardinalityInjector`：按 catalog 项 `cardinality`（single/list/map）在调用方注入——single=直接 module 调用、list=`count = N`、map=`for_each = tomap({...})`；per_instance 字段从 each.value 取，shared 字段直接注入；**模块 variables.tf 全 scalar，零侵入**
- [x] 5.3 catalog 项 CRUD 加 cardinality 配置字段（cardinality/instance_key/per_instance_fields/shared_fields）+ layer_logical_id + stack_grouping；表单渲染按 cardinality 动态（single=普通表单、list/map=实例清单动态表格）
- [x] 5.4 stack outputs.tf 自动聚合：cardinality=map 时生成 `output "x" { value = { for k,m in module.y : k => m.z } }`
- [x] 5.5 生成代码强制 `terraform fmt` 校验、禁止 local backend（对应 spec 07）
- [x] 5.6 测试：`go test ./server/core/codegen/...`（golden file：固定输入 → 固定目录树与文件内容；覆盖 single/list/map 三种 cardinality + bundle 有/无 + 自定义 layer；**用社区模块 fixture 验证零侵入**）

## 06-Terramate 适配器与编排引擎

- [x] 6.1 实现 `server/core/orchestrator` 工单状态机：`submitted → generating → planning → plan-ready → pending-approval → applying → reconciling → succeeded | failed-retryable | failed-terminal | waiting-manual | reconcile-pending | blocked-policy`；Phase 2 增加 `pending-admission`；每步持久化 `request_events` + 乐观锁 version
- [x] 6.2 串接流水线：codegen → `TerramateAdapter.RunPlan` → 审批/OPA → `TerramateAdapter.RunApplySavedPlan`（同一 `pinned_commit + plan_binary + sha256 + toolchain hash`）→ 资源回写
- [x] 6.3 接入 OPA 适配器：plan 后策略评估，deny 阻断 apply
- [x] 6.4 测试：`go test ./server/core/orchestrator/...`（状态机迁移、OPA 阻断、plan artifact TTL、apply 中断转 waiting-manual、reconcile-pending、幂等，用 fake terramate）

## 07-执行目录治理（workspace git）

- [ ] 7.1 实现 `server/core/workspace`：基于 go-git 的工作仓库 clone/fetch、`git worktree` 分配、commit/push
- [ ] 7.2 实现 checkout 租约（`workspace_checkouts`）与并发隔离（每工单独占 worktree + 分支锁）
- [ ] 7.3 实现平台重启 reconcile：按 `pinned_commit` 重建 worktree、复活挂起工单；apply 中断不得自动重试，转 `waiting-manual`（对应 docs/10 §4）
- [ ] 7.4 实现一致性对账任务：元数据 stacks ↔ 工作仓库目录 ↔ 远程 state key 三方校验
- [ ] 7.5 测试：`go test ./server/core/workspace/...`（用内存 fake repo 验证 clone/worktree/重启恢复/对账）
- [ ] 7.6 脚本：在 `scripts/编排执行/` 产出 worktree 清理、对账触发、流水线手动触发脚本

## 08-状态后端与漂移检测

- [ ] 8.1 实现 state backend 适配器默认实现（S3/MinIO + 锁），强制每 stack 独立 state key（key 由路径推导）
- [ ] 8.2 实现 `server/cmd/drift-scheduler` + `server/core/drift`：调度器（分层时间窗 + 令牌桶限流）、worker（只读 plan + `terraform show -json` 解析差异）
- [ ] 8.3 实现漂移事件 `drift.detected` 与同步策略执行（adopt-cloud / restore-desired，走工单留审批）（对应 docs/13）
- [ ] 8.4 测试：`go test ./server/core/drift/...`（plan JSON 样本解析、限流不超上限、同步策略执行）
- [ ] 8.5 脚本：在 `scripts/状态同步/` 产出 state key 对账、漂移手动触发、同步策略执行辅助脚本

## 09-平台 API 与 RBAC 审批

- [ ] 9.1 实现 `server/core/api`：`/api/v1` 下 catalog/requests/approvals/drift/modules/audit 端点 + OpenAPI 描述
- [ ] 9.2 实现 `server/internal/auth`：OIDC/token 鉴权 + RBAC（`role_bindings` 评估，按 team/project/bundle/stack/layer × action）
- [ ] 9.3 实现审批流（多级/串行/会签）与 `POST /requests` 同步建单 + 异步推进
- [ ] 9.4 实现 `server/core/events` + `audit`：事件总线、Webhook 分发、append-only 审计日志
- [ ] 9.5 测试：`go test ./server/core/api/... ./server/internal/auth/...`（端点契约、越权 403、审批流转、审计写入）

## 10-前端、e2e 与整体验证

- [ ] 10.1 搭建 `frontend/`（React + 表单引擎，由 `form_schema_json` 声明式渲染），覆盖目录/申请/审批/漂移/历史视图
- [ ] 10.2 在 `e2etests/`（新增）落端到端：表单提交 → 工单 → 生成代码 → 断言 stack 目录与 state key；漂移检测端到端
- [ ] 10.3 全量验证：`make build`（Terramate 主二进制不受影响）+ `make test`（Terramate 回归绿色）+ `go test ./server/...`
- [ ] 10.4 文档：更新 AGENTS.md / README，说明 `server/` 边界、运行方式、scripts 用法

## 11-身份认证与组织同步（扩展，对应 D10+D10.1 / specs/09；依赖 02 元数据 + 09 RBAC）

- [ ] 11.1 **Phase 1**：实现本地 identity / team / role_binding 管理 + bootstrap admin，平台不对接外部用户中心也可完整运行
- [ ] 11.2 **Phase 1**：实现单 OIDC issuer + `claim_mapping` 登录（`server/internal/auth/oidc`），外部组只作为输入，最终权限仍由平台 RBAC 判定
- [ ] 11.3 **Phase 1**：实现 RBAC 评估与审批人解析（team + role → identities），覆盖 Web/CLI/CI/CD/AI actor
- [ ] 11.4 **Phase 2**：实现 `DirectorySyncer` 抽象 + SCIM 2.0 / 飞书 / 钉钉 三适配器（增量事件 + 定时全量兜底）
- [ ] 11.5 **Phase 2**：实现组织 → 团队/角色映射引擎（`org_mappings` 规则评估，联动 RBAC），外部用户中心只作为 identity/org source
- [ ] 11.6 **Phase 2**：多源去重与主源、同步健康监控与告警
- [ ] 11.7 **Phase 3**：实现多 OIDC issuer 并存（D10.1）+ 登录页多 provider 按钮
- [ ] 11.8 **Phase 3**：实现 dex 可选桥接适配器与 CAS OIDC proxy 接入文档（平台代码仍只讲 OIDC）
- [ ] 11.9 测试：Phase 1 覆盖本地 identity/RBAC + 单 OIDC；Phase 2+ 增加 mock IdP / SCIM 推送 / 飞书回调
- [ ] 11.10 脚本：`scripts/初始化/` 本地用户/团队/角色初始化；Phase 2+ 增加身份源装配与组织映射规则初始化

## 12-审批流程引擎（扩展，对应 D11/D21 / specs/10；依赖 09 API + 11 身份）

- [ ] 12.1 实现审批 DSL（YAML）解析与 `ApprovalEngine` 接口（Start/Decide/Status）
- [ ] 12.2 状态机执行：多级 / 会签 / 条件分支 / 超时升级 / 驳回回退；持久化 `approval_runs`、`approval_node_runs`
- [ ] 12.3 审批人动态解析（team + role → identities，联动身份同步）
- [ ] 12.4 **双门禁（D21）**：编排流水线两阶段挂起独立审批流——准入（pre-plan，OPA 自动放行低危）+ 执行确认（pre-apply，输入=plan 差异+成本+漂移校验）；plan/apply 解耦（plan 后销毁沙箱+持久化 plan 产物，apply 新沙箱通过 `TerramateAdapter.RunApplySavedPlan` 执行 `terramate run -- terraform apply "<plan>"` 不重 plan）；漂移护栏
- [ ] 12.5 前端审批流可视化（节点链 + 进度 + plan diff 展示，对接原型）
- [ ] 12.6 测试：`go test ./server/core/approval/...`（各 mode、超时升级、驳回、RBAC 路由、双门禁时序、plan 产物输入）
- [ ] 12.7 预留 Temporal 升级路径（DSL → workflow stub 编译），文档化切换方式

## 13-工具链版本与执行隔离（扩展，对应 D12/D13/D15/D20/D22 / specs/11；依赖 01 骨架 + 07 workspace）

- [ ] 13.1 worker 镜像构建 CI（terramate + tenv tf/tofu + provider mirror + OPA + Infracost），tag 与三层版本对齐
- [ ] 13.2 平台 terramate 多版本缓存 + exec 选版本 + 灰度（`TerramateAdapter` 扩展）
- [ ] 13.3 [tenv](https://github.com/tofuutils/tenv) 集成（per-stack `required_version` 解析）+ provider registry mirror 配置
- [ ] 13.4 实现 `server/core/executor`（`Executor` 接口 + 四模式实现：`ProcessExecutor` 节点预装工具链+cgroups+netns / `ContainerExecutor` 宿主 docker/podman / `K8sExecutor` Job/Pod / `RemoteNodeExecutor` SSH 节点池），统一契约（worktree 挂载、资源/网络限制、退出码、日志流式回传）；编排引擎与漂移检测均通过接口调用（D20）
- [ ] 13.5 实现 `server/core/executor/imagebuilder` + 可插拔 `BuildDriver`（docker/buildah/kaniko）+ `ImageRegistry` 适配器（Harbor/ACR/ECR/ACR/Docker Registry/内嵌本地/文件中转）：Web 保存工具链版本 → 渲染 Dockerfile → 异步构建（K8s 用 kaniko 无 daemon，进程用 buildah）→ tag → push → 灰度；process 模式不触发构建（D22）
- [ ] 13.6 实现 `server/core/installer`（process 模式按 Web 基线自动 reconcile 节点：mirror 下载→checksum→装+symlink+`toolchain_manifest`；离线手动投放兜底）
- [ ] 13.7 版本兼容校验三时机（注册/catalog/执行前 `validateCompatibility`，镜像解 manifest 或预装读 `toolchain_manifest`），不兼容阻断给可操作提示（D22）
- [ ] 13.8 测试：`go test ./server/core/executor/... ./server/core/executor/imagebuilder/... ./server/core/installer/...`（四模式契约一致、BuildDriver 三驱动、镜像构建 push、兼容校验阻断、installer reconcile）+ e2e（同一工单四模式退出码/日志一致；process 零镜像；k8s kaniko 构建；离线手动投放）
- [ ] 13.9 脚本：`scripts/初始化/` worker 镜像/mirror/registry 部署 + process 节点工具链预装；`scripts/编排执行/` 镜像灰度切换、executor 模式切换

## 14-存量导入（扩展，对应 D14 / specs/12；依赖 03 模块注册 + 06 编排 + 08 stack）

- [ ] 14.1 实现 import 向导后端（选模块 + 输入资源 ID → 生成声明式 `import` 块 + module 调用骨架）
- [ ] 14.2 容器内 `terraform apply`(import only) + `plan -generate-config-out`，落 state
- [ ] 14.3 plan 零变更强制校验 + 待 review 标记 + limited（不支持 import）/ sensitive（敏感字段）处理；默认进入 `managed-readonly`
- [ ] 14.4 批量 `for_each` 导入 + 逐台进度
- [ ] 14.5 前端 import 向导对接（原型已有，联调）
- [ ] 14.6 测试：`go test ./server/core/importer/...` + e2e（真实资源 import、不支持资源、敏感字段、批量）

## 15-扩展能力整体验证

- [ ] 15.1 全量验证：`make build` + `make test` + `go test ./server/...`（含 11-14 新包）
- [ ] 15.2 端到端联调：身份登录 → 提交申请 → 审批（DSL 多级）→ 容器执行（版本隔离）→ 漂移；存量 import 一条龙
- [ ] 15.3 更新 docs/AGENTS.md，纳入新能力与新 scripts

## 16-CICD 集成与审批门禁（扩展，对应 D16 / specs/13；依赖 09 API + 12 审批 + 11 身份）

- [ ] 16.1 支持 yaml 申请单入口：`POST /api/v1/requests` 接受 `application/yaml`，解析为工单，与表单等价
- [ ] 16.2 CI 上下文与幂等：`trigger.cicd`（pipeline/commit/artifact/form_hash）持久化 + `pipeline:commit:catalogItem:form_hash` 幂等键去重
- [ ] 16.3 gate API：`GET .../gate` 状态（区分 `approval_granted` 与 `apply_succeeded`）+ `POST .../subscribe` webhook 订阅
- [ ] 16.4 阻塞/回调双模 + gate 超时联动审批引擎 `on_timeout`
- [ ] 16.5 通用 CLI `aether gate`（request/wait/timeout，退出码 0/1/2）+ Jenkins/GitLab/Argo/Flux/GitHub Actions 适配器与示例；`aether-gate` 仅作为兼容 shim
- [ ] 16.6 测试：`go test ./server/core/cicd/...` + e2e（yaml 入口、幂等、轮询/回调、审批通过释放/驳回终止）
- [ ] 16.7 脚本：`scripts/编排执行/` 提供 `aether gate` 示例与各 CICD 流水线片段

## 17-平台 CLI 与 AI 原生扩展（扩展，对应 D17 / specs/14；依赖 09 API + 11 身份 + 12 审批）

- [ ] 17.1 实现 `server/internal/cli`（cobra 单二进制 `aether`）：auth/catalog/request/stack/drift/cost/approval/gate 子命令，复用 `server/core/api` service 层，支持 `--output {table|json|yaml|llm}`；D16 gate 以 `aether gate` 为权威入口，历史 `aether-gate` 仅作兼容 shim
- [ ] 17.2 实现 `server/internal/identity/aksk`：service account + AK/SK 签发、HMAC SigV4-like 签名校验、timestamp 防重放、双 AK 轮换、scope 限定、审计联动（actor=sa:xxx）
- [ ] 17.3 实现 `aether mcp serve`（`server/internal/mcp`）：把所有命令 + skills 暴露为 MCP tools，schema 自描述（基于 mark3labs/mcp-go 或自实现），支持 stdio 与 SSE
- [ ] 17.4 实现 `server/internal/skills`：YAML skill 解析（trigger/steps/output）、执行引擎（LLM 抽参 + CLI 编排 + 模板渲染）、平台内置 skill 集（new-rds / drift-explain / cost-estimate / bulk-import）
- [ ] 17.5 实现团队自定义 skill 注册与版本化（DB/git）、skill 可见性按 RBAC
- [ ] 17.6 实现 `aether ai` 自然语言入口（平台后端 LLM 意图路由 → 匹配 skill 或分解 CLI 步骤）
- [ ] 17.7 落实安全边界：LLM 生成的 yaml 走 OPA + 审批；高危（destroy/跨层依赖变更）强制人工审批；agent 操作全审计（含 skill 编排全链路）
- [ ] 17.8 测试：`go test ./server/internal/cli/... ./server/internal/identity/aksk/... ./server/internal/mcp/... ./server/internal/skills/...`（命令契约、签名校验、MCP tool schema、skill 编排、安全边界拦截）
- [ ] 17.9 e2e：agent（mock MCP client / Claude Code）走 skill 完成新 RDS 申请 + drift 解释；高危操作被强制人工审批拦截
- [ ] 17.10 脚本：`scripts/初始化/` service account 与 AK/SK 签发辅助；`scripts/编排执行/` MCP server 部署与 agent 接入示例

## 18-CMDB 与 FinOps（扩展，对应 D18 / specs/15；依赖 03 模块注册 + 06 编排 + 07 漂移 + 10 审批）

- [ ] 18.1 **Phase 1**：实现 `server/core/cmdb` resource ingester（apply 后从 state JSON 解析 upsert `resources`）、tag 归一化、CMDB 查询 API；ingester 失败进入 `reconcile-pending`，不回滚 apply
- [ ] 18.2 **Phase 1**：强制 tag 策略：codegen 自动注入平台 tag；catalog 注册校验含 tag；tag 缺失阻断注册
- [ ] 18.3 **Phase 1**：实现 Infracost 申请时估算与预算提示，只做 guardrail 展示和审批提示，不做账单核销
- [ ] 18.4 **Phase 1**：前端最小 CMDB 资源清单 + Run Health / Catalog Usage 所需资源索引；不做完整 FinOps 看板
- [ ] 18.5 **Phase 2**：实现云账单拉取 job（AWS CUR / 阿里云 / Azure 适配器），按 tag 归集 `cost_records`，用于月度核销与预算达成率
- [ ] 18.6 **Phase 2**：实现预算治理 `cost_budgets`：多级阈值告警 + 申请时预估超预算触发审批升级（联动 specs/10 条件分支）
- [ ] 18.7 **Phase 2**：实现 FinOps 看板（团队成本趋势 / 预算燃尽 / Top资源），外部成本平台仅作为导出/对账对象
- [ ] 18.8 **Phase 3**：实现孤儿资源检测：漂移引擎 + 云侧资源清单对比 CMDB，标 orphan + 持续成本估算
- [ ] 18.9 **Phase 3**：实现成本优化建议引擎：rightsize / release_orphan / reserved_instance / tag_missing，一键转申请单
- [ ] 18.10 测试：Phase 1 覆盖 ingester、tag 注入、Infracost 估算；Phase 2+ 增加账单归集、预算告警、孤儿检测、优化建议
- [ ] 18.11 e2e：Phase 1 为 apply → 资源入库 → Infracost 提示；Phase 2+ 扩展到账单归集 → 预算告警 → 优化建议转申请
- [ ] 18.12 脚本：`scripts/状态同步/` CMDB 对账；Phase 2+ 增加云账单拉取与预算配置

## 19-云账号与凭据管理（扩展，对应 D23 / specs/16；依赖 03 模块注册 + 06 编排 + 11 工具链/Executor + 18 FinOps）

- [ ] 19.1 实现 `server/core/cloudcreds`：云账号纳管（`cloud_accounts` CRUD + bootstrap 凭据 secret store 引用）
- [ ] 19.2 实现 `team_cloud_grants` 授权表 CRUD + RBAC（仅平台运维可授予）；申请页"目标云"过滤 API：`GET /requestable-cloud-accounts?team_id=X` + `GET /catalog?team_id&cloud_account_id` 按 grant 过滤
- [ ] 19.3 实现三层凭据模型：①bootstrap（每云账号 1 套，双人审批 + 周期轮换 + 全审计）②执行凭据（per-team/per-bundle，OIDC 优先无长存）③个人（仅 RBAC，**禁持执行凭据**——DB 校验无个人 ↔ cloud_credentials 关联）
- [ ] 19.4 实现 OIDC 联邦：平台作 Trusted Issuer（生成/暴露 JWKS）；Executor 执行时拿平台 token → 云 `AssumeRoleWithWebIdentity`/`AssumeRoleWithOIDC` 换 STS（aws/alicloud 适配器）；fallback 长 AK/SK 从 secret store 取
- [ ] 19.5 实现 `CredentialResolver`：按 `(team, cloud_account, catalog_item)` 解析 → 返回 env map；Executor.Run 前调用注入子进程/容器 env
- [ ] 19.6 codegen 占位符机制：敏感字段写 `__TM_SECRET_<NAME>__`（不写真值）；worktree/git 始终保持占位符；CI 跑 secret scanning 兜底
- [ ] 19.7 日志脱敏管道：AK 形态正则（AKIA*/LTAI*）+ 已知 secret SHA256 前缀匹配 → 替换 `***REDACTED-<hash>***`，覆盖 terraform/terramate/Executor/审计全链路
- [ ] 19.8 IAM 角色模板化：catalog 项声明 `required_permissions`；平台聚合生成 `iam_role_templates`；bootstrap 凭据在云内 provision 实际角色（alicloud RAM / aws IAM）+ 配 OIDC trust
- [ ] 19.9 OPA 二次校验：申请项 ↔ 团队执行角色权限匹配，不匹配阻断执行
- [ ] 19.10 凭据生命周期：扫描过期/将过期（< 7 天告警）；长存 90 天强制轮换；泄漏检测（secret scanning 扫日志/git）→ 强制轮换；bootstrap 双人审批
- [ ] 19.11 前端"云账号与凭据"管理页：云账号清单 + 团队授权矩阵 + 凭据列表（密文永不回显）+ OIDC/角色配置入口 + 申请页"目标云"下拉按 grant 过滤
- [ ] 19.12 测试：`go test ./server/core/cloudcreds/...`（grant 过滤、OIDC 路径、占位符填充、日志脱敏、角色权限聚合、轮换窗口）
- [ ] 19.13 e2e：注册云账号 → 授权 DBA 团队 → OIDC trust 配置 → DBA 申请 RDS → Executor 拿 STS 跑 plan/apply → 日志无明文凭据 → 业务 B 团队看不到该云账号
- [ ] 19.14 脚本：`scripts/初始化/` 云账号纳管 + bootstrap 凭据双人配置；`scripts/凭据轮换/` 周期轮换与泄漏紧急轮换

## 20-环境与租户管理（扩展，对应 D27 / specs/17；依赖 02 元数据 + 04 分层 + 16 云账号 + 18 FinOps）

- [ ] 20.1 实现 `server/core/envtenant/envmanager`：Environment CRUD（env_logical_id UNIQUE + stage 治理强度 + cloud_account_id + region + tag_namespace_json + network_topology）；UI 入"环境管理"顶级导航
- [ ] 20.2 实现 `server/core/envtenant/tenantmanager`：Tenant CRUD（默认初始化 platform-default，isolation_level ∈ {vpc-per-env, account-per-env}，拒绝非法值含 shared-vpc）；UI 入"租户管理"顶级导航
- [ ] 20.3 实现 `environment_tenant_bindings` CRUD + 校验：三元组 (env, tenant, layer_logical_id) UNIQUE；vpc_stack_id MUST 是 Global 层 stack + outputs 含 vpc_id/vswitch_ids；account-per-env 模式 override_cloud_account_id 必填且 ≠ env.cloud_account_id；layer_logical_id MUST 在 layer_logical_refs 存在（D26）
- [ ] 20.4 实现 `BindingResolver`：codegen Stage 4 查询入口，按 (env_id, tenant_id, layer_logical_id) 返回 vpc_id/subnet_ids/region/cloud_account_id；缺 binding 时返回结构化错误 `"binding_missing"` 触发工单 blocked-policy
- [ ] 20.5 扩展 `team_cloud_grants` 加 env_scope_json 字段 + 申请页"目标云+env+层"三维过滤 API；env_scope=[] 全 env；env_scope=["prod"] 限定 env_logical_id
- [ ] 20.6 PathGenerator 加 `${tenant}` 变量（默认 platform-default，零迁移）；layer.path_template 继续遵守 D29 layer-first（如 `application/${tenant}/${owning_team}/${component}-${env}`），不得回退到 tenant-first 模板
- [ ] 20.7 跨租户网络治理：Global 层独立 cen-<env> stack（CEN/VPC Peering/Transit Gateway 适配器化 D7）；路由策略走 catalog 项 + 审批（"corpA 申请访问 DBA 的 RDS"作为 cross-layer 依赖请求）
- [ ] 20.8 lifecycle：environments/tenants/bindings.status = active/frozen/deprecated；frozen 阻断新工单（409），deprecated 禁止新建 binding（存量走 D26 sunset）
- [ ] 20.9 前端：环境管理页（env 列表 + 5 步创建向导 + binding 矩阵 env×tenant×layer）；租户管理页（tenant 列表 + isolation_level 切换 + binding 关联视图）
- [ ] 20.10 测试：`go test ./server/core/envtenant/...`（三元组 UNIQUE 校验、vpc_stack outputs 校验、account-per-env override 校验、BindingResolver 查询、env_scope 过滤、freeze/deprecate lifecycle）
- [ ] 20.11 e2e：创建 env=prod + tenant=corp-a(vpc-per-env) + binding(prod,corp-a,Application → vpc-corpA-prod) → 业务申请 ECS 自动从 binding 解析 vpc_id → resolved_params_json.source=env_context
- [ ] 20.12 脚本：`scripts/初始化/` 默认 platform-default tenant 引导 + 首个 env 创建向导

## 21-标签分层与参数解析管道（扩展，对应 D28 / specs/18；依赖 03 catalog + 05 codegen + 17 tenancy + 20 env/tenant）

> **Phase 策略**：Phase 1 用简化 5 阶段合并（contract + defaults + governance + user + dependency），不上完整 9 阶段；Phase 2 完整 9 阶段 + provenance + S3-S9 治理类 + tie-breaker。tasks 21.1-21.16 描述的是 Phase 2 目标架构。

- [ ] 21.1 实现 `server/core/codegen/paramresolver`：9 阶段管道主循环（S1-S9），每阶段含 source 加载 + rank 覆盖 + 校验门
- [ ] 21.2 实现 S3 layer rule 注入（rank 2，治理硬规则）：从 layer_rule_set_versions（D26 pinned）读规则，强制注入如 encrypt_at_rest/allowed_regions；用户值违反 → 拒绝
- [ ] 21.3 实现 S4 环境上下文绑定（rank 6）：调 BindingResolver（20.4）拿 vpc_stack_id/subnet_ids/region，注入元变量（S7 用 vpc_stack_id 生成 remote_state）
- [ ] 21.4 实现 S5 tenant 上下文绑定（rank 5）：从 tenants.tag_namespace_json 拿 L3 tag 注入；tenant.kind=external 时校验不引用共享资源
- [ ] 21.5 实现 S6 team 策略（rank 3，治理类）：从 teams.policy_json 读 allowed_regions/cost_cap/mandatory_tags；**双阈值**：>2x cap=reject（安全阀）；1x~2x cap=标记 cost_overrun 透传 pre-apply 门（D21 成本升级审批）
- [ ] 21.6 实现 `server/core/codegen/tagmerger`：Tag 7 层合并（L1-L7）+ L1 永远赢 + L2-L6 后层覆盖前层 + L7 白名单约束（catalog.user_allowed_tag_keys_json）
- [ ] 21.7 实现 L1 强制注入（S9 平台常量）：platform-managed=true / platform-team / platform-bundle / platform-stack；用户尝试覆盖 silently ignore + user_overrides_blocked 记录
- [ ] 21.8 实现 `server/core/codegen/provenance`：ProvenanceWriter 写 requests.resolved_params_json，每变量含 {value, source, rank, overridden_from/rule_id/env_id/tenant_id/binding_id/user_attempted_override/user_attempted_value}；tags 字段含 layers_applied + user_overrides_blocked + layers_merged_summary
- [ ] 21.9 实现校验门：S1 必填 / S2 类型 / S3 layer rule 违反 / S4 binding 缺失 / S5 tenant 隔离 / **S6 成本超 2x cap 拒绝 + 1x~2x cap 标记透传** / S7 依赖不存在 / S8 类型/枚举/白名单 / S9 L1 tag 完整性
- [ ] 21.10 实现云厂商 tag 约束校验（S9 阶段）：per-provider adapter，alicloud（128 字符）/aws（含保留字前缀 aws:）/azure（特殊字符）→ 违规默认拒绝，运维显式开 sanitize 才软化
- [ ] 21.11 扩展 catalog_items：加 default_tags_json + user_allowed_tag_keys_json 字段；catalog 注册页表单加这两项配置
- [ ] 21.12 扩展 requests：加 env_id + tenant_id + resolved_params_json 字段；工单详情页加"参数来源溯源"面板（provenance 树形展示）
- [ ] 21.13 前端：catalog 注册表单加 default_tags_json / user_allowed_tag_keys_json；申请表单右侧实时展示 7 层 tag 合并预览 + provenance 面板
- [ ] 21.14 测试：`go test ./server/core/codegen/paramresolver/... ./server/core/codegen/tagmerger/... ./server/core/codegen/provenance/...`（9 阶段顺序、rank 优先级、L1 不可覆盖、L7 白名单拒绝、治理类 user_attempted_override 留痕、云 tag 约束校验、provenance 完整性）
- [ ] 21.15 e2e：① 用户填 storage=200 vs catalog default=100 → 最终 200 + provenance user_form ② layer rule 强制 encrypt=true vs 用户填 false → 最终 true + user_attempted_override=true ③ 用户 tag 白名单外被拒绝 ④ env context 自动注入 vpc_id + provenance source=env_context ⑤ 用户尝试 platform-managed=false silently ignored + 审计
- [ ] 21.16 脚本：`scripts/初始化/` 平台默认 tag_policies（platform scope，mandatory_keys=[platform-managed/team/bundle/stack]）+ catalog 默认白名单模板

## 22-安全加固（扩展，对应 D30；依赖 02 元数据 + 06 编排 + 09 API + 13 Executor + 19 云凭据）

- [ ] 22.1 Break-Glass emergency_mode：双人凭据（vault path `secret/aether-break-glass/{cloud_account}`，季轮换）→ 绕审批提交 emergency 工单 → Executor 全程录屏存对象存储 90 天 → 24h 内补审批回填（超时→incident+CISO 邮件）
- [ ] 22.2 State 敏感字段保护：state 后端 KMS CMK 加密；CMDB ingester strip `sensitive_field_blacklist` 字段（`*password*`/`*secret*`/`*private_key*`/`*certificate*`）；state download API 高权限 RBAC + 审计 + IP 白名单
- [ ] 22.3 Terramate CLI sha256 pin：CI 下载二进制后校验 `terramate_checksums.txt`，不匹配阻断+告警
- [ ] 22.4 IAM policy 聚合校验：catalog `required_permissions` 聚合后 OPA 二次校验（禁 `Action:*` / 禁 `Resource:*` / 禁通配 region / 禁 root），失败拒绝 catalog 注册
- [ ] 22.5 STS 缓存限制：OIDC STS token 仅活在单次 `Executor.Run` 进程内（env 注入，不落盘）；日志脱敏增 base64 解码扫描 + 已知 secret SHA256 前缀匹配
- [ ] 22.6 gitleaks CI：worktree commit 前跑 gitleaks scan，检测到 AK/SK 形态→阻断+触发 19.10 轮换 SOP
- [ ] 22.7 测试：`go test ./server/core/security/...`（break-glass 凭据验证+录屏+retroactive 超时、sensitive stripper 覆盖、OPA IAM 聚合校验规则、STS 不缓存断言）

## 23-商业级工作流（Phase 2+，对应 specs/20/21 + docs/07/13/16）

- [ ] 23.1 实现 PR-first 入口：监听 VCS PR/MR 事件，定位 changed stacks，触发 speculative plan
- [ ] 23.2 实现 PR plan summary：回写受影响 stack、资源变更、OPA/RunHook、成本估算和下一步动作
- [ ] 23.3 实现 apply requirements：branch protection、PR approval、平台审批、policy allow、plan artifact 未过期
- [ ] 23.4 实现受控评论命令：`tm plan` / `tm apply` / `tm unlock`，按评论 actor 做 RBAC 校验并审计
- [ ] 23.5 实现 Run Hooks：pre-plan/post-plan/pre-apply/post-apply，支持 fail-open/fail-closed/warn-only
- [ ] 23.6 接入至少一个安全扫描 hook（Checkov/tfsec/custom scanner）和一个 CMDB gate hook
- [ ] 23.7 Scheduled Runs 扩展：drift-plan、nightly-plan、maintenance-apply、stack-health-check
- [ ] 23.8 Environment Promotion：dev→staging→prod 参数 diff、binding diff、审批升级、目标 env plan 对比
- [ ] 23.9 测试：PR-first 与 form-first 统一进入 RequestLifecycle；hook deny 阻断 apply；promotion 不复制 state

## 24-平台产品化运营（Phase 1+，对应 docs/18/19 + specs/22）

- [ ] 24.1 实现 Run Health 看板：成功率、失败原因 TopN、plan/apply 耗时、waiting-manual 债务
- [ ] 24.2 实现 Approval Health 看板：审批等待、超时、升级、团队瓶颈
- [ ] 24.3 实现 Catalog Usage 看板：申请量、成功率、无人使用目录项、团队覆盖
- [ ] 24.4 实现 failed run reason 结构化归类：user_input/policy_denied/cloud_quota/cloud_api/toolchain/state_backend/platform_bug/manual_required
- [ ] 24.5 实现 catalog health checks：usage/reliability/governance/cost/version/ownership
- [ ] 24.6 实现 scorecard：catalog item 90+ 标记 golden path，低分项触发治理任务
- [ ] 24.7 输出月度运营报告：self-service ratio、ticket deflection、DORA/SPACE、reconcile debt

## 25-企业级证据链与运维演练（Phase 1+，对应 docs/20/21）

- [ ] 25.1 审计 HMAC chain：`prev_hash`、`entry_hash`、`signing_key_id`、`sealed_at`
- [ ] 25.2 合规证据包导出：request、actor、approval、policy、plan hash、apply run、CMDB、cost、audit chain
- [ ] 25.3 Break-glass 证据链：incident id、双人授权、录屏/命令记录、24h 补审批、postmortem
- [ ] 25.4 Runbook 执行记录：apply interrupted、state lock stuck、provider mirror down、OIDC JWKS expired、DB restore、executor lost、CMDB reconcile failed
- [ ] 25.5 DR drill：state restore、DB restore、break-glass、OIDC JWKS rotation、provider mirror outage
- [ ] 25.6 验收：每个 P0/P1 runbook 至少完成一次演练并写 `drill_results`

## 26-差异化创新保留（贯穿所有 Phase，对应 design §02c）

- [ ] 26.1 Terramate-native：PR-first、Scheduled Runs、drift、promotion 均使用 Terramate stack/tag/DAG 语义定位影响面
- [ ] 26.2 参数 provenance：审批页、AI 输出、失败解释和证据包都展示 `resolved_params_json`
- [ ] 26.3 存量 read-only：import 默认 `managed-readonly`，晋级 `managed-changeable` 前必须通过 owner/tag/drift/plan 验证
- [ ] 26.4 D26 迁移安全：StateMover 默认关闭，所有层规则变更先出迁移计划和影响面报告
- [ ] 26.5 AI 不绕过治理：MCP/skills/tm ai 共享 RBAC/OPA/approval/audit，禁止 AI actor 自审高危操作

