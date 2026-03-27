package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newStopCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running sandbox session and container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")
	return cmd
}

func runStop(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	if sess.Status != state.StatusRunning {
		return fmt.Errorf("session is not running (status: %s)", sess.Status)
	}

	// Stop the Docker container if running
	if container.IsRunning(sess.WorktreePath) {
		cmd.Println("Stopping container...")
		if err := container.Stop(sess.WorktreePath); err != nil {
			cmd.PrintErrf("Warning: %v\n", err)
		}
	}

	sess.Status = state.StatusFailed
	sess.Error = "stopped by user"
	if err := state.Update(repoPath, sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	cmd.Println("Session stopped.")
	return nil
}
