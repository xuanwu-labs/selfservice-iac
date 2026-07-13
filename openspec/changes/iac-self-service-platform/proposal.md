## Why

Terramate 当前是一个强大的 IaC 编排 CLI，但它直接面向"会写 HCL、会跑命令"的工程师。要让 DBA、中间件团队、业务开发都能按需获取基础设施，必须在其上加一层**基础设施产品化**能力：标准化模块库 + 服务目录表单 + 自动化编排引擎 + 状态/漂移治理。Spacelift / Env0 / Scalr 已验证此模式，但团队需要**可插拔、自主掌控、深度以 Terramate 为引擎**的自研平台，避免厂商锁定、贴合内部多团队分层治理（网络运维 / DBA / 中间件 / 业务项目组）的组织模型。

核心诉求可归纳为一条流水线：**用户填表 → 平台生成代码 → Terramate 编排执行 → 平台反馈与治理结果**。

## What Changes

- **新增平台层**：独立的 Go Orchestrator 服务 + 轻量自研 Web 前端，Terramate 作为编排引擎被调用（**不侵入 Terramate CLI / HCL / 配置核心**）。
- **模块注册表**：把 Git 上已有的原子 Terraform 模块注册进平台，做版本管理与变量契约提取。
- **服务目录**：把模块的几十个变量抽象为简洁申请表单，其余由平台以最佳实践默认值自动填充。
- **分层与归属模型**：Global（网络 / 集群，平台运维）→ Middleware（**DBA + 中间件两部门并列同层**：DBA 管 RDS/Redis、中间件团队管 Kafka/MQ）→ Application（**不同业务部门各自独立**：业务团队动态申请），每个资源归属到固定团队。**层定义可配置**（默认三层，管理员可增删层/改路径模板，D24）；**bundle 可选**（小业务部门直接 component，大业务部门按项目组分组）。
- **部署单元（bundle，可选）**：一组 stack 的逻辑单元，对应一个业务部门内项目组/产品线的资源集合；**bundle 可选**——小业务部门直接 `<team>/<component>`，大业务部门多产品线用 `<team>/<bundle>/<component>`；每个原子组件一个 stack（逻辑治理单元），避免状态爆炸。
- **编排流水线**：表单提交 → 生成 Terramate stack + Terraform 代码 → `terramate run ... plan` → 审批 → `apply` → 回写资源信息。
- **状态与漂移治理**：远程 backend（S3 / MinIO + 锁），周期性漂移检测（**开源 Terramate 无此引擎，需平台自研**），漂移事件通知用户在界面确认同步策略。
- **可插拔适配器**：Git 提供商、云提供商、状态后端、策略引擎（OPA）、成本估算（Infracost）、通知器均接口化，按需替换。
- **执行目录治理**：平台管理"Terramate 工作仓库"的克隆 / 同步 / 元数据回写，解决平台重启后的执行目录恢复问题。
- **身份与组织同步**：OIDC（dex 桥接企业 IDP/LDAP/AD）认证，SCIM/飞书/钉钉多源目录同步，组织→团队/角色自动映射。
- **工具链版本管理**：terramate / terraform(tenv) / provider(lock.hcl + registry mirror) 三层锁定，打包进 worker 镜像。
- **执行运行时（可插拔 Executor）**：每工单独立沙箱执行；四模式 `process`（节点预装工具链 + 目录/cgroups 隔离，零容器/镜像/仓库，默认）/ `container`（docker/podman）/ `kubernetes`（K8s Job）/ `remote`（SSH 节点池）；容器类模式配 `ImageBuilder`（Web 配版本→构建→push）+ 可插拔 `ImageRegistry`。K8s 与镜像仓库均非必需，借鉴 Atlantis（进程派）+ Spacelift/Env0（容器派）两派共识。
- **审批流程引擎**：自研轻量 YAML DSL（节点绑 RBAC，非具体人），支持会签/条件/超时升级/驳回；保留 Temporal 升级路径。
- **存量导入**：Terraform 1.5+ 声明式 `import` 块 + 平台向导（选模块→输入资源 ID→import→plan 验证零变更→纳管）。
- **CICD 集成审批门禁**：CI 完成后以声明式 yaml 申请单提交平台（与 Web 表单同构），CD 阻塞等待，平台 RBAC 审批通过→释放 CD / 驳回→终止；权威 CLI `tm gate` + 多 CICD 适配器，历史 `tm-gate` 仅作兼容 shim。
- **平台 CLI 与 AI 原生扩展**：单二进制 `tm` CLI（catalog/request/stack/drift/approval/cost 全覆盖，gate 能力以 `tm gate` 为权威入口），机器身份 AK/SK（HMAC 签名），内置 MCP server 让 agent 标准化接入（Claude Code/Cursor 通用），声明式 skills（自然语言触发 + CLI/LLM 编排）+ `tm ai` 自然语言入口 + `--output llm`；AI 操作不豁免 OPA/审批/RBAC 治理，高危强制人工审批。
- **CMDB 与 FinOps**：CMDB 作为 state 的可查询索引（不复制 state），apply 后 ingester 解析资源入库；强制平台 tag（team/bundle/stack）作为成本归集锚点；FinOps 双成本源（Infracost 预估 + 云账单核销）+ 预算治理（超预算审批升级）+ 孤儿资源检测 + 成本优化建议一键转申请。
- **云账号与凭据管理**：三层凭据模型（bootstrap 每云账号 1 套双人审批/执行 per-team OIDC 优先免长存 AK/SK/个人仅 RBAC 禁持执行凭据）+ 团队↔云账号授权（`team_cloud_grants` 驱动申请页"目标云"按部门过滤）+ OIDC 联邦作 Trusted Issuer + 凭据注入 Executor（占位符 + env，不入 git/日志）+ IAM 角色模板化（catalog `required_permissions` 聚合）+ 生命周期（90 天轮换 + 泄漏紧急 + bootstrap 双审）。业界（Spacelift/Env0/TFC/Vault）共识。

## Capabilities

### New Capabilities
<!-- 平台是全新层；以下每项对应一个 specs/NN-中文名.md -->

- `module-registry`（→ `specs/01-模块注册.md`）：原子 Terraform 模块的注册、版本、变量契约提取。
- `service-catalog`（→ `specs/02-服务目录.md`）：模块抽象为申请表单、最佳实践默认值、可见性控制。
- `tenancy-model`（→ `specs/03-团队与归属.md`）：团队 / 项目组 / 部署单元（bundle）的归属与权限边界。
- `stack-layout`（→ `specs/04-分层与stack组织.md`）：**Layer 规则模型可配置（默认三层 Global/Middleware/Application，管理员可增删层/改路径模板，D24）**+ PathGenerator 路径模板化 + bundle 可选 + StackGranularity 粒度策略 + stack 边界=逻辑治理单元 + 跨层依赖解析 + **层规则集整体版本化与迁移操作模型（D26：整套分层方案不可变 v+1 + layer_logical_id 跨版本稳定 + per-stack Tier 1/2/3 分类 + Worker 自动 state mv + sunset 强制迁移 + CMDB 路径同步 + diff viewer）**。
- `code-generation`（→ `specs/05-代码生成引擎.md`）：表单 + 模块契约 + 默认值 + 依赖图原四输入合并**演化为 9 阶段参数解析管道（详见 specs/18）**生成 Terramate stack + Terraform 代码；强制注入 tag/state key；**模块零侵入（D25：cardinality 在 catalog 项配置，codegen 在调用方注入 for_each/count，社区模块直接复用零适配）**；PathGenerator 渲染路径；三种 cardinality（single/list/map）覆盖同质+异构多实例。
- `orchestration-engine`（→ `specs/06-编排引擎.md`）：表单→代码生成（specs/05）→plan→审批→apply 的流水线，调度 Terramate。
- `state-drift`（→ `specs/07-状态与漂移.md`）：远程 backend 约定、漂移检测、漂移事件与同步策略。
- `platform-api`（→ `specs/08-平台API与网关.md`）：HTTP API、RBAC、审批流、事件通知、Webhook。
- `identity-sync`（→ `specs/09-身份与组织同步.md`）：OIDC（dex 桥接）认证 + 目录同步（SCIM/飞书/钉钉）+ 组织→团队/角色映射。
- `approval-engine`（→ `specs/10-审批引擎.md`）：审批 DSL、节点绑 RBAC、会签/条件/超时/驳回、**双门禁（准入 pre-plan + 执行确认 pre-apply，D21）**、plan/apply 解耦（两次 Executor 调用 + 沙箱释放 + plan 产物持久化）。
- `toolchain-versioning`（→ `specs/11-工具链版本管理.md`）：terramate/terraform/provider 三层版本锁定 + worker 镜像 + 执行隔离（可插拔 `Executor` 四模式：process 预装工具链 / container / kubernetes / remote，K8s 与镜像仓库均非必需；容器类模式配 `ImageBuilder` + 可插拔 `ImageRegistry`）。
- `legacy-import`（→ `specs/12-存量导入.md`）：存量资源 Terraform 1.5+ 声明式 import 块纳管。
- `cicd-integration`（→ `specs/13-CICD集成与审批门禁.md`）：声明式 yaml 申请单 + 审批 gate 嵌入 CICD（CD 阻塞-审批-释放）。
- `platform-cli-ai`（→ `specs/14-平台CLI与AI原生扩展.md`）：平台 CLI（`tm`，gate 能力以 `tm gate` 为权威入口）+ 机器身份（AK/SK）+ 内置 MCP server + 声明式 AI skills（自然语言触发 + CLI/LLM 编排 + LLM-friendly 输出 + `tm ai` 入口）；AI 操作不豁免 OPA/审批/RBAC 治理。
- `cmdb-finops`（→ `specs/15-CMDB与FinOps.md`）：CMDB 资源实例索引（state 可查询视图，不复制 state）+ 强制 tag 策略 + FinOps 双成本源（Infracost 预估 + 云账单核销）+ 预算治理（超预算审批升级）+ 孤儿资源检测 + 成本优化建议一键转申请。
- `cloud-credentials`（→ `specs/16-云账号与凭据管理.md`）：云账号纳管 + 团队↔云账号授权（`team_cloud_grants` 驱动申请过滤，"看部门开通了哪些云厂商/账号"，D27 引入 env_scope_json env 维度授权实现分级治理）+ 三层凭据模型（bootstrap 双人审批/执行 OIDC 优先免长存/个人仅 RBAC 禁持执行凭据）+ OIDC 联邦优先 + 凭据注入 Executor（占位符 + env 注入，不入 git/日志/codegen）+ IAM 角色模板化（catalog `required_permissions` 聚合生成）+ 凭据生命周期与轮换（90 天强制 + 泄漏紧急 + bootstrap 双审）。
- `environment-and-tenant`（→ `specs/17-环境与租户.md`）：**Environment 一等治理对象**（dev/staging/prod/dr，决定治理强度 + cloud_account + region + 默认 tag_namespace）+ **Tenant 一等对象**（默认 platform-default 单租户外部客户/独立 BU 时创建）+ **两档 isolation_level（vpc-per-env 默认 / account-per-env escape hatch，业界 2026+ 已收敛 VPC 级，shared-vpc 已废弃）** + **EnvironmentTenantBinding 三元组**（env × tenant × layer → VPC stack 引用 + 子网 + 安全组，codegen Stage 4 查询入口）+ VPC 仍是 Global 层 stack 不引入新抽象 + 跨租户网络走独立 CEN stack + team_cloud_grants.env_scope_json（prod 严 dev 松分级治理）+ lifecycle（freeze/deprecate）。
- `tag-layering-and-parameter-resolution`（→ `specs/18-标签与参数解析.md`）：**Tag 来源 7 层模型**（L1 platform-mandated 永远赢 / L2 env / L3 tenant / L4 team-bundle / L5 stack 自动派生 / L6 catalog defaults / L7 user form 受白名单约束）+ **参数解析 9 阶段管道**（S1 模块契约 → S2 catalog defaults → S3 layer rule 治理 → S4 env 上下文 → S5 tenant 上下文 → S6 team 策略 → S7 跨层依赖 → S8 用户表单 → S9 平台强制注入；治理类高于用户类高于上下文类）+ **provenance 审计（resolved_params_json）**（每变量含 source/rank/overridden_from/user_attempted_override）+ 校验门（每阶段可拒绝）+ 云厂商 tag 约束校验。是 specs/05「四输入合并」的权威细化（演化不推翻）。

### Modified Capabilities
<!-- 无：平台是全新层，不改动 Terramate 现有 spec 级行为 -->

（无。Terramate 既有 CLI / HCL / stack / generate / cloud 行为均不修改。）

## Impact

**影响层级**：
- **平台层（新增）**：本次变更的主体。新增 `platform/`（Go Orchestrator）、`frontend/`（Web）、`platform/db`（元数据 schema）等目录（或拆为独立仓库，由 design 决策）。
- **Terramate CLI / LSP / HCL / config / stack / generate / cloud**：**不修改**，作为引擎被平台通过子进程（或后续可选 import Go 包）调用。

**兼容性**：不破坏任何已有 Terramate 配置或 CLI 用法。平台是独立外层，Terramate 用户完全无感知；即便平台下线，工作仓库内的 Terramate 配置仍可独立运行。

**HCL 语法 / config schema 变更**：**无**。完全复用 Terramate 现有 HCL 配置与 stack 定义能力，平台仅生成符合现有语法的配置文件。

**新增依赖**：Go Web 框架、关系型 DB（元数据）、对象存储（state/产物）、OPA client、Infracost、go-git、消息/任务队列。

**复用 Terramate 的方式**：默认通过 `exec` 调用 Terramate CLI（可插拔）；`generate` 用于生成 stack 骨架与静态配置，运行期多实例交给 Terraform 原生 `for_each`（取舍见 design）。

**关键风险**：开源 Terramate 无漂移检测引擎，需平台自研 `plan` 对比 + 事件机制；多团队分层的状态隔离与依赖编排是落地难点。

## Phase 拆分索引（每阶段独立 change）

> 本 change 是设计总纲（D1-D30 + 17 capability），**不直接实现**。各阶段按 OpenSpec config `tasks` 规则独立为 change，propose → apply → archive 各自走完整生命周期。拆分时在本节登记。

| Phase / 阶段 | 独立 change | 状态 | 覆盖 capability |
|---|---|---|---|
| Phase 0 契约冻结 | `platform-contract-freeze` | ✅ 已归档 | platform-api（契约冻结：5 域 / 6 service / 24 RPC） |
| 脚手架 | `platform-tech-stack-and-scaffold` | ✅ 已归档 | server/ Go module 骨架 + 技术栈决策 D31-D45 |
| Wave 1-8 | （待创建各 wave change） | ⏳ 未开始 | module-registry / service-catalog / code-generation / orchestration-engine / ... |

**契约层 MVP 范围**（Phase 0 冻结的 5 域 / 24 RPC）：
- `registry/`（RegistryAdminService）← module-registry（specs/01）
- `catalog/`（CatalogService + CatalogAdminService）← service-catalog（specs/02）
- `lifecycle/`（LifecycleService）← orchestration-engine（specs/06）+ approval-engine（specs/10）
- `cloud/`（EntitlementService）← cloud-credentials 的用户侧授权查询（specs/16）
- `common/`（共享 enum + dto）

Phase 2+ 能力（tenancy / stack-layout / state-drift / cicd / cmdb-finops / tag-layering 等）的 RPC 推迟到对应 Wave 再冻结，Phase 0 不预留 proto（避免过度设计）。
