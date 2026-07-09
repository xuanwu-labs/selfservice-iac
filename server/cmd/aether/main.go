// Package main is the Aether CLI entry point.
// Thin: just ldflags + Execute. All command logic lives in internal/cli/.
package main

import (
	"os"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmdutil.Version = version
	cmdutil.Commit = commit
	cmdutil.Date = date

	f := cmdutil.NewFactory()
	root := cli.NewRootCmd(f)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
