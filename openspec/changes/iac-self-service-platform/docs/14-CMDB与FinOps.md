# 14-CMDB与FinOps

> 对应 design `D18`、spec `specs/15-CMDB与FinOps`。回应：CMDB 与 FinOps 界面如何设计、表结构、整个流程如何串通。

## 1. 定位：CMDB 是索引，FinOps 是成本视角

CMDB 与 FinOps 是本平台内建能力，不是外部系统适配层。平台可以独立维护资源索引、成本记录、预算与优化建议；若企业已有外部 CMDB、财务系统或成本平台，后续只作为数据同步/导出/对账对象，不改变本平台内部事实模型。

```
state（执行面真相源）  ──ingest──>  CMDB（可查询索引）  ──tag归集──>  FinOps（成本视角）
       │                              │                              │
       └─ 敏感/全量                    └─ 元数据/关系/归属            └─ 预算/优化/核销
```

核心原则：**CMDB 不复制 state**。state 是执行面真相源（在工作仓库 + 远程 backend），CMDB 只存可查询的资源元数据（type / address / cloud_id / tags / 归属 / 估算成本），避免双写不一致。FinOps 基于 CMDB + 云账单做成本归集与治理。

### 1.1 一致性边界

| 场景 | 一致性 | 处理 |
|------|--------|------|
| apply 写 Terraform state | 强一致，由 Terraform backend + state lock 保证 | 失败则 apply 失败 |
| state → CMDB ingester | 最终一致 | ingester 失败进入 `reconcile-pending`，不回滚资源 |
| CMDB → FinOps 成本视图 | 最终一致 | 账单/估算延迟只影响看板，不阻断 apply |
| cloud inventory → orphan 标记 | 最终一致 | 标记为待确认，不自动释放 |

apply 成功但 CMDB/FinOps/通知失败时，工单进入 `reconciling` 后可落到 `reconcile-pending`；平台通过 `outbox_events` 和 `reconcile_jobs` 重放。只有 Terraform apply 本身失败才影响资源变更结果。

## 2. 表结构（补充 docs/04）

```
resources(id, stack_id, bundle_id, team_id, layer, address, type,
          cloud_provider, region, cloud_resource_id, name,
          tags_json, attributes_json, monthly_cost_estimate_cents, currency,
          first_seen_at, last_synced_at,
          status[active|drifted|orphan|destroyed], managed bool)
resource_relations(id, source_resource_id, target_resource_id, relation_type)
cost_records(id, period_month, team_id, bundle_id, stack_id, resource_id nullable,
             cloud_provider, service_code, amount_cents, currency,
             cost_source[bill|estimate], tags_json, recorded_at)
cost_budgets(id, scope_type[team|bundle|stack|layer], scope_id,
             period_month, budget_cents, alert_thresholds_json,
             alert_status[ok|warning|exceeded])
finops_recommendations(id, kind[rightsize|release_orphan|reserved_instance|tag_missing],
                       resource_id, detail_json, estimated_saving_cents,
                       status[open|dismissed|applied], created_at)
```
> **权威定义见 docs/04 §2.11**（含 `resources.managed` / `cost_records.cost_source` / `cost_budgets.alert_thresholds_json` 等字段完整说明）。`cloud_accounts` 表本身归 D23，定义在 docs/04 §2.12 / docs/06 §4，本表不再重复。

关键设计：
- `resources.managed`：区分平台管理（true）vs 外部 / 孤儿（false）。
- `cost_records.resource_id nullable`：未打 tag 的成本按 tag 尽力匹配，匹配不到记 `unallocated`。
- `cost_budgets.alert_thresholds_json`：多级阈值（如 50% / 80% / 100%）。
- `resources` 与 `stacks`（docs/04 §2.3）通过 `stack_id` 关联，不重复归属信息。

## 3. 强制 tag 策略（成本归集的锚点，7 层 tag 来源详见 docs/08 §2）

平台给所有生成的云资源强制打：
```
platform-team    = <team_id>           # L4 team/bundle 层
platform-bundle  = <bundle_id>         # L4 team/bundle 层
platform-stack   = <stack_id>          # L5 stack 层（codegen 自动派生）
platform-managed = true                # L1 platform-mandated 层（绝对，不可覆盖）
```

**Tag 来源 7 层模型（D28 权威见 docs/08 §2 + docs/04 §2.14）**：
- L1 platform-mandated（永远赢，用户尝试覆盖 silently ignore + 审计）
- L2 env（`environments.tag_namespace_json`，注入 env/cost-center）
- L3 tenant（`tenants.tag_namespace_json`，注入 tenant/compliance）
- L4 team/bundle（`teams.tags_json` + `bundles.tags_json`）
- L5 stack（codegen 自动派生 platform-stack）
- L6 catalog defaults（`catalog_items.default_tags_json`）
- L7 user form（受 `catalog_items.user_allowed_tag_keys_json` 白名单约束）

- **catalog 注册校验**：模块输出必须含 L1 四 tag（platform-managed/team/bundle/stack）或声明由平台注入。
- **codegen S9 自动注入**：codegen 在 S9 阶段合并 7 层 tag 后注入 module 调用层，L1 永远覆盖用户输入防绕过。
- **ingester 校验**：apply 后从 state 读 tag，**校验 7 层合并后的预期 tag 与实际 tag 一致**，缺失则告警（区分 L1 缺失=critical / L2-L6 缺失=warning）。
- **resolved_params_json.tags**：每次 codegen 写入 tags 完整层级审计（layers_applied / user_overrides_blocked / layers_merged_summary），便于追溯。
- tag 是多云成本归集 + 孤儿检测 + 资源归属的事实手段（借鉴云厂商 Tag 策略 + Spot.io / Flexera + AWS Tag Policies）。

## 4. 流程串通（端到端）

### 4.1 预算校验时机（精确化）

| 时机 | 动作 | 触发升级条件 |
|------|------|------------|
| **申请提交时** | Infracost 实时预估月成本，对比 `cost_budgets.alert_thresholds_json` | 预估超 50% → 黄色提示用户；超 80% → 警告；超 100% → 阻断提交或强制确认 |
| **准入审批 Gate1** | 平台二次校验（用户可能改了规格再提交） | 超 100% → 审批流自动插入"成本审批节点"（路由到团队负责人/财务，DSL conditional） |
| **执行确认 Gate2** | 拉取最新 Infracost 估算（plan 后）+ 最近 3 月 cost_records 趋势 | 若突增 > 50%（vs 历史均值）→ 高亮警示，要求审批人显式确认 |
| **apply 后核销** | 月末云账单同步，实际成本 vs 预估对比写 cost_records | 实际超预算 → 告警通知团队 owner + 看板标红 |

升级到"成本审批节点"的具体路由：DSL `when: cost.over_budget` 触发，节点绑 `role: cost-approver`（通常是 team lead 或 FinOps 角色），由 RBAC role_bindings 解析实际人。

### 4.2 双成本源时间对齐策略（精确化）

| 维度 | Infracost（estimate）| 云账单（bill）|
|------|---------------------|--------------|
| **延迟** | 实时（plan 时算）| T+1 日级 / 月级 |
| **精度** | 不含折扣/RI/促销 | 精确（含所有调整） |
| **用途** | 决策（申请时预估、Gate1/Gate2 校验、预算预警）| 核销（月末实际成本、对账、预算达成率）|
| **预算判定** | **优先用 estimate 实时触发告警/升级**（避免账单延迟漏报）| **月末用 bill 核销 + 校正阈值**（如 estimate 持续高估，下调告警阈值）|
| **去重** | 同一 period_month + resource_id 同时有 estimate + bill 时，**近期查询默认显示 estimate**（最新），**月报显示 bill**（权威）；两者都保留，不互删 |
| **趋势判断** | Gate2 展示"最近 3 月 bill 趋势 + 本次 estimate"，让审批人看是否突变 | — |

### 4.3 端到端数据流

```
[申请填表]
   │ Infracost 实时预估 → 展示用户 + 审批人
   │ §4.1 预算校验：超预算 → 审批升级（插入成本审批节点）
   ▼
[审批 → apply 成功]
   │
   ▼
[resource ingester] 读 state JSON → upsert resources 表（失败进入 reconcile-pending，不回滚 apply）
   │
   ├──> CMDB 资源拓扑可查询
   │
   ▼
[漂移检测周期跑]
   │ 云侧资源清单 vs CMDB → 孤儿资源标记 orphan（resources.managed=false，需人工确认）
   │
   ▼
[云账单每日同步 job]
   │ 按 tag 归集 → cost_records（cost_source=bill）
   │
   ├──> FinOps 看板（团队成本趋势 / 预算燃尽 / Top资源 / 孤儿）
   │
   ▼
[优化建议引擎] Infracost 成本差 + MetricsProvider 利用率 → finops_recommendations（带 confidence_score）
   │
   ▼
[一键转申请]（降配 / 释放走正常审批流）
```

### 4.4 Ingester 补偿流程

| 步骤 | 行为 |
|------|------|
| 1 | apply 成功后写 `outbox_events(event_type=cmdb.ingest.requested)` |
| 2 | ingester 读取 state JSON，脱敏后 upsert `resources` |
| 3 | 成功则 request 从 `reconciling` 进入 `succeeded` |
| 4 | 失败重试 3 次，仍失败则 outbox 入 dead-letter，request 进入 `reconcile-pending` |
| 5 | reconcile job 后续从 state 重建 resources；仍失败则生成 `manual_intervention_tasks` |

## 4.5 利用率数据源（MetricsProvider 接口）

优化建议（rightsize）需要真实利用率数据，但**利用率采集是监控平台的职责，不是 IaC 平台的**。平台通过可插拔 `MetricsProvider` 接口对接外部监控系统，不自己采集指标。

### 职责边界

```
Aether（IaC 平台）知道：               监控平台知道：
  ✅ 创建了什么资源（state）              ✅ CPU/内存/IO 利用率曲线
  ✅ 资源归属（tag）                      ✅ 峰值/P95/均值
  ✅ 资源规格和预估成本（Infracost）       ✅ 业务峰值时间窗口
  ✅ 实际账单总数（云账单）               ✅ Kafka topic 流量、MySQL 连接数

Aether 不做：                            监控平台不做：
  ❌ 自己采集 CPU/内存                    ❌ 知道资源归属哪个团队
  ❌ 自己算 Pod 级成本                    ❌ 知道资源是否该降配
  ❌ 自己存指标时序数据                   ❌ 审批降配建议
```

### MetricsProvider 接口

```go
// MetricsProvider 查询资源利用率（对接外部监控系统）。
// 实现可插拔：云监控 / OpenCost / Prometheus，按配置选择。
type MetricsProvider interface {
    // GetUtilization 拉取资源最近 N 天的利用率指标摘要。
    GetUtilization(ctx context.Context, resourceID, metric string, days int) (*UtilizationSummary, error)
}

// UtilizationSummary 利用率摘要，供优化建议引擎决策。
type UtilizationSummary struct {
    AvgPercent  float64 // 平均利用率
    PeakPercent float64 // 峰值利用率
    P95Percent  float64 // P95（更能反映真实需求）
    Samples     int     // 采样点数（数据可信度：< 7 天 → low，> 30 天 → high）
    Source      string  // 数据来源标识（cloudwatch / opencost / prometheus）
}
```

### 实现矩阵

| 实现 | 覆盖资源 | 数据来源 | 适用场景 |
|------|----------|----------|----------|
| `CloudWatchMetricsProvider` | AWS ECS/RDS/ELB 等 | AWS CloudWatch GetMetricStatistics API | AWS 云主机/数据库 |
| `AliyunMetricsProvider` | 阿里云 ECS/RDS/SLB 等 | 阿里云云监控 DescribeMetricList API | 阿里云资源 |
| `OpenCostMetricsProvider` | K8s Pod/Node/Namespace | OpenCost API（CNCF，K8s 原生成本分摊） | K8s 容器场景 |
| `PrometheusMetricsProvider` | 任意（按自定义查询） | Prometheus Query API（PromQL） | 已有 Prometheus 的用户 |

### OpenCost 定位

OpenCost（CNCF 项目）解决 K8s 场景的**Pod 级成本分摊 + 利用率**——这是云账单和 Infracost 都做不到的：

```
K8s 集群（5台 ECS，月费 $2000）
  └── 100 个 Pod
       ├── team-a 的订单服务：应分摊多少？
       ├── team-b 的日志服务：应分摊多少？
       └── OpenCost 按 CPU/memory request + 实际用量分摊
```

- **不强制依赖**：没有 K8s 的用户不需要 OpenCost，MetricsProvider 可空。
- **Phase 2 可选**：Phase 1 只实现 CloudWatch / 阿里云监控适配器；OpenCost 等用户有 K8s 场景时再加。

### 资源类型与利用率获取方式

| 资源类型 | 利用率指标 | 获取方式 | 分摊可行性 |
|----------|-----------|----------|-----------|
| ECS（云主机） | CPU / 内存 / 网络 IO | 云监控 API | ✅ 直接（1:1 对应账单） |
| RDS（数据库） | 连接数 / IOPS / 存储 | 云监控 API | ✅ 直接 |
| ELB/SLB（负载均衡） | 活跃连接数 / 流量 | 云监控 API | ✅ 直接 |
| Kafka（托管） | 分区数 / 消息吞吐 | 云监控 API | ✅ 实例级；❌ topic 级需 Kafka metrics |
| Kafka（自建） | 无独立账单 | Prometheus + JMX exporter | ⚠️ 需用户自配监控，平台只读指标 |
| K8s Pod | CPU / 内存 request vs actual | OpenCost | ✅ Pod 级分摊 |
| Redis（托管） | 内存使用率 / 连接数 / 命中率 | 云监控 API | ✅ 直接 |
| S3 / OSS（对象存储） | 存储量 / 请求次数 | 云监控 API / 账单 | ✅ 按 bucket tag |

> **关键原则**：平台只**读**利用率指标（GetUtilization），不**写**（不安装 agent、不部署 exporter）。用户负责自己的监控基础设施，平台负责对接和决策。

### 优化建议引擎 + 置信度

rightsize 建议生成时附带 `confidence_score`，让审批人判断建议可信度：

```go
// 优化建议引擎综合三个维度产出建议：
func (e *OptimizationEngine) Analyze(
    resource *Resource,          // CMDB 资源（规格/类型/归属）
    utilization *UtilizationSummary, // MetricsProvider 返回
    costDelta *CostDelta,       // Infracost 当前 vs 建议规格成本差
) *Recommendation {
    // 置信度规则：
    //   数据天数 < 7 → low（可能偶发低负载，不建议降配）
    //   数据天数 7-30 → medium
    //   数据天数 > 30 → high
    //   P95 < 10% + 数据 > 30天 → high + "长期低负载，强烈建议降配"
    //   P95 > 60% → 不建议降配（峰值接近瓶颈）
}
```

**审批人看到的信息**：
```
建议：将 ecs.g6.large（2核8G）降配为 ecs.g6.medium（1核4G）
依据：最近 30 天 CPU 平均 3.2%，P95 8.1%，峰值 12%
预估节省：$42/月
置信度：高（30天数据，P95 < 10%）
数据来源：CloudWatch（可信）
```



| 源 | 用途 | 时效 | 精度 |
|----|------|------|------|
| Infracost | 申请时预估 + 未出账估算 | 实时 | 估算（基于公开价目） |
| 云账单 API | 实际核销、预算判定 | 滞后（日 / 月） | 精确（含折扣 / RI / 促销） |

二者互补：**决策用 Infracost，核销用账单**。`cost_records.cost_source` 区分。Infracost 估算值也会写入 `resources.monthly_cost_estimate_cents` 便于 CMDB 直接展示。

## 6. 界面设计

### CMDB 视图
- **资源拓扑图**：bundle → stack → 资源 → 依赖关系（可下钻到资源详情）
- **筛选**：按团队 / 层 / 资源类型 / 云 / 区域 / 状态（active / drifted / orphan）
- **资源详情**：归属、tags、state 关键属性（脱敏）、漂移历史、成本趋势

### FinOps 视图
- **团队成本趋势**：月度成本曲线 + 预算线（燃尽图）
- **Top 资源**：当月消耗 Top N 资源
- **预算看板**：各团队 / 项目组预算达成率，超限标红
- **孤儿资源**：可释放资源清单 + 持续成本估算 + 一键处置
- **优化建议**：可降配 / 可购 RI / 缺 tag，一键转申请

### 申请时嵌入
- 表单右侧实时显示 Infracost 月成本预估
- 预算超限提示「将超 team-a 月度预算 ¥X，需额外成本审批」

## 7. 多云适配

- `cloud_accounts` 表支持多 provider（AWS / 阿里云 / Azure / ...）。
- 账单拉取适配器：AWS CUR / 阿里云账单 API / Azure Cost Management。
- tag key 各云可能受限（如阿里云 tag 值长度），ingester 做归一化映射。
- 成本币种统一折算（配置基准币种 + 汇率源）。

## 8. 与现有设计的关系

- **specs/07 状态漂移**：漂移引擎同时驱动孤儿资源检测（云侧 vs CMDB）。
- **specs/06 编排引擎**：apply 成功是 CMDB ingester 的触发点。
- **specs/10 审批引擎**：预算超限作为条件分支插入成本审批节点。
- **specs/02 服务目录**：catalog 注册校验含平台 tag。
- **docs/04 数据库**：本节表是 docs/04 的扩展，归属同一元数据库，迁移一并落地。

## 9. 选型

| 用途 | 选型 | 说明 |
|------|------|------|
| 成本预估 | [Infracost](https://github.com/infracost/infracost) | 开源，多云价目 |
| state 解析 | [terraform-json](https://github.com/hashicorp/terraform-json) + 自实现 walker | 从 state 提取资源 |
| 云账单 | 各云 SDK / CUR / 账单 API | 适配器化 |
| 资源依赖（可选） | 内置 `resource_relations` 表起步，规模大可引入 Neo4j / RedisGraph | 先关系表后图 |

## 10. 关键链接

- [Infracost](https://github.com/infracost/infracost) · [terraform-json](https://github.com/hashicorp/terraform-json)
- [AWS CUR](https://docs.aws.amazon.com/cur/latest/userguide/what-is-cur.html) · [阿里云账单 API](https://help.aliyun.com/document_detail/100420.html) · [Azure Cost Management](https://learn.microsoft.com/azure/cost-management-billing/)
