# tenancy-management Specification

## Purpose
TBD - created by archiving change w1-tenancy-layer. Update Purpose after archive.
## Requirements
### Requirement: 团队归属治理（TenancyService CRUD + 资源归属规则）

平台 MUST 在 `server/core/tenancy/` 实现 TenancyService，注入 TeamRepo/ProjectRepo/SpaceRepo（W1-02），提供 team/project/space 的 CRUD 操作（薄包装 Repo）+ 资源归属规则判定。资源归属规则按 layer→default_owner_team 映射：global→platform-ops，middleware→dba/middleware（按 component 细分），application→request.team_id。MVP 用硬编码映射，Phase 2 走 team_cloud_grants（D23）。

#### Scenario: 创建团队

- **WHEN** 调用 TenancyService.CreateTeam(ctx, name, slug, kind)
- **THEN** TeamRepo.Create 写入 teams 表（snowflake ID + status=active）
- **AND** 返回 created team

#### Scenario: 资源归属判定（global 层 → platform-ops）

- **WHEN** 调用 TenancyService.ResolveOwner(layer="global", component="vpc")
- **THEN** 返回 platform-ops team（global 层固定归属平台运维）

#### Scenario: 资源归属判定（middleware 层 → DBA/middleware 按 component）

- **WHEN** 调用 TenancyService.ResolveOwner(layer="middleware", component="rds")
- **THEN** 返回 dba team（RDS 归属 DBA）

- **WHEN** 调用 TenancyService.ResolveOwner(layer="middleware", component="kafka")
- **THEN** 返回 middleware team（Kafka 归属中间件团队）

#### Scenario: 资源归属判定（application 层 → request team）

- **WHEN** 调用 TenancyService.ResolveOwner(layer="application", component="ecs", requestingTeamID=5)
- **THEN** 返回 team 5（application 层归属申请团队）

