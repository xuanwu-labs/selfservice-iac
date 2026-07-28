// Package granularity maps catalog_items.stack_grouping to the granularity
// level that the stack materializer (codegen) should use when expanding a
// catalog item into one or more stacks.
//
// Granularity levels (planned):
//
//	"per-component" — one stack per (component, env) pair. MVP default. Each
//	                  component (vpc, rds, ecs) in a catalog item becomes its
//	                  own stack, regardless of how many spaces request it.
//	"per-space"     — one stack per (component, space, env) pair. Phase 2.
//	                  Lets the same component run with different inputs per
//	                  space (e.g. dev-stage-orders-ecs vs prod-stage-orders-ecs).
//	"per-tenant"    — one stack per (component, tenant, env) pair. Phase 2.
//
// MVP: Evaluate is a pure function that always returns "per-component". This
// lets the rest of the pipeline (pathgenerator, codegen, dependency) be built
// and tested against a single, deterministic granularity before the
// multi-granularity branching lands in Phase 2. The function signature is
// stable: Phase 2 only changes the body to consult stack_grouping (and possibly
// a rule-set override).
package granularity

// Evaluate returns the granularity level for a catalog item given its
// stack_grouping value (from catalog_items.stack_grouping).
//
// MVP: always returns "per-component" regardless of input. Non-default inputs
// (e.g. "per-space") are deliberately ignored so that the rest of the pipeline
// only needs to handle the per-component code path. Phase 2 will branch on
// stackGrouping to honor the other levels.
func Evaluate(stackGrouping string) string {
	return "per-component"
}
