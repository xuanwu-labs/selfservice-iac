// Package adapters provides the wire ProviderSet for all six pluggable
// adapters (D7). Each adapter is bound to its noop stub by default;
// future changes replace stubs with real implementations via wire.Bind.
package adapters

import (
	"github.com/google/wire"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/cloud"
	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/cost"
	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/git"
	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/notify"
	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/policy"
	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/state"
)

// ProviderSet binds all six adapter interfaces to their noop defaults.
// When a real implementation is added, add a wire.Bind here:
//
//	wire.Bind(new(git.GitProvider), new(goGitProvider.Provider))
var ProviderSet = wire.NewSet(
	// Noop stubs as both concrete type and interface binding.
	wire.Bind(new(git.GitProvider), new(git.NoopGit)),
	wire.Bind(new(cloud.CloudProvider), new(cloud.NoopCloud)),
	wire.Bind(new(state.StateBackend), new(state.NoopState)),
	wire.Bind(new(policy.PolicyEngine), new(policy.NoopPolicy)),
	wire.Bind(new(cost.CostEstimator), new(cost.NoopCost)),
	wire.Bind(new(notify.Notifier), new(notify.NoopNotifier)),
)
