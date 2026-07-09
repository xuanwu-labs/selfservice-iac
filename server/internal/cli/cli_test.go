package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/cli"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/cmdutil"
)

var allExpectedSubcommands = []string{
	"catalog", "request", "stack", "drift", "approval", "cost", "gate", "mcp", "ai",
}

func newTestFactory() *cmdutil.Factory {
	return cmdutil.NewFactory()
}

func TestRootHelpContainsAllSubcommands(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	output := buf.String()
	for _, sub := range allExpectedSubcommands {
		assert.True(t, strings.Contains(output, sub), "help should contain %q", sub)
	}
}

func TestSubcommandCount(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	var cmds []string
	for _, cmd := range root.Commands() {
		if cmd.IsAvailableCommand() && cmd.Name() != "completion" && cmd.Name() != "help" {
			cmds = append(cmds, cmd.Name())
		}
	}
	assert.Len(t, cmds, 9)
}

func TestVersion(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "dev")
}

func TestCommandGroups(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	groups := root.Groups()
	require.Len(t, groups, 4)
	ids := make(map[string]bool)
	for _, g := range groups {
		ids[g.ID] = true
	}
	assert.True(t, ids["core"])
	assert.True(t, ids["manage"])
	assert.True(t, ids["observe"])
	assert.True(t, ids["ai"])
}

func TestGlobalFlags(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	flags := root.PersistentFlags()
	assert.NotNil(t, flags.Lookup("server-url"))
	assert.NotNil(t, flags.Lookup("profile"))
	assert.NotNil(t, flags.Lookup("output"))
	assert.NotNil(t, flags.Lookup("debug"))
}

func TestGateSubSubcommands(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	gateCmd, _, err := root.Find([]string{"gate"})
	require.NoError(t, err)
	var names []string
	for _, sc := range gateCmd.Commands() {
		names = append(names, sc.Name())
	}
	assert.Contains(t, names, "wait")
	assert.Contains(t, names, "release")
	assert.Contains(t, names, "reject")
	assert.Contains(t, names, "status")
}

func TestRequestSubSubcommands(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	reqCmd, _, err := root.Find([]string{"request"})
	require.NoError(t, err)
	var names []string
	for _, sc := range reqCmd.Commands() {
		names = append(names, sc.Name())
	}
	assert.Contains(t, names, "create")
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "wait")
	assert.Contains(t, names, "show")
}

func TestDriftSubSubcommands(t *testing.T) {
	root := cli.NewRootCmd(newTestFactory())
	driftCmd, _, err := root.Find([]string{"drift"})
	require.NoError(t, err)
	var names []string
	for _, sc := range driftCmd.Commands() {
		names = append(names, sc.Name())
	}
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "explain")
}
