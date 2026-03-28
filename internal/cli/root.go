package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claude-sandbox",
		Short:   "Sandboxed execution environment for Claude Code",
		Long:    `claude-sandbox enables autonomous Claude Code execution in isolated containers with quality gates and external action blocking.`,
		Version: version,
	}

	cmd.AddCommand(newSpecCommand())
	cmd.AddCommand(newExecuteCommand())
	cmd.AddCommand(newShipCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newCleanCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newBuildCommand())

	return cmd
}
