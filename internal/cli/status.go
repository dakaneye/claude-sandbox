package cli

import (
	"fmt"
	"os"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of current sandbox session",
		RunE:  runStatus,
	}
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree")
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		cmd.Println("No session found in this worktree.")
		return nil
	}

	cmd.Printf("Session: %s\n", sess.ID)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Started: %s\n", sess.StartedAt.Format("2006-01-02 15:04:05"))
	if !sess.CompletedAt.IsZero() {
		cmd.Printf("Completed: %s\n", sess.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	cmd.Printf("Duration: %s\n", sess.Duration().Round(1e9))
	cmd.Printf("Spec: %s\n", sess.SpecPath)
	cmd.Printf("Log: %s\n", sess.LogPath)

	return nil
}
