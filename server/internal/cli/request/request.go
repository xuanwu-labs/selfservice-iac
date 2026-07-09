package request

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdRequest — submit and track infrastructure requests.
func NewCmdRequest(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "request",
		Short:   "Submit and track infrastructure requests",
		GroupID: "core",
	}
	// Sub-subcommands (skeleton)
	cmd.AddCommand(
		newCreateCmd(f),
		newListCmd(f),
		newWaitCmd(f),
		newShowCmd(f),
	)
	return cmd
}

func newCreateCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Submit a new infrastructure request",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List your requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newWaitCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "wait <id>",
		Short: "Wait for a request to complete",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newShowCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a request",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
