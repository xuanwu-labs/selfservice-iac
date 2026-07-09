# 19-服务目录与 Scorecard

> 服务目录不是资源清单，而是平台产品的销售货架。商业级平台必须知道哪些 catalog item 健康、可推荐、需治理或应下架。

## 1. CatalogItem 生命周期

| 状态 | 含义 | 行为 |
|------|------|------|
| `draft` | 草稿 | 不对用户可见 |
| `active` | 可申请 | 正常展示 |
| `deprecated` | 禁新申请 | 存量可用，提示迁移 |
| `archived` | 下架 | 不允许引用 |
| `blocked` | 临时阻断 | 安全/合规/工具链问题，待恢复 |

## 2. Catalog Health

| 维度 | 指标 |
|------|------|
| Usage | 近 30 天申请数、成功数、团队覆盖 |
| Reliability | plan 成功率、apply 成功率、失败原因 TopN |
| Governance | 必填 tag 完整率、OPA 通过率、审批退回率 |
| Cost | 预算越限次数、估算与实际偏差 |
| Version | 模块版本漂移、provider lock 漂移 |
| Ownership | owner 是否有效、SLA 是否配置 |

## 3. Scorecard

| 分数 | 含义 |
|------|------|
| 90-100 | Golden catalog item，可推荐为默认入口 |
| 75-89 | 可用但有优化项 |
| 60-74 | 需治理，限制推广 |
| < 60 | 阻断或下架候选 |

计算建议：

| 维度 | 权重 |
|------|------|
| Reliability | 30% |
| Governance | 20% |
| Usage | 15% |
| Cost | 15% |
| Version Freshness | 10% |
| Ownership | 10% |

### 3.1 Scorecard 可信度

Scorecard 必须同时展示 `score` 和 `confidence`。早期平台数据不足时，低可信度分数只能用于观察，不能自动下架或阻断 catalog item。

| 可信度 | 条件 | 可触发动作 |
|--------|------|------------|
| `low` | 样本 < 10 次 run，或缺少 cost/version/ownership 任一关键维度 | 只提示 owner 复核 |
| `medium` | 样本 10-30 次 run，关键维度完整但时间窗口不足 30 天 | 可降低推荐等级，不自动阻断 |
| `high` | 样本 ≥ 30 次 run，近 30 天数据完整，owner/SLA 有效 | 可触发治理任务、降级推广或下架流程 |

数据完整度字段建议：

| 字段 | 说明 |
|------|------|
| `data_completeness` | 0-100，表示 usage/reliability/governance/cost/version/ownership 六维数据覆盖率 |
| `confidence` | `low` / `medium` / `high` |
| `sample_size` | 评分窗口内 run 数 |
| `missing_dimensions` | 缺失维度列表 |

运营规则：`score < 60` 且 `confidence=high` 才能进入阻断或下架候选；否则只能进入 owner 复核。

## 4. 服务成熟度

| Level | 标准 |
|-------|------|
| L0 | 有模块但无 catalog |
| L1 | 可通过 catalog 申请 |
| L2 | 有默认值、tag、审批和成本估算 |
| L3 | 有 drift、run hooks、scorecard 和 owner SLA |
| L4 | 支持 promotion、PR-first、自动健康修复建议 |

## 5. 用户黄金路径

每个 active catalog item 必须回答：

- 谁可以申请。
- 需要填什么。
- 默认会创建什么。
- 审批人是谁。
- 预计多久完成。
- 失败后找谁。
- 成本归属哪里。

## 6. 运营动作

| 条件 | 动作 |
|------|------|
| 30 天无人使用 | owner 复核是否下架 |
| apply 成功率低于 90% | 降级推广，创建治理任务 |
| owner 无效 | 自动分配给平台运营处理 |
| 模块版本落后超过 3 个小版本 | 生成升级建议 |
| 连续 policy_blocked | 阻断申请并要求修复 catalog defaults |
