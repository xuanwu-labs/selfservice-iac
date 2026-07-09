package stack

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdStack — inspect and manage Terramate stacks.
func NewCmdStack(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "stack",
		Short:   "Inspect and manage Terramate stacks",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
