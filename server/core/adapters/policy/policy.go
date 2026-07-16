// Package policy defines the PolicyEngine adapter interface (D7).
//
// The gate module uses this to evaluate OPA policies during the 9-stage
// parameter pipeline (D28 S3/S6) and admission checks. The default
// implementation is a noop stub; Phase 1 ships a native Go policy engine
// before adding OPA.
package policy

import (
	"context"
	"fmt"
)

// Result holds the outcome of a policy evaluation.
type Result struct {
	Allow      bool
	Violations []string
}

// PolicyEngine abstracts policy evaluation so the platform can use OPA,
// native Go checks, or any other engine.
type PolicyEngine interface {
	// Evaluate runs the named policy against the given input and returns
	// the decision. If the policy denies, Violations lists the reasons.
	Evaluate(ctx context.Context, policy string, input any) (Result, error)
}

// NoopPolicy is the default stub.
type NoopPolicy struct{}

// Evaluate returns a structured error.
func (NoopPolicy) Evaluate(_ context.Context, _ string, _ any) (Result, error) {
	return Result{}, fmt.Errorf("policy adapter not configured: set adapters.policy.impl in config")
}
