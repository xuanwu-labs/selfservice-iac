package cost

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdCost — estimate and view infrastructure costs (Infracost).
func NewCmdCost(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "cost",
		Short:   "Estimate and view infrastructure costs",
		GroupID: "observe",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
