// Package tenancy_test: ownership_test.go — covers ResolveOwnerKind (D5).
//
// These tests pin the MVP hardcoded ownership rules so that the Phase 2 swap to
// team_cloud_grants can be verified rule-by-rule. ResolveOwnerKind is a pure
// function, so no DB setup is required — these run in -short mode.
package tenancy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xuanwu-labs/selfservice-iac/server/core/tenancy"
)

func TestResolveOwnerKind_Global(t *testing.T) {
	// Global layer is always platform-owned, regardless of component.
	assert.Equal(t, "platform", tenancy.ResolveOwnerKind("global", "vpc"))
	assert.Equal(t, "platform", tenancy.ResolveOwnerKind("global", "iam"))
	assert.Equal(t, "platform", tenancy.ResolveOwnerKind("global", ""))
}

func TestResolveOwnerKind_MiddlewareDBA(t *testing.T) {
	// Middleware datastores → DBA team.
	for _, comp := range []string{"rds", "redis", "mongodb", "polardb", "mysql", "oss", "nas"} {
		assert.Equal(t, "dba", tenancy.ResolveOwnerKind("middleware", comp),
			"component %q should be dba-owned", comp)
	}
	// Token-boundary + case-insensitivity: "alicloud-RDS-mysql" still resolves.
	assert.Equal(t, "dba", tenancy.ResolveOwnerKind("middleware", "alicloud-RDS-mysql"))
	assert.Equal(t, "dba", tenancy.ResolveOwnerKind("middleware", "Polardb-Postgres"))
}

func TestResolveOwnerKind_MiddlewareDBA_NoFalsePositive(t *testing.T) {
	// P1-2 fix: token-boundary matching must NOT false-positive on substrings.
	// "my-sysql-service" contains "sql" but not the token "mysql" → middleware.
	assert.Equal(t, "middleware", tenancy.ResolveOwnerKind("middleware", "my-sysql-service"))
	// "predises" contains "redis" as substring but not as token → middleware.
	assert.Equal(t, "middleware", tenancy.ResolveOwnerKind("middleware", "predises"))
}

func TestResolveOwnerKind_MiddlewareOther(t *testing.T) {
	// Middleware non-datastores → middleware team.
	for _, comp := range []string{"kafka", "vpc-peering", "nlb", "cdn"} {
		assert.Equal(t, "middleware", tenancy.ResolveOwnerKind("middleware", comp),
			"component %q should be middleware-owned", comp)
	}
}

func TestResolveOwnerKind_Application(t *testing.T) {
	// Application layer is always business-team-owned (resolved elsewhere via
	// stacks.owner_team_id).
	assert.Equal(t, "business", tenancy.ResolveOwnerKind("application", "ecs"))
	assert.Equal(t, "business", tenancy.ResolveOwnerKind("application", "orders-api"))
}

func TestResolveOwnerKind_UnknownLayer(t *testing.T) {
	// Unknown / empty layer defaults to business (defensive — should not happen
	// in practice since layers are constrained to the seed set).
	assert.Equal(t, "business", tenancy.ResolveOwnerKind("unknown", "anything"))
	assert.Equal(t, "business", tenancy.ResolveOwnerKind("", "anything"))
}
