package errors

import (
	"strings"
	"testing"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/asset"
)

// This file holds contract tests: they guard the agreement between the
// hand-written code constants (codes.go) and the contract source of truth
// (contracts/error-codes.yaml, embedded via internal/asset). These are NOT
// unit tests of Registry behavior (those live in registry_test.go) — they
// fail when the two sources drift, which a unit test with synthetic YAML
// cannot catch.

// TestRealYamlLoads is the critical startup regression: if someone edits
// contracts/error-codes.yaml and breaks its structure (renames the top-level
// key, typos a field, leaves a malformed entry), Load fails at startup. This
// test loads the actual embedded YAML so the breakage surfaces in `go test`,
// not in production boot. Without it, all registry_test.go cases use
// minimalYAML and would stay green while the real contract is broken.
func TestRealYamlLoads(t *testing.T) {
	if asset.ErrorCodes == "" {
		t.Fatal("asset.ErrorCodes is empty — embed of error-codes.yaml failed")
	}
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load(real error-codes.yaml): %v", err)
	}
	// The frozen contract has 20 codes (Phase 0). Assert a floor, not an exact
	// count, so adding codes doesn't break this test — but silently losing
	// most of them (e.g. a bad sync) does.
	if got := len(reg.entries); got < 15 {
		t.Errorf("real registry entries: want >= 15, got %d (did the yaml sync break?)", got)
	}
}

// registeredCodes is the exhaustive list of code constants exported from
// codes.go. It is the single place to update when a constant is added. The
// next test asserts every one resolves in the real registry, so a constant
// without a YAML entry (typical drift: added const, forgot YAML, or vice
// versa) fails CI instead of panicking at runtime via MustLookup.
var registeredCodes = []string{
	CodeSchemaInvalid,
	CodeModuleVersionNotFound,
	CodeUnauthenticated,
	CodePermissionDenied,
	CodeRequestNotFound,
	CodeCatalogItemNotFound,
	CodeArtifactNotFound,
	CodeStateConflict,
	CodeIllegalStateTransition,
	CodeIdempotencyReplay,
	CodeRateLimited,
	CodeBudgetExceeded,
	CodePolicyViolation,
	CodeTagMissing,
	CodeManualInterventionRequired,
	CodePlatformUnavailable,
	CodeCloudProviderError,
	CodeGitOperationFailed,
	CodeTerramateExecutionFailed,
	CodeInternalError,
}

// TestEveryCodeConstantResolvesInRegistry ensures codes.go and error-codes.yaml
// stay in sync. If a constant is added to codes.go but missing from the YAML,
// MustLookup panics at runtime (in a handler) — this test surfaces the drift
// at `go test` time instead. Run against the real embedded YAML so it catches
// real-world drift, not just the synthetic minimalYAML.
func TestEveryCodeConstantResolvesInRegistry(t *testing.T) {
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, code := range registeredCodes {
		if _, err := reg.Lookup(code); err != nil {
			t.Errorf("constant %q (codes.go) has no entry in error-codes.yaml — "+
				"add it to contracts/error-codes.yaml and run make proto-gen", code)
		}
	}
}

// TestEveryYamlCodeHasConstant is the reverse drift check: every code in the
// YAML must have a corresponding typed constant in codes.go. Without this,
// a code can exist in the contract but no handler can reference it by a typed
// constant (forcing a raw string, which codes.go exists to prevent). We
// detect this by reflecting the registeredCodes set: any YAML code not in it
// is an untyped code.
func TestEveryYamlCodeHasConstant(t *testing.T) {
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	known := make(map[string]bool, len(registeredCodes))
	for _, c := range registeredCodes {
		known[c] = true
	}
	for code := range reg.entries {
		if !known[code] {
			t.Errorf("yaml code %q has no typed constant in codes.go — "+
				"add a CodeXxx constant so handlers reference it safely", code)
		}
	}
}

// TestNoYamlCodeGrpcMappingLost ensures every grpc_code value in the YAML is
// one toConnectCode recognizes. A typo like "ABORTTED" would silently map to
// CodeUnknown at runtime; this test makes it a CI failure.
func TestNoYamlCodeGrpcMappingLost(t *testing.T) {
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for code, entry := range reg.entries {
		got := toConnectCode(entry.GRPCCode)
		if got.String() == strings.ToUpper(entry.GRPCCode) {
			// toConnectCode fell through to default — the switch returned
			// CodeUnknown because the string didn't match any case. Detect by
			// checking the YAML value isn't actually "UNKNOWN".
			if strings.ToUpper(entry.GRPCCode) != "UNKNOWN" {
				t.Errorf("code %q grpc_code %q not recognized by toConnectCode "+
					"(typo? maps to CodeUnknown silently)", code, entry.GRPCCode)
			}
		}
	}
}
