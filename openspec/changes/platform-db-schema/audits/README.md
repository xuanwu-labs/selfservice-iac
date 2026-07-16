# 审计文档索引

本目录存放 platform-db-schema change 的全部审计报告。按时间顺序：

| 文件 | 审计维度 | 发现 | 状态 |
|---|---|---|---|
| `audit-postgresql-skill.md` | PG 表设计 skill 规范对账 | 5 P0 + 4 P1 | ✅ 全修复 |
| `audit-docs-sweep.md` | docs 全量通读（24 文件 5107 行）| 6 MVP 致命 + 4 非MVP 致命 + 11 丢失表 | ✅ 全修复 |
| `audit-link-completeness.md` | 跨表链路完整性（14 条业务链路）| 4 断裂 + 2 MVP 边界 | ✅ 全修复 |
| `audit-honest-assessment.md` | proto enum vs DB CHECK 对账 | 7 处枚举不一致 | ✅ 全修复 |
| `audit-apply-verification.md` | apply 后回查验证 | 三份审计问题逐项 grep 确认 | ✅ 全验证 |
| `proto-change-checklist.md` | proto message 缺字段清单 | 4 message 缺字段 | ✅ 已补全 |
| `audit-three-layer-alignment.md` | 三层架构（atomic/control/declarative）对齐 | 2 BLOCKER + 6 RISK | ✅ 全修复 |
| `audit-state-deps-sublayers.md` | 远程状态+依赖关系+子层级+初始化配置 | 2 RISK（Wave 2 补）| ✅ 确认设计合理 |

## 审计修复在 tasks.md 的标记

所有审计发现的修复都在 `tasks.md` 对应任务项标记 `[x]`：
- §10 验收清单的 10.8-10.31 对应各审计项的 grep 验证
- §10.1 标注 DDL 幂等测试通过（testcontainers PG，Up→Down→Up）

## 关键结论

- **MVP 核心链路无阻塞**：proto 契约 + DB 表结构 + design.md 文档三方一致
- **4 BLOCKER + 9 RISK 全修复**：proto enum 改值 + message 加字段 + DB CHECK 对齐 + 列补全
- **三层架构对齐**：modules.module_type（atomic/control/declarative）+ module_versions.outputs_contract_json
- **状态/依赖设计合理**：state JSON 在对象存储不进 DB；依赖存引用关系不存资源 ID（Spacelift 式最优解）
- **层级是平台级**：layer_rule_set_versions 无 tenant_id（业界标准）
- **DDL 执行验证通过**：testcontainers PG 跑 Up→Down→Up，20 表建完，3 层 seed 落地
- **非 MVP 扩展空间充足**：B1-B15 全量定稿（~68 表），D26/D27/D11 升级路径无阻碍
- **Wave 2 需补**：stack_outputs 缓存表 + module_compositions 组合关系表（增量演进，非架构调整）
