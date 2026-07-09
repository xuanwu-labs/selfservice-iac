package mcp

import (
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

// NewCmdMcp — expose Aether as an MCP (Model Context Protocol) server.
func NewCmdMcp(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "mcp",
		Short:   "Expose Aether as an MCP server for AI agents",
		GroupID: "ai",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = f
			return nil
		},
	}
}
