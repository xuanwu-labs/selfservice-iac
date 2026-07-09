package approval

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdApproval — approve or reject pending requests.
func NewCmdApproval(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "approval",
		Short:   "Approve or reject pending requests",
		GroupID: "manage",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
