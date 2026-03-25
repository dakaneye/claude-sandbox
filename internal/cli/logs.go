package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View session logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}

func runLogs(cmd *cobra.Command, follow bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree: %w", err)
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		return fmt.Errorf("no session found: %w", err)
	}

	if _, err := os.Stat(sess.LogPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", sess.LogPath)
	}

	var tailCmd *exec.Cmd
	if follow {
		tailCmd = exec.Command("tail", "-f", sess.LogPath)
	} else {
		tailCmd = exec.Command("tail", "-100", sess.LogPath)
	}

	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr
	return tailCmd.Run()
}
