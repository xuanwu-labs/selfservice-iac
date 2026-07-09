package cli

import (
	"fmt"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"

	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/ai"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/approval"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/catalog"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/cost"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/drift"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/gate"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/mcp"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/request"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli/stack"
)

// Flag names as constants (multica flags.go pattern).
const (
	FlagServerURL = "server-url"
	FlagProfile   = "profile"
	FlagOutput    = "output"
	FlagDebug     = "debug"
)

// NewRootCmd creates the root cobra command for the Aether CLI.
// Takes a *Factory for dependency injection (gh pattern).
func NewRootCmd(f *cmdutil.Factory) *cobra.Command {
	root := &cobra.Command{
		Use:           "aether",
		Short:         "Aether — IaC self-service platform CLI",
		Long:          "Aether is the unified CLI for the Aether IaC self-service platform. It connects teams, modules, environments, and infrastructure through a single command-line interface.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit: %s, built: %s)", cmdutil.Version, cmdutil.Commit, cmdutil.Date),
	}

	// Persistent flags → inherited by every subcommand
	root.PersistentFlags().String(FlagServerURL, "", "Aether platform API URL (or $AETHER_SERVER_URL)")
	root.PersistentFlags().String(FlagProfile, "default", "config profile to use")
	root.PersistentFlags().StringP(FlagOutput, "o", "table", "output format: table|json|yaml")
	root.PersistentFlags().Bool(FlagDebug, false, "enable debug logging")

	// Command groups
	root.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "manage", Title: "Management Commands:"},
		&cobra.Group{ID: "observe", Title: "Observability Commands:"},
		&cobra.Group{ID: "ai", Title: "AI Commands:"},
	)

	// Register subcommands (each takes the factory)
	root.AddCommand(
		catalog.NewCmdCatalog(f),
		request.NewCmdRequest(f),
		stack.NewCmdStack(f),
		drift.NewCmdDrift(f),
		approval.NewCmdApproval(f),
		cost.NewCmdCost(f),
		gate.NewCmdGate(f),
		mcp.NewCmdMcp(f),
		ai.NewCmdAi(f),
	)

	// Promote flag overrides before any subcommand runs
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		serverURL, _ := cmd.Flags().GetString(FlagServerURL)
		profile, _ := cmd.Flags().GetString(FlagProfile)
		output, _ := cmd.Flags().GetString(FlagOutput)
		debug, _ := cmd.Flags().GetBool(FlagDebug)
		f.ApplyFlagOverrides(serverURL, profile, output, debug)
		return nil
	}

	return root
}
