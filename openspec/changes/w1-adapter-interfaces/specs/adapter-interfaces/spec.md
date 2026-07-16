# adapter-interfaces

## ADDED Requirements

### Requirement: Six pluggable adapter interfaces

The platform SHALL define six adapter interfaces in `server/core/adapters/` following D7: GitProvider, CloudProvider, StateBackend, PolicyEngine, CostEstimator, Notifier. Each interface SHALL have a noop/stub default implementation that returns a structured error (not silent pass).

#### Scenario: NoopGit returns error when not configured

- **WHEN** GitProvider is the noop stub
- **AND** Clone is called
- **THEN** it returns an error containing "git adapter not configured"

#### Scenario: NoopPolicy returns error when not configured

- **WHEN** PolicyEngine is the noop stub
- **AND** Evaluate is called
- **THEN** it returns an error containing "policy adapter not configured"

### Requirement: TerramateAdapter exec boundary (D1)

The platform SHALL define a TerramateAdapter interface in `server/core/terramate/` that wraps the terramate CLI as a subprocess. The exec implementation SHALL set cmd.Dir to the stack directory (D29), capture stdout/stderr/exit code, respect context cancellation, and never import github.com/terramate-io/terramate internal packages.

#### Scenario: TerramateAdapter runs terramate in stack dir

- **WHEN** Run is called with dir="/path/to/stack" and args=["run", "--tags", "env:prod", "--", "terraform", "plan"]
- **THEN** exec.Command is invoked with cmd.Dir="/path/to/stack"
- **AND** RunResult captures exit code, stdout, stderr

#### Scenario: Context cancellation propagates

- **WHEN** Run is called with a cancelled context
- **THEN** the subprocess is killed and an error is returned

### Requirement: D1 boundary guard test

The platform SHALL have a test in server/internal/audit/ that walks all .go files under server/ and asserts none import github.com/terramate-io/terramate.

#### Scenario: No terramate imports in server/

- **WHEN** the D1 guard test runs
- **THEN** it passes (no file under server/ imports terramate internals)

### Requirement: wire ProviderSet for adapters

All six adapters SHALL be registered in a wire ProviderSet (server/core/adapters/provider.go), binding interfaces to noop defaults.

#### Scenario: ProviderSet binds all six interfaces

- **WHEN** wire generates the dependency graph
- **THEN** GitProvider, CloudProvider, StateBackend, PolicyEngine, CostEstimator, Notifier are all bound to their noop stubs
