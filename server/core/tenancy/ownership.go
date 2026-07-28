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
//	middleware   → "dba"        IF component is a datastore (see dbaOwnedComponents),
//	                              else "middleware"
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
// owns. Matched by token-boundary (split component on - or /, check membership)
// to avoid false positives (P1-2 fix: substring Contains was too loose).
//
// Includes datastores: rds, redis, mongodb, polardb, mysql, oss, nas
// (OSS/NAS are data storage → DBA-owned per doc 02 §1.1 middleware layer definition).
//
// Phase 2 replaces this with a query against team_cloud_grants.
var dbaOwnedComponents = map[string]bool{
	"rds":     true,
	"redis":   true,
	"mongodb": true,
	"polardb": true,
	"mysql":   true,
	"oss":     true, // object storage → datastore
	"nas":     true, // file storage → datastore
}

// ResolveOwnerKind returns the team kind that should own a stack for the given
// (layer, component). It is a pure function: no DB access, no receiver, fully
// deterministic.
//
// NOTE (P1-1, D24.1 tension): the layer-name switch hardcodes "global"/
// "middleware"/"application" which D24.1 says MUST NOT be hardcoded. Phase 1
// accepts this (only 3 seed layers, no admin edit); Phase 2 will read layer
// config from DB and this switch becomes a lookup. See design.md D5.
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
// team owns. Uses token-boundary matching (split on - or /, check exact token
// membership) to avoid false positives like "my-sysql" matching "mysql".
// Empty component never matches (defensive: returns middleware).
func isDbaOwned(component string) bool {
	if component == "" {
		return false
	}
	lower := strings.ToLower(component)
	// Split on common separators (- and /) and check if any token is DBA-owned.
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == '-' || r == '/' || r == '_'
	}) {
		if dbaOwnedComponents[token] {
			return true
		}
	}
	return false
}
