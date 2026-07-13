# Design: platform-contract-mvp-completeness

## 01-设计决策

### D1: Registry 独立成域（职责单一）

`RegistryAdminService` 从 `catalog/` 拆出，独立为 `registry/` 域。理由：

| 维度 | registry（模块注册） | catalog（服务目录） |
|---|---|---|
| 输入 | git source + module path | module_id + module_version |
| 产物 | Module / ModuleVersion（含变量契约提取） | CatalogItem（含表单 schema） |
| 操作者 | 平台管理员 / 模块 owner | 平台管理员（发布）+ 终端用户（浏览/申请） |
| 生命周期 | 注册 → 校验 → 版本 → 废弃 | 发布 → 更新 → 废弃 |
| 父提案 capability | module-registry（specs/01） | service-catalog（specs/02） |

两者是上下游：registry 产出 `ModuleVersion`，catalog 把某个 `ModuleVersion` 发布为用户可申请的 `CatalogItem`。`CatalogItem.module_version` 跨域引用 `registry.ModuleVersion`（catalog/dto.proto `import registry/dto.proto`）。

**为什么不让 catalog 同时管两者**：单一职责 + capability 对齐。父提案已把 module-registry 和 service-catalog 列为两个独立 capability（各自 specs 文件、各自 owner、各自演进节奏）。proto 域划分应与 capability 划分一致，避免"一个 proto 域横跨两个 capability"导致改动耦合。

### D2: MVP RPC 补全依据（消除主链路断点）

审计对照 docs/12（审批引擎）、docs/17（CLI）、docs/04（表设计），确认 3 个必需 RPC：

| RPC | 缺失症状 | 证据 |
|---|---|---|
| `ListRequests` | 用户无法列出自己的工单；重连/换设备后拿不到 request_id | docs/17 line 42 `tm request list`；docs/04 §2.4 "工单当前状态"查询 |
| `ListPendingApprovals` | 审批人无法拿到待自己审批的 run_id（RBAC 绑角色不绑人，需平台解析） | docs/17 line 46 `tm approval list`；docs/12 §2.3 节点绑 team+role |
| `GetApprovalRun` | 审批人无法查看会签节点链进度 + plan_diff/成本/漂移等决策上下文 | docs/12 §7 前端节点链渲染；§3.1 pre-apply inputs |

**Codegen 维持 Internal（无 proto）**：codegen 是 generating 阶段 orchestrator 内部管道（docs/09 §7），用户通过 `GetRequest`/`ListRequestEvents` 感知 `status` 迁移即可——无缺口，不补 proto。

### D3: ApprovalRun 字段补全依据（对齐 docs/04 + docs/12）

原 `ApprovalRun` 仅 4 字段（run_id/status/decided_by/decided_at），无法支撑审批工作台。补全对齐 docs/04 §2.10 表 + docs/12 审批模型：

| 补充字段 | 类型 | 依据 |
|---|---|---|
| `request_id` | string | docs/04 approval_runs.request_id；审批人跳回 request 详情 |
| `gate` | ApprovalGate enum | docs/12 §3.1 D21 双门禁（pre-plan 准入 / pre-apply 执行确认） |
| `current_node` | string | docs/04 approval_runs.current_node；docs/12 §7 节点链定位 |
| `started_at`/`finished_at`/`expires_at` | string | docs/04 approval_runs；docs/12 §2.3 超时升级 |

新增 message（`GetApprovalRunResponse` 需要）：
- `ApprovalNodeRun`：node_id / mode / decided_count / required_count / status / timeout_at（docs/04 approval_node_runs；会签进度可视化）
- `ApprovalDecisionRecord`：approver_id / decision / comment / decided_at（docs/04 approval_decisions；审计轨迹）

新增枚举：`ApprovalGate` / `ApprovalNodeMode` / `ApprovalNodeStatus`（放 common/enum.proto，跨域可复用）。

### D4: LifecycleRequest 字段补全

| 字段 | 依据 |
|---|---|
| `requester_id` | docs/04 §2.4 requests.requester_id；"我的工单"查询 + 审批人侧归因 |
| `current_stage` | docs/00 GetRequest 响应字段；docs/04 requests.current_stage；UI 粗粒度阶段提示（权威状态仍是 status + version） |

## 02-MVP 全链路覆盖（修订后）

```
管理员链路（registry + catalog Admin service）：
  RegistryAdminService.RegisterModule（registry 域）
    → CatalogAdminService.PublishCatalogItem（catalog 域）
    → 用户可见（CatalogService.ListItems）

用户链路（lifecycle + catalog Server service）：
  CatalogService.GetCatalogItem
    → LifecycleService.CreateRequest
    → (codegen: Internal 函数调用)
    → LifecycleService.StartPlan
    → LifecycleService.GetArtifact
    → LifecycleService.EvaluateGate
    → [审批人] ListPendingApprovals → GetApprovalRun → DecideApproval
    → LifecycleService.StartApply
    → LifecycleService.GetRequest / ListRequests（= succeeded）

用户列表入口：
  LifecycleService.ListRequests（按 requester_id/team_id/status 过滤）
```

修订后契约：**6 service / 24 RPC**，主链路无断点。

## 03-与 platform-contract-freeze 的关系

| 维度 | platform-contract-freeze（已归档） | platform-contract-mvp-completeness（本 change） |
|---|---|---|
| 定位 | Phase 0 契约首次冻结 | Phase 0 契约修订（职责修正 + 补全） |
| proto 域 | 4 域（common/lifecycle/catalog/cloud） | 5 域（+ registry） |
| RPC 数 | 21 | 24（+3） |
| 破坏性 | 否（首次冻结） | 是（message 迁移 + 字段编号扩展），但无运行时消费者 |

本 change 是 freeze 的增量修订，不复活 freeze（已归档不可变）。修订后的契约规格在 `specs/03-平台契约.md` 用 MODIFIED 标注。

## 04-验收标准

- [x] `buf lint` + `buf generate` + `go build ./...` + `go vet ./...` 全绿。
- [x] registry 域独立（RegistryAdminService + Module* 在 registry/，catalog/ 不含）。
- [x] MVP 主链路无断点（ListRequests / ListPendingApprovals / GetApprovalRun 存在）。
- [x] ApprovalRun 字段完整（request_id / gate / current_node / 节点链 / 决策历史）。
- [x] 三层模型归类正确（RegistryAdmin / CatalogAdmin = Admin；其余 = Server；codegen = Internal 无 proto）。
