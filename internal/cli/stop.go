package cli

import (
	"fmt"
	"os"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running sandbox session",
		RunE:  runStop,
	}
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree: %w", err)
	}

	sess, err := session.FindActive(wt.Path)
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

	sess.Complete(session.StatusFailed)
	sess.Error = "stopped by user"
	if err := sess.Save(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	cmd.Println("Session stopped.")
	return nil
}
