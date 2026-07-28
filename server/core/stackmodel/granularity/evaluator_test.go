// Package granularity_test: evaluator_test.go — pins the MVP behavior of
// Evaluate. These tests run in -short mode (pure function, no DB).
package granularity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/granularity"
)

func TestEvaluate_Default(t *testing.T) {
	// "per-component" is the MVP default and the canonical input.
	assert.Equal(t, "per-component", granularity.Evaluate("per-component"))
}

func TestEvaluate_IgnoresNonDefault(t *testing.T) {
	// MVP ignores non-default groupings and still returns "per-component".
	// Phase 2 will make these branch.
	assert.Equal(t, "per-component", granularity.Evaluate("per-space"))
	assert.Equal(t, "per-component", granularity.Evaluate("per-tenant"))
	assert.Equal(t, "per-component", granularity.Evaluate("anything-else"))
}

func TestEvaluate_Empty(t *testing.T) {
	// Empty stack_grouping (NULL or unset catalog item) defaults to per-component.
	assert.Equal(t, "per-component", granularity.Evaluate(""))
}
