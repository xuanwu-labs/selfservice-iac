# contracts/

Aether platform **API contract single source of truth** (proto-first, Connect-native).

## Connect-native principle

Proto files are the only contract source. No hand-maintained openapi.yaml /
schemas / protocol-mapping.md. Connect-RPC natively serves gRPC / gRPC-Web /
Connect-JSON — one handler covers all protocols.

```
contracts/platform/v1/<domain>/*.proto   ← single source of truth (humans edit only these)
       │
       │  buf generate (2 plugins)
       ▼
  server/internal/proto/*.pb.go            ← Go message types
  server/internal/proto/.../*connect.go    ← Connect service interfaces (handler + client)
       │
       │  frontend (future)
       ▼
  web/src/gen/*.ts                         ← connect-es TypeScript client
```

## Service layer model (Server / Admin / Internal)

Every proto service belongs to exactly one of three layers. The layer
determines authentication, authorization, and network exposure.

| Layer | Who calls | Auth | Network | Location |
|---|---|---|---|---|
| **Server** (user) | End users (Web/CLI/AI) | OIDC JWT + RBAC | Exposed via Connect (`/api/`) | `LifecycleService`, `CatalogService`, `EntitlementService` |
| **Admin** (operator) | Platform admins | OIDC JWT + **admin role** | Exposed via Connect (`/api/`) | `RegistryAdminService`, `CatalogAdminService` |
| **Internal** (in-process) | Platform internal logic (codegen, executor, CMDB, drift) | **No RPC** — direct function call | Not exposed | N/A (no proto) |

> **Capability alignment.** Domains map 1:1 to the capability split in
> the parent proposal (`iac-self-service-platform`): `registry/` =
> module-registry (specs/01), `catalog/` = service-catalog (specs/02),
> `lifecycle/` = the request lifecycle spanned by orchestration-engine +
> approval-engine (specs/06, 10), `cloud/` = cloud-credentials entitlement
> surface (specs/16). Keeping registry and catalog as separate domains
> prevents the module registry (raw module ingestion/versioning) from
> blurring into the service catalog (user-facing items published from
> module versions).

### Why no InternalService proto?

Internal operations (codegen, executor, CMDB ingest, drift scan) are
**in-process function calls** within the same Go binary (D39: direct function
call, no message broker). They do not need Connect-RPC exposure. Only
cross-process / cross-team boundaries get a proto service.

### Admin authorization

Admin services (`RegistryAdminService`, `CatalogAdminService`) require the
admin role. The Connect interceptor chain checks the service name prefix:

```go
// internal/middleware/connect.go
if strings.HasPrefix(req.Spec().Procedure, "/aether.platform.v1.RegistryAdmin") ||
   strings.HasPrefix(req.Spec().Procedure, "/aether.platform.v1.CatalogAdmin") {
    // require admin role
}
```

### Decision criteria: which layer?

| Question | If yes → |
|---|---|
| Does an end user call this? | Server |
| Does an admin configure/manage this? | Admin |
| Does the platform call this internally (same process)? | Internal (no proto) |

## File organization: domain × (srv/dto/enum)

Each business domain is a directory under `platform/v1/`. Inside a domain,
files are split by responsibility:

| File | Contains | Rule |
|---|---|---|
| `srv.proto` | `service` definitions only — no messages | one file per domain |
| `dto.proto` | all messages (domain objects + RPC request/response) | one file per domain |
| `enum.proto` | enums only | **`common/enum.proto` is the single home** for cross-domain enums |

Shared types (pagination, actor) and all enums live in `common/`, imported by
every domain that needs them. This keeps `srv.proto` readable as a pure
interface list and lets dto/enum evolve without touching service signatures.

### Current domains

| Domain | Services | Layer |
|---|---|---|
| `common/` | (none — shared `dto.proto` + `enum.proto`) | — |
| `lifecycle/` | `LifecycleService` (12 RPC: request CRUD + list, plan, artifact, gate, approval list/detail/decide, apply) | Server |
| `catalog/` | `CatalogService` (Server, 4 RPC) + `CatalogAdminService` (Admin, 3 RPC) | Server + Admin |
| `registry/` | `RegistryAdminService` (Admin, 4 RPC: register/list/get/deprecate module) | Admin |
| `cloud/` | `EntitlementService` (Server, 1 RPC) | Server |

**Total: 6 services / 24 RPC** covering the full MVP main chain with no
dead ends (admin register → publish → user request → plan → gate →
approval queue/detail/decide → apply → succeeded).

## Directory structure

```
contracts/
├── platform/v1/                 proto service definitions (single contract source)
│   ├── common/
│   │   ├── enum.proto           shared enums (RequestStatus, ApprovalGate, CloudProvider, ...)
│   │   └── dto.proto            shared messages (PageRequest, PageResponse, Actor)
│   ├── lifecycle/
│   │   ├── srv.proto            LifecycleService (Server)
│   │   └── dto.proto            LifecycleRequest, PlanArtifact, GateResult, ApprovalRun,
│   │                            ApprovalNodeRun, ApprovalDecisionRecord, ...
│   ├── catalog/
│   │   ├── srv.proto            CatalogService (Server) + CatalogAdminService (Admin)
│   │   └── dto.proto            CatalogItem, ModuleDependencyInfo, AvailableStack, ...
│   ├── registry/
│   │   ├── srv.proto            RegistryAdminService (Admin)
│   │   └── dto.proto            Module, ModuleVersion, ModuleDependency, ...
│   └── cloud/
│       ├── srv.proto            EntitlementService (Server)
│       └── dto.proto            CloudAccount, ...
├── error-codes.yaml             error code registry (proto can't express remediation/owner)
├── fixtures/                    test data (state machine / adapter / skeleton seed)
├── buf.yaml                     buf module config
├── buf.gen.yaml                 buf code generation config
└── README.md                    this file
```

## Why no openapi.yaml?

Connect-RPC natively supports curl / Postman (Connect/JSON protocol). API
documentation = the proto files themselves (the most precise interface
definition). If REST docs are needed for non-Connect clients later, enable the
`protoc-gen-openapi` plugin in `buf.gen.yaml` to auto-generate from proto.

## Adding a new domain or service

1. Create `contracts/platform/v1/<domain>/{srv.proto,dto.proto}` (add the
   domain under `buf.yaml` if new).
2. Put shared enums in `common/enum.proto`; shared messages in `common/dto.proto`.
3. Choose the service layer: Server (user-facing) or Admin (operator-facing).
4. Follow naming conventions: proto3 enum prefix (`REQUEST_STATUS_...`),
   `snake_case` fields, English comments only.
5. Run `buf lint && buf generate`.
6. Implement handler in `server/api/connect/`.
7. Register in `server/internal/server/connect.go` (`ProvideServerConfig`).
8. Run `make wire-gen`.

## Toolchain

```bash
cd contracts
buf lint                # check proto conventions
buf generate            # generate Go code to server/internal/proto/
# future: buf generate also produces TS client to web/src/gen/
```
