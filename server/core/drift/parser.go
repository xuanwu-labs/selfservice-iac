// Package drift implements drift detection: scheduler (D2), worker (D3),
// and terraform plan JSON parsing (D4).
//
// This file implements PlanParser (D4). Terraform emits a stable JSON shape
// from `terraform show -json <plan>`; the parser extracts the
// resource_changes array and keeps only resources whose change.actions is not
// ["no-op"] (i.e. create / update / delete / create-then-delete).
package drift

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DiffSummary is the post-parse summary of a terraform plan: only resources
// that diverge from the prior state (actions != ["no-op"]).
type DiffSummary struct {
	// ChangedResources is the list of resources with non-no-op actions,
	// in the order they appeared in the plan JSON.
	ChangedResources []ResourceChange
}

// ResourceChange is a single changed resource from the plan JSON (D4).
type ResourceChange struct {
	// Address is the Terraform resource address, e.g.
	// "alicloud_db_instance.this".
	Address string
	// Actions is the change action set, e.g. ["create"], ["update"],
	// ["delete"], or ["no-op"].
	Actions []string
}

// planJSON mirrors the subset of `terraform show -json` output we need (D4).
type planJSON struct {
	ResourceChanges []resourceChange `json:"resource_changes"`
}

// resourceChange is the per-resource object in the plan JSON.
type resourceChange struct {
	Address string     `json:"address"`
	Change  changeBody `json:"change"`
}

// changeBody is the nested "change" object (P2-8 fix: a dedicated nested
// struct, NOT a dotted json tag like "change.actions" which encoding/json
// does not support).
type changeBody struct {
	Actions []string `json:"actions"`
}

// ParsePlan decodes terraform plan JSON (as emitted by
// `terraform show -json <plan>`) and returns a DiffSummary containing only
// resources whose actions != ["no-op"].
//
// An empty/whitespace input or a plan with no resource_changes yields an
// empty DiffSummary (no drift) — callers decide what an empty plan means in
// context (a real plan-failure path produces exit code 1 upstream and never
// reaches ParsePlan; see worker.go exit code mapping).
func ParsePlan(jsonData []byte) (DiffSummary, error) {
	if len(strings.TrimSpace(string(jsonData))) == 0 {
		return DiffSummary{}, nil
	}

	var plan planJSON
	if err := json.Unmarshal(jsonData, &plan); err != nil {
		return DiffSummary{}, fmt.Errorf("drift: parse plan json: %w", err)
	}

	summary := DiffSummary{ChangedResources: []ResourceChange{}}
	for _, rc := range plan.ResourceChanges {
		if isNoOp(rc.Change.Actions) {
			continue
		}
		summary.ChangedResources = append(summary.ChangedResources, ResourceChange{
			Address: rc.Address,
			Actions: append([]string(nil), rc.Change.Actions...),
		})
	}
	return summary, nil
}

// isNoOp reports whether actions is exactly ["no-op"]. An empty actions slice
// (defensive) is treated as no-op as well — a real change always carries an
// action.
func isNoOp(actions []string) bool {
	if len(actions) == 0 {
		return true
	}
	if len(actions) == 1 && actions[0] == "no-op" {
		return true
	}
	return false
}

// String renders a DiffSummary as a single-line digest for logs / records.
func (d DiffSummary) String() string {
	if len(d.ChangedResources) == 0 {
		return "no drift"
	}
	var b strings.Builder
	for i, rc := range d.ChangedResources {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s:%s", rc.Address, strings.Join(rc.Actions, "+"))
	}
	return b.String()
}
