package drift

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdDrift — detect and report infrastructure drift.
func NewCmdDrift(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "drift",
		Short:   "Detect and report infrastructure drift",
		GroupID: "observe",
	}
	cmd.AddCommand(
		newListCmd(f),
		newExplainCmd(f),
	)
	return cmd
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List drift records",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}

func newExplainCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "explain <id>",
		Short: "Explain a drift record in detail",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
