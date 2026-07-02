# IaC 平台顶级优化后续文档更新路线

## 1. 更新顺序

| 顺序 | 文档 | 目标 |
|------|------|------|
| 1 | `docs/00-工程契约.md` | 冻结 API、schema、状态机、错误、幂等、correlation id |
| 2 | `specs/19-API与Schema契约.md` | 把工程契约转成可验收需求 |
| 3 | `docs/12a-状态机测试矩阵.md` | 把 RequestLifecycle 转成测试用例 |
| 4 | `docs/06a-云账号Bootstrap手册.md` | 解决首账号、OIDC trust、execution role、team grant |
| 5 | `docs/09-代码生成机制.md` | 冻结 ResolvedParams、PathGeneratorOutput、PlanArtifact |
| 6 | `specs/20-VCS与PR工作流.md` | 补 PR-first 商业工作流 |
| 7 | `specs/21-RunHooks与策略扩展.md` | 补 Run Hooks 和外部策略扩展 |
| 8 | `docs/07`、`docs/13`、`docs/16` | 补 environment promotion、scheduled runs、run lifecycle gate |
| 9 | `docs/18`、`docs/19`、`specs/22` | 补平台产品运营和 scorecard |
| 10 | `docs/20`、`docs/21`、`docs/04` | 补企业证据链、runbook、数据库承载 |
| 11 | `design.md` | 明确保留差异化创新边界 |
| 12 | `tasks.md` | 把 Phase 0、商业级能力、演练和验收纳入路线 |

## 2. Phase 0 验收标准

进入大规模编码前必须满足：

- API contract 覆盖 request、catalog、approval、gate、webhook。
- RequestCreate、ResolvedParams、PathGeneratorOutput、PlanArtifact schema 冻结。
- 状态机测试矩阵覆盖主链路、异常链路、幂等、并发。
- 首云账号 bootstrap 可完成首个 plan/apply。
- 一个 golden catalog item 可跑通 walking skeleton。

## 3. Phase 1 验收标准

Phase 1 不是“功能多”，而是“主链路可用且可恢复”：

- 标准 catalog item 可自助申请。
- codegen 输出确定性文件。
- plan/apply 分离，plan artifact 可校验。
- pre-apply 审批有效。
- apply 后 reconcile 可成功或可追踪补偿。
- failed run reason 结构化。
- Run Health、Approval Health、Catalog Usage 三张基础看板可用。

## 4. Phase 2+ 验收标准

商业级能力进入 Phase 2+：

- PR-first 与 form-first 统一进入 RequestLifecycle。
- Run Hooks 可接入至少一个安全扫描器和一个 CMDB gate。
- Scheduled Runs 覆盖 drift-plan 与 stack-health-check。
- Environment Promotion 展示参数 diff、binding diff、审批升级。
- Catalog Scorecard 可计算并驱动推荐/治理。
- 合规证据包可按 request 导出。

## 5. 不做事项保持

- 不引入 Backstage/Port 作为主门户。
- 不默认开启 StateMover 自动化。
- 不把 FinOps 账单核销放入 Phase 1。
- 不让 AI/MCP/skills 绕过 RBAC、OPA、审批和审计。
