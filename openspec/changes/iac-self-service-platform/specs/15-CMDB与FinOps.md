# 17-CMDB与FinOps（cmdb-finops）

能力 ID：`cmdb-finops`
覆盖：资源实例索引（CMDB）、强制 tag 策略、成本归集与预算治理（FinOps）、孤儿资源检测、成本优化建议、申请时预算校验联动审批。

## ADDED Requirements

### Requirement: CMDB 资源实例索引
平台 SHALL 维护资源实例索引（CMDB）：apply 成功后 ingester 从 state JSON 解析资源 upsert `resources` 表，关联 stack / bundle / team / layer；CMDB MUST NOT 复制 state 全文，只存可查询元数据（type / address / cloud_id / tags / 估算成本）。

#### Scenario: apply 后资源入库
- **WHEN** 工单 apply 成功
- **THEN** 平台 ingester 读最新 state，解析资源 upsert CMDB，关联归属团队与 stack，记录 first_seen / last_synced

#### Scenario: CMDB 查询资源拓扑
- **WHEN** 用户查看 bundle `team-a/orders` 的资源
- **THEN** 平台返回该 bundle 下所有 stack 的资源清单含依赖关系，不暴露 state 敏感字段

#### Scenario: layer 规则迁移后 CMDB 路径同步（D26 衔接）
- **WHEN** layer_rule_set 升 v2 触发 Tier 2 state mv，某 stack 的 `repo_path` / `state_key` 从 `application/platform-default/team-a/ecs-prod` 变为 `application/platform-default/team-a/orders/ecs-prod`
- **THEN** StateMover 在 Worker 完成 state mv 且 plan=0 后，平台 MUST 自动更新 CMDB `cmdb_resources.stack_path`（及关联索引），并立即触发一次 on-demand 漂移对账，验证 CMDB ↔ state ↔ 云资源三方一致；MUST NOT 留下指向旧 path 的孤儿 CMDB 记录

### Requirement: 强制 tag 策略
平台 SHALL 给所有生成的云资源强制打标签（`platform-team` / `platform-bundle` / `platform-stack` / `platform-managed`），作为成本归集与孤儿资源检测的锚点；catalog 注册时 MUST 校验模块输出含这组 tag（或由平台 codegen 自动注入）。

#### Scenario: 无 tag 模块注册被拒
- **WHEN** 注册的模块未输出平台 tag 且未声明由平台注入
- **THEN** 注册校验失败，提示需补 tag 或开启平台注入

#### Scenario: tag 驱动成本归集
- **WHEN** 云账单同步按 tag 归集
- **THEN** 资源成本精确归到 team / bundle / stack，未匹配的资源记 `unallocated`

### Requirement: FinOps 成本归集（双源）
平台 SHALL 用双成本源：Infracost 预估（申请时 + 未出账估算）+ 云账单 API（实际，按 tag 归集），统一存 `cost_records` 并用 `cost_source` 区分；MUST 支持按团队 / 项目组 / stack / 层 / 资源类型多维聚合。

#### Scenario: 云账单按团队归集
- **WHEN** 月度账单同步
- **THEN** 每条成本记录按 tag 归到 team / bundle / stack，可向上聚合到层与团队

#### Scenario: 申请时成本预估
- **WHEN** 用户填表申请 RDS
- **THEN** 平台调 Infracost 实时预估月成本，展示给用户与审批人

### Requirement: 预算治理与告警
平台 SHALL 支持按 team / bundle / stack / layer 设月度预算（`cost_budgets`），多级阈值告警；申请时预估超预算 MUST 触发审批升级（路由到成本审批节点），实际成本超限 MUST 通知团队负责人。

#### Scenario: 申请超预算升级审批
- **WHEN** 申请预估成本使 bundle 月度超预算阈值
- **THEN** 审批流自动插入成本审批节点（如平台运维 / 财务），未超则按正常流走

#### Scenario: 预算超限告警
- **WHEN** 某团队当月实际成本超预算阈值
- **THEN** 平台告警通知团队负责人，FinOps 看板标红并展示超额金额

### Requirement: 孤儿资源检测
平台 SHALL 通过漂移引擎 + 云侧资源清单对比 CMDB，识别云上存在但 CMDB 无记录的孤儿资源；孤儿资源 MUST 在 FinOps 看板高亮并估算持续成本，提示处置（纳管 / 释放）。

#### Scenario: 发现孤儿 ECS
- **WHEN** 云侧存在 ECS 实例但不在 CMDB
- **THEN** 平台标 orphan，估算其月度持续成本，提示团队处置

### Requirement: 成本优化建议
平台 SHALL 结合 Infracost + 资源利用率产出优化建议（rightsize / release_orphan / reserved_instance / tag_missing），建议 MUST 可一键转申请单（降配 / 释放走正常审批流）。

#### Scenario: 降配建议转申请
- **WHEN** 平台发现某 RDS 长期低利用率，建议降配
- **THEN** 用户可在 FinOps 看板一键生成降配申请单，走审批执行并回写建议状态

#### Scenario: 缺 tag 资源补全建议
- **WHEN** 云账单中存在未匹配到 team 的成本（缺 tag）
- **THEN** 平台产出 `tag_missing` 建议，提示补 tag 以正确归集
