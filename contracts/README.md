# contracts/

Aether platform **API contract single source of truth** (proto-first, Connect-native).

## Connect-native principle

Proto files are the only contract source. No hand-maintained openapi.yaml / schemas / mapping docs.
Connect-RPC natively serves gRPC / gRPC-Web / Connect-JSON — one handler covers all protocols.

```
contracts/platform/v1/*.proto    ← single source of truth (humans edit only these)
       │
       │  buf generate (2 plugins)
       ▼
  server/internal/proto/*.pb.go          ← Go message types
  server/internal/proto/.../*.connect.go ← Connect service interfaces (handler + client)
       │
       │  frontend (future)
       ▼
  web/src/gen/*.ts                       ← connect-es TypeScript client
```

## Service layer model (Server / Admin / Internal)

All proto services fall into exactly one of three layers. This determines
authentication, authorization, and network exposure.

| Layer | Who calls | Auth | Network | Proto location |
|---|---|---|---|---|
| **Server** (user) | End users (Web/CLI/AI) | OIDC JWT + RBAC | Exposed via Connect (`/api/`) | `request.proto`, `catalog.proto`, `planning.proto`, `approval.proto`, `entitlement.proto`, `dependency.proto` |
| **Admin** (operator) | Platform admins | OIDC JWT + **admin role** | Exposed via Connect (`/api/`) | `admin.proto` |
| **Internal** (in-process) | Platform internal logic (codegen, executor, CMDB, drift) | **No RPC** — direct function call | Not exposed | N/A (no proto needed) |

### Why no InternalServer proto?

Internal operations (codegen, executor, CMDB ingest, drift scan) are
**in-process function calls** within the same Go binary (D39: direct
function call, no message broker). They don't need Connect-RPC exposure.

ferret uses InternalServer because it's a microservice architecture
(orchestrator calls email service via RPC). We are a monolith —
internal operations stay as function calls, not proto services.

### Admin authorization

Admin services (`ModuleRegistryService`, `CatalogAdminService`) require
admin role. The Connect interceptor chain checks service name prefix:

```go
// internal/middleware/connect.go
if strings.HasPrefix(req.Spec().Procedure, "/aether.platform.v1.ModuleRegistry") ||
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

## Directory structure

```
contracts/
├── platform/v1/              proto service definitions (single contract source)
│   ├── common.proto          shared types (PageRequest, Actor)
│   ├── request.proto         RequestService (user: create/get/events/cancel)
│   ├── planning.proto        PlanningService + ArtifactService + GateService
│   ├── approval.proto        ApprovalService + ApplyService
│   ├── catalog.proto         CatalogService (user: list/get catalog items)
│   ├── entitlement.proto     EntitlementService (user: cloud accounts)
│   ├── admin.proto           ModuleRegistryService + CatalogAdminService (admin)
│   └── dependency.proto      DependencyService (user: dependency query)
├── error-codes.yaml          error code registry (proto can't express remediation)
├── fixtures/                 test data (state machine / adapter / skeleton seed)
├── buf.yaml                  buf module config
├── buf.gen.yaml              buf code generation config
└── README.md                 this file
```

## Why no openapi.yaml?

Connect-RPC natively supports curl / Postman (Connect/JSON protocol).
API documentation = proto files themselves (the most precise interface definition).

If REST docs are needed for non-Connect clients in the future, enable the
`protoc-gen-openapi` plugin in buf.gen.yaml to auto-generate from proto.

## Adding a new service

1. Create `contracts/platform/v1/<domain>.proto`
2. Choose layer: Server (user-facing) or Admin (operator-facing)
3. Follow naming conventions: proto3 enum prefix, snake_case fields
4. Run `buf lint && buf generate`
5. Implement handler in `server/api/connect/`
6. Register in `server/internal/server/connect.go` (ProvideServerConfig)
7. Run `make wire-gen`

## Toolchain

```bash
cd contracts
buf lint                # check proto conventions
buf generate            # generate Go code to server/internal/proto/
# future: buf generate also produces TS client to web/src/gen/
```
