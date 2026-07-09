package gate

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdGate — CICD integration gate (wait/release/reject/status).
func NewCmdGate(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gate",
		Short:   "CICD integration gate (wait/release/reject)",
		GroupID: "manage",
	}
	cmd.AddCommand(
		newWaitCmd(f),
		newReleaseCmd(f),
		newRejectCmd(f),
		newStatusCmd(f),
	)
	return cmd
}

func newWaitCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "wait <id>",
		Short: "Block until gate <id> is released or rejected",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newReleaseCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "release <id>",
		Short: "Release gate <id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newRejectCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject gate <id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newStatusCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show gate <id> status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
