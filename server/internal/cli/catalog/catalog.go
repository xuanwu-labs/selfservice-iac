package catalog

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdCatalog — browse and search the service catalog.
func NewCmdCatalog(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "catalog",
		Short:   "Browse and search the service catalog",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: list catalog items via API client
			_ = f // factory available for deps
			return nil
		},
	}
	// TODO: add list/search subcommands
	return cmd
}
