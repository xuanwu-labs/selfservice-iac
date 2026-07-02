## 01-背景与现状

Terramate 是纯 CLI 编排引擎，定位是"面向会写 HCL 的工程师"。基于对当前仓库的核查（非推测）：

- **纯 CLI，无 server**：除 `terramate-ls` 语言服务器外，无任何长驻 HTTP/gRPC 服务（`cmd/` 下只有 CLI 二进制）。
- **漂移检测为 Cloud 专属**：`cloudsync/create_drift.go`、`commands/cloud/drift/`、`cloud/api/drift/` 全部是 Terramate Cloud 的客户端/同步逻辑，开源版**无自主漂移检测引擎**。
- **无内置 git clone/pull/push**：`git/` 仅做 git status / diff（供 change detection），不管理工作仓库生命周期。
- **stack 支持嵌套**：`stack/manager.go`、`stack/clone.go` 存在 parent/children 概念，目录树即 stack 层级。
- **generate 是生成期静态能力**：`generate/` 在编译期渲染模板，**不是运行期 foreach**；运行期多实例由 Terraform 原生 `for_each`/`count` 承担。
- **不管理 Terraform state**：state 完全交给 Terraform backend 配置。

**现状缺口**：无服务目录、无表单、无审批、无状态治理、无漂移检测、无多团队分层治理。本设计在不侵入 Terramate 核心的前提下，新增一个"平台层"补齐这些缺口。

## 02-目标与非目标

**Goals**
- 在 Terramate 之上构建可插拔的 IaC 自助平台：模块注册 → 服务目录 → 编排流水线 → 状态/漂移治理。
- 支持多团队分层治理（平台运维 / DBA / 中间件 / 业务项目组）。
- 平台与 Terramate 解耦：Terramate 作为引擎被调用，可独立升级与替换。
- 全程不修改 Terramate CLI / HCL / config / stack / generate 行为。

**Non-Goals**
- 不重写 Terramate，不自研 Terraform 替代品。
- 不直接采用 Backstage（参考其服务目录思想，但前端自研轻量方案）。
- 不引入云厂商 SDK 重造资源管理轮子（云资源仍由 Terraform 管理）。
- 不在本期实现成本核销、SSO 细粒度组织树等非核心治理特性。

## 02b-优化后的设计收敛原则

> 本节是后续 docs/specs/tasks 的约束入口。平台定位从"全功能资源管理平台"收敛为**受治理的 IaC 变更控制面**，避免把服务目录、审批、CMDB、FinOps、AI、迁移工具全部压入同一条强同步主链路。

**主链路保持单向**：`Catalog → Request → Codegen → Git Commit → Plan Artifact → Approval → Apply → Reconcile`。其他能力（CMDB、FinOps、通知、AI、孤儿检测、StateMover）只能挂在主链路之后，不能反向成为 apply 的同步前置，除非该能力直接影响安全或 state 一致性。

**四个权威模型**：
- **Path Contract**：以 D29 的单仓 layer-first 拓扑为准。PathGenerator 必须同时输出 `repo_path`、`state_key`、`stack_id`、Terramate tags；`docs/02` 不再保留 tenant-first 或 legacy `globals/<env>` 作为并列权威模板。
- **RequestLifecycle**：以 D21 + `docs/12` 的统一状态机为准：`submitted → generating → pending-admission? → planning → plan-ready → pending-approval → applying → reconciling → succeeded`，异常分支进入 `rejected` / `cancelled` / `expired` / `failed-retryable` / `failed-terminal` / `waiting-manual` / `reconcile-pending`。
- **ParameterResolution**：以 `docs/08` / `specs/18` 为唯一 rank 权威。Phase 1 可使用 5 阶段简化实现，但语义必须可无损映射到 D28 的 9 阶段。
- **Metadata Schema**：以 `docs/04` 为数据库权威，任何 codegen、approval、drift、import、CMDB、CICD 引用的实体必须先在 `docs/04` 落 schema 或明确为后续扩展。

**核心事实源边界**：
- 元数据 DB 是控制面事实源：request、审批、授权、resolved params、审计。
- Git pinned commit 是代码事实源：生成代码和 Terramate stack 定义。
- Terraform state 是资源事实源：CMDB 不复制 state 全文，只做查询索引。
- 云账单是成本事实源；Infracost 只做申请时估算和审批提示。

**韧性模式**：跨 DB/Git/对象存储/state/云 API/CMDB 的操作采用 Saga + Outbox + Reconcile。平台不能自动闭环的异常必须生成 `manual_intervention_tasks`，带 owner、SLA、上下文、恢复入口和审计链，不能只返回错误码。

## 02c-商业级优化后的差异化创新边界

> 对标 Terraform Cloud、Spacelift、env0、Atlantis、Backstage/Port 只能补齐行业标准能力，不能把本方案做成普通仿制品。以下能力是本方案的差异化主线，后续文档和实现必须保留并放大。

| 创新 | 保留方式 | 放大方式 |
|------|----------|----------|
| Terramate-native | `stack.tm.hcl`、tags、after/watch、DAG、monorepo layer-first 拓扑作为主执行模型 | PR-first 和 Scheduled Runs 也必须通过 Terramate changed stack / DAG 语义定位影响面 |
| 参数 provenance | `resolved_params_json` 记录每个变量来源、rank、覆盖链路 | 审批页、失败解释、AI 输出和审计证据包都展示 provenance |
| 存量 read-only 过渡 | import 默认进入 `managed-readonly`，不直接变更历史资源 | 通过 health、owner、tag 缺口、drift 观察逐步晋级到 `managed-changeable` |
| D26 迁移安全 | Layer rule 版本化、StateMover 默认关闭、迁移计划优先 | commercial promotion / PR-first 不得绕过迁移计划和 state safety |
| AI 不绕过治理 | AI/MCP/skills 与人类入口共享 RBAC、OPA、审批和审计 | AI 只做意图翻译、解释和自动化编排，不能成为 privileged channel |

商业级增强优先补齐工程契约、工作流、运营和证据链；高级能力（AI/MCP、半自动 StateMover、FinOps 优化建议）必须建立在这些契约之上。

## 03-总体架构

分两轴理解：**控制面**（平台 Orchestrator）与**执行面**（Terramate 工作仓库 + Terraform）。

```
[Web 前端] --HTTP--> [平台 API 网关 + RBAC] --> [Orchestrator 编排引擎]
                                                  |  代码生成 + 工单状态机
        +-----------------------------------------+-----------+-----------+
        v                v              v          v           v
[模块注册/服务目录] [Terramate Adapter]  [DriftDetector] [OPA/Infracost/Notifier 适配器]
                         | exec
                         v
                 [Terramate CLI] --> [Terraform] --> [远程 State Backend(S3/MinIO+锁)] --> [云]
                         ^
                         |
                 [工作仓库 git 同步]  <-- go-git，由平台管理 clone/fetch/commit
```

- **平台层代码位置**：新增顶层目录 `platform/`（独立二进制 `cmd/platform`），与 Terramate 主仓库分层隔离（详见决策 D2）。
- **不触碰的 Terramate 包**：`cmd/`(terramate 主 CLI)、`commands/`、`engine/`、`hcl/`、`config/`、`stack/`、`generate/`、`cloud/`、`ls/` 一律不修改。
- 完整组件图、数据流、跨层依赖机制见 `docs/01-总体架构.md` 与 `docs/02-分层与stack模型.md`。

## 04-关键决策

### D1 — 平台与 Terramate 的集成方式：exec 适配器
- **决策**：定义 `TerramateAdapter` 接口，默认实现通过 `exec` 调用 `terramate` CLI；未来可选 import Terramate Go 包做内嵌实现。
- **理由**：解耦、可插拔、Terramate 升级不破坏平台；exec 开销相对编排耗时可忽略。
- **备选**：①import 包内嵌（耦合高，升级痛）②fork Terramate 深度定制（违背可插拔，舍弃）。
- **影响**：需对依赖的 CLI 子命令输出做契约测试，锁定 Terramate 版本范围。

### D2 — 平台代码归属：当前仓库新增 `platform/`
- **决策**：在 `D:/project/iac/terramate` 新增 `platform/` 顶层目录与 `cmd/platform` 二进制，与 Terramate 共用仓库但分层隔离；是否拆独立 go module 由 tasks 阶段评估。
- **理由**：复用 CI、便于联调、单仓库演进；同时通过目录边界与 lint 保持 Terramate 主仓库不被平台代码污染。
- **备选**：独立仓库（后期可平滑拆出，目录隔离使迁移成本低）。

### D3 — 同类多实例：generate 生成骨架 + Terraform `for_each` 运行
- **决策**：`generate` 负责"生成 stack 目录骨架与静态配置"；同 stack 内同类多资源（如 5 台 ECS）由 Terraform 原生 `for_each`/`count` 承担。
- **理由**：gen 是生成期、静态；运行期多实例是 Terraform 强项；这样 stack 数量可控，状态边界清晰。
- **备选**：为每个实例生成独立 stack（会导致 stack/state 爆炸，舍弃）。

### D4 — 执行目录治理：平台内置 git（go-git）+ 元数据驱动恢复
- **决策**：平台用 `go-git` 管理"Terramate 工作仓库"（clone/fetch/commit/push）；元数据库持久化 `remote + branch + commit + stack 路径`，平台重启后据此恢复工作目录到一致状态。
- **理由**：Terramate 无内置 git clone；纯元数据（不存 git）无法恢复历史与代码一致性；内置 git 是"完整落地"的最优解。
- **备选**：①每次重新生成代码丢弃 git 历史（丢失变更追溯，舍弃）②外部 CI 仓库存代码（职责分散，运维复杂）。
- **影响**：并发执行需为每个工单分配独立 worktree/分支锁（详见 `docs/10-执行目录治理.md`）。

### D5 — 漂移检测：平台自研 DriftDetector
- **决策**：新增 `platform/internal/drift`，调度器对每个 stack 执行只读 `terraform plan`，对比真实云与期望状态，产出漂移记录。
- **理由**：开源 Terramate 无漂移引擎；接入 Terramate Cloud 是商业方案（用户要求自主掌控）。
- **备选**：接入 Terramate Cloud（备选，可作为适配器实现供后续切换）。
- **影响**：需限流、时间窗、并发控制（详见 `docs/13-漂移检测设计.md`）。

### D6 — 状态隔离：每 stack 独立 state key + 远程 backend + 锁
- **决策**：强制远程 backend（S3 / MinIO 兼容 + DynamoDB/锁），state key 由 stack 路径确定性推导；禁止 local backend 进入生产。
- **理由**：避免状态爆炸，缩小爆炸半径；跨 stack 引用走 `terraform_remote_state` data source。

### D7 — 可插拔适配器：接口化六个扩展点
- **决策**：定义接口 `GitProvider`、`CloudProvider`、`StateBackend`、`PolicyEngine`(OPA)、`CostEstimator`(Infracost)、`Notifier`，提供默认实现，支持按配置替换。
- **理由**：可插拔是用户核心诉求；接口化使后续切换云、替换策略引擎、对接内部 IM 低成本。

### D8 — 表单触发与长任务：工单状态机 + 异步队列
- **决策**：`POST /api/v1/requests` 同步创建工单返回 ID，流水线异步推进（任务队列 + 状态机），前端轮询或订阅事件。
- **理由**：plan/apply 是长任务，不能阻塞 HTTP；状态机使每步可恢复、可审计。

### D9 — 前端：轻量自研，参考 Backstage 思想
- **决策**：自研轻量 Web（React + 现代组件库），实现服务目录、申请表单、审批、漂移视图、变更历史；不引入 Backstage。
- **理由**：用户明确不使用 Backstage；自研可精准贴合多团队分层治理与内部风格。

### D10 — 身份认证与组织同步：OIDC（dex 桥接）+ 目录同步抽象层
- **决策**：平台作 SP，认证统一走 OIDC（用 [dex](https://github.com/dexidp/dex) 桥接 SAML/LDAP/AD/企业 IDP，不直接 LDAP 认证）；组织数据走 `DirectorySyncer` 抽象——SCIM 2.0 适配器（国际标准）+ 飞书/钉钉专用适配器（开放 API + 事件订阅，定时拉取兜底）；组织节点 → 平台团队/项目组/角色 自动映射。
- **理由**：OIDC 是现代 SSO 标准；LDAP/AD 仍普遍但应套 OIDC（dex 是 CNCF 成熟的 IdP bridge）；SCIM 是国际标准但飞书/钉钉不原生支持，需专用适配器统一抽象。
- **备选**：①直接 LDAP 认证（旧、不利 SSO/MFA）②单一 SCIM（国内场景不够）。
- **影响**：引入 dex；维护 3 个 DirectorySyncer 实现。详见 `docs/05-身份与组织同步.md`。

### D10.1 — 认证模式可配：多 OIDC issuer 并存 + dex 降级为可选适配器 + CAS 代理接入

- **决策**：平台**同时信任多个 OIDC issuer**（每个 issuer 一个 `oidc.Provider` + `Verifier`），认证源列表在 DB 中可配（Web 改、热加载）。dex 从 D10 的「必选桥接」**降级为可选适配器之一**——企业自建 OIDC OP（Keycloak/Authing/手写）**直接对接，不经 dex**。CAS 等非 OIDC 协议通过 **OIDC 代理**接入（平台永不直接说 CAS/SAML/LDAP）。
- **为什么比「dex 独占」更优**：
  - ① **单 IdP 企业零开销**：企业已有 OIDC OP（如 Keycloak），直接配 `issuer_url + client_id`，不部署 dex。
  - ② **多 IdP 联邦天然支持**：集团下子公司各有 IdP（A 公司用 Okta + B 公司用 Authing + C 公司用自建 OIDC），平台同时信任全部 issuer，登录页列多个按钮（同 Grafana/K8s Dashboard 模式）。
  - ③ **平台永 OIDC-native**：认证代码只做 OIDC 验证 + claim 映射，不引入 CAS/SAML/LDAP 客户端库，降低复杂度。
  - ④ **dex 仍有价值但非必须**：需要桥接 SAML/LDAP/AD/多源联邦时部署 dex；单一 OIDC OP 场景跳过 dex。
  - **业界对照**：Kubernetes `--oidc-issuer` 多 flag 并存 / Grafana `auth.generic_oauth` 多 provider / ArgoCD 多 OIDC config / Vault JWT auth method 多 role → **共识：多 OIDC issuer 并存是标准模式**。
- **配置模型**（存 DB，`oidc_providers` 表）：
  ```yaml
  auth:
    oidc_providers:                    # 可配多个，平台同时信任全部
      - name: "enterprise-op"          # 企业自建 OIDC OP（直连，不经 dex）
        issuer: "https://sso.corp.com"
        client_id: "iac-platform"
        client_secret: "${OIDC_SECRET}"
        claim_mapping:                 # 不同 OP 的 claim 字段名不同，需映射
          username: "preferred_username"
          email: "email"
          groups: "groups"             # Azure AD 可能是 "oid" / Keycloak 是 "realm_access.roles"
        scopes: ["openid", "profile", "email", "groups"]
      
      - name: "internal-dex"           # dex 桥接（SAML/LDAP/AD 等非 OIDC 源）
        issuer: "https://dex.internal"
        client_id: "platform"
        client_secret: "..."
        bridge_mode: true              # dex 已归一化 claim，不需 mapping
    
    default_provider: "enterprise-op"  # 登录页默认跳转
  ```
- **CAS 接入路径**（不引入 CAS 客户端）：
  ```
  [CAS Server] ← CAS 协议 → [cas-oidc-proxy] ← OIDC → [平台]
  ```
  cas-oidc-proxy（开源或自研轻量 Go 服务）将 CAS ticket 验证转为 OIDC token 签发，平台视其为普通 OIDC issuer。**平台代码零改动**。
- **登录流程**（多 provider）：
  1. 用户访问平台 → 未登录 → 登录页显示 `enterprise-op` / `internal-dex` / ... 按钮。
  2. 用户点某 provider → OIDC redirect → 企业 IdP 认证 → 回调。
  3. 平台从 callback code 换 token → 按 token 的 `iss` claim 路由到对应 `Verifier` → 验签 → claim_mapping 归一化 → 建 session。
- **claim_mapping 的必要性**：不同 OIDC OP 的 claim 格式差异大（Azure AD `upn` vs 标准 `preferred_username` vs 飞书 `user_id`），平台内部统一用 `{sub, email, name, groups}` 模型，mapping 在 provider 配置中声明。
- **与 DirectorySyncer 的关系**：认证（OIDC 验证 + claim 映射）与目录同步（SCIM/飞书/钉钉推组织数据）**完全解耦**。认证解决「你是谁」，同步解决「你属于哪个团队/角色」。同一用户可经 OIDC 登录 + SCIM 同步组织归属，两者独立。
- **影响**：`oidc_providers` 表（存 DB 运行时可变）；`platform/internal/identity/oidc` 包（多 Verifier + ClaimMapper）；登录页多 provider 按钮；CAS 接入文档（docs/05）；dex 文档从「必选」改为「可选适配器」。详见 `docs/05-身份与组织同步.md` §1.1。

### D11 — 审批流程引擎：自研轻量 DSL 为主，保留 Temporal 升级路径
- **决策**：MVP 用自研审批 DSL（YAML 定义流程：节点绑 RBAC 角色/组，支持多级/会签/条件/超时升级/驳回）+ 状态机执行；**不**引入 Camunda（Go 社区公认对轻量审批过重，需 BPMN+ES），也不一开始上 Temporal。当流程复杂度爆发（跨系统编排、可视化建模）再评估 Temporal（Go-native、workflow-as-code、durable execution）。
- **理由**：IaC 审批核心是"基于 RBAC 的多级路由 + 会签"，复杂度中等；Spacelift/Env0 实践也是简单角色路由而非完整 BPMN；自研零运维依赖。WebSearch 验证（2025）：Temporal 适合代码即工作流，Camunda 适合非技术干系人 BPMN 建模。
- **备选**：①Camunda 8（Zeebe+ES 重）②Temporal（重，需集群）③Flowable（Java，排除）。
- **影响**：自建 DSL 解析与前端可视化渲染。详见 `docs/12-审批引擎与流程.md`。

### D12 — 工具链版本管理：三层锁定 + worker 镜像
- **决策**：①terramate 版本平台全局锁定（多版本共存，exec 时选版本）；②terraform/opentofu 用 [tenv](https://github.com/tofuutils/tenv)（HashiCorp 推荐的 tf 版本管理器，替代 tfenv）多版本管理，per-stack `required_version` 约束；③provider 用 `.terraform.lock.hcl` 锁定，私有 provider 走 registry mirror（自建 Terraform Registry 兼容服务或 network mirror）。三层工具链打包进 worker 镜像。
- **理由**：借鉴 Spacelift 自定义 worker image 模式；tenv 是当前 tf/tofu 版本管理事实标准；lock.hcl 是 provider 锁定标准。
- **备选**：asdf（通用但非 tf 专用）；per-stack 独立容器（粒度过细）。
- **影响**：维护 worker 镜像 CI 与 registry mirror。详见 `docs/11-工具链版本与执行隔离.md`。

### D13 — 执行隔离：每工单独立容器（worker 镜像）
- **决策**：每个 plan/apply 在独立容器内执行（基于 D12 的 worker 镜像），工单级资源/CPU/网络隔离；容器内挂载该工单的 git worktree（复用 D4）。借鉴 Spacelift/Env0。
- **理由**：多租户/多并发安全；工具链版本隔离；故障爆炸半径小；与 D4 worktree 治理叠加形成完整执行沙箱。
- **备选**：①进程级（隔离弱）②K8s pod（重，需 K8s 依赖）。
- **影响**：执行节点需容器运行时；需容器调度（K8s/ Nomad / 自管 docker run 均可）。

### D14 — 存量导入：TF 1.5+ 声明式 import 块 + 平台向导
- **决策**：用 Terraform 1.5+ [声明式 import 块](https://developer.hashicorp.com/terraform/language/import)（持久、可 review、可 `for_each` 批量）+ `terraform plan -generate-config-out` 自动生成配置。平台向导：选模块 → 输入资源 ID → 生成 import 块 + module 调用骨架 → import 落 state → plan 验证 0 变更 → 纳管。不支持 import 的资源标 `limited`（半自动 state 编辑）；敏感字段留空待填。
- **理由**：import 块是 2023+ 最佳实践（HCL 原生、版本控制、批量）；Terramate 官方有[完整 import 指南](https://terramate.io/rethinking-iac/a-comprehensive-guide-to-importing-existing-infrastructure-into-terraform-and-opentofu/)，与平台引擎契合；命令式 `terraform import` 不持久。
- **备选**：terraformer/Terracognosa 自动发现（维护放缓、质量参差，仅作辅助发现工具）。
- **影响**：要求 terraform >= 1.5；provider 需支持 import。详见 `docs/15-存量导入.md`。

### D15 — 配置管理：启动配置(DB, Web 可改) + worker 镜像(版本固化)
- **决策**：双层配置——**运行时可变**（适配器装配、身份源、RBAC、审批 DSL、目录项、漂移调度）存 DB，平台 Web 可改、热加载；**工具链版本**固化进 worker 镜像（CI 构建，版本变更新镜像 + 灰度）。借鉴 Spacelift（worker image + Web 配置组合）。
- **理由**：分离"易变业务配置"（敏捷）与"工具链版本"（稳定），各自走合适的变更通道。
- **影响**：worker 镜像 CI 流水线；所有配置变更入审计。

### D16 — CICD 集成与审批门禁（gate）
- **决策**：平台作为审批 gate 嵌入 CICD；提供声明式 yaml 申请单入口（与 Web 表单同构，复用 D11 审批引擎）；gate API（`GET .../gate` 状态 + webhook 订阅）支持**阻塞轮询/回调异步双模**；提供权威 CLI `tm gate` + Jenkins/GitLab/Argo/Flux/GitHub Actions 适配器；历史 `tm-gate` 仅作为兼容 shim；yaml 申请单按 `pipeline+commit+catalogItem` 幂等。
- **理由**：CI 完→资源申请→CD 阻塞→审批→释放 是企业 IaC+应用交付联动的标准诉求；审批逻辑统一到平台 RBAC 而非散落各 CICD；阻塞/回调双模覆盖有无 inbound webhook 能力的各类 CICD。
- **备选**：①各 CICD 自实现审批（逻辑分散、权限难统一）②仅 webhook 无轮询（部分企业 CICD 在防火墙后无 inbound 能力）。
- **影响**：新增 `platform/internal/cicd`（适配器）+ `tm gate` CLI 子命令；工单支持 yaml 入口与 CI 上下文幂等。详见 `docs/16-CICD集成与审批门禁.md`。

### D17 — 平台 CLI 与 AI 原生扩展：统一 CLI + AK/SK + MCP + 声明式 skills
- **决策**：①平台提供单二进制 CLI `tm`（覆盖 catalog/request/stack/drift/approval/cost，复用 `platform/internal/api` service 层），D16 gate 能力以 `tm gate` 子命令组为权威入口，历史 `tm-gate` 仅作为兼容 shim，MUST NOT 维护双 CLI；②机器身份用 service account + AK/SK（HMAC 签名，类 AWS SigV4），与人的 OIDC 会话统一到同一 RBAC 引擎；③CLI 内置 MCP server（`tm mcp serve`），把命令 + skills 暴露为 MCP tools，所有支持 MCP 的 agent（Claude Code/Cursor/Continue/自研）一次接入通用，避免每 agent 写专有适配；④声明式 skills（YAML：trigger 自然语言模式 / steps CLI+LLM 序列 / output contract），平台内置 + 团队自定义；**skill 声明 `approval_scope: atomic`（首次审批覆盖全链步骤，不逐步审批消除用户疲劳），但 `high_risk_steps`（destroy / cross-layer / prod）仍独立门禁**；⑤`--output llm` 语义化 markdown；⑥自然语言入口 `tm ai` 把 LLM 放平台后端做意图路由（不把治理决策交本地 LLM）；**⑦`tm ai` 与 MCP 的职责边界**：`tm ai`=平台后端 LLM 统一意图路由 + prompt 版本管理 + 治理（用户通过平台 UI/CLI 对话）；MCP server=外部 agent 标准协议接入（Claude Code/Cursor 等，agent 主导推理）；**两者共用同一 RBAC/OPA/审批引擎，治理边界不因接入路径不同而不同**。
- **理由**：CLI 是 agent 天然 tool boundary（exit code + stdout 契约），比让 agent 硬编码 HTTP 稳；MCP 是 LLM↔工具标准协议，避免每 agent 写适配；AK/SK 是机器身份事实标准；LLM 放后端便于统一 prompt/版本/治理边界，所有 AI 操作不豁免 OPA/审批/RBAC；**skill atomic approval 消除多步操作的审批疲劳**（业界共识：ArgoCD Workflow atomic step、GitHub Actions job-level approval）。
- **备选**：①只给 HTTP API（CICD/agent 场景痛苦，shell 拼 curl）②每 agent 写专有适配（散乱、版本耦合）③LLM 放 CLI 本地（行为难统一、治理难收敛）。
- **影响**：新增 `platform/internal/cli` + `platform/internal/identity/aksk` + `platform/internal/mcp` + `platform/internal/skills`；高危操作强制人工审批，agent 不豁免治理。详见 `docs/17-平台CLI与AI原生扩展.md`。

### D18 — CMDB 与 FinOps：资源实例索引 + 强制 tag + 双成本源 + 预算治理
- **决策**：①CMDB 作为 state 的可查询索引视图层（**不复制 state 全文**），apply 成功后 ingester 从 state JSON 解析资源 upsert `resources` 表，关联 stack / bundle / team / layer；**ingester 补偿机制**（§2.8-3）：apply 成功但 ingester 失败 → 异步 retry 3 次 → 仍失败入 dead-letter queue → 定期 reconcile job 从 state 重建 resources（对账 state JSON vs CMDB records，差异 → upsert/标记 stale）；②强制 tag 策略，平台给所有生成资源打 `platform-team/bundle/stack/managed` 标签，catalog 注册校验含 tag（或 codegen 自动注入），作为成本归集与孤儿检测的锚点；③FinOps 双成本源：Infracost 预估（申请时 + 未出账）+ 云账单 API（实际，按 tag 归集），统一 `cost_records` 用 `cost_source` 区分；④预算治理 `cost_budgets`（team / bundle / stack / layer × 月度，多级阈值），申请时预估超预算触发**成本双阈值**（>2x cap S6 拒绝 / 1x~2x cap 标记 cost_overrun 透传 pre-apply 升级审批）；⑤漂移引擎 + 云侧资源清单对比 CMDB 发现孤儿资源，优化建议引擎产出 rightsize / release / reserved-instance / tag-missing 并可一键转申请。
- **理由**：CMDB 不复制 state（state 是执行面真相源）只做查询索引，避免双写不一致；强制 tag 是多云成本归集与孤儿检测的事实手段（借鉴云厂商 Tag 策略 + Spot.io / Flexera）；Infracost 用于决策时预估，云账单用于实际核销，二者互补；预算联审批使成本治理嵌入变更流程而非事后报表。
- **备选**：①CMDB 全量存 state（双写不一致风险，敏感字段泄露面大）②无 tag 依赖命名 / 正则匹配归集（脆弱、多云难统一）③仅 Infracost 无账单（滞后 / 不准，无法核销）④FinOps 独立报表系统不联审批（治标不治本，成本失控仍发生）。
- **影响**：新增 `platform/internal/cmdb` + `platform/internal/finops` + 云账单拉取 job；强制 tag 策略需模块库配合（catalog 注册校验）。详见 `docs/14-CMDB与FinOps.md`。

### D20 — 执行运行时：可插拔 Executor 四模式（process / container / kubernetes / remote）+ 镜像构建管线，K8s 与镜像仓库均非必需
- **决策**：定义 `Executor` 接口（`platform/internal/executor`）抽象"在隔离沙箱内运行命令"——签名 `Run(ctx, spec) → (exit, stdout, stderr)`；编排引擎（specs/06）与漂移检测（specs/07）均通过接口调用，**MUST NOT 硬编码任何运行时**。提供四个可插拔实现：
  1. **process**（`ProcessExecutor`，**默认**）：执行节点**预装工具链**（terramate / tenv / terraform·opentofu / opa / infracost，平台 installer 或 admin 维护），每工单 = 独立 worktree 目录 + 子进程 + cgroups v2（CPU/内存）+ 可选 network namespace 隔离。**无容器、无镜像、无镜像仓库**。零基础设施，最快冷启动。（Atlantis / Jenkins-terraform 模型。）
  2. **container**（`ContainerExecutor`）：平台宿主机 docker/podman，每工单一容器（worker 镜像，D12）。需本地镜像。
  3. **kubernetes**（`K8sExecutor`）：每工单提交一个 Job/Pod 到指定 namespace（worker 镜像），按 ResourceQuota 限资源，水平扩展。需 K8s + 镜像仓库可达。
  4. **remote**（`RemoteNodeExecutor`）：SSH worker 节点池，每节点可配预装工具链（process 式）或容器运行时（container 式）。离线 / 裸机 / 复用存量主机。
- **镜像构建管线**（仅 container / kubernetes / remote-container 模式需要，process 模式完全不需要）：Web "保存工具链版本" → `ImageBuilder`（`platform/internal/executor/imagebuilder`）渲染 Dockerfile 模板（distroless/debian-slim 基镜像 + terramate 版本 + tenv tf/tofu + provider mirror + opa + infracost，全部 pin）→ 异步构建 → tag（`tm-worker:tm0.17-tf1.9-20260615`）→ push 到可插拔 `ImageRegistry` 适配器（Harbor / 阿里云 ACR / AWS ECR / Azure ACR / Docker Registry / 内嵌本地 registry / 文件中转（离线））。构建执行可在平台内置 docker API（构建节点）或外部 CI webhook。版本组合变 = 触发新镜像构建 + 灰度。
- **选择与切换**：`adapters_config.executor.mode = process|container|kubernetes|remote` + `adapters_config.executor.image.*`（DB，D15），Web 可切换、热加载、审计。MVP 每平台实例一个活跃 mode（按 tenant/layer 分流到不同 mode 为后续扩展）。
- **统一契约**（四模式 MUST 都满足）：worktree 挂载（D4）、工具链（预装或镜像）、CPU/内存/磁盘限制、网络出口策略（egress 白名单 / 关键 stack 断网）、超时、退出码捕获、stdout/stderr 流式回传 `request_events`。
- **K8s 与镜像仓库均非必需**：process 为零依赖默认；仅当需强隔离 / 多租 / 水平扩展才上容器或 K8s。
- **理由**：业界两派——容器派（Spacelift / Env0：worker image + registry，强隔离但重）vs 进程派（Atlantis ~8k 星 / Jenkins-tf：预装工具链 + 目录隔离，无 registry，广泛落地）。用户"可插拔 + 不锁定"诉求 ⇒ 两者都支持，process 为最轻默认。纯容器方案强制 registry+build 依赖（用户已识别该成本），process 模式消除该依赖；容器模式再用 ImageBuilder + 可插拔 registry 把构建路径打通。
- **备选**：①仅容器（强制 registry+build 依赖，违背轻量）②仅 process（多租隔离弱，不适合大规模 SaaS 化）③强制 K8s（过重）④Nomad（额外组件，用户未提）。
- **影响**：新增 `platform/internal/executor`（接口 + 4 实现 + 共享 contract）+ `platform/internal/executor/imagebuilder` + `ImageRegistry` 适配器；编排引擎与漂移检测均通过 Executor 执行（漂移只读 plan 走同一沙箱）；e2e 四模式 + 镜像构建各覆盖一遍（同一工单在四模式下退出码 / 日志 / worktree 一致）。详见 `docs/11-工具链版本与执行隔离.md` §3。

### D19 — 代码生成机制：自研 Go codegen + Phase 1 五阶段简化（D28 完整 9 阶段）+ 强制注入 + 契约驱动 for_each
- **决策**：①代码生成由平台自研 `platform/internal/codegen`（Go `text/template` 驱动）承担，**不**依赖 Terramate generate 生成业务 module 调用（Terramate generate 仅可选用于批量初始化静态骨架）；②Phase 1 使用 5 阶段简化管道：`contract → defaults → governance → user → dependency`，并可无损映射到 D28 的完整 S1-S9；③平台强制注入 tag（D18）/ state key（D6）MUST NOT 被任何来源覆盖；④**for_each 多实例不用 wrapper module**——在模块契约声明 `cardinality: single|list|map` + 实例标识变量，codegen 按契约在 main.tf 生成 TF 原生 `for_each`，原子模块保持单实例设计（业界共识，对照 HashiCorp 官方 module registry / Spacelift / Env0 均不用 wrapper）；⑤默认用 `for_each`（key = 实例标识，稳定）而非 `count`（索引删除触发整列重建）；⑥敏感变量（sensitive=true）占位符 + 运行期 secret 注入，不进 git 明文；⑦生成后强制 `terraform fmt`，确定性路径（同工单重跑产出相同代码）。
- **理由**：表单动态需多来源合并 + 优先级仲裁，Terramate generate 是静态模板不适合；Phase 1 5 阶段管道保留最小治理能力，Phase 2 再展开为 S1-S9 provenance；wrapper module 引入额外 state 路径（`module.wrapper["i"].module.atomic.x`）与版本双重维护负担，TF 原生 for_each 已足够（业界不用 wrapper 属反模式）；**catalog 项驱动 cardinality**（D25 详化：cardinality 在 catalog 项配置非模块契约）使 codegen 通用化，原子模块零感知多实例。
- **备选**：①Terramate generate 生成业务代码（静态模板无法合并动态表单）②wrapper module 包装多实例（反模式，state 复杂、维护负担）③用户手填 tfvars（违背产品化初衷）④count 替代 for_each（索引不稳）。
- **影响**：`platform/internal/codegen` + 契约提取（**纯 scalar，零侵入**）+ catalog 项 cardinality 配置（D25）+ 模板库；与 D3（foreach 运行）、D6（state key）、D18（tag）联动。详见 `docs/09-代码生成机制.md`。

### D21 — 审批双门禁：准入审批（pre-plan）+ 执行确认审批（pre-apply）；plan/apply 解耦为两次 Executor 调用
- **决策**：工单流水线设两道审批门，复用同一 `ApprovalEngine`（D11 DSL/RBAC/审计/超时），各为独立审批流定义（不同 YAML、不同审批人、不同条件）：
  1. **准入审批**（admission，pre-plan）：生成代码后、plan 前。判断"此申请是否被授权执行"。轻量——OPA 策略自动放行低危 + 单审批人（项目经理/资源 owner）审中高危；低危可配免审（策略直通）。目的：在花 plan 计算资源前过滤未授权/明显不当申请。
  2. **执行确认审批**（pre-apply，post-plan）：plan 完成后、apply 前。判断"此 plan 产物是否可安全落地"。**核心门**。审批输入 = plan 差异摘要 + Infracost 成本 + plan 后轻量漂移校验（若世界已变则告警/要求重 plan）。审批人 = 理解 diff 的人（DBA/运维/资源 owner），可与准入审批不同。
- **plan/apply 解耦（关键）**：plan 与 apply 是**两次独立 Executor 调用**，中间由状态机 `pending-approval` 阻塞：
  - **plan**：Executor 跑只读 plan → 解析产出（差异/成本/漂移校验）→ 上传 plan 产物到对象存储（关联 request_id）→ **销毁沙箱**（不占容器/进程等待人数小时的人审）。
  - **阻塞**：状态机置 `pending-approval`，挂起执行确认审批流，通知审批人，UI 展示 plan 差异 + 漂移状态。
  - **apply**：审批通过 → **新沙箱**检出同一 `pinned_commit` + 下载 plan 产物 → `TerramateAdapter.RunApplySavedPlan`（`terramate run -- terraform apply "<plan>"`）精确按已审 plan 执行（不重 plan）→ 回写 → 销毁。
- **漂移护栏**：apply 前若 plan 后漂移检测发现世界已变（state/云被改），平台告警并要求重 plan（避免 apply 已过期 plan）。
- **不是"二选一"**：不把两门合并成单审批（pre-plan 审=盲批、pre-apply 审=准入无门，各有缺陷），也不另造审批引擎（D11 足够通用）。同一引擎 + 流水线两阶段编排。
- **理由**：业界共识——Terraform Cloud（plan 权限 + confirm&apply）、Spacelift（access + plan approval）、Atlantis（PR + plan 注释 + apply 注释）、Env0 均"准入 + pre-apply 确认"两门。plan 是投机只读（安全），apply 是破坏性（必人审 plan 产物）。保持沙箱等人数小时人审不合理（Spacelift/Env0 均释放 worker）；plan 产物持久化保证 apply 精确执行已审内容。
- **备选**：①单门审批（准入或 pre-apply 二选一，各有缺陷）②保持沙箱 plan→apply 不释放（占资源等审）③apply 时重 plan（审批 diff 与实际执行不一致，违背审计意图）。
- **影响**：编排状态机（specs/06）显式两门 `generating → pending-admission? → planning → pending-approval(pre-apply) → applying`；`ApprovalEngine`（specs/10）支持"流水线阶段触发审批 + plan 产物作为输入"语义；plan 产物对象存储 + request 关联；漂移引擎提供 plan 后轻量校验 API；UI 审批详情展示 plan diff + 漂移。详见 `docs/12-审批引擎与流程.md`。

### D23 — 云账号与凭据管理：三层凭据模型（bootstrap/执行/个人）+ 团队↔云账号授权（申请过滤）+ OIDC 优先免长存 + 凭据注入 Executor（不入 git/日志）+ 角色模板化
- **决策（五段一体）**：
  1. **三层凭据模型**（按"作用域 + 风险"分层）：
     | 层 | 作用域 | 用途 | 形态 | 谁配置 |
     |----|--------|------|------|--------|
     | **Bootstrap / Admin**（引导） | 每云账号 **1 套** | 开通云账号、IAM 配置、配 OIDC trust、读账单、建子角色 | 长期 AK/SK（vault 级，**双人审批 + 周期轮换 + 全审计**） | 平台运维（仅 1–2 名守护人） |
     | **执行凭据**（Execution） | **per-team / per-bundle × per-cloud_account** | 平台代跑 `terraform/terramate` 时 assume 的 IAM 身份 | **OIDC 联邦优先**（短期 ephemeral STS，**无长期 AK/SK**）；fallback 长期 AK/SK（强制轮换） | 平台运维 + 团队 owner |
     | **个人**（Personal） | 每用户 | UI 操作 RBAC、查看、审批、提交申请 | **平台身份（OIDC token）/ 不发云 AK/SK** | 用户自助（无云凭据） |
     - **个人 MUST NOT 持有执行云凭据**——平台执行（平台代跑 terraform），不是人手跑；个人只走 RBAC。
     - **per-team 不是 per-person**：①执行主体是平台不是人；②team/bundle = 治理 + 成本归集 + 爆炸半径边界；③审计可追溯"哪个团队的执行身份动了什么资源"。
  2. **团队↔云账号授权（`team_cloud_grants`，驱动申请过滤）**：`team_cloud_grants(team_id, cloud_account_id, allowed_layers, iam_role_template, budget_quota, expires_at)`——显式声明"哪个团队可在哪个云账号下申请哪些层的资源"。**申请入口 `GET /catalog` 按调用者团队 join 此表过滤**：只展示该团队被授权的云账号 + catalog 项；无 grant 的云厂商/账号**不出现**在申请表单的"目标云"下拉。这就是"看部门开通了哪些云厂商/账号"的实现。
  3. **OIDC 联邦优先（业界共识）**：平台作 Trusted OIDC Issuer（每云账号配 `oidc:platform.example/assume-role/<team>` trust policy），Executor 执行时拿平台内部 short-lived token 换云 STS（aws `AssumeRoleWithWebIdentity` / alicloud `AssumeRoleWithOIDC`），**零长期凭据、按需签发、自然过期**。长存 AK/SK 仅作 fallback（云不支持 OIDC / 离线）。**业界**：Spacelift/Env0/Terraform Cloud/HashiCorp Vault 全部 OIDC-first（"动态凭据"是事实标准）。
  4. **凭据注入 Executor（不入 git/日志/codegen）**：
     - codegen 阶段对敏感字段用**占位符**（如 `__TM_SECRET_AWS_ACCESS_KEY_ID__`）写入 staging；
     - **运行期** `Executor.Run` 按 `(team, cloud_account, catalog_item)` 解析所需凭据 → 注入子进程/容器 `env`（`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN` 或 `AWS_WEB_IDENTITY_TOKEN_FILE`），**绝不写入 worktree/git/日志**；
     - 日志脱敏（正则屏蔽 AK 形态 + 已知 secret 值哈希替换）；
     - STS ephemeral 凭据仅活在单次 `Run` 内，结束销毁。
  5. **IAM 角色与权限模板化（catalog 声明 → 平台生成/校验）**：catalog 项声明 `required_permissions`（如 `dba/rds` 声明 `alicloud:rds:*` + `vpc:DescribeVSwitches`）；平台 bootstrap 凭据为每团队在云账号内 provision 标准 IAM 角色（`TmExec-DBA-RDS` / `TmExec-Business-ECS` / `TmExec-Platform-VPCAdmin`），角色策略由 catalog `required_permissions` 聚合生成（**最小权限**）；执行凭据 assume 匹配角色，OPA 二次校验"申请项 ↔ 角色权限"一致。
  6. **生命周期管理**：
     | 类型 | 轮换 | 注销 |
     |------|------|------|
     | OIDC ephemeral | 无需（按需签发，自然过期） | 撤 trust policy |
     | 长期执行 AK/SK | **强制周期轮换**（默认 90 天）+ 泄漏紧急轮换 | 删 grant 即失效 |
     | Bootstrap Admin | **双人审批**轮换 + 操作全审计 | 平台下线/迁移时 |
     - 平台定期扫描过期 / 将过期凭据 → 告警运维；泄漏检测（日志/GitHub secret scanning 同款）→ 强制轮换。
- **理由**：业界（Spacelift/Env0/TFC/Vault）—— **OIDC-first** + **平台代跑** + **per-team 而非 per-person** + **角色模板化**，避免给个人发云凭据（人离职/调岗即风险）。`team_cloud_grants` 是多团队分层治理的"白名单闸"，直接对接申请入口。Bootstrap 凭据最小化 + 双人审批是 vault 级管控共识。
- **备选**：①给每人发云 AK/SK（人离职即风险，无审计聚合，爆炸半径 = 个人，被否）②单一 admin 跑一切（爆炸半径 = 整个平台，无团队隔离，被否）③无 OIDC 全长存 AK/SK（轮换负担 + 泄漏面大，被否）④凭据写入 git/工作仓库（灾难性，被否）。
- **影响**：新增 `specs/16-云账号与凭据管理` + `docs/06-云账号与凭据管理`；新增表 `cloud_accounts` / `team_cloud_grants` / `cloud_credentials`（凭据元数据，密文存 Vault/KMS 或平台内嵌 secret store，DB 仅存引用） / `iam_role_templates`；`Executor.Run` 接凭据解析器；`GET /catalog` + `POST /requests` 校验 grant；OPA 增"申请项 ↔ 角色权限"规则；UI 增"云账号与凭据"管理页（云账号清单 + 团队授权矩阵 + AK/SK 生命周期 + OIDC/角色配置 + 申请页"目标云"下拉按 grant 过滤）。

### D22 — 工具链版本校验（三时机）+ 节点安装（自动优先/手动兜底）+ 镜像构建管线（BuildDriver 可插拔），贯通设计
- **决策（三段贯通）**：
  1. **版本校验三时机**：①注册时——提取模块 `required_version`/provider 约束，校验存在满足它的工具链 profile；②catalog 发布时——目录项声明目标工具链 profile（terramate + tf/tofu + providers 版本组合）；③**执行前**（orchestrator 调 Executor 前）—— `validateCompatibility(stack, sandbox)`：sandbox 是镜像（解 tag/manifest 取版本）或预装（取节点 toolchain manifest），用 semver 匹配 `stack.required_version` + provider 存在性 + terramate 版本；不兼容 MUST 阻断并给可操作提示（"切到镜像 X" / "节点装 provider Y"）。
  2. **节点安装（process 模式）**：**自动优先**——`platform/internal/installer` 按平台 Web 设的工具链基线（D15 存 DB）reconcile 节点：从平台 mirror/制品仓下载 → checksum 校验 → 装到 `/opt/tm/toolchains/<ver>/` + symlink active + 写 `toolchain_manifest`（含 `mode=process` 标签）；**手动兜底**——离线/air-gap 由 admin 投放二进制到约定目录，installer 仅注册（不下载）。节点 `toolchain_manifest` 是校验与可见性的**真相源**。**模式切换一致性**：admin 在 Web 切 Executor mode（如 process→k8s）后，触发 `reconcile_manifest` job——重跑 `validateCompatibility` 校验新模式下工具链是否可用（k8s 模式校验镜像 manifest，process 模式校验节点 manifest），不兼容则阻断执行并提示「切回 process 或构建 k8s 镜像 X」；manifest 加 `mode` 标签区分来源，切模式后旧 manifest 标 `superseded`。
  3. **镜像构建（container 类模式）**：Web 保存版本组合 → 异步入构建队列 → `ImageBuilder` 渲染 Dockerfile（pin 全部版本）→ `BuildDriver` 构建（**可插拔**：`docker`=本地/构建节点 docker daemon；`buildah`=daemonless 进程；`kaniko`=K8s 内无 daemon 构建）→ tag（版本组合）→ push 可插拔 `ImageRegistry`（Harbor/ACR/ECR/内嵌/文件中转）→ 灰度。**构建方式本身可进程化**（buildah/kaniko 无需 docker daemon），不强制宿主有 docker。
- **理由**：业界——Spacelift 自定义 worker image（你在 CI 构建push，平台拉）= "自带镜像"；Atlantis 预装工具链节点 = "自带节点"；HashiCorp TFC 托管（无自定义）。自研平台两派都要支持 + 平台代建（ImageBuilder）作为最省心默认。版本校验三时机借鉴 Spacelift（stack required_version vs worker image）+ Atlantis（runner toolchain 检查）。安装自动优先降低运维负担，手动兜底覆盖离线（air-gap 共识）。BuildDriver 可插拔避免锁定 docker daemon（kaniko/buildah 是 K8s/daemonless 主流，对照 Kubernetes 社区镜像构建事实标准）。
- **备选**：①仅手动安装（运维负担重、版本易漂）②仅 docker daemon 构建（K8s/daemonless 场景受限）③无执行前校验（不兼容到 plan 才报错，体验差）。
- **影响**：新增 `platform/internal/installer`（节点工具链 reconcile + manifest）+ `platform/internal/executor/imagebuilder`（Dockerfile 渲染 + BuildDriver：docker/buildah/kaniko）+ 版本校验三时机接入 registry/catalog/orchestrator；`toolchain_manifest`（节点级，DB + 节点本地双写）；镜像灰度 CI。详见 `docs/11-工具链版本与执行隔离.md` §2/§3.3/§5。

### D24 — stack 模型可配置化：Layer 规则模型（默认三层）+ PathGenerator 模板 + bundle 可选 + StackGranularity 策略
- **决策（四段）**：
  1. **Layer 规则模型（DB 表 `layers`，不硬编码层名）**：层定义全部可配置——`name` / `order`（层序，决定依赖方向）/ `owning_team_pattern`（如 `dba|middleware` 表达"DBA + 中间件两部门并列同层"）/ `path_template`（Go text/template）/ `depends_on`（下层依赖）。**出厂默认 = 三层**（Global/Middleware/Application），管理员可增删层（如加"security/compliance"第 4 层、或合并成 2 层）、改路径模板、改依赖方向。
  2. **PathGenerator（codegen 调用，路径不硬编码）**：`PathGenerator(layer.path_template, stack_metadata)` 渲染 stack 目录路径与 state key。模板变量：`env / team / bundle / component / layer / layer_order / custom_kv`。codegen MUST 调用 PathGenerator，MUST NOT 字符串拼接路径。
  3. **bundle 可选**：Application 层默认模板 `application/{{.team}}/{{if .bundle}}{{.bundle}}/{{end}}{{.component}}-{{.env}}`——小业务部门直接 `<team>/<component>`，大业务部门多产品线用 `<team>/<bundle>/<component>`。元数据 `stacks.bundle_id` NULL 表示无 bundle。两种路径平台都识别。
  4. **StackGranularity 策略（stack 粒度可配）**：默认 `per-component`（一个组件一个 stack，控制爆炸半径）；可选 `per-bundle` / `per-team` / `custom`（catalog 项声明 `stack_grouping`，如简单 SLB 规则 per-component，复杂微服务全套 per-bundle）。
- **理由**：业界（Spacelift space + stack 模板 / TFC workspace naming policy / Env0 project + environment / Atlantis 目录组织）**都不硬编码层名**，硬编码限制企业组织多样性（4 层加合规层 / 2 层简化 / Middleware 拆数据+消息两层等现实诉求）。bundle 强制反而给小团队添堵。PathGenerator 让"路径规则变更"成配置而非代码改动。
- **备选**：①硬编码三层（限制组织多样性，被否）②bundle 强制（小团队负担）③无 PathGenerator 直接字符串拼接（路径规则变更需改代码，被否）。
- **影响**：新增 `platform/internal/stackmodel`（Layer 规则集版本化引擎 + PathGenerator + StackGranularity 评估器 + MigrationPlanner/StateMover/Rollback/Sunset，D26）；DB 加 `layer_rule_set_versions` + `layer_logical_refs` + `stack_grouping_rules` 表（D24+D26）；catalog 项加 `layer_logical_id`（稳定身份，不直接绑版本）+ `stack_grouping` 字段；stacks 表加 `layer_rule_set_version_id` pin（D26）；codegen 接 PathGenerator（specs/05）。详见 `docs/02` §7 + `docs/04` §2.9。

### D25 — 模块零侵入：cardinality 调用方注入，模块定义单实例语义
- **决策（四段）**：
  1. **模块零侵入原则（最高约束）**：原子 Terraform 模块 MUST 保持标准单实例语义——`variables.tf` 全 scalar、`main.tf` 单实例 resource 定义、`outputs.tf` scalar 输出；平台 MUST NOT 要求模块作者按平台规范修改模块。**社区模块（terraform-aws-modules / 官方 alicloud 模块 / 任意标准 Terraform module）直接复用，零适配**。
  2. **cardinality 是 catalog 项元数据，不是模块契约**：`modules.variables_contract_json` 只存从 variables.tf 提取的**纯 scalar 契约**（零侵入）；多实例配置（`cardinality` / `instance_key` / `per_instance_fields` / `shared_fields`）存在 `catalog_items` 表（平台层配置），**MUST NOT 写入模块仓库**。模块 commit hash 与 cardinality 配置正交。
  3. **调用方注入（codegen 在 stack main.tf 注入 for_each）**：codegen 按 catalog 项 cardinality 在**生成的调用方**注入——`single` = 直接 `module "x" {...}` 无 for_each；`list` = `count = N`（同质）；`map` = `for_each = tomap({...})`（异构，key=角色名/实例标识）。per_instance 字段从 `each.value` 取，shared 字段直接注入；**模块内部完全不知道自己被 for_each**。
  4. **三种 cardinality**（同一模块可被不同 catalog 项以不同 cardinality 调用）：
     | cardinality | 调用方生成 | 适用 |
     |------------|-----------|------|
     | `single`（默认） | 直接 module 调用 | 单实例 RDS |
     | `list` | `count = N` + `each.value` 同质 | 3 台同规格 ECS |
     | `map` | `for_each = tomap({...})` + `each.value` 异构 | 3 角色不同规格 ECS（web/API/batch） |
- **理由**：`for_each` 是 Terraform **调用语法**不是**定义语法**——模块零感知是事实标准。HashiCorp 官方 registry 模块按单实例设计 + 消费者 for_each 是社区共识；Spacelift/Env0/TFC/Terramate 全部消费侧 for_each 不包装。**零侵入降低注册负担（社区模块直接用）、版本升级无迁移成本、避免 wrapper 反模式（state 路径复杂/版本双维护/diff 噪音）**。
- **备选**：①wrapper module 包装（反模式：state 路径 `module.wrapper["i"].module.atomic.x`、版本双维护、diff 噪音，被否）②模块内 for_each（侵入式，要求模块作者按平台规范改，社区模块不能用，被否）③cardinality 写入 modules 表契约（污染注册元数据，注册负担，被否）。
- **影响**：`catalog_items` 表加 `cardinality` / `instance_key` / `per_instance_fields` / `shared_fields` 字段；codegen 加 CardinalityInjector（specs/05）；**模块注册流程零改动**（仍提取 scalar 契约）；stack outputs.tf 由 codegen 自动聚合 map 输出。详见 `docs/09` §8 + `docs/04`。

### D24 + D25 衔接（codegen 单管道，两 Generator 协同）
- **关键**：D24（stack 模型）与 D25（模块零侵入）共享 **codegen 单一管道**，两个 Generator 是管道里的阶段，**都消费 catalog 项元数据，都不污染模块仓库**：
  ```
  [9 阶段参数解析管道（详见 specs/18 / docs/08）]
    S1 模块契约(scalar) → S2 catalog defaults → S3 layer rule(治理)
    → S4 env 上下文(D27) → S5 tenant 上下文(D27) → S6 team 策略
    → S7 跨层依赖 → S8 用户表单 → S9 平台强制注入(tag/state key)
     ↓
  [PathGenerator（D24）]  按 layer.path_template 渲染 stack 目录 + state key
     ↓
  [CardinalityInjector（D25）]  按 catalog.cardinality 注入 for_each/count 调用语法
     ↓
  [Render] Go text/template 渲染 .tf
     ↓
  [terraform fmt] → [write to worktree] → [git commit]
     ↓
  [Provenance] resolved_params_json 写入 requests（每变量 source/rank 审计）
  ```
- **元数据分工**：catalog 项同时承载 D24 与 D25 的配置——`layer` / `stack_grouping`（给 PathGenerator 决定路径）+ `cardinality` / `instance_key` / `per_instance_fields` / `shared_fields`（给 CardinalityInjector 决定调用语法）。`modules` 表只存纯 scalar 契约（零侵入）。
- **stack 划分 ↔ 模块调用的衔接契约**：一个 stack = 一个目录 = 一个 state key = 一份 `main.tf`（含 0..N 个 module block，每个 module block 按 cardinality 决定 single/count/for_each）。**stack 边界 = 逻辑治理单元**（一个服务/一个组件类别），不是规格/实例数——同服务不同规格 ECS 用 map cardinality 放一个 stack，跨服务/跨团队/跨环境拆 stack。
- **业界对照**：Spacelift（stack = 目录 + 内含 for_each 模块调用）/ Env0（environment = stack 概念）/ Terraform Cloud（workspace = stack）—— 都是"stack 目录 + 调用方 for_each + 模块零感知"的同构实现。

### D26 — 层规则集版本化与迁移操作模型：整体版本化（非单 layer）+ layer_logical_id 跨版本稳定 + per-stack Tier 分类 + Worker 自动 state mv + sunset 强制迁移窗口 + CMDB 路径同步

回应：D24 让 layer 规则可配置，但生产环境运行期改 layer 规则（reorg / 加层 / 改 path_template）会动到**已部署 stack 的 path 与 state key**——直接 UPDATE 是生产事故（state_key 漂移 → terraform 误判重建）。本决策定义安全的版本化与迁移操作模型。

**（1）版本化对象 = 整个分层方案（layer_rule_set），不是单 layer**。Global/Middleware/Application 是**一套协同的分层方案**，不是 3 个独立 entity。改任何层 = 整个 set bump v+1（v1→v2→v3...），旧版本 `status=superseded` 保留不可变。借鉴 K8s CRD 整体版本化（v1alpha1→v1beta1→v1）与 Spacelift blueprint 整体版本化，**不做字段级精细化**（同张表有的字段可变有的不可变 → 心智负担大、实现复杂）。

**（2）layer_logical_id（uuid，跨版本稳定）= 层的逻辑身份**。`catalog_items` 引用 `layer_logical_id`（稳定），不直接引用 version；`stacks` 创建时把 `layer_logical_id` + 当前 active `layer_rule_set_version` 解析出具体 `path_template`，pin 到 `stacks.layer_rule_set_version_id`。layer 规则升级时 catalog_items **不动**，自然只影响新 stack。

**（3）Tier 分类必须 per-stack，不是 per-change**。同一个 layer_rule_set 变更（v1→v2），对不同 stack 影响不同——平台对每个 stack dry-run 重渲染 path 对比：
- **Tier 1 Auto-migrate**：path 不变（如改了 owning_team_pattern / depends_on 方向 / 加了 custom_kv 变量但 stack 未引用）→ 自动 bump `stacks.layer_rule_set_version_id`，admin 仅审批
- **Tier 2 Assisted**：path 变可推导 → StateMover 在 Worker 跑 `terraform state mv` + 自动 plan 校验（exit 0=zero-diff）+ 失败自动从 StateBackup 回滚 → admin 审批 + 2 人签 + 静默期
- **Tier 3 Forked**：path 冲突 / 删层 / 不可逆变更 → 旧 stack 永久 pin vN，新 stack 走 vN+1，标 `deprecated_at` + `sunset_at`（如+6mo），期间必须 destroy+recreate 迁完

**（4）StateMover 默认 Worker 执行，不是「生成脚本给 admin 跑」**。terraform state pull/push 跨 backend 极易出错，让人手跑风险大。平台默认通过 Executor 在受控 Worker 跑（2 人审批 + 静默期），plan exit code 必须为 0 否则自动回滚；保留 Manual Override 给特殊场景。state mv 后**自动同步 CMDB**（`cmdb_resources.stack_path`）+ 跑一次 on-demand 漂移对账。**安全阀：StateMover 自动化受 feature_flag `state_mover_auto` 控制，默认 `false`（关闭）**——仅人工显式开启 + 双人审批门通过后才执行自动 state mv；关闭时仅生成迁移计划文档供人工 SOP 执行。

**（5）QuiesceMode 粒度 = 迁移批次**（不是 stack/team/全局三级）。MigrationPlanner 把 stack 分批（按 team 或风险等级），每批一个 freeze scope，期间该批 stack 工单/plan/apply 全部阻塞、漂移检测**自动静默**（不是全局暂停），静默期计入迁移窗口。**In-flight 工单处理**：冻结开始时该批 stack 所有未 apply 的 plan 产物标 `expired(quiesce)`，apply 前强制重 plan（防 v1 plan 套 v2 path）。

**（6）RollbackEngine 支持逆向 state mv**。v2 上线后发现 bug，从 v2 回 v1 也是 Tier 2（路径反向 mv），sunset 窗口内可逆。

**（7）8 步标准 Playbook**（所有 Tier 共用）：宣告（MigrationPlanner 输出 per-stack Tier + 影响面）→ 静默（冻结=批次）→ 备份（StateBackup + rollback_token）→ 干跑（Tier 1 自动 / Tier 2 Worker 跑 state mv + plan=0 / Tier 3 标 deprecated）→ 灰度（1 低风险 stack 先跑必须 plan=0）→ 分批（按 team 滚动）→ 验证（全 stack plan=0 + CMDB 同步 + 漂移对账）→ 回滚窗口（sunset 前可逆含逆向 mv）+ 必须有 v1↔v2 diff viewer。

**（8）核心不变量**：**state_key 不经 admin 显式操作不能漂移**。平台宁可阻塞也不自动改 state_key——自动重建资源是生产事故。

**业界对照**：Spacelift（blueprint 版本化 + per-stack adoption + preview changes UI）/ Argo CD（ApplicationSet 改了只影响新 app，旧 app templateGeneration 不动）/ TFC（workspace template 改不影响已部署 workspace）/ K8s Operator（CRD v1alpha1→v1 + ConversionWebhook + deprecation policy 5 版本窗口强制迁移）/ Consul Vault（config entry versioned + CAS + canary）—— **共识：preview 必备 / canary 必走 / rollback 必留 / 永不全量自动**。

### D27 — 环境与租户网络模型：Environment/Tenant 一等对象 + EnvironmentTenantBinding 三元组 + VPC-per-tenant-env 默认 + Account-per-tenant escape hatch + VPC 仍是 Global 层 stack（不引入新抽象）

回应：D1-D26 覆盖了 layer / stack / bundle / team / cloud_account 五个维度，但**多租户、多独立 VPC、env 维度授权全部空白**。当前 `stacks.env` 只是反规范化字符串，没有 env/tenant/VPC 的绑定关系，无法回答"这个团队在 prod 环境用哪个 VPC、哪段子网"、"外部客户 corp-a 怎么独立隔离"。

**（1）Environment 是一等治理对象，不是 stack 字段**。`environments` 表存：env_logical_id（dev/staging/prod/dr）+ stage 分类 + cloud_account_id + region + 默认 tag_namespace + network_topology 模式。`stacks.env` 字段保留为反规范化便利，权威以 `environments` 表为准（参考 Spacelift「environment = label + Contexts」与 TFC workspace + variable set 的分层）。**理由**：env 决定治理强度（prod 强审批 / dev 自动放行）、网络位置、成本预算基线——这些是「治理对象」属性，不该散落在每个 stack 上重复配置。

**（2）Tenant 是一等对象，默认 `platform-default` 单租户（"内部共享"语义）**。新建外部客户/独立 BU 时创建 tenant。**只保留两档 isolation_level**（业界趋势：内部平台/B2B SaaS 已收敛到 VPC 级，账号级退缩到强合规场景）：
- `vpc-per-env`（默认）：**一个 tenant × 一个 env = 一个独立 VPC**。如"租户 corp-a 的 test 环境 = 一个 VPC"。目录划分清晰（`tenants/corp-a/test/...`），state 命名空间简单（state_key 前缀天然隔离），跨租户通 CEN/VPC Peering。覆盖内部多 BU / B2B SaaS / 中等合规场景。
- `account-per-env`（escape hatch）：强合规/外部客户场景，每个 tenant × env 独立云账号。binding 多一个 `override_cloud_account_id` 字段，覆盖 `environments.cloud_account_id`。

**关键设计取舍**：**不保留 shared-vpc 档**（曾考虑但舍弃）。理由：①"内部多团队共享 VPC 省成本"场景用 `platform-default` 单租户表达即可（一个 VPC，所有团队共享），不需要专门模式 ②三档可配过度设计，UI/表/校验复杂度激增 ③业界 2026+ 已收敛到 VPC 级，shared-vpc 是退步。

**（3）EnvironmentTenantBinding = (env × tenant × layer) 三元组 → 网络作用域解析**。表字段：env_id + tenant_id + layer_logical_id + vpc_stack_id（引用 Global 层某个 VPC stack）+ subnet_blocks_json + security_group_base_id + override_cloud_account_id（仅 account-per-env 模式填）。**这是 codegen 参数解析管道 Stage 4「环境上下文绑定」的查询入口**——codegen 收到 (request.env_id, request.tenant_id, catalog.layer_logical_id) 后查此表，拿 `vpc_stack_id`（元变量）注入到 resolved_params。Stage 7（跨层依赖）用此 `vpc_stack_id` 生成 `terraform_remote_state` data source + 变量绑定。**S4 与 S7 协作**：S4 决定引用**哪个** VPC stack，S7 生成**如何**引用。三元组 UNIQUE，缺 binding 时 Application 层 stack MUST 被 codegen Stage 4 拒绝（结构化错误：请管理员配置 binding）。

**（4）VPC 仍是 Global 层的普通 stack，不引入新抽象**。`vpc-platform-prod`、`vpc-tenantA-prod` 都是 Global 层的 stack，输出 vpc_id/vswitch_ids。Binding 只是引用它们的 stack_id + remote_state key。**备选**：①引入 `vpcs` 一等表（多余抽象，VPC 生命周期 Terraform 已管）②直接把 vpc_id 字符串存 binding（不可变性问题）→ 舍弃。**影响**：Global 层需为每个 (tenant, env) 组合预创建 VPC stack。

**（5）team_cloud_grants 增加 env_scope_json 维度**。当前只有 (team × cloud_account × layer) 授权，新增 `env_scope_json`：空数组表示全 env；非空限定 env_logical_id 列表。理由：prod 环境的云账号授权应比 dev 严格（如 dev 允许自服务、prod 必须 DBA 审批），env 维度授权是实现"分级治理"的关键。

**（6）跨租户网络通过 Global 层 CEN stack 治理**。租户之间需要互通（如业务租户访问共享 DBA 租户的 RDS），由 Global 层的 CEN（阿里云云企业网）/ VPC Peering / Transit Gateway stack 统一治理。**不在 EnvironmentTenantBinding 表里**——binding 只声明"租户用什么 VPC"，路由互通是独立的网络治理 stack。借鉴阿里云 CEN 标准模式。

**（7）核心不变量**：
- Application 层 stack 创建时 MUST 有 (env, tenant, layer) 的 binding，否则拒绝
- isolation_level=account-per-env 时 EnvironmentTenantBinding.override_cloud_account_id 必填，且与 environments.cloud_account_id 不同
- EnvironmentTenantBinding.layer_logical_id MUST 在 layer_logical_refs 存在（D26 衔接）
- bindings.vpc_stack_id 引用的 stack MUST 是 Global 层（layer_logical_id 校验）
- vpc_stack_id 引用的 stack 的 outputs MUST 含 `vpc_id` + `vswitch_ids`
- **D27 binding vs D26 layer 版本化交互**：binding.vpc_stack_id 引用 **stack_id（稳定不随 layer 版本变）**，不引用 path。codegen Stage 4 解析链：`binding.vpc_stack_id → stacks 表 → stacks.layer_rule_set_version_id → layer_rule_set_versions.path_template → 实际 path`。layer v1→v2 迁移改 path 时，binding 零改动（仍引用同一个 stack_id），只是 stack 自身迁移后 path 变了。

**业界对照**：AWS Landing Zone（Account Factory，仅强合规场景推）/ 阿里云 Resource Directory + CEN（资源账号=escape hatch，主流是 VPC 级 + CEN 互通）/ HashiCorp Terra Cloud + Spacelift（env = label + variable sets + VPC binding，**事实上是 VPC-per-env**）/ Crossplane XR（Environment = composite，VPC 是 composed resource）/ Env0（environment promotion：dev→staging→prod 同模块不同 VPC）—— **业界共识（2026+）：内部平台默认 VPC 级隔离；账号级退缩到金融/政企/外部客户硬合规；shared-vpc 已退场**。

### D28 — 标签分层与参数解析管道：Tag 7 层来源 + 参数 9 阶段解析 + Provenance 审计 + 校验门 + 冲突规则明确化

回应：docs/14 §3 定义了 4 个 platform-mandated tag（platform-managed/team/bundle/stack），但**没有定义 tag 来源分层**——env/tenant/team/catalog/user 谁先谁后？docs/09 表单 tags 字段与 specs/05 强制注入互相冲突（用户填了被覆盖，体验割裂）。specs/05 定义了参数合并的 4 来源 + 优先级，但**缺 env/tenant/team/layer-rule 4 个业界必有的来源层**，**没有 provenance 审计字段**——无法回答"这个 vpc_id 值哪来的、为什么 encrypt=true"。

**（1）Tag 来源分层 = 7 层，优先级递减，但 L1 永远赢**：

| 层 | 来源 | 示例 | 谁配 |
|----|------|------|------|
| **L1 platform-mandated** | 平台常量 | `platform-managed=true` | 不可改（D7 强制注入） |
| **L2 env** | `environments.tag_namespace_json` | `env=prod`, `cost-center=CC-P-01` | 平台运维 |
| **L3 tenant** | `tenants.tag_namespace_json` | `tenant=corp-a`, `compliance=pci-dss` | 平台运维 |
| **L4 team/bundle** | `teams.tags_json` + `bundles.tags_json` | `platform-team=dba`, `platform-bundle=orders` | 团队 owner |
| **L5 stack** | codegen 自动派生 | `platform-stack=rds-orders-prod` | 自动 |
| **L6 catalog defaults** | `catalog_items.default_tags_json` | `app=order-service` | catalog 注册者 |
| **L7 user form** | `requests.form_json.tags` | `owner=zhangsan` | 申请者（受白名单约束） |

**冲突规则**：L1 永远赢，用户尝试覆盖 silently ignore + 审计告警；L2-L6 后层覆盖前层（如 L4 覆盖 L2 的同名 key）；L7 受 catalog_items.user_allowed_tag_keys 白名单约束（默认空数组 = 用户不能加任何自定义 tag，需 catalog 注册者显式开白名单）。**借鉴 AWS Tag Policies + Azure Policy 的强制 tag 模型**。

**（2）参数解析管道 = 9 阶段，每阶段 source + priority + 校验门**。这是 specs/05「四输入合并」的演化——四输入保留但细化为 9 阶段：

| # | 阶段 | source | 优先级 rank |
|---|------|--------|------------|
| S1 | 模块契约载入 | `module_versions.variables_contract_json` | 基线骨架（rank 8 兜底） |
| S2 | catalog defaults | `catalog_items.default_values_json` + `default_tags_json`（L6） | 7 |
| S3 | layer rule 注入 | `layer_rule_set_versions`（D26 pinned，stack 创建时锚定） | **2（治理硬规则）** |
| S4 | 环境上下文绑定 | `environments` + `environment_tenant_bindings` (D27)，注入 vpc_stack_id/subnet_ids/region/az_list/sg_base | 6（上下文类） |
| S5 | tenant 上下文绑定 | `tenants.tag_namespace_json` (L3) + tenant 网络 | 5 |
| S6 | team 策略 | `teams.policy_json`（allowed_regions、cost_cap、mandatory_tags L4） | **3（治理类）** |
| S7 | 跨层依赖注入 | `stack_dependencies` 解析图 → remote_state/data source | 6（上下文类，同 rank S4 优先） |
| S8 | 用户表单 | `requests.form_json`（含 L7 user tags） | **4（用户显式）** |
| S9 | 平台强制注入 | 平台常量：platform-mandated tags（L1）、ownership metadata | **1（绝对）** |

**最终优先级**：S9(1) > S3(2) > S6(3) > S8(4) > S5(5) > S4/S7(6) > S2(7) > S1(8)——**治理类（S3/S6/S9）高于用户类（S8）；用户显式高于上下文类（S4/S5/S7）；上下文高于 catalog 默认（S2）；契约默认兜底（S1）**。**同 rank tie-breaker**：S4(env binding) > S7(dep graph)——S4 注入 `vpc_stack_id` 元变量，S7 用它生成 remote_state。

**（3）Provenance 审计 MUST 写入 `requests.resolved_params_json`**。每个解析后的变量值必须含 `{value, source, rank, optional: rule_id/env_id/tenant_id/user_attempted_override}`。例如：
```json
{
  "instance_type": {"value": "mysql.n2.large.1c", "source": "user_form", "rank": 7},
  "vpc_id": {"value": "vpc-bp1xxx", "source": "env_context", "env_id": "...", "binding_id": "...", "rank": 5},
  "encrypt_at_rest": {"value": true, "source": "layer_rule", "rule_id": "rule-prod-encrypt", "user_attempted_override": false, "rank": 2},
  "tags": {"source": "merged", "layers_applied": ["L1","L2","L3","L4","L5","L6","L7"], "user_overrides_blocked": ["platform-managed","platform-team"]}
}
```
**为什么 provenance 必须存**：审批人看申请时需知道"这个 vpc_id 哪来的、为什么 encrypt 强制 true"，AI 审计追溯 (D17) 需要参数决策链路，故障排查 (D14/D26 迁移) 需要知道变量为什么是当前值。**借鉴 Crossplane Function pipeline 的 resource history + Spacelift "Mounted Files" 来源标注**。

**（4）校验门：每阶段可拒绝，拒绝时阻断 codegen 返回结构化错误**：
- S3 layer rule：用户表单 region 不在 allowed_regions → 拒绝（"layer rule 要求 prod 仅 cn-hangzhou/cn-shanghai"）
- S4 env context：(env, tenant, layer) 无 binding → 拒绝（"管理员未配置 prod+corp-a+Application binding"）
- S5 tenant context：tenant.kind=external 但 form 引用了 shared 资源 → 拒绝
- S6 team policy：成本预估超 **2x cost_cap（硬上限）→ 拒绝**（安全阀，不可覆盖）；**1x~2x cap（软上限）→ 标记 `cost_overrun=true` 透传 pre-apply 门**，D21 审批 DSL `when: cost_overrun` 插入成本升级审批节点（更高级别审批人）
- S9 platform mandatory：用户尝试传 `platform-managed=false` → 静默忽略 + 审计告警（不拒绝，因 L1 永远赢）

**（5）冲突解析示例（精确化）**：
- `instance_type`：defaults=S2(mysql.n2.medium) → user=S8(mysql.n2.large) → 最终 large（rank 4 < 7）+ provenance 记 user
- `vpc_id`：user 不传 → S4 注入 vpc-bp1xxx + provenance 记 env_context
- `encrypt_at_rest`：layer_rule=S3(true) → user=S8(false) → 最终 true（rank 2 < 4）+ provenance 记 user_attempted_override=true（治理违规留痕）
- `tags`：L1+L2+L3+L4+L5+L6 合并 → L7 用户加 owner=zhangsan（白名单内）→ 7 层合并；用户加 platform-managed=false → silently ignore + user_overrides_blocked 记录

**（6）核心不变量**：
- 每个 requests MUST 在 codegen 完成后写 `resolved_params_json`（每变量含 source + rank）
- L1 platform-mandated tags MUST 永远在最终 tags 中存在（codegen 后校验，缺失则报警）
- L7 user tags MUST 受 `catalog_items.user_allowed_tag_keys` 白名单约束（白名单外 → 拒绝）
- 同一变量的 source MUST 唯一（不允许两个 source 同时为真，rank 高者覆盖）
- 治理类阶段（S3/S6）拒绝 MUST 阻断 codegen，不能 silently fix

**业界对照**：Helm values 层级合并（user > parent chart > subchart defaults，每层有 source）/ Kustomize base+overlay（strategic merge patch，per-field 来源可追溯）/ Terraform 变量优先级（TF_VAR_* > tfvars > -var > default，业界已成标准）/ Crossplane Function pipeline（每个 Function 可 mutate + provenance 写入 resource history）/ Spacelift Contexts（多 Context mount 时 priority 显式配置 + UI 展示来源）/ AWS Tag Policies + SCP（mandatory tags + 组织级强制 + 用户不能绕过）—— **共识：分层来源 + 显式优先级 + provenance 可审计 + 治理永远赢**。

### D29 — 工作仓库拓扑：单仓 + 分层根目录 + Terramate 使用边界 + stack.tm.hcl 生成规范

- **决策**：工作仓库采用**单仓（monorepo）+ 分层根目录**拓扑，与 Terramate 源码原生设计一致。Terramate 在平台中的定位为**保留编排引擎**，使用边界精确到 4 个场景；codegen MUST 生成符合 Terramate 规范的 `stack.tm.hcl`（含 tags/after/watch/ID），使平台能利用 Terramate 的栈发现、依赖排序、标签过滤、变更检测能力。
- **理由（源码级证据）**：
  - ① `config/config.go:121-162` `TryLoadConfig`：向上递归找 `terramate.tm.hcl`（`required_version` 非空=根配置），**不是 git root**；所有 stack MUST 在同一项目根下。
  - ② `stack/manager.go:128-402` `ListChanged`：`git diff HEAD..base` 扫**整个 git 仓库**的变更文件→匹配 stack 目录；跨仓无法 diff→**变更检测失效**。
  - ③ `config/stack.go:304-341` `LoadAllStacks`：stack ID **全局唯一**（`^[a-zA-Z0-9_-]{1,64}$`，大小写不敏感），重复 ID 报 `ErrStackDuplicatedID`；`input.from_stack_id` 跨栈引用依赖同项目。
  - ④ `run/dag/dag.go`：`after`/`before` DAG 拓扑排序（Kahn 算法），`--parallel N` 无依赖栈并发。
  - ⑤ `config/filter/filter.go`：`--tags` 支持 AND（`:`）/ OR（`,`）/ NOT（`~`）语义，是租户隔离的执行层机制。
  - ⑥ `config/stack.go` `Watch` 字段 + `stack/manager.go`：`watch` 文件变更→下游 stack 标记 changed，实现跨层变更传播。
  - **分层多仓后果**：变更检测失效 + 依赖排序断裂 + 跨层 remote_state 跨仓极难 + D26 state mv 跨仓不可能。
- **目录拓扑（layer-first + tenant/env 在 stack 名中）**：
  ```
  infra-repo/                              ← 单 bare repo，平台管理
  ├── terramate.tm.hcl                     ← 项目根（required_version 非空）
  ├── global/                              ← Global 层根目录
  │   ├── vpc-{tenant}-{env}/              ← 每个(tenant,env)一个 VPC stack
  │   │   └── stack.tm.hcl                 ← tags:["layer:global","tenant:X","env:Y"]
  │   ├── cen-{env}/                       ← 跨租户 CEN（不属任何租户）
  │   │   └── stack.tm.hcl                 ← tags:["layer:global","cross-tenant","env:Y"]
  │   └── ack-{tenant}-{env}/              ← ACK 集群等其他 Global 栈
  ├── middleware/                          ← Middleware 层
  │   └── {tenant}/{component}-{env}/      ← DBA/中间件团队管
  └── application/                         ← Application 层
      └── {tenant}/{team}/[bundle/]{component}-{env}/
  ```
- **Terramate 使用边界（4 个场景，不重写）**：
  | 场景 | Terramate 能力 | 平台如何调用 |
  |------|----------------|-------------|
  | ① 栈发现+依赖排序 | `terramate run` 按 `after` DAG 拓扑序执行 | codegen 生成 `after`，平台调 `terramate run --tags "..." -- terraform plan` |
  | ② 标签过滤 | `--tags "tenant:X,env:Y,layer:Z"` | codegen 生成 tags，平台按工单 tenant/env 构造过滤条件 |
  | ③ 变更检测 | `terramate list --changed` | D5 漂移检测 + D16 CICD 选择性执行（**与 D5 互补不冲突**：Terramate 检测代码变更，D5 检测状态漂移） |
  | ④ 并行执行 | `--parallel N` | D20 Executor 管沙箱隔离（**不同层面**），Terramate 管栈间并行 |
- **平台不用 Terramate 的能力**：`terramate generate`（D19 自研 codegen）、`input.from_stack_id`（用 `terraform_remote_state` 替代，标准 Terraform 零依赖）、Terramate Cloud（自建）。
- **stack.tm.hcl 生成规范（codegen MUST 遵守）**：
  ```hcl
  # codegen 生成的 stack.tm.hcl 模板
  stack {
    id    = "{layer}-{tenant}-{team}-{component}-{env}"  # PathGenerator 输出，全局唯一
    name  = "{component} for {team} in {env}"
    tags  = [
      "layer:{global|middleware|application}",  # 层过滤
      "tenant:{tenant}",                         # 租户隔离
      "env:{env}",                               # 环境过滤
      "team:{team}",                             # 团队归属
    ]
    after = ["{上游 stack 的相对路径}"]   # 跨层依赖（如 Application after Global VPC）
    watch = ["{上游 stack}/outputs.tf"]  # VPC 输出变了→本 stack 重 plan
  }
  ```
  - **ID 规范**：`{layer}-{tenant}-{team}-{component}-{env}`，全小写，`-` 分隔，≤64 字符。PathGenerator 同时输出路径和 ID。
  - **tags 是租户/环境/团队隔离的执行层机制**：平台按工单上下文构造 `terramate run --tags "tenant:X,env:Y"` 过滤条件，Terramate 只发现匹配的 stack。
  - **after 声明依赖序**：Application stack `after` 同租户同 env 的 Global VPC stack；跨租户 CEN stack `after` 所有关联 VPC stack。
  - **watch 声明变更传播**：Application stack `watch` 上游 VPC 的 `outputs.tf`，VPC 变更→Application 自动标 changed→下次 plan/apply 时感知。
- **VPC 租户模式三层边界模型**：
  - **① 平台治理边界**（Terramate 不感知）：RBAC（用户只看/改自己 tenant 的 stack）+ EnvironmentTenantBinding（tenant×env×layer→VPC stack 引用）+ 审批路由（cross-tenant→平台运维签）+ team_cloud_grants（tenant→cloud_account 授权）。
  - **② Terramate 执行边界**：栈发现 + 依赖排序 + tags 过滤 + 变更检测 + 并行执行。**Terramate 层面没有"租户"概念**，只认 stack + tags。
  - **③ Terraform 状态边界**：每 stack 独立 state_key（PathGenerator 渲染含 tenant/env），`terraform_remote_state` 跨层引用（codegen 注入）。
- **Worktree 兼容性**：Terramate 在 worktree 中完全正常工作（`config/config.go` 基于文件系统路径定位项目根，不依赖 git root）。平台为每工单创建独立 worktree（D4），在 worktree 中调 `terramate run --tags "..." -- terraform plan/apply`。
- **业界对照**：Atlantis/Terrateam 均单仓模式 / Spacelift 支持 multi-repo 但推荐单仓（stack 依赖天然同仓）/ Terraform Cloud workspace 命名约定等价于单仓目录结构 / Crossplane XR composite 同集群不跨集群 —— **共识：单仓 + 分层根目录是 IaC 编排的主流最优解**。
- **影响**：`platform/internal/codegen` MUST 生成 `stack.tm.hcl`（新增 Generator 阶段，在 Render 前）；`platform/internal/executor` MUST 通过 `terramate run --tags` 调用（而非直接 `terraform plan`）；`docs/02` path_template MUST 同时输出 stack ID；`docs/10` 补充 Terramate worktree 调用方式；`docs/09` 补充 stack.tm.hcl 生成规范。详见 `docs/09-代码生成机制.md` §6 + `docs/10-执行目录治理.md` §5。

### D30 — Break-Glass 紧急模式 + State 敏感字段保护 + 供应链 sha256 pin + IAM 聚合校验 + STS 缓存限制（安全加固）

- **决策**：五项安全加固合并为一个设计决策点：
  - **①Break-Glass emergency_mode**：生产故障时双人 break-glass 凭据（独立 vault path `secret/tm-break-glass/{cloud_account}`，仅 2 名 platform-ops oncall 持有，每季轮换）→ 提交 emergency 工单绕过双门禁审批 → **自动全程录屏**（Executor session recording 存对象存储 90 天）→ 操作后 **24h 内必须补审批回填**（`approval_runs` 标 `gate=break-glass-retroactive`，超时未补→自动 incident + CISO 邮件）→ emergency 操作触发 platform-ops oncall + CISO 实时通知。emergency_runs 表记录：`{request_id, break_glass_operators[2], reason, recording_url, retroactive_approval_id, retroactive_deadline}`。
  - **②State 敏感字段保护**：state 后端强制服务端加密（KMS CMK / 云 KMS）；CMDB ingester MUST strip 已知敏感字段（按 `sensitive_field_blacklist`：`*password*`/`*secret*`/`*private_key*`/`*certificate*`）；state download API 走 RBAC 高权限（仅 `platform-admin` + `team-owner` + 审计 + IP 白名单）。
  - **③Terramate CLI sha256 pin**：D1 版本 lock 追加 sha256 哈希校验——CI 下载 Terramate 二进制后 MUST 校验 sha256（`terramate_checksums.txt`），不匹配阻断 CI + 告警。供应链安全防 tampering。
  - **④IAM policy 聚合校验**：D23 catalog 项 `required_permissions` 聚合生成 IAM policy 后，MUST 经 OPA 二次校验：禁 `Action: "*"` / 禁 `Resource: "*"` / 禁通配 region（`*` region 仅 platform-admin 可批）/ 禁 root account 操作。聚合后 policy 随 catalog 注册存 `iam_role_templates.policy_json`，OPA 校验失败→catalog 注册拒绝。
  - **⑤STS 缓存限制 + 脱敏增强**：OIDC 联邦 STS token 仅活在单次 `Executor.Run` 进程内（env 注入，进程退出销毁，**不落盘不缓存到节点**）；日志脱敏在正则基础上增加 base64 解码扫描（检测 base64 编码的 AK/SK）+ 已知 secret 值 SHA256 前缀匹配（CredentialResolver 把当天用过的 secret hash 注入日志过滤器）。
- **理由**：①Break-Glass——生产故障等不起审批，但必须可追溯（业界共识：TFC/Spacelift 均有 emergency run + retroactive approval）；②State 加密——Terraform state 含明文密码是已知风险（TF 官方文档 + CIS Benchmark 均要求 state encryption）；③sha256 pin——供应链攻击（SolarWinds 模式）是 2024+ 安全趋势；④IAM 聚合校验——多 catalog 项聚合可能合并出 `Action: *` 过宽权限（AWS IAM Analyzer 共识）；⑤STS 不缓存——短命 token 缓存=泄漏窗口扩大。
- **影响**：新增 `emergency_runs` 表（docs/04）；`platform/internal/security/breakglass`（凭据管理 + 录屏 + retroactive tracking）；`platform/internal/cmdb/sensitive_stripper`；CI 加 sha256 校验 step；catalog 注册加 OPA policy 校验；Executor 日志加 base64 扫描。
- **业界对照**：Terraform Cloud emergency run / Spacelift tracked run + retroactive / AWS IAM Analyzer + Access Analyzer / Terraform state encryption (CIS Benchmark 2.0) / SLSA framework sha256 verification。

## 05-风险与权衡

- **Terramate CLI 输出变更破坏解析** → 锁定 Terramate 版本 + 对所依赖的子命令输出做契约测试（golden file）。
- **平台与 Terramate 同仓库的边界污染** → `platform/` 独立目录 + golangci 规则禁止 platform 包反向依赖被保护目录之外的内部；CI 跑 `make test` 验证 Terramate 主仓库不受影响。
- **漂移检测打满后端** → 全局限流 + 分层时间窗（Global 低频、Application 工作时间外）+ 退避。
- **多团队并发改同一工作仓库** → 每工单独立 git worktree + 分支锁；详见 `docs/10-执行目录治理.md`。
- **外部依赖（OPA/Infracost）故障** → 适配器接口化 + 降级策略（如 Infracost 不可用时仅告警不阻断）。
- **元数据与实际 stack 漂移** → 对账任务定期校验元数据 ↔ 工作仓库 ↔ 远程 state 三者一致。
- **平台下线风险** → 平台是独立外层，工作仓库内的 Terramate 配置可独立运行，无锁定。

## 06-迁移与上线

- **Phase 1（最小可用闭环）**：固定三层 + `platform-default` tenant + 单云账号 + 5 阶段简化 codegen + 单 pre-apply 审批 + 单 Executor 模式，跑通 `标准模块 → 表单申请 → 生成代码 → plan → 人审 → apply → 审计/CMDB index`。
- **Phase 2（生产治理）**：双门禁、完整 9 阶段参数管道与 provenance、OIDC-first 执行凭据、漂移检测、存量导入 read-only→changeable 生命周期、CI/CD gate、CMDB reconcile 与基础成本视图。
- **Phase 3（高级平台化）**：多 tenant/account-per-env、Layer 规则版本化、半自动 StateMover（默认关闭）、AI/MCP/skills、FinOps 优化建议、云侧 inventory 孤儿检测。
- **明确后置**：自动 StateMover、FinOps 账单核销、孤儿自动释放、四 Executor 全量实现、复杂审批 DSL、AI/MCP 不进入 Phase 1 主链路。
- **回滚**：平台独立部署，任何阶段下线都不影响 Terramate 与已部署资源；资源级回滚走人工 SOP，不做自动 destroy/apply 旧 commit。

## 07-测试策略

### 7.1 单元测试
- `go test ./platform/...`：覆盖代码生成、状态机、RBAC、漂移比对、git 适配器（内存 fake repo）、9 阶段参数解析管道、provenance 写入、tag 7 层合并。
- **codegen 确定性测试**：同一工单重跑 generating → 断言 git diff 为空（路径 + 文件内容 + stack.tm.hcl 完全一致）。

### 7.2 Terramate 契约测试（golden file）
- 锁定 `terramate run --tags ...` / `terramate list --changed` 的**输出格式**（stdout/stderr/exit code）为 golden file。
- 每次 Terramate 版本升级 → 回归 golden file，格式变更阻断 CI。
- 覆盖场景：tags 过滤、DAG 排序、变更检测、worktree 兼容性。

### 7.3 端到端集成（e2e）
在 `e2etests/platform/` 跑全链路：
1. **D21 双门禁端到端**：form → codegen → stack.tm.hcl 校验 → plan → 准入审批 → plan 产物上传 → 执行确认审批 → apply → 资源回写 → 断言状态机迁移正确。
2. **D26 迁移安全**：layer rule v1→v2 → Tier 2 stack state mv → plan=0 校验 → rollback → 断言 state 恢复。
3. **D28 参数管道**：同一 catalog 项在 dev（自助）/ prod（强治理）下 → 断言 S3 layer rule 注入 + S9 强制 tag + provenance 完整。
4. **漂移检测**：apply 成功 → 手动改云资源 → 漂移检测跑 → 断言 drift_record 正确 + 事件触发。

### 7.4 多云适配器 contract test
- 定义 `CloudAdapter` contract test 接口：tag 约束校验 / 凭据注入 / state backend 读写。
- 每个云厂商实现（alicloud/aws/azure）MUST 通过同一套 contract test（参数化测试）。

### 7.5 性能与并发
- worktree 并发压力（100 并发工单 → 断言无 git lock 死锁、无 worktree 泄漏）。
- `terramate run --parallel N` 在 500+ stack 仓库中的延迟基准。

### 7.6 Terramate 主仓库回归
- `make test` 与 `make build` 必须保持绿色，证明 `platform/` 代码未侵入 Terramate 核心包。

## 07b-可观测性

> 平台作为长驻服务，MUST 具备生产级可观测性。原则：结构化日志 + 分布式 tracing + 关键 metric + 告警。

- **Metrics**（Prometheus 兼容）：
  - `tm_plan_duration_seconds`（按 layer/env/tenant 维度）、`tm_apply_duration_seconds`
  - `tm_drift_coverage_ratio`（已检测/总 stack）、`tm_drift_detected_total`
  - `tm_approval_wait_seconds`（按 gate=pre-plan/pre-apply 维度）
  - `tm_executor_queue_depth`、`tm_executor_active_count`（按 mode=process/container/k8s）
  - `tm_worktree_active_count`、`tm_worktree_stale_count`
  - `tm_cost_estimate_delta_usd`（Infracost 预估 vs 云账单实际差异）
- **Tracing**（OpenTelemetry）：
  - trace 贯穿：API handler → orchestrator → codegen(9 阶段) → git commit → Executor → terramate run → terraform plan/apply → 回写。
  - 每个工单一个 root span，`request_id` + `tenant_id` + `env_id` 作为 trace 属性。
- **Logging**（结构化 JSON）：
  - worktree 级关联（`worktree_id` + `request_id` 贯穿日志链）。
  - plan/apply stdout/stderr 捕获到对象存储（关联 artifacts 表），日志正文只记摘要 + 引用。
  - 敏感字段脱敏（AK/SK 正则 + secret hash 替换，详见 specs/16）。
- **Alerting**：
  - 漂移检测发现 → 通知 stack owner team。
  - 审批超时（escalation 触发）→ 通知升级审批人。
  - Executor 失败率 > 阈值 → 通知 platform-ops。
  - 成本预算超限 → 通知 team lead + FinOps。

## 07c-运维韧性 + 灾难恢复（Operational Resilience）

> 回应架构评估 §3.5 / §6 Chaos Review / §12 P0 中的 12 项运维正确性缺口。每项 MUST 在实现时落地。

### 1. 并发正确性（双锁 + 幂等键 + UI advisory lock）
- **同 stack 并发 apply race**：orchestrator 在 `planning`/`applying` 前 `SELECT ... FOR UPDATE WHERE id = stack_id`（PG 行级锁）+ state backend DynamoDB lock = **双锁**；行锁失败→返回 409「该 stack 正被工单 #X 占用」。
- **用户双击防护**：requests 表加 `idempotency_key VARCHAR UNIQUE`（`sha256(user_id + catalog_item_id + form_hash + 24h_window)`），重复 POST → 返回已有 request_id + 当前状态，不建新工单。
- **UI advisory lock**：工单进入 `planning`/`applying` 时，stack 详情页显示「该 stack 被工单 #1234 占用中，预计 N 分钟」。

### 2. Plan 产物 TTL + TF 版本双校验（D21 增强）
- plan artifact 加 `expires_at = plan_completed_at + 4h`；apply 前校验 `now < expires_at`，过期 → 强制重 plan + 重 pre-apply 审批。
- plan artifact 加 `tf_version_sha256`；apply sandbox 的 terraform 版本 MUST 与 plan 时一致（sha256 匹配），否则拒绝 apply。

### 3. State Backend 健康检查 + Circuit Breaker
- orchestrator 启动时 + 每 60s 跑 `state_backend_health_probe`（S3/OSS HeadObject + DynamoDB CreateTable describe）。
- 不可用 → circuit breaker open → 全工单自动转 `blocked` 状态 + 通知 oncall；恢复 → circuit close → blocked 工单自动续跑。
- state 后端 **versioning + object lock (governance mode) + 跨区复制**（S3 cross-region replication / OSS 同区域冗余）；月度 restore 演练。

### 4. Apply 中断恢复 SOP
- apply 超时（默认 30min）→ state lock DynamoDB TTL 自动释放 → 工单标 `failed(interrupted)` → **不自动重试 apply**（Terraform apply 中断可能 state 半写入）。
- 人工 SOP：`terraform state list` 检查 → `terraform plan` 确认差异 → 手动 `terraform apply` 或 `terraform destroy` 清理半成品 → 工单标 resolved。

### 5. 回滚失败 SOP（资源级）
- **平台不自动回滚资源**（destroy+apply 旧 commit 风险极高）。
- 资源级回滚 = 人工 SOP：①git revert 到旧 commit ②`terramate run -- terraform plan`（看 diff）③人工评估 ④`terramate run -- terraform apply`（用旧 code）。
- D26 RollbackEngine 仅针对 **layer 迁移** state mv 回滚，不覆盖资源级回滚。

### 6. Executor Heartbeat + 失败转移
- Executor 进程/容器启动后每 **60s** 发 heartbeat（`executor_heartbeats` 表或 Redis key TTL）。
- 失联 **5min**（5 次心跳未到）→ 标 `unhealthy` → 该节点工单自动失败转移到健康节点（D20 remote 模式）或标 `failed(retry)`（process 模式）。

### 7. 云 API 限流 + 自适应退避
- 全局退避器（per cloud_account）：`rate_limiter` 配额表 `{cloud_account_id, api_per_sec, burst}`。
- plan/apply 调用云 API 前过退避器；429/Throttling → 指数退避（base=1s, max=60s, jitter）→ 超过 3 次退避 → 工单标 `failed(throttled)` + 通知 oncall。
- 漂移检测额外限流：分层时间窗（Global 低频 / Application 工作时间外）+ per stack 间隔。

### 8. 凭据过期预警 + 双 Key 过渡
- OIDC 联邦 STS：无过期（ephemeral），不预警。
- 长期 AK/SK：`cloud_credentials.expires_at` 提前 **14 天**预警 → 自动触发轮换 job → 新旧 **双 key 并存 7 天**（两条 cloud_credentials 记录，Executor 优先用新 key，7 天后旧 key 标 `revoked`）。

### 9. 平台升级窗口策略
- 升级前 **30min** 停止接受新工单（`platform_maintenance` flag）+ 存量工单等自然结束或人工 pause。
- DB migration MUST 前向兼容（新代码读旧 schema）+ 后向兼容（旧代码读新 schema，多加列不删列不改类型）。
- 蓝绿部署：新版本启动 → 健康检查通过 → 流量切换 → 旧版本 keep 10min for rollback → terminate。

### 10. PG 事务 vs Git Commit 跨事务补偿
- codegen 流程：①生成代码到临时目录 ②`git add + commit` ③DB 写 `requests.pinned_commit`。
- **故障窗口**：②成功但③失败 → `pinned_commit` 悬空（git 有 commit 但 DB 无引用）。
- **补偿机制**：reconcile job 每 5min 扫 git log 最近 30min 的 commit → 查 DB `requests.pinned_commit` 是否存在 → 不存在则 `git reset --hard HEAD~1` 丢弃孤儿 commit。
- codegen 临时目录 → 验证 → 原子 `mv` 到 worktree（失败丢弃整个临时目录，不污染 worktree）。

### 11. 灾难恢复 SOP（元数据 DB 完全丢失）
- **日备**：PG pg_dump 每日 → 对象存储（跨区）。
- **工作仓库 = source of truth**：stack 目录 + stack.tm.hcl + .tf 文件全在 git，不依赖 DB。
- **可恢复**：teams/modules/catalog_items/stacks（从 git 目录反推）+ environments/tenants/binding（从 stack.tm.hcl tags 反推）+ drift history（从 state + git 重建）。
- **不可恢复（需人工补录）**：approval_runs 历史审计 / audit_logs / cost_records 历史账单 / identities 用户映射。
- **季度演练**：恢复测试（pg_restore → 启动平台 → 验证核心功能）。

### 12. 漂移堆积降级
- 某 stack 漂移检测连续 **>N 次/天**（默认 5）→ 该 stack 自动 `drift_pause` 暂停新申请 + 通知 stack owner team「请先处理漂移」。
- 漂移全平台 **>M 次/小时**（默认 50）→ DriftDetector 降级到天级扫描 + 告警 oncall。

### 13. 跨层依赖反向变更阻断（L3）
- Application stack `watch` Global VPC outputs，但 VPC 删除/重大变更只触发下游 plan——**不阻断 VPC apply**。
- **修复**：orchestrator 在受理 Global 层 stack 的 destroy/major-change 工单时，MUST 检查 `stack_dependencies` 下游（谁依赖我）→ 有活跃下游 → 拒绝 destroy 工单（返回「下游 N 个 stack 依赖此资源，请先迁移」）+ 通知下游 owner team。
- Global 层 stack 的 **create/update** 不阻断（只有 destroy 阻断）。

### 14. Cloud Account 注销 Cascade（L6）
- `cloud_accounts.status` 从 `active` → `deprecating` 时，触发 cascade job：①`team_cloud_grants` 标 `status=revoked`（新申请过滤排除）②`cloud_credentials` 标 `status=revoked` + 触发轮换 SOP 中止 ③`iam_role_templates` 标 `archived` ④执行中的工单若依赖此 cloud_account → 标 `failed(cloud_account_deprecated)` + 通知。
- cascade 完成后 `cloud_accounts.status=deprecated`（终态，不可恢复，审计保留）。

### 15. Catalog 项 Sunset 三态机（L5）
- `catalog_items.status`：`active`（正常申请）→ `deprecated`（禁新申请，已创建的 stack 可继续 plan/apply）→ `archived`（拒引用，已创建 stack 标 `catalog_archived` 需迁移）。
- `deprecated` 时申请页隐藏该 catalog 项；`archived` 时 codegen 拒绝引用（返回「catalog 项已归档，请迁移到替代项 X」）。
- 管理员可通过 catalog 管理页操作 sunset（需标替代 catalog_item_id）。

## 07e-SLO + 容量模型 + 基础设施前置条件

### SLO 定义

| SLI | 目标 SLO | 窗口 | 错误预算 |
|-----|---------|------|---------|
| 平台 API 可用性 | 99.9% | 30 天 | 43min/月 |
| Apply 成功率（排除业务侧错误） | 99.5% | 30 天 | 3.6h/月 |
| Apply 中位延迟 | < 8 min | 7 天 | — |
| Apply P99 延迟 | < 30 min | 7 天 | — |
| 变更失败率 | < 2% | 30 天 | — |
| MTTR（平台侧故障） | < 30 min | 季度 | — |
| 漂移检测覆盖率 | > 95% | 7 天 | — |
| State Lock 等待 P99 | < 60s | 7 天 | — |

### 容量模型（small / medium / large 三档基线）

| 维度 | Small | Medium | Large |
|------|-------|--------|-------|
| Stack 数 | < 100 | 100-500 | 500-2000+ |
| 并发工单 | < 5 | 5-20 | 20-100 |
| 团队数 | < 10 | 10-50 | 50-200 |
| PG 规格 | 4C/8G | 8C/16G | 16C/64G + read replica |
| 对象存储 | 单桶 1TB | 单桶 5TB + versioning | 多桶 + 跨区复制 |
| Executor 节点 | 2 (process) | 5 (process/k8s mix) | 10+ (k8s autoscale) |
| Worktree 池 | 10 | 50 | 200+ (GC 周期 1h) |
| Vault | 单节点 dev mode | 2 节点 HA | 3+ 节点 HA + DR replica |

**自动扩缩容触发线**：worktree 池利用率 > 70% → 加 Executor 节点；PG 连接池 > 80% → 加 read replica。

### 基础设施前置条件（MUST 满足否则平台不可用）

| # | 前置条件 | 影响 | 要求 |
|---|---------|------|------|
| 1 | **NTP 时钟同步** | AK/SK SigV4 签名、plan TTL、审批超时全依赖时钟精度 | Executor 节点 MUST 装 chrony/ntpd + 启动时校验 `clock_offset < 500ms`，超标拒绝启动 |
| 2 | **DNS 可用性** | terraform init / provider download / registry mirror 依赖 DNS | 生产环境部署本地 DNS cache（dnsmasq/coredns）+ mirror 本地缓存 |
| 3 | **Provider mirror 本地缓存** | mirror 不可用 = 全平台 plan 失败 | provider lock.hcl + 节点本地 mirror 缓存（`/opt/tm/providers/`）+ init 失败降级提示「mirror 不可用，请联系运维」 |
| 4 | **OIDC JWKS 自动刷新** | issuer 证书过期 → 所有登录失效 | Verifier 定期拉 JWKS（15min cache）+ 过期前 7 天告警 `oidc_jwks_expiring` |
| 5 | **Terramate CLI sha256 pin** | 拉错版本 = 生成代码不一致 | CI 下载后校验 sha256（D30 ③）+ 节点预装版本校验 `terramate version` hash |

## 08-开放问题

- 平台最终是否拆独立 Go module（倾向：先同仓库 `platform/`，后续按需拆）。
- 工作仓库拓扑：~~单仓分层目录 vs 分层多仓~~ → **已定 D29：单仓+分层根目录**（Terramate 源码原生设计）。
- 工单/审批流引擎：自研轻量状态机 vs 引入 Temporal（倾向：先自研，规模上来再引入）。
- 模块校验是否在注册时强制 `terraform init`（成本较高）vs 仅 `validate`（倾向：默认 validate，可选启用 init）。

## 09-合规可选模块（声明性，按需启用）

> 回应架构评估 §7.6 / §7.8 / §7.9。以下能力**非默认**，仅在特定合规要求下启用。目标架构预留接口，实现按需。

### 9.1 审计完整性证明（HMAC Chain + Immutable Storage）
- **触发条件**：SOX / PCI-DSS / 等保三级 客户。
- **机制**：`compliance.audit_hmac = true` 时启用——audit_logs 双写到 S3 object lock（governance mode，不可删）+ 每日 HMAC chain 签名（每条 log 的 hash 包含前一条 hash，形成链式证明）+ 季度审计员可独立验证链完整性。
- **不启用时**：audit_logs 仅 DB append-only（默认行为，满足内部审计需求）。

### 9.2 多租户物理隔离 Escape Hatch
- **触发条件**：强合规客户（金融 / 外部硬隔离要求）不接受单仓 git log 跨租户可见。
- **机制**：`tenant.isolation_level = account-per-env` + 独立 git 仓库 + 独立 backend bucket + 独立 DB schema（per-tenant PG schema 或独立 DB 实例）。accept 单租户运维成本翻倍。
- **不启用时**：默认单仓 + 平台 RBAC（tags 过滤 + 行级 tenant_id 隔离），满足内部多团队治理。
- **最低保障**：DB 行级 RLS（Row-Level Security）——所有业务表强制 `tenant_id` 列 + PG RLS policy `WHERE tenant_id = current_setting('app.tenant_id')`，即使 SQL 拼错也不跨租户泄露。

### 9.3 国密支持声明（等保 2.0 三级）
- **触发条件**：等保 2.0 三级认证客户需声明国密 SM2/3/4 支持。
- **机制**：设计预留 `CryptoProvider` 接口（`platform/internal/crypto`），默认实现 = 标准 TLS/RSA/SHA256；等保客户启用 `crypto.provider = gmsm` → SM2（非对称加密/签名）+ SM3（哈希）+ SM4（对称加密）替代 RSA/SHA256/AES。影响范围：OIDC token 签名验证 + AK/SK HMAC 签名 + state 后端加密 + audit hash。
- **不启用时**：标准加密栈（TLS 1.3 / RSA / SHA-256 / AES-256-GCM），满足非等保场景。
- **实现依赖**：`github.com/tjfoc/gmsm` Go 国密库（开源，等保认证兼容）。

详细设计参见 `docs/`（按依赖先后排序：架构地基 → 身份与授权 → 多租户与参数管道 → 主干流水线 → 治理扩展）：
- `docs/01-总体架构.md`
- `docs/02-分层与stack模型.md`
- `docs/03-平台代码与脚本目录.md`（包结构 / 代码地图）
- `docs/04-数据库设计.md`（schema 参考）
- `docs/05-身份与组织同步.md`（对应 D10）
- `docs/06-云账号与凭据管理.md`（对应 D23，申请前置：team_cloud_grants 驱动过滤）
- `docs/07-环境与租户网络模型.md`（对应 D27，env/tenant/binding 三元组 + vpc-per-env 默认）
- `docs/08-标签分层与参数解析管道.md`（对应 D28，7 层 tag + 9 阶段管道 + provenance 审计）
- `docs/09-代码生成机制.md`（对应 D3/D19，表单→代码 / for_each）
- `docs/10-执行目录治理.md`（对应 D4，codegen→git→Executor 衔接）
- `docs/11-工具链版本与执行隔离.md`（对应 D12/D13/D15/D20/D22，Executor 沙箱）
- `docs/12-审批引擎与流程.md`（对应 D11/D21，双门禁 + plan/apply 解耦）
- `docs/13-漂移检测设计.md`（对应 D5/D7，复用 Executor）
- `docs/14-CMDB与FinOps.md`（对应 D18）
- `docs/15-存量导入.md`（对应 D14）
- `docs/16-CICD集成与审批门禁.md`（对应 D16，CICD gate）
- `docs/17-平台CLI与AI原生扩展.md`（对应 D17，平台 CLI / AI 原生）
