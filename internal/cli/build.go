package cli

import (
	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
)

func newBuildCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the sandbox container image",
		Long: `Builds the claude-sandbox container image using apko and Docker.

Requires apko and Docker to be installed and running.
The image is built for the current architecture (amd64 or arm64).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return container.Build(cmd.OutOrStdout(), force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Rebuild even if image exists")

	return cmd
}
