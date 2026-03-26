package cli

import (
	"fmt"

	"github.com/samueldacanay/claude-sandbox/internal/container"
	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running sandbox session and container",
		RunE:  runStop,
	}
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	wt, err := requireWorktree()
	if err != nil {
		return err
	}

	sess, err := session.FindActive(wt.Path)
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

	// Stop the Docker container if running
	if container.IsRunning(wt.Path) {
		cmd.Println("Stopping container...")
		if err := container.Stop(wt.Path); err != nil {
			cmd.PrintErrf("Warning: %v\n", err)
		}
	}

	sess.Complete(session.StatusFailed)
	sess.Error = "stopped by user"
	if err := sess.Save(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	cmd.Println("Session stopped.")
	return nil
}
