// probe.go — DELIBERATELY BUGGY EVALUATION TARGET (checked in on purpose).
// This file tests whether the code-reviewer / security-reviewer / architect
// subagents catch planted issues. It is NOT real platform code and MUST NOT be
// compiled by the normal build (it is outside any Go module root, under .zcode/).
// Every issue is tagged with the agent that should flag it.
//
// Context: simulates a (wrong) TerramateAdapter implementation that violates
// design decision D1, plus has generic quality and security bugs.
//
// Why checked in: serves as a regression target for reviewer subagents and
// documents the kinds of violations they must catch. It lives under .zcode/
// (agent tooling area), NOT under server/ (the Go module), so `go build ./...`
// never sees it.

package probe

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	// ARCHITECT P0 — D1 VIOLATION: importing Terramate internal packages.
	// D1 requires exec-based adapter, MUST NOT import internals.
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/config"
)

// SECURITY P0 — hardcoded cloud credential (should come from cloud_credentials table / D23).
const awsAccessKey = "AKIAIOSFODNN7EXAMPLE"
const awsSecretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" // SECURITY P0: hardcoded secret

// Adapter is a (wrong) Terramate adapter.
type Adapter struct {
	db *sql.DB
}

// RunPlan simulates running `terramate run ... plan`.
func (a *Adapter) RunPlan(stackID string) error {
	// ARCHITECT P0 — D1 VIOLATION: calling Terramate internals directly instead of exec.
	stks := config.LoadAllStacks(stackID) // wrong: internal API, not CLI exec
	for _, s := range stks {
		_ = stack.Run(s) // QUALITY P0: unchecked error return
	}
	return nil
}

// FindRequest is dead/unused on purpose (QUALITY P2: dead code).
func (a *Adapter) FindRequest(id int) (string, error) {
	return "", nil
}

// QueryRequests — SECURITY P0: SQL injection via string concatenation.
func (a *Adapter) QueryRequests(teamFilter string) (*sql.Rows, error) {
	q := fmt.Sprintf("SELECT id, status FROM requests WHERE team = '%s'", teamFilter) // SECURITY P0: SQLi
	return a.db.Query(q)
}

// ExecApply — ARCHITECT P0: D20 VIOLATION. Orchestrator/adapter MUST NOT hardcode a
// specific executor mode. Here it shells out to docker directly.
func (a *Adapter) ExecApply(stackID string) error {
	// SECURITY P0: shell injection via sh -c with stackID (user-influenced).
	cmd := exec.Command("sh", "-c", "terraform apply "+stackID) // SECURITY P0 + ARCHITECT P0 (D20)
	cmd.Run()                                                    // QUALITY P0: unchecked error; also no stdout/stderr capture (spec 06)
	return nil
}

// LogCredential — SECURITY P0: credential written to logs in cleartext.
func (a *Adapter) LogCredential() {
	fmt.Printf("using credential: %s\n", awsSecretKey) // SECURITY P0: secret in logs
}

// ResolveParams — ARCHITECT P0: D28 VIOLATION. 9-stage param pipeline must write
// provenance to resolved_params_json. This bypasses provenance entirely.
func (a *Adapter) ResolveParams(vals []string) map[string]string {
	out := make(map[string]string)
	for i := 0; i <= len(vals); i++ { // QUALITY P0: off-by-one (<= should be <)
		k := fmt.Sprintf("p%d", i)
		out[k] = vals[i] // QUALITY P0: will panic on last iter (slice bounds)
	}
	// ARCHITECT P0: no provenance written — D28 requires resolved_params_json with provenance.
	return out
}

// BreakGlass — ARCHITECT P0: D30 VIOLATION. break-glass requires audit + dual-control.
func (a *Adapter) BreakGlass(stackID string) error {
	// No audit log, no dual-control, no break-glass record.
	return a.ExecApply(stackID) // ARCHITECT P0: D30 bypassed
}

// ResolvePath — ARCHITECT P0: D29 VIOLATION. PathGenerator must be layer-first,
// env+tenant aware (D27). This hardcodes a single-dimension path.
func (a *Adapter) ResolvePath(tenant string) string {
	return "/stacks/" + tenant // ARCHITECT P0: missing env dimension (D27); no layer-first (D29)
}

// Sum is a trivial helper with an unused result to silence linters in real code.
func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	_ = strings.TrimSpace(fmt.Sprintf("%d", total)) // QUALITY P2: useless work / dead
	return total
}
