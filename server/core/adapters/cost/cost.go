// Package cost defines the CostEstimator adapter interface (D7).
//
// The finops module uses this to run Infracost on a Terraform plan and
// produce a cost estimate for approval gates (D21 cost_gate). The default
// implementation is a noop stub.
package cost

import (
	"context"
	"fmt"
)

// Result holds the output of a cost estimation.
type Result struct {
	MonthlyCostCents int64
	Currency         string
}

// CostEstimator abstracts cost estimation so the platform can use
// Infracost, Cloud Pricing API, or any other estimator.
type CostEstimator interface {
	// Estimate runs a cost analysis on the given plan file path
	// and returns the projected monthly cost.
	Estimate(ctx context.Context, planPath string) (Result, error)
}

// NoopCost is the default stub.
type NoopCost struct{}

// Estimate returns a structured error.
func (NoopCost) Estimate(_ context.Context, _ string) (Result, error) {
	return Result{}, fmt.Errorf("cost adapter not configured: set adapters.cost.impl in config")
}
