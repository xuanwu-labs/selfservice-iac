// Package tenancy: ownership.go — resource ownership resolution (D5).
//
// ResolveOwnerKind maps a (layer, component) pair to the team *kind* that owns
// the resulting stack. The kind is then resolved to a concrete team via
// teams.kind (e.g. kind="dba" → the single DBA team row).
//
// MVP RULES (hardcoded here, deterministic and unit-testable):
//
//	global       → "platform"   (platform team owns global infra: org-wide VPC,
//	                              IAM, landing zone — no tenant/team scoping)
//	middleware   → "dba"        IF component is a datastore (rds|redis|mongodb|
//	                              polardb|mysql), else "middleware"
//	                              (datastores are centrally owned for backup/DR
//	                              policy; other middleware like kafka/vpc-peering
//	                              stays with the middleware team)
//	application  → "business"   (the owning business team, resolved elsewhere
//	                              via stacks.owner_team_id)
//	default      → "business"
//
// PHASE 2: this mapping moves into team_cloud_grants (a DB table that records,
// per team, which (layer, component, cloud) triples it may own). The function
// signature stays the same, so callers won't change — only the implementation
// swaps from a switch to a repo lookup.
package tenancy

import "strings"

// dbaOwnedComponents is the set of middleware component slugs that the DBA team
// owns. Matched case-insensitively against the component basename (e.g.
// "alicloud-rds-mysql" contains "rds" → DBA-owned).
//
// Phase 2 replaces this with a query against team_cloud_grants.
var dbaOwnedComponents = []string{
	"rds",
	"redis",
	"mongodb",
	"polardb",
	"mysql",
}

// ResolveOwnerKind returns the team kind that should own a stack for the given
// (layer, component). It is a pure function: no DB access, no receiver, fully
// deterministic. The component match is case-insensitive and matches any
// component that contains a DBA-owned keyword as a substring (so "rds-mysql"
// and "alicloud-rds" both resolve to "dba").
func ResolveOwnerKind(layer, component string) string {
	switch layer {
	case "global":
		return "platform"
	case "middleware":
		if isDbaOwned(component) {
			return "dba"
		}
		return "middleware"
	case "application":
		return "business"
	default:
		return "business"
	}
}

// isDbaOwned reports whether the component references a datastore that the DBA
// team owns. Empty component never matches (defensive: returns middleware).
func isDbaOwned(component string) bool {
	if component == "" {
		return false
	}
	lower := strings.ToLower(component)
	for _, kw := range dbaOwnedComponents {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
