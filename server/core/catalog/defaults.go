// Package catalog: defaults.go — best-practice catalog defaults generator
// (W1-03 task 4.2).
//
// catalog_items.defaults_json is the S2 stage of the doc-08 parameter pipeline:
// the catalog's opinionated defaults for a module, applied AFTER the form
// schema (S1) but BEFORE envtenant injection (S3-S5). The user can still
// override these via the form.
//
// MVP scope: a hardcoded best-practice table keyed by a substring match on the
// module name (case-insensitive). Only keys that actually exist on the
// contract are emitted — we never inject a variable the module doesn't
// declare. When no rule matches, or the matching rule has no overlap with the
// contract, the result is `{}`.
//
// This is intentionally a free function of (moduleName, contract) so it stays
// trivially testable and side-effect free. W2 may swap the table for a
// per-module DSL stored in the DB without changing the call sites.

package catalog

import (
	"encoding/json"
	"strings"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
)

// defaultsRule is one best-practice entry: if moduleName matches `match` (as a
// case-insensitive substring), the listed defaults are candidates. A single
// module name may match several rules (they are unioned, later wins on clash).
type defaultsRule struct {
	match    string
	defaults map[string]any
}

// moduleDefaults is the MVP best-practice table. Keys are common Terraform
// variable names; values are the recommended cloud-side defaults.
//
// RDS (Alicloud / AWS): a small but production-sane MySQL/Postgres shape.
// Redis: a single standard cache node.
// Slb / ALB: a generic shared load balancer.
// ECS / VM: a general-purpose compute shape.
//
// These are deliberately conservative — they exist so a freshly published
// catalog item is immediately requestable without the operator having to
// hand-author a defaults blob. Operators override via catalog_items.Update.
var moduleDefaults = []defaultsRule{
	{
		match: "rds",
		defaults: map[string]any{
			"instance_type": "rds.mysql.s2.large",
			"storage_size":  100,
		},
	},
	{
		match: "redis",
		defaults: map[string]any{
			"instance_type": "redis.master.small.default",
		},
	},
	{
		match: "slb",
		defaults: map[string]any{
			"spec": "slb.s1.small",
		},
	},
	{
		match: "ecs",
		defaults: map[string]any{
			"instance_type": "ecs.s6.large",
		},
	},
}

// ApplyDefaults computes catalog_items.defaults_json for a module (W1-03 4.2).
//
// It intersects the rule table with the contract's declared variables so the
// output only ever references keys the module actually accepts (no spurious
// instance_type on a VPC module). Order of application is rule-table order;
// when multiple rules match, later rules override earlier ones on conflict.
//
// Returns `{}` when nothing matches. The returned RawMessage is always a
// non-nil JSON object.
func ApplyDefaults(moduleName string, contract *registry.Contract) (json.RawMessage, error) {
	out := make(map[string]any)

	// Fast path: nothing to match against, or no rule applies.
	if contract == nil || len(contract.Variables) == 0 {
		return json.Marshal(out)
	}

	// Build the set of declared variable names for O(1) membership tests.
	declared := make(map[string]struct{}, len(contract.Variables))
	for _, v := range contract.Variables {
		declared[v.Name] = struct{}{}
	}

	name := strings.ToLower(moduleName)
	for _, rule := range moduleDefaults {
		if !strings.Contains(name, rule.match) {
			continue
		}
		for k, v := range rule.defaults {
			if _, ok := declared[k]; !ok {
				// Don't inject a variable the module doesn't declare.
				continue
			}
			out[k] = v
		}
	}

	buf, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
