package errors

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/asset"
	commonv1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
)

// This file holds contract tests guarding the agreement between the proto
// ErrorCode enum (code identity, buf-generated) and contracts/error-codes.yaml
// (code behavior). They fail when the two sources drift, which unit tests with
// synthetic data cannot catch.

// TestRealYamlLoads is the critical startup regression: if someone edits
// contracts/error-codes.yaml and breaks its structure (renames the top-level
// key, typos a field, leaves a malformed entry), Load fails at startup. This
// test loads the actual embedded YAML so the breakage surfaces in `go test`,
// not in production boot.
func TestRealYamlLoads(t *testing.T) {
	if asset.ErrorCodes == "" {
		t.Fatal("asset.ErrorCodes is empty — embed of error-codes.yaml failed")
	}
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load(real error-codes.yaml): %v", err)
	}
	if got := reg.NumEntries(); got < 15 {
		t.Errorf("real registry entries: want >= 15, got %d (did the yaml sync break?)", got)
	}
}

// TestEveryEnumValueHasYamlEntry iterates every ErrorCode enum value (except
// UNSPECIFIED) via proto reflection and asserts each has a behavior row in
// error-codes.yaml. This replaces the old hand-maintained registeredCodes list:
// because constants are now buf-generated from the enum, we can enumerate them
// authoritatively via reflection — no hand-written list to drift. If a code is
// added to the enum but the YAML row is missing, Lookup panics at runtime in a
// handler; this test surfaces it at CI time.
func TestEveryEnumValueHasYamlEntry(t *testing.T) {
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	enumValues := commonv1.ErrorCode_ERROR_CODE_UNSPECIFIED.Descriptor().Values()
	for i := 0; i < enumValues.Len(); i++ {
		vd := enumValues.Get(i)
		name := string(vd.Name())
		if name == "ERROR_CODE_UNSPECIFIED" {
			continue
		}
		key := enumKeyFromValueName(name)
		if _, ok := reg.EntryByKey(key); !ok {
			t.Errorf("enum value %s (key %q) has no behavior row in error-codes.yaml — "+
				"add it to contracts/error-codes.yaml and run make proto-gen", name, key)
		}
	}
}

// TestEveryYamlEntryHasEnumValue is the reverse drift check: every code in the
// YAML must correspond to an enum value. Without this, a code can exist in the
// contract but no handler can reference it via the typed enum (forcing a raw
// string, defeating the purpose). Detected by checking each YAML key, stripped
// of nothing, appears as ERROR_CODE_<key> in the enum.
func TestEveryYamlEntryHasEnumValue(t *testing.T) {
	enumValues := commonv1.ErrorCode_ERROR_CODE_UNSPECIFIED.Descriptor().Values()
	known := make(map[string]bool, enumValues.Len())
	for i := 0; i < enumValues.Len(); i++ {
		known[enumKeyFromValueName(string(enumValues.Get(i).Name()))] = true
	}
	for _, key := range yamlKeys(t) {
		if !known[key] {
			t.Errorf("yaml code %q has no corresponding ErrorCode enum value — "+
				"add ERROR_CODE_%s to common/enum.proto and run make proto-gen",
				key, key)
		}
	}
}

// TestNoYamlCodeGrpcMappingLost ensures every grpc_code value in the YAML is
// one toConnectCode recognizes. A typo like "ABORTTED" would silently map to
// CodeUnknown at runtime; this test makes it a CI failure.
//
// "OK" is exempted: IDEMPOTENCY_REPLAY uses grpc_code OK / http 200 because it
// returns the original resource on a duplicate request (not an error). toConnectCode
// deliberately maps OK to CodeUnknown (an OK error is nonsensical for New), but
// the replay path returns a success response, never calling New — so OK is valid
// in the YAML even though toConnectCode won't map it.
func TestNoYamlCodeGrpcMappingLost(t *testing.T) {
	reg, err := Load(asset.ErrorCodes)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	bogusBaseline := toConnectCode("BOGUS_NEVER_MATCH") // == CodeUnknown
	for _, key := range yamlKeys(t) {
		entry, ok := reg.EntryByKey(key)
		if !ok {
			continue
		}
		if entry.GRPCCode == "OK" {
			continue // known special case (idempotency replay returns success)
		}
		got := toConnectCode(entry.GRPCCode)
		// If toConnectCode fell through to default (CodeUnknown) but the YAML
		// value isn't actually "UNKNOWN", that's a typo.
		if got == bogusBaseline && entry.GRPCCode != "UNKNOWN" {
			t.Errorf("yaml code %q grpc_code %q not recognized by toConnectCode", key, entry.GRPCCode)
		}
	}
}

// enumKeyFromValueName converts an enum value name to its YAML key:
// ERROR_CODE_SCHEMA_INVALID → SCHEMA_INVALID.
func enumKeyFromValueName(name string) string {
	const prefix = "ERROR_CODE_"
	if len(name) > len(prefix) && name[:len(prefix)] == prefix {
		return name[len(prefix):]
	}
	return name
}

// yamlKeys loads the real YAML and returns all code keys. Uses the internal
// yamlConfig to avoid exposing registry internals.
func yamlKeys(t *testing.T) []string {
	t.Helper()
	var cfg yamlConfig
	if err := yaml.Unmarshal([]byte(asset.ErrorCodes), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := make([]string, 0, len(cfg.Errors))
	for _, e := range cfg.Errors {
		keys = append(keys, e.Code)
	}
	return keys
}

// _ keeps protoreflect referenced (used by enum reflection above).
var _ = protoreflect.EnumNumber(0)
