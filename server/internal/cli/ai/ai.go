package ai

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdAi — natural language interface to Aether (D17).
func NewCmdAi(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "ai",
		Short:   "Natural language interface to Aether",
		Long:    "ai converts natural language intent into platform actions (skill-matched). E.g., aether ai \"create a 4C8G MySQL for order-service\"",
		GroupID: "ai",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
