# 04-分层与 stack 组织（stack-layout）

能力 ID：`stack-layout`
覆盖：Global / Middleware / Application 三层模型、stack 与 bundle 的映射、跨层依赖解析，以及"多同类资源"的生成策略取舍。

## ADDED Requirements

### Requirement: 分层模型可配置（默认三层）
平台 SHALL 用可配置的 Layer 规则模型（DB 表 `layers`：name/order/owning_team_pattern/path_template/depends_on）定义分层，**MUST NOT 硬编码层名**。出厂默认提供三层——`Global`（VPC/交换机/NAT/ACK 集群等全局共享，归属平台运维）、`Middleware`（RDS/Redis/Kafka 等被多业务共享，归属 **DBA + 中间件两部门并列同层**）、`Application`（业务 ECS/SLB 规则/账号等，归属**不同业务部门各自独立**）——管理员可增删层（如加"security/compliance"第 4 层、或合并成 2 层）、改路径模板、改依赖方向。每层有独立的目录根、归属团队模式与依赖方向（上层 depends_on 下层，单向）。

> Phase 1 MAY 固定三层与 D29 layer-first Path Contract，仅开放读取；管理员自定义层与迁移能力进入 Phase 2/3。

#### Scenario: 默认三层开箱即用
- **WHEN** 平台首次部署未自定义层配置
- **THEN** 出厂默认三层（Global/Middleware/Application）生效，路径与归属团队按 docs/02 §1 规则

#### Scenario: 自定义层
- **WHEN** 管理员新增"security"层（路径模板 `security/{{.env}}/{{.component}}`，归属安全团队，order=2，插在 Middleware 前）
- **THEN** 新层生效，相关 catalog 项可声明 `layer=security`，PathGenerator 按新模板渲染路径

#### Scenario: Middleware 两部门并列同层
- **WHEN** DBA 团队申请 RDS、中间件团队申请 Kafka
- **THEN** 两者均在 `middleware/` 层根下（`middleware/platform-default/rds-*` / `middleware/platform-default/kafka-*`），**同层级、互不统属**，RBAC/审批/成本归集按部门独立路由

### Requirement: PathGenerator（路径模板化）
平台 SHALL 提供 PathGenerator 按 layer 的 `path_template`（Go text/template）渲染 stack 目录路径、state key、stack_id 与 Terramate tags；codegen MUST 调用 PathGenerator，MUST NOT 字符串拼接路径。模板变量：`env / tenant / team / bundle / component / layer / layer_order / custom_kv`。state key 与 stack 路径一一对应。默认采用 D29 layer-first 单仓拓扑。

#### Scenario: Application 层 bundle 可选路径
- **WHEN** catalog 项 layer=application，路径模板 `application/{{.tenant}}/{{.team}}/{{if .bundle}}{{.bundle}}/{{end}}{{.component}}-{{.env}}`，tenant=platform-default，team=team-b，bundle 为空，component=ecs，env=prod
- **THEN** PathGenerator 渲染 `application/platform-default/team-b/ecs-prod`（无 bundle 段）；若 bundle=orders 则渲染 `application/platform-default/team-b/orders/ecs-prod`

#### Scenario: 确定性路径推导
- **WHEN** 元数据：layer=application、tenant=platform-default、team=team-a、bundle=orders、component=ecs、env=prod
- **THEN** PathGenerator 渲染 stack 路径 `application/platform-default/team-a/orders/ecs-prod`，state key 与之一一对应，并输出 stack_id 与 tags

#### Scenario: Global 层 stack 独立
- **WHEN** 创建 VPC 与 ACK 集群
- **THEN** 它们各自作为独立的 Global stack（如 `vpc-global-prod`、`ack-global-prod`），归属平台运维团队，独立状态文件

### Requirement: stack 边界 = 逻辑治理单元
平台 SHALL 按"逻辑治理单元"切分 stack 边界——一个 stack = 一个独立的治理/变更/状态单元（一个服务/一个组件类别），**不是按规格或实例数切**。同一逻辑单元内多实例（含异构规格）放一个 stack 用 for_each/count（specs/05 cardinality）；跨逻辑边界（不同服务/不同团队/不同环境/不同审批流）拆 stack。

#### Scenario: 多组件不合并 state
- **WHEN** Application 层 `order-service` 包含 ECS×5、SLB 规则、RDS 账号（三个独立组件）
- **THEN** 平台生成 3 个独立 stack 分别承载，而非把所有资源塞进一个 stack 的单一 state

#### Scenario: 同服务异构规格不拆 stack
- **WHEN** `order-service` 的 ECS 有 web/API/batch 三种角色、三种不同规格
- **THEN** 平台生成 1 个 stack（逻辑单元 = order-service 的 ECS），内部用 `for_each = tomap({web=..., api=..., batch=...})`（cardinality=map，specs/05），**不拆 3 个 stack**

#### Scenario: 跨治理边界必拆 stack
- **WHEN** 业务 A 的 ECS 和业务 B 的 ECS（哪怕同规格）
- **THEN** 拆成 2 个 stack（不同归属团队/不同 RBAC/不同审批流），各自独立 state

### Requirement: 跨层依赖解析
平台 SHALL 维护跨层依赖元数据：Application 层 stack 通过 Terraform `data source` 引用 Global / Middleware 层 stack 的输出（如 VPC ID、RDS 实例 ID），平台 MUST 在代码生成时注入正确的 `data` 引用与 remote state 配置。

#### Scenario: 应用层引用全局 VPC
- **WHEN** `order-service` 需要部署到 `vpc-global-prod` 的交换机
- **THEN** 生成的 Terraform 代码通过 `data "terraform_remote_state"` 或云厂商 data source 读取 `vpc-global-prod` 的 vsw 输出，而非硬编码

#### Scenario: 应用层依赖中间件 RDS
- **WHEN** `order-service` 需要连接 `rds-orders-prod`
- **THEN** 平台注入对 `rds-orders-prod` stack 输出的引用，并在依赖图中标记 `order-service` depends-on `rds-orders-prod`

### Requirement: 多实例生成策略（cardinality 驱动，模块零侵入）
平台 SHALL 对多实例资源按 catalog 项声明的 `cardinality`（`single`/`list`/`map`）由 codegen 在调用方注入 `for_each`/`count`（specs/05 D25），**原子模块保持单实例语义零感知**；MUST NOT 用 wrapper module 包装，MUST NOT 为每个运行期实例生成独立 stack。

#### Scenario: 一个 RDS stack 单实例（cardinality=single）
- **WHEN** 业务申请一台 RDS
- **THEN** 平台生成 stack 骨架 + 普通 module 调用（无 for_each）

#### Scenario: 同规格多实例（cardinality=list）
- **WHEN** `order-service` 需 5 台同规格 ECS
- **THEN** codegen 生成 `count = 5` 或 `for_each = toset(...)`，原子模块零感知，**不生成 5 个 stack**

#### Scenario: 异构规格多实例（cardinality=map）
- **WHEN** `order-service` 的 ECS 有 web/API/batch 三角色不同规格
- **THEN** codegen 生成 `for_each = tomap({web={spec...}, api={spec...}, batch={spec...}})`，原子模块的 variables.tf 全 scalar 不变，**不生成 3 个 stack**

### Requirement: stack 粒度策略（StackGranularity）
平台 SHALL 支持可配置的 stack 粒度策略（默认 `per-component`——一个组件一个 stack）；可选 `per-bundle`（bundle 内合并）/ `per-team`（业务部门内合并）/ `custom`（catalog 项声明 `stack_grouping` 规则）。粒度策略存 DB 表 `stack_grouping_rules`，管理员可配。

#### Scenario: 默认 per-component
- **WHEN** 业务 A 申请 ECS + SLB + RDS 账号三个组件
- **THEN** 平台生成 3 个独立 stack（每组件一个），爆炸半径最小

#### Scenario: catalog 项声明 per-bundle
- **WHEN** 某 catalog 项（微服务全套）声明 `stack_grouping=per-bundle`
- **THEN** 平台把该 catalog 项的多个子组件合并到 bundle 内一个 stack，减少 stack 数量

### Requirement: 层规则集版本化与迁移（D26）
平台 SHALL 把整套分层方案（layer_rule_set）作为**不可变版本化对象**——管理员改任何层规则 = 整个 set bump v+1（v1→v2→v3），旧版本 `status=superseded` 保留不可变。每个层有**跨版本稳定的逻辑身份** `layer_logical_id`（uuid），`catalog_items` 引用 `layer_logical_id`，`stacks` 创建时 pin 当前 active `layer_rule_set_version_id`。运行期改 layer 规则时，平台 MUST 按 **per-stack dry-run** 把每个受影响 stack 归到三档（Tier 1/2/3）执行迁移；**核心不变量：state_key 不经 admin 显式操作不能漂移**，平台宁可阻塞也不自动改 state_key。

> 自动 StateMover MUST 默认关闭。Phase 1/2 只生成迁移计划、影响面报告和人工 SOP；半自动 state mv 进入 Phase 3，且必须双人审批、state snapshot、plan=0。

#### Scenario: 整体版本化（非单 layer）
- **WHEN** 管理员改 Global 层的 `path_template`（Middleware/Application 未改）
- **THEN** 平台创建新 `layer_rule_set_versions` 记录（v2，复制全部三层的最新定义），旧 v1 标记 `superseded`，**不**对单 layer 单独版本化

#### Scenario: catalog_items 引用稳定身份
- **WHEN** layer 规则从 v1 升 v2
- **THEN** `catalog_items.layer_logical_id`（uuid）不变，新创建 stack 解析 `layer_logical_id` + active v2 → 渲染新 path；catalog_items 表零改动

#### Scenario: Tier 1 自动迁移（path 不变）
- **WHEN** v1→v2 改了 `owning_team_pattern`（cosmetic），MigrationPlanner 对某 stack dry-run 重渲染 path
- **THEN** 若新旧 path 一致，平台自动 bump `stacks.layer_rule_set_version_id` v1→v2，无需 state 操作，admin 仅审批即可

#### Scenario: Tier 2 辅助迁移（path 变可推导，Worker 跑 state mv）
- **WHEN** v1→v2 改了 `path_template`（如 `<team>/<component>-<env>` → `<team>/<bundle>/<component>-<env>`），某 stack 新 path 可推导且不冲突
- **THEN** 平台 StateMover 通过 Executor 在受控 Worker 跑 `terraform state pull/push` 跨 backend mv，跑 `terraform plan -detailed-exitcode` 必须 exit 0（zero-diff），否则**自动从 StateBackup 回滚** + 告警；mv 完成后**同步 CMDB `cmdb_resources.stack_path`** + on-demand 漂移对账；admin 需 2 人审批 + 静默期

#### Scenario: Tier 3 双轨分叉 + sunset 强制迁移
- **WHEN** v1→v2 不可逆变更（删层 / path 冲突），某些 stack 无法迁移
- **THEN** 平台把这些 stack 永久 pin v1（`layer_rule_set_version_id` 不变），标 `deprecated_at` + `sunset_at`（默认+6 个月），新 stack 走 v2；管理员须在 sunset 前通过 `terraform destroy` + 新版本下 recreate 完成迁移，sunset 到期旧版本 `archived` 拒绝新建

#### Scenario: 迁移批次静默 + 回滚窗口
- **WHEN** 迁移批次进行中
- **THEN** 平台对该批 stack 进入 QuiesceMode（工单/plan/apply 阻塞、漂移检测**自动静默**，非全局暂停），StateBackup 出 rollback_token；sunset 窗口内可经 RollbackEngine 逆向 state mv 回退 vN→vN-1

#### Scenario: diff viewer 必备
- **WHEN** 管理员提交新 layer_rule_set 草案
- **THEN** 平台 UI 显示 v1↔v2 diff（哪些 path_template 改了、加了哪层、改了哪些 depends_on），盲改被禁止
