## ADDED Requirements

### Requirement: 模块注册引擎（Git clone + HCL 解析 + 状态机）

平台 MUST 在 `server/core/registry/` 实现模块注册引擎。RegistryService 注入 ModuleRepo（W1-02）+ GitProvider（W1-01），执行注册流程：clone Git 仓库 → 用 `terraform-config-inspect`（tfconfig）解析 variables.tf/outputs.tf → 提取纯 scalar 契约（D25 零侵入）→ ModuleRepo.CreateWithVersion 事务落 DB → 状态机转换。

#### Scenario: 注册一个 Git 模块并提取契约

- **WHEN** 调用 RegisterModule(git_source="git@...:tf-modules.git", module_path="//atomic/rds-mysql", version="v1.0.0", owner_team_id=1)
- **THEN** GitProvider.Clone 把仓库拉到临时目录
- **AND** ContractExtractor 用 tfconfig.LoadDir 解析 module_path 子目录
- **AND** 提取 variables_contract_json（纯 scalar：name/type/default/description/sensitive）
- **AND** ModuleRepo.CreateWithVersion 事务原子写入 modules + module_versions
- **AND** 返回 module_id + version_id

#### Scenario: 模块状态机 pending_validation → validated

- **WHEN** 模块注册时 HCL 解析成功
- **THEN** modules.status 从 `pending_validation` 转为 `validated`（UpdateStatus）
- **AND** module_versions.variables_contract_json 填入提取的契约

#### Scenario: 模块状态机 pending_validation → validation_failed

- **WHEN** 模块注册时 HCL 解析失败（如 variables.tf 语法错误）
- **THEN** modules.status 转为 `validation_failed`
- **AND** 错误信息记录到 modules.description 或返回给调用方

### Requirement: ContractExtractor 输出纯 scalar 契约（D25 零侵入）

平台 MUST 用 `terraform-config-inspect/tfconfig` 解析 `.tf` 文件，提取的 `variables_contract_json` MUST 只包含纯 scalar 信息（name/type/default/description/sensitive/required）。复杂类型（list/map/object）的 default 值 MUST 置 nil（但保留 type 声明）。这是 D25 零侵入原则的代码体现——原子模块保持单实例语义，社区模块直接复用。

#### Scenario: scalar 变量完整提取

- **WHEN** variables.tf 含 `variable "instance_type" { type = string default = "rds.mysql.s2.large" }`
- **THEN** 契约含 `{name:"instance_type", type:"string", default:"rds.mysql.s2.large", required:false}`

#### Scenario: complex default 置 nil

- **WHEN** variables.tf 含 `variable "tags" { type = map(string) default = {env="prod"} }`
- **THEN** 契约含 `{name:"tags", type:"map(string)", default:null, required:false}`（default 置 nil，type 保留）

#### Scenario: sensitive 标记保留

- **WHEN** variables.tf 含 `variable "password" { type = string sensitive = true }`
- **THEN** 契约含 `{name:"password", type:"string", sensitive:true, required:true}`（无 default）

### Requirement: GitProvider go-git 真实实现

平台 MUST 用 `github.com/go-git/go-git/v5`（已在 go.mod）实现 GitProvider 接口（W1-01 定义的 Clone/Fetch/CommitSHA）。MVP 凭据用环境变量（GIT_SSH_COMMAND / GIT_TOKEN），D23 完整凭据注入（Vault/KMS）留 W2。

#### Scenario: clone SSH 仓库到临时目录

- **WHEN** 调用 Clone(git_source="git@github.com:org/repo.git", ref="main")
- **THEN** go-git 用环境变量 GIT_SSH_COMMAND 的 SSH key clone 到临时目录
- **AND** 返回 Repo 句柄（含 Worktree + CommitSHA）

#### Scenario: CommitSHA 返回当前 HEAD

- **WHEN** clone 完成后调用 CommitSHA()
- **THEN** 返回 HEAD commit 的 SHA（用于 module_versions.commit_sha）
