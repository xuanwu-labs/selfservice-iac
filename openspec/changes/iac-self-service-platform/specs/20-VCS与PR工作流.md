# 20-VCS 与 PR 工作流（vcs-pr-workflow）

能力 ID：`vcs-pr-workflow`

覆盖：对标 Atlantis / Spacelift / Terraform Cloud 的 GitOps/PR-first 工作流。该能力是 Phase 2+，与 form-first 自助入口并存，不进入 Phase 1 主链路。

## ADDED Requirements

### Requirement: 双入口模型
平台 SHALL 同时支持 form-first 与 PR-first 两种入口。form-first 由平台生成代码；PR-first 由工程师或自动化直接提交 HCL 变更，平台负责 plan、policy、approval、apply 和审计。

#### Scenario: Web 表单入口
- **WHEN** 用户通过服务目录提交申请
- **THEN** 平台生成代码并创建 request，进入标准流水线

#### Scenario: PR 入口
- **WHEN** 工程师对工作仓库提交 PR 修改 stack 代码
- **THEN** 平台识别受影响 stack，触发 speculative plan，并把 plan summary 回写 PR

### Requirement: PR Plan Summary
平台 SHALL 在 PR/MR 中评论 plan summary，包含受影响 stack、资源变更数、风险等级、Infracost 估算、OPA/RunHook 结果和 next action。

#### Scenario: PR 自动 plan
- **WHEN** PR 修改 `application/platform-default/team-a/ecs-prod`
- **THEN** 平台运行 `terramate list --changed` 定位 stack，执行 plan，并在 PR 评论 summary

### Requirement: Apply Requirements
平台 SHALL 支持 apply requirements：branch protection 通过、PR approval 满足、OPA allow、RunHooks allow、平台审批通过、plan artifact 未过期。

#### Scenario: apply 被 branch protection 阻断
- **WHEN** PR plan 已通过但代码 review 未满足
- **THEN** 平台拒绝 apply，返回 `apply_requirement_not_met`

### Requirement: PR Comment Commands
平台 MAY 支持受控评论命令，如 `tm plan`、`tm apply`、`tm unlock`。评论命令必须按 actor 的 RBAC 校验，并写审计。

#### Scenario: 评论触发 apply
- **WHEN** 有权限审批人在 PR 评论 `tm apply`
- **THEN** 平台校验 apply requirements，若通过则使用已审 plan artifact 执行 apply

### Requirement: Form 与 PR 的审计统一
PR-first 与 form-first MUST 最终进入同一 RequestLifecycle，状态、artifacts、approval_runs、audit_logs 一致。

#### Scenario: 审计统一
- **WHEN** 审计员查询某次 apply
- **THEN** 不论来源是 Web 表单还是 PR，均能看到 actor、commit、plan artifact、approval、apply run 和 reconcile 结果
