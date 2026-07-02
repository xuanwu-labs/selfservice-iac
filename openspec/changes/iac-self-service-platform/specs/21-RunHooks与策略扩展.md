# 21-Run Hooks 与策略扩展（run-hooks-policy-extension）

能力 ID：`run-hooks-policy-extension`

覆盖：对标 Terraform Cloud Run Tasks、Spacelift policy hooks 的通用扩展点。Run Hooks 用于接入 Checkov、tfsec、自研安全扫描、CMDB gate、变更窗口校验等能力。

## ADDED Requirements

### Requirement: Run Hook 阶段
平台 SHALL 支持以下 hook 阶段：`pre-plan`、`post-plan`、`pre-apply`、`post-apply`。每个 hook MUST 声明输入、超时、失败策略和是否阻断。

#### Scenario: post-plan 安全扫描
- **WHEN** plan artifact 生成后
- **THEN** 平台调用 `post-plan` hooks，把 plan JSON、resolved params、stack metadata 传给扫描器

### Requirement: Hook 结果模型
每次 hook 执行 SHALL 写入 `run_hook_results`，包含 `hook_id`、`request_id`、`phase`、`status`、`decision`、`summary`、`details_ref`、`started_at`、`finished_at`。

#### Scenario: hook 阻断 apply
- **WHEN** Checkov hook 发现高危公网暴露
- **THEN** `decision=deny`，request 进入 `blocked-policy`，审批人可看到 hook summary

### Requirement: 失败策略
hook SHALL 支持 `fail-open`、`fail-closed`、`warn-only` 三种失败策略。生产环境默认 `fail-closed`，非生产可配置。

#### Scenario: hook 服务不可用
- **WHEN** post-plan hook 超时且策略为 `fail-closed`
- **THEN** 平台阻断后续审批或 apply，返回 `run_hook_unavailable`

### Requirement: Hook 安全边界
hook 执行不得获取云执行凭据，除非显式声明 `requires_credentials=true` 并经平台管理员批准。默认只读取 plan JSON、metadata、artifact refs。

#### Scenario: 默认无凭据
- **WHEN** 第三方扫描 hook 运行
- **THEN** 它只能读取 plan 与 metadata，不得 assume cloud role

### Requirement: OPA 与 Run Hooks 分工
OPA 用于平台内建策略和强约束；Run Hooks 用于外部工具和团队自定义检查。两者结果都进入统一 policy summary。

#### Scenario: 多策略汇总
- **WHEN** OPA allow、Checkov warn、CMDB gate deny
- **THEN** 平台 policy summary 展示三者结果，并按最严格决策阻断 apply
