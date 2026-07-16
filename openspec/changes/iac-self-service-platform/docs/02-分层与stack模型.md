# 02-分层与 stack 模型

> 对应 design.md `04 D3/D6`、spec `specs/04-分层与stack组织` 的详细展开。回应：基础网络与中间件如何与应用关联、foreach vs gen、stack/space 组织。

## 1. 三层定义与归属

| 层 | 内容 | 归属团队 | 典型 stack |
|----|------|----------|-----------|
| Global（第一层） | VPC、交换机、NAT、ACK 集群等全局共享 | **平台运维**（单一部门） | `vpc-global-prod`、`ack-global-prod` |
| Middleware（第二层） | RDS、Redis、Kafka、RabbitMQ 等被多业务共享 | **DBA + 中间件**（两个部门**并列同层**，互不统属） | `rds-orders-prod`（DBA）、`kafka-platform-prod`（中间件） |
| Application（第三层） | 业务 ECS、SLB 规则、数据库账号等 | **不同业务部门**（业务A / 业务B / … 各自独立） | `order-service-prod`（业务A）、`user-service-prod`（业务B） |

### 1.1 层级归属要点（重要）

- **第二层 = DBA + 中间件两个部门并列**——DBA 团队管 RDS / Redis，中间件团队管 Kafka / RabbitMQ，二者**同一层级、互不统属、各自维护自己的 stack 与 state**。平台 RBAC、审批、成本归集按部门独立路由（DBA 的工单不进中间件团队审批）。
- **第三层 = 不同业务部门各自独立**——业务A、业务B、… 多个业务部门在同一层级，每个业务部门下有自己的项目组（space），**互不干扰、状态隔离、资源不可见**。
- **跨层单向依赖**：Global → Middleware → Application，上层 stack 通过 `terraform_remote_state` / data source 读下层 outputs（§4），**下层不反向依赖上层**。

### 1.2 stack 粒度原则：一个组件（component）一个 stack 目录

**每个原子组件 = 一个独立 stack 目录**——一个 RDS 实例、一个 Kafka 集群、一组 ECS、一套 SLB 规则、一个 RDS 账号，都各自对应一个 stack 目录，独立 state key、独立漂移检测、独立审批与回滚。这是控制爆炸半径、状态隔离、独立治理的基本单元。

| 反例（禁止） | 正例 |
|--------------|------|
| 一个 space 全塞一个 stack | space 下每个 component 一个 stack |
| 同一 component 多实例拆多个 stack | 同一 stack 内多实例用 `for_each`（§5） |
| 跨业务共用一个 Application stack | 每个业务部门各自的 component 独立 stack |

例：业务部门 `team-a` 下含 3 个 component → 3 个 stack 目录（space 可选，见 §2）：

```
# 模式 A：业务部门下直接 component（小部门/资源少，无 space）
application/platform-default/team-a/
├── ecs-prod/          # component：ECS × 5（for_each 多实例，单 stack）
├── slb-rules-prod/    # component：SLB 规则
└── rds-account-prod/  # component：RDS 只读账号

# 模式 B：业务部门下按 space 分组（大部门/多产品线）
application/platform-default/team-b/
├── orders/            # space：订单项目组
│   ├── ecs-prod/
│   └── rds-account-prod/
└── users/             # space：用户项目组
    └── ecs-prod/
```

**同 stack 内多实例（5 台 ECS、3 个 Kafka Topic）用 Terraform `for_each`/`count`**，不拆 stack（避免 stack/state 爆炸，详见 §5）。

**1:1 粒度的业界依据**（客观论证，对齐 Terramate 官方 + Spacelift 共识）：

| 维度 | 证据 |
|------|------|
| Terramate 官方推荐 | ["How to structure and size Terraform Stacks"](https://terramate.io/rethinking-iac/how-to-structure-and-size-terraform-stacks/) 明确推荐 **Service Stack 模式**（one stack per service and environment）。1 component = 1 stack 对齐该模式的中位粒度。 |
| Spacelift 业界实践 | 社区共识 "one Spacelift stack per enabled root module"（masterpoint.io automation）；2025 趋势是拆 stack 不是合并（addshore.com "splitting a stack in 2"），核心理由是 blast radius + 独立审批 + 变更检测。 |
| 不是过度拆分 | component 允许 `for_each` 多实例（5 台 ECS = 1 stack），不等于"每资源一个 stack"（那才是 Microservice Stack 过度拆分）。 |
| blueprint 补偿 | 组合申请（一次 N 个资源）用 blueprint 展开 N stack + DAG 排序，用户体验不退化。 |

**结论**：1:1 在"独立生命周期 + 独立 state + 独立审批"三要素下是最优，前提是 blueprint（组合模板）+ stack_dependencies（DAG）+ Terramate `--parallel N`（并行）三者配合（见 §5 + docs/06）。

### 1.3 space 不支持嵌套（重要约束）

> **约束**：space 是**单层可选**的路径分组，不支持任意层级嵌套（即不能 `orders/prod/ecs-prod/` 这种 space 下再嵌套 space）。

| 场景 | 正确做法 | 错误做法 |
|------|---------|---------|
| 业务部门下多个团队/产品线 | `team`（路径第 2 段）+ `space`（路径第 3 段）两层组合 | ❌ 用 space 嵌套表达 |
| 需要 3+ 层级（大部门→子部门→产品线） | 用 `tags_json` 表达深层归属（如 `division=ecommerce-domestic`），路径保持 2 层 | ❌ space 嵌套 |
| SaaS 多租户频繁 3+ 层 | 未来扩展 space 为 `ltree + parent_id`（非 MVP，YAGNI）| ❌ MVP 就引入树形 |

**理由**：① 当前 `spaces` 表无 `parent_id`、非 ltree（doc 04）；② PathGenerator 契约是 `[space/]` 单段；③ 嵌套会引入 RBAC 继承 + 审批路由复杂度，MVP 不值得；④ 90% 业务场景两层够用。

## 2. 目录布局（确定性路径）

> **权威 Path Contract**：本节以 `design.md` D29 为准。工作仓库采用单仓 + layer-first 拓扑；`tenant/env/team/space/component` 是路径和 Terramate tags 的维度，但不再保留 tenant-first 或 legacy `globals/<env>` 作为并列模板。PathGenerator 必须一次性输出 `repo_path`、`state_key`、`stack_id`、Terramate tags，四者共同构成 stack 身份契约。

工作仓库根目录：

```
infra-repo/
├── terramate.tm.hcl
├── global/
│   ├── vpc-{tenant}-{env}/
│   │   ├── stack.tm.hcl
│   │   ├── main.tf
│   │   └── backend.tf              # state_key = global/vpc-{tenant}-{env}
│   ├── ack-{tenant}-{env}/
│   └── cen-{env}/                  # 跨租户网络，不绑定单一 tenant
├── middleware/
│   └── {tenant}/{component}-{env}/
│       ├── stack.tm.hcl
│       └── main.tf
└── application/
    └── {tenant}/{team}/[space/]{component}-{env}/
        ├── stack.tm.hcl
        └── main.tf
```

**路径推导规则**（确定性，由元数据唯一生成；`space` 可选）：
- Global：`global/<component>-<tenant>-<env>`，如 `global/vpc-platform-default-prod`
- Middleware：`middleware/<tenant>/<component>-<env>`，如 `middleware/platform-default/rds-orders-prod`
- Application 无 space：`application/<tenant>/<team>/<component>-<env>`
- Application 有 space：`application/<tenant>/<team>/<space>/<component>-<env>`

state key 与 stack 路径**一一对应**，默认等于 `repo_path`。stack ID 由 PathGenerator 输出，建议格式为 `{layer}-{tenant}-{team}-{component}-{env}`，全小写、`-` 分隔、全局唯一、≤64 字符。Terramate tags 至少包含 `layer:*`、`tenant:*`、`env:*`、`team:*`；跨租户 CEN stack 使用 `cross-tenant` tag，不写单一 tenant tag。

**space 何时用**：业务部门内资源少（< 10 个 component）可不用 space，直接 `<team>/<component>`；业务部门内有多个产品线/项目组（如电商业务部门下"订单""用户""支付"）建议按 space 分组，便于按项目组批量编排/审计/成本归集。两种路径平台都识别，元数据 `stacks.space_id` 为空表示无 space。

## 3. space 与 stack 映射（space 可选）

- **space**：平台层概念，= 一个业务部门内的项目组/产品线（一组相关 component 的逻辑集合）。Terramate 本身无 space 概念。**space 可选**——小业务部门直接 `<team>/<component>`，大业务部门多产品线用 `<team>/<space>/<component>` 分组。
- **space → stacks**：一个 space 在工作仓库中映射到一个目录根（如 `application/platform-default/team-b/orders/`），其下每个 component 一个 stack。无 space 时业务部门目录就是直接的 stack 父目录。
- **编排粒度**：支持"按 space 整体编排"（`terramate run` 指向 space 根，递归所有子 stack）、"按业务部门整体编排"（无 space 时指向 `<team>` 根）、或"按单 stack 操作"。

示例 space `team-b/orders`（大业务部门用 space）：
```
application/platform-default/team-b/orders/
├── ecs-prod/          # ECS × 5（for_each 多实例）
├── slb-rules-prod/    # SLB 规则
└── rds-account-prod/  # 申請到的 RDS 只读账号
```

无 space 示例 `team-a`（小业务部门）：
```
application/platform-default/team-a/
├── ecs-prod/
└── slb-rules-prod/
```

## 4. 跨层依赖机制（基础网络/中间件如何与应用关联）

**核心**：上层 stack 通过 Terraform `data source` 读取下层 stack 的输出，**不硬编码**。两种实现：

### 4.1 `terraform_remote_state`（跨 stack 读输出）

> **2026-07-16 修订**：bucket/region 不再硬编码——codegen 从 `state_backends` 表（A9）读取父级配置，从 `stack_dependencies` 表读取上游 state_key，动态渲染。下方示例保留结构，具体值由 codegen 注入。

```hcl
# application/platform-default/team-a/orders/ecs-prod/cross-layer.tf
# codegen 生成：bucket/region 从 state_backends 表读，key 从 stack_dependencies.to_stack_id 解析
data "terraform_remote_state" "vpc" {
  backend = var._tm_backend_kind               # ← state_backends.kind (s3|oss)
  config = {
    bucket = var._tm_backend_bucket            # ← state_backends.bucket
    key    = var._tm_upstream_vpc_state_key    # ← stack_dependencies → stacks.state_key
    region = var._tm_backend_region            # ← state_backends.region
  }
}
module "ecs" {
  source  = "github.com/org/terraform-alicloud-ecs"
  vswitch_id = data.terraform_remote_state.vpc.outputs.vswitch_ids[0]
}
```

**解析链路**（codegen 生成 cross-layer.tf 时）：
1. 读 `module_dependencies`：本模块需要 vpc.vswitch_id
2. 读 `stack_dependencies`：to_stack_id = vpc stack → stacks.state_key = "global/vpc-platform-default-prod"
3. 读 `state_backends`（is_default 或 stacks.state_backend_id override）：bucket/region/encrypt
4. 渲染 data 块（变量从 DB 来，不硬编码）

### 4.2 云厂商 data source（直接查资源）
```hcl
data "alicloud_instances" "db" {
  instance_name = "rds-orders-prod"
}
```

**平台职责**：在代码生成阶段，依据元数据中的"依赖关系"，自动注入对应的 `data` 块与变量绑定（详见 spec `specs/04-分层与stack组织` 的"跨层依赖解析"）。

## 5. 多实例策略（cardinality 驱动，模块零侵入）

> 对应 D25、specs/05、docs/09 §8。核心：**模块定义单实例语义，平台调用方注入多实例语法**。

### 5.1 foreach vs gen 决策表

| 场景 | 用什么 | 为什么 |
|------|--------|--------|
| 生成一个 stack 目录骨架（stack.tm.hcl、backend.tf、main.tf 调模块） | 平台 codegen（PathGenerator 渲染路径） | 生成期、静态、确定性 |
| 同一 stack 内同质多实例（5 台同规格 ECS） | Terraform `count` 或 `for_each toset` | 运行期、状态聚合、便于滚动 |
| 同一 stack 内**异构多实例**（web/API/batch 三角色不同规格） | Terraform `for_each = tomap({...})` | 同治理单元、状态聚合、角色稳定 key |
| 不同业务各申请一台独立 RDS | 每业务一个 stack（codegen 各自生成） | 状态隔离、爆炸半径小 |
| 给某 stack 加新组件 | codegen 追加新 stack | 静态扩展 |

**禁止**：用 codegen 为每个运行期实例生成独立 stack（会导致 stack/state 爆炸）。

### 5.2 三种 cardinality（catalog 项配置，模块零侵入）

模块的 `variables.tf` **永远是 scalar**，平台在调用方注入：

```hcl
# cardinality = single（默认）：直接调用
module "rds" {
  source         = "github.com/org/tf-rds"
  instance_type  = "mysql.n2.large.1c"
  storage_size   = 200
}

# cardinality = list（同质多实例）：count
module "ecs" {
  source        = "github.com/org/tf-ecs"
  count         = 3
  instance_type = "ecs.c6.large"   # 共用规格
  name          = "order-${count.index + 1}"
}

# cardinality = map（异构多实例）：for_each tomap，key=角色名
module "ecs" {
  source   = "github.com/org/tf-ecs"   # 同一个模块，变量全 scalar
  for_each = tomap({
    web   = { instance_type = "ecs.c6.large",   disk = 40,  count = 3 }
    api   = { instance_type = "ecs.c6.xlarge",  disk = 100, count = 2 }
    batch = { instance_type = "ecs.r6.2xlarge", disk = 200, count = 1 }
  })
  name          = "order-${each.key}"
  instance_type = each.value.instance_type   # 模块 var 仍是 scalar string
  disk_size     = each.value.disk_size
  count         = each.value.count
  # shared 字段（不进 map）
  vpc_id        = data.terraform_remote_state.vpc.outputs.vpc_id
}
```

**模块内部完全不知道自己被 for_each**——`variable "instance_type" { type = string }` 不变。详见 docs/09 §8。

### 5.3 何时拆 stack（stack 边界 = 逻辑治理单元）

| 情况 | 决策 |
|------|------|
| 同服务多角色 ECS（web/api/batch） | 1 stack + for_each map ✅ |
| 同服务同规格 ECS×5 | 1 stack + count/list ✅ |
| 不同服务/不同团队的 ECS | 拆 stack（RBAC 驱动） |
| 不同环境（dev/staging/prod） | 拆 stack（环境隔离） |
| 一个 stack 内角色 > 10 或实例 > 50 | 考虑按角色族拆（state 太大） |

## 6. 完整示例：order-service 跨层依赖（含两部门并列 + 业务部门独立）

> **注**：以下路径示例用**出厂默认三层**（globals/middleware/application）的目录根名，仅为可读性。实际层根名由 `layer_rule_set_versions.layers_json[].path_template` 渲染，**层名不硬编码**（D24，§7 可配置），管理员可改名/增删层。

```
global/vpc-platform-default-prod/                    outputs: vpc_id, vswitch_ids[]       (平台运维)
global/ack-platform-default-prod/                    outputs: cluster_id                   (平台运维)
middleware/platform-default/rds-orders-prod/         outputs: rds_id, rds_conn             (DBA)
middleware/platform-default/kafka-platform-prod/     outputs: kafka_brokers                (中间件)
application/platform-default/team-a/ecs-prod/        读 vpc + ack outputs，部署 ECS×5     (业务A，无 space)
application/platform-default/team-a/rds-account-prod/读 rds_id，创建只读账号（DBA 审批）
application/platform-default/team-b/orders/ecs-prod/ 业务B 订单线 ECS（有 space，与业务A 互不可见）
application/platform-default/team-b/orders/kafka-topic/读 kafka_brokers，创建 Topic（中间件审批）
```
依赖图（平台元数据维护）：`ecs-prod depends-on vpc-global-prod, ack-global-prod`；`rds-account-prod depends-on rds-orders-prod`；`kafka-topic depends-on kafka-platform-prod`。编排时 Terramate 按目录执行，平台在工单层确保下层先成功。**业务A 与业务B 的 stack 互不可见**（RBAC + team_cloud_grants 双重隔离）。

## 7. Layer 规则模型（可配置，D24）

> 对应 D24、specs/04。层定义全部可配置，**不硬编码 globals/middleware/application**——当前三层只是"出厂默认"。

### 7.1 layers 表结构（管理员 Web 配置）

```yaml
# AI Generated Start
# 出厂默认（开箱即用）
layers:
  - name: global
    order: 1
    owning_team_pattern: "platform-ops"             # 单一部门
    path_template: "global/{{.component}}-{{.tenant}}-{{.env}}"
    depends_on: []                                   # 全球底层，无依赖
  - name: middleware
    order: 2
    owning_team_pattern: "dba|middleware"            # DBA + 中间件两部门并列同层
    path_template: "middleware/{{.tenant}}/{{.component}}-{{.env}}"
    depends_on: [global]
  - name: application
    order: 3
    owning_team_pattern: "business-*"                # 不同业务部门（通配）
    path_template: "application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}"
    depends_on: [global, middleware]
# AI Generated End
```
> **注**：`{{.tenant}}` 默认值为 `platform-default`。多租户场景如 `application/corp-a/team-a/orders/ecs-prod` 实现租户间目录与 state 天然隔离。旧 `globals/<env>` 或 tenant-first 模板不再作为新 stack 的输出目标；如需兼容存量路径，必须通过 import/迁移生命周期显式标记。

### 7.2 管理员可做什么

- **增删层**：加 `security`（合规层）/ `data`（DBA 专属）/ `messaging`（中间件专属，把 Middleware 拆两层）/ 合并 Global+Middleware 成 `shared` 一层
- **改路径模板**：换命名约定、加 custom_kv 变量（如 region/az/business_unit）
- **改归属团队模式**：owning_team_pattern 支持通配与正则
- **改依赖方向**：上层 depends_on 下层，单向无环

### 7.3 PathGenerator（codegen 调用）

```go
// AI Generated Start
// codegen MUST 调用 PathGenerator，MUST NOT 字符串拼接
path := PathGenerator(layer.PathTemplate, StackMetadata{
    Env: "prod", Tenant: "platform-default", Team: "team-a", Space: "orders",
    Component: "ecs", Layer: "application",
})
// → "application/platform-default/team-a/orders/ecs-prod"
// state key 与 path 一一对应
// AI Generated End
```

模板变量：`env / team / space / component / layer / layer_order / custom_kv / tenant`（**tenant 由 D27 引入，默认 platform-default**）。Go text/template 语法。

### 7.4 StackGranularity（stack 粒度策略）

| 策略 | 含义 | 用例 |
|------|------|------|
| `per-component`（默认） | 一个组件一个 stack | ECS 一个 / SLB 一个 / RDS 账号一个 |
| `per-space` | space 内合并 | 微服务全套（ECS+SLB+账号）合一个 stack |
| `per-team` | 业务部门内合并 | 小业务部门全部资源一个 stack |
| `custom` | catalog 项声明 stack_grouping 规则 | 复杂场景 |

粒度策略存 DB 表 `stack_grouping_rules`，catalog 项可声明 `stack_grouping` 覆盖默认。

### 7.5 层规则集版本化与迁移操作模型（D26）

> 回应：D24 让 layer 规则可配置，但生产环境运行期改 layer 规则（reorg / 加层 / 改 path_template）会动到**已部署 stack 的 path 与 state key**——直接 UPDATE 是生产事故。本节定义安全的版本化与迁移操作模型。

#### a) 版本化对象 = 整个分层方案（不是单 layer）

Global/Middleware/Application 是**一套协同的分层方案**，不是 3 个独立 entity。改任何层 = 整个 set bump v+1（v1→v2→v3...），旧版本 `status=superseded` 保留不可变。

| 反例（被否决） | 正例（采用） |
|---|---|
| 单 layer 版本化（改 Global 不动 Middleware）→ 版本碎片化 | 整个 layer_rule_set 版本化 |
| 字段级精细化（path_template 单独版本化）→ 同张表混合可变性 | 整个 set 一个原子版本单位 |

业界对照：K8s CRD（整个 CRD v1alpha1→v1beta1→v1）、Spacelift blueprint（整个 blueprint 版本化）都是整体版本化。

#### b) layer_logical_id = 层的逻辑身份（跨版本稳定）

每个层有 uuid 形式的 `layer_logical_id`，跨版本不变。引用规则：
- `catalog_items.layer_logical_id`（稳定）——catalog 项不随版本变动
- `stacks.layer_rule_set_version_id`（创建时 pin，不可变）——stack 创建时把 `layer_logical_id` + active 版本解析成具体 `path_template` 写入

layer 规则升 v2 时 catalog_items **零改动**，自然只影响新 stack。

#### c) Tier 分类 = per-stack，不是 per-change

同一个 v1→v2 变更，对不同 stack 影响不同——平台对每个 stack dry-run 重渲染 path 对比：

| 阶 | 判定（per-stack dry-run） | state_key | 平台动作 | admin |
|---|---|---|---|---|
| **Tier 1 Auto** | path 不变（改了 owning_team / depends_on / 未引用的 custom_kv） | 不变 | 自动 bump `stacks.layer_rule_set_version_id` | 仅审批 |
| **Tier 2 Assisted** | path 变可推导 | 变 | StateMover 在 Worker 跑 `terraform state mv` + 自动 plan=0 校验 + 失败自动回滚 + CMDB 同步 | 2 人审批 + 静默期 |
| **Tier 3 Forked** | path 冲突 / 删层 / 不可逆 | 不可推导 | 旧 stack 永久 pin vN，新 stack 走 vN+1，标 `deprecated_at` + `sunset_at`（+6mo） | 双轨期内 destroy+recreate |

**核心不变量**：**state_key 不经 admin 显式操作不能漂移**。平台宁可阻塞也不自动改 state_key。

#### d) StateMover 默认 Worker 执行（不是生成脚本给人跑）

terraform state pull/push 跨 backend 极易出错（push 错 key 就完蛋）。平台默认通过 Executor 在受控 Worker 跑：

```
# 平台为每个 Tier 2 stack 在 Worker 内执行（不是 admin 本地）：
1. terraform init（新规则渲染出的新目录，空 state）
2. terraform state pull（旧 backend key）→ push（新 backend key）
   或直接 S3/OSS copy（取决于 backend 类型）
3. terraform plan -detailed-exitcode（MUST exit 0 = zero-diff）
   └─ exit 2（有变更）→ 自动从 StateBackup 回滚 + 告警
4. 写 .lock 文件到旧 key 防误用
5. 更新 stacks.layer_rule_set_version_id + repo_path + state_key
6. 更新 CMDB cmdb_resources.stack_path
7. 触发 on-demand 漂移对账（再次 plan=0 验证）
```

保留「Manual Override」按钮给特殊场景（如 admin 想本地定制 mv 顺序），但默认零人工干预。

#### e) 8 步标准 Playbook（所有 Tier 共用）

```
1. 宣告 (Announce)
   admin UI 提交 layer_rule_set 草案 → 平台 dry-run 影响面分析
   输出：受影响 stack 清单 + per-stack Tier 分类 + 预估耗时 + v1↔v2 diff viewer

2. 静默 (Quiesce)
   平台对受影响 stack 进入 QuiesceMode（迁移批次粒度，非全局）
   新工单/plan/apply 阻塞，漂移检测自动静默该批
   已运行工单等其自然结束

3. 备份 (Backup)
   平台 StateBackup（S3 versioning / OSS snapshot）
   生成 rollback_token（含旧 version_id + 旧 path 映射）

4. 干跑 (Dry-Run)
   Tier 1: 平台模拟 bump + 重渲染 path 对比=不变 → 标可执行
   Tier 2: StateMover 在隔离 Worker 跑 state mv + plan 必须 exit 0
   Tier 3: 标 deprecated，不走迁移

5. 灰度 (Canary)
   选 1 个低风险 stack（如 dev 环境）实际跑迁移
   验证 plan=0、apply 不重建资源、CMDB 路径更新

6. 分批 (Batch)
   按 team 维度滚动迁移（爆炸半径=1 个 team）
   每 team 完成后观察 N 小时（漂移告警/成本曲线/CMDB 同步）

7. 验证 (Verify)
   所有迁移完 stack 跑 terraform plan → 全 zero-diff
   CMDB 与实际云资源对账（漂移检测跑一次）
   审计日志：哪些 stack vN→vN+1、谁操作、何时、rollback_token

8. 回滚窗口保留 (Rollback Window)
   旧 layer_rule_set_version 状态=archived 保留 30/90 天
   sunset 窗口内可一键 revert（含逆向 state mv）
   过期后才 GC 旧版本
```

#### f) 典型场景演练

| 场景 | Tier | 动作 | 耗时 |
|---|---|---|---|
| 加 security 第 4 层（不挤占已有路径） | 全 Tier 1 | v1→v2 自动 bump，已部署 stack 不动 | 分钟级 |
| space 从可选变必选（path_template 改） | 全 Tier 2 | 每 stack 走 state mv，按 team 灰度 | 天级 |
| DBA + Middleware 合并（仅改 owning_team_pattern） | 全 Tier 1 | 自动 bump，无 state 操作 | 分钟级 |
| DBA + Middleware 合并 + 改 path | Tier 2 | state mv | 天级 |
| 删 Global 层（合并到 Middleware） | Tier 3 | 旧 stack 永久 pin，新 stack 走新版本，逐步 destroy+recreate | 月级 |

#### g) 业界对照

| 系统 | 模板变更机制 | 关键设计 |
|---|---|---|
| **Spacelift** | blueprint 版本化 + drift detection + per-stack adoption | "preview changes" UI 必看 |
| **Argo CD** | ApplicationSet 改了只影响新 app | `templateGeneration` 字段冻结旧 app |
| **TFC** | workspace template 改不影响已部署 workspace | 显式 re-apply |
| **K8s Operator** | CRD v1alpha1→v1beta1→v1 + ConversionWebhook | "5 版本窗口" 强制迁移 |
| **Consul/Vault** | config entry versioned + CAS + canary | rolling update |
| **共识** | **preview 必备 / canary 必走 / rollback 必留 / 永不全量自动** | |

### 7.6 环境与租户 + 标签分层（D27/D28 衔接）

D27/D28 是 D24 PathGenerator 的下游消费者，对 §7 不破坏只扩展：

- **PathGenerator 新增 `${tenant}` 变量**（默认 `platform-default`），见 §7.3 模板变量列表；path_template 由管理员自由选用（不强制带 tenant 前缀）。
- **EnvironmentTenantBinding 是 codegen Stage 4 的查询入口**：见 docs/07 §2.3 + specs/17「EnvironmentTenantBinding 三元组」。
- **Tag 来源 7 层**：见 docs/08 §2 + specs/18「Tag 来源 7 层」。L4 team/space 层 tag 由本节团队归属驱动，L5 stack 层 tag 由 PathGenerator 输出的 stack_logical_id 派生。
- **9 阶段参数解析管道**：见 docs/08 §3 + specs/18。layer rule（S3）作为治理硬规则 rank 2，高于用户表单 rank 7。

**核心不变量**：D27/D28 不修改 §7.1-§7.5 任何表/字段定义；PathGenerator 输入仍只接收 layer.path_template + StackMetadata；新增的 tenant/env 变量是 StackMetadata 的扩展字段，PathGenerator 算法本身不变。

## 8. stack 划分 ↔ 模块零侵入的衔接（D24 + D25 协同）

> 关键：D24（stack 模型可配置）与 D25（模块零侵入）共享 **codegen 单一管道**，两个 Generator 是管道里的阶段。

### 8.1 codegen 管道（两 Generator 协同）

```
[9 阶段参数解析管道（详见 specs/18 / docs/08）]
  S1 模块契约(scalar) → S2 catalog defaults → S3 layer rule(治理)
  → S4 env 上下文(D27) → S5 tenant 上下文(D27) → S6 team 策略
  → S7 跨层依赖 → S8 用户表单 → S9 平台强制注入(tag/state key)
   ↓
[PathGenerator（D24）]   按 layer.path_template 渲染 stack 目录 + state key
   ↓
[CardinalityInjector（D25）]  按 catalog.cardinality 注入 for_each/count 调用语法
   ↓
[Render] Go text/template 渲染 .tf
   ↓
[terraform fmt] → [write to worktree] → [git commit]
   ↓
[Provenance] resolved_params_json 写入 requests（每变量 source/rank 审计）
```

### 8.2 元数据分工（catalog 项承载 D24 + D25 配置）

catalog 项同时承载两 Generator 的配置：

| 字段 | 给谁 | 作用 |
|------|------|------|
| `layer` | PathGenerator (D24) | 决定路径模板选哪个 layer |
| `stack_grouping` | PathGenerator (D24) | 决定 stack 粒度（per-component/per-space/...） |
| `cardinality` | CardinalityInjector (D25) | 决定调用语法（single/list/map） |
| `instance_key` | CardinalityInjector (D25) | map 模式的 key 含义（角色名/实例名） |
| `per_instance_fields` | CardinalityInjector (D25) | 进入 map value 的字段（每实例独立） |
| `shared_fields` | CardinalityInjector (D25) | stack 级共享字段（不进 map） |

**`modules` 表只存纯 scalar 契约**（从 variables.tf 提取），零侵入。

### 8.3 衔接契约

- **一个 stack** = PathGenerator 输出的一个目录 = 一个 state key = 一份 `main.tf`（含 0..N 个 module block）
- **每个 module block** 按 catalog 项的 cardinality 决定 single/count/for_each
- **stack 边界 = 逻辑治理单元**（§5.3），不是规格/实例数
- **跨 stack 引用** 走 `terraform_remote_state` / data source（§4）

### 8.4 业界对照

Spacelift（stack = 目录 + 内含 for_each 模块调用）/ Env0（environment = stack 概念）/ Terraform Cloud（workspace = stack）—— 都是"stack 目录 + 调用方 for_each + 模块零感知"的同构实现。本平台 D24+D25 与之同构，叠加"层可配置 + space 可选"的灵活性。
