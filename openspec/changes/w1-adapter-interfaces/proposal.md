# Proposal: w1-adapter-interfaces

## What Changes

Define the D7 six pluggable adapter interfaces (GitProvider, CloudProvider, StateBackend, PolicyEngine, CostEstimator, Notifier) with noop/stub default implementations, plus the TerramateAdapter exec adapter that wraps `terramate run` as a subprocess. Also add a D1 boundary guard test that enforces `server/**` MUST NOT import `github.com/terramate-io/terramate` internal packages.

This is **W1 module 01** (platform skeleton + adapter interfaces) from the `iac-self-service-platform` tasks.md. It is the first of four W1 changes — adapter interfaces have zero dependencies and are consumed by downstream modules (03 registry uses GitProvider, 05 codegen uses StateBackend, 06 orchestrator uses TerramateAdapter).

## Why

1. **D7 pluggability is a core user requirement** — the platform must support swapping cloud providers, policy engines (OPA), cost estimators (Infracost), and notifiers without code changes. Interface-first design makes this possible.
2. **TerramateAdapter is the D1 boundary guardian** — the platform calls terramate via `exec` (subprocess), never via Go import. The adapter encapsulates this contract so the rest of the codebase never touches terramate internals directly.
3. **D1 guard test closes a known gap** — `.golangci.yml` depguard is configured but non-enforcing (terramate not in go.mod → typechecker silently drops the rule). A dedicated compile-time test makes the boundary explicit and CI-enforceable.
4. **Downstream modules cannot start without these interfaces** — W1 modules 03 (registry) and 05 (codegen, W2) depend on GitProvider/StateBackend; W2 module 06 (orchestrator) depends on TerramateAdapter.

## Scope

### In scope (this change)
- 6 adapter interfaces in `server/core/adapters/{git,state,policy,cost,notify,cloud}/` with noop defaults
- TerramateAdapter interface + exec implementation in `server/core/terramate/`
- D1 guard test in `server/internal/audit/`
- wire ProviderSet wiring for all adapters

### Out of scope (later changes)
- Real adapter implementations (actual go-git clone, OPA eval, Infracost, S3 state read) — stubs only
- DB store layer (W1 module 02: `feat/w1-db-store`)
- Module registry (W1 module 03: `feat/w1-module-registry`)
- Layer model (W1 module 04: `feat/w1-layer-model`)
- Adapter config persistence (`adapters_config` table is non-MVP)

## Decisions

- **No entity classes** — core uses sqlc-generated `generated.*` types directly; `internal/mapping/` handles generated↔proto conversion. This is the sqlc standard pattern (unlike ferret's hand-written ORM which needs entity classes).
- **Noop stubs return structured errors** — stubs are not silent; they return `errors.New("adapter not configured")` so missing adapters fail loud at runtime, not silently.
- **TerramateAdapter returns Result struct** — captures exit code, stdout, stderr, duration; enables deterministic testing with fake terramate scripts.

## Impact

- New packages: `server/core/adapters/{git,state,policy,cost,notify,cloud}/`, `server/core/terramate/`, `server/internal/audit/`
- No existing code modified (pure additive)
- No DB schema changes
- No proto contract changes
