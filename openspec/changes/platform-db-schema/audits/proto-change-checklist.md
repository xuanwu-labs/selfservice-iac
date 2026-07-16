# Proto Change 清单：message 字段补全（已完成）

> 架构审计发现 proto dto.proto 的 4 个 message 缺字段（DB 有列但 proto 不暴露），导致 RPC 响应不完整、乐观锁无法工作。
>
> **状态：已全部完成**——4 个 message 字段已补充到 dto.proto，buf generate 成功，build+vet 全绿。这是纯 append（不改变现有字段编号），向后兼容。

## 变更 1：LifecycleEvent 加状态轨迹字段

**文件**：`contracts/platform/v1/lifecycle/dto.proto`
**message**：`LifecycleEvent`（行 45-52）

**现状**：只有 id/request_id/event_type/actor/occurred_at/correlation_id
**DB 有但 proto 缺**：`from_status / to_status / stage / message`

**后果**：ListRequestEvents 拿不到状态迁移轨迹（前端无法画时间线），doc 00 §3 events 端点语义落空。

**改法**：
```protobuf
message LifecycleEvent {
  string id = 1;
  string request_id = 2;
  string event_type = 3;
  aether.platform.v1.common.Actor actor = 4;
  string occurred_at = 5;
  string correlation_id = 6;
  // 新增 ↓
  string from_status = 7;   // 状态迁移起点
  string to_status = 8;     // 状态迁移终点
  string stage = 9;          // 当前阶段（codegen/plan/apply）
  string message = 10;       // 事件描述
}
```

## 变更 2：PlanArtifact 加版本校验字段

**文件**：`contracts/platform/v1/lifecycle/dto.proto`
**message**：`PlanArtifact`（行 54-64）

**现状**：只有 id/request_id/status/plan_hash/summary/cost/storage_uri/expires_at/created_at
**DB 有但 proto 缺**：`sha256 / pinned_commit / toolchain_profile_hash / provider_lock_hash / tf_version_sha256 / stack_id / state_key / size_bytes`

**后果**：D21 plan/apply 解耦的核心不变量（apply 校验 sha256 防篡改）无法从 API 获取；doc 00 §4.4 PlanArtifact schema 明确冻结这些字段。

**改法**：
```protobuf
message PlanArtifact {
  string id = 1;
  string request_id = 2;
  aether.platform.v1.common.ArtifactStatus status = 3;
  string plan_hash = 4;
  PlanSummary summary = 5;
  int64 cost_estimate_cents = 6;
  string storage_uri = 7;
  string expires_at = 8;
  string created_at = 9;
  // 新增 ↓
  string sha256 = 10;                    // plan 二进制哈希（apply 校验防篡改）
  string pinned_commit = 11;             // apply 必须检出同一 commit
  string toolchain_profile_hash = 12;    // apply TF 版本必须匹配
  string provider_lock_hash = 13;        // apply .terraform.lock.hcl 必须匹配
  string tf_version_sha256 = 14;         // apply terraform 二进制必须匹配
  string stack_id = 15;                  // PathGenerator 输出
  string state_key = 16;                 // PathGenerator 输出
  int64 size_bytes = 17;                 // plan 文件大小
}
```

## 变更 3：ApprovalRun 加 version 字段

**文件**：`contracts/platform/v1/lifecycle/dto.proto`
**message**：`ApprovalRun`（行 80-99）

**现状**：有 run_id/status/decided_by/decided_at/request_id/gate/current_node/started_at/finished_at/expires_at
**DB 有但 proto 缺**：`version`（乐观锁）

**后果**：DecideApprovalRequest 有 `expected_run_version` 参数，但 ApprovalRun 不返回 version → 客户端无法得知当前 version → 乐观锁无法工作。doc 12a CONC-003 是 Phase 1 验收项。

**改法**：
```protobuf
message ApprovalRun {
  // ... 现有字段 1-10 ...
  int32 version = 11;  // 乐观锁（DecideApproval expected_run_version）
}
```

## 变更 4：LifecycleRequest 加 resolved_params_json

**文件**：`contracts/platform/v1/lifecycle/dto.proto`
**message**：`LifecycleRequest`（行 17-43）

**现状**：有 20 字段但无 resolved_params_json
**DB 有但 proto 缺**：`resolved_params_json`（D28 provenance 审计）

**后果**：D28 mandatory provenance 写入了 DB（requests.resolved_params_json），但 GetRequest 不返回 → 审批人/前端看不到参数来源（"这个 vpc_id 哪来的"），D28 审计意图落空。

**改法**：
```protobuf
message LifecycleRequest {
  // ... 现有字段 1-20 ...
  string resolved_params_json = 21;  // D28 provenance（每变量 source/rank）
}
```

## 变更 5（可选）：命名对齐（5 处零映射违规）

**违反 design.md §01 "proto 字段名 = DB 列名 零映射"原则**：

| proto 字段 | DB 列 | 改法 |
|---|---|---|
| CatalogItem.name | catalog_items.display_name | proto 改 `display_name` 或 DB 改 `name` |
| PublishCatalogItemRequest.default_values | catalog_items.defaults_json | proto 改 `defaults_json` |
| PublishCatalogItemRequest.visible_to_teams | catalog_items.visibility_json | proto 改 `visibility_json` |
| LifecycleEvent.actor.user_id | request_events.actor_id | proto 改 `actor_id` 或 DB 改 `user_id` |
| ApprovalRun.run_id | approval_runs.id | proto 改 `id` 或保留（业界惯例 run_id 可接受） |

**优先级**：变更 1-4 是功能阻塞（必须改），变更 5 是优雅性（可改可不改，但开工前改成本最低）。

## 发起方式

按 lifecycle-ownership 规则，proto change 由你发起：
1. 创建 `openspec/changes/proto-message-field-completion/proposal.md`
2. 上述 5 个变更新增到 dto.proto
3. `make proto-gen` 重新生成 Go 代码
4. 更新 handler 层映射

本 change（platform-db-schema）不包含 proto dto.proto 的修改——只含 enum.proto（已改）+ DB 迁移。dto.proto 的 message 字段补充是独立的 proto change。
