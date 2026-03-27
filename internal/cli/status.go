package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/session"
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
	wt, err := requireWorktree()
	if err != nil {
		return err
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		cmd.Println("No session found in this worktree.")
		return nil
	}

	// Basic session info
	cmd.Printf("Session: %s\n", sess.ID)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Elapsed: %s\n", sess.Duration().Round(time.Second))
	cmd.Println()

	// If completed, skip analysis
	if sess.Status != session.StatusRunning {
		if sess.Status == session.StatusSuccess {
			cmd.Println("Session completed successfully. See COMPLETION.md")
		} else {
			cmd.Printf("Session failed: %s\n", sess.Error)
		}
		return nil
	}

	// Read log and analyze
	logContent := readLogTail(sess.LogPath, 500)
	if logContent == "" {
		cmd.Println("Log not available yet")
		return nil
	}

	if !claudeAvailable() {
		cmd.Println("(Claude CLI not available for analysis)")
		return nil
	}

	// Show spinner while analyzing
	spinChars := []string{"|", "/", "-", "\\"}
	done := make(chan string, 1)
	ctx := cmd.Context()
	go func() {
		done <- analyzeLog(ctx, logContent)
	}()

	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\033[K") // Clear spinner
			return ctx.Err()
		case analysis := <-done:
			fmt.Print("\r\033[K") // Clear spinner
			if analysis == "" {
				cmd.Println("Could not analyze log")
			} else {
				cmd.Println(analysis)
			}
			return nil
		case <-ticker.C:
			fmt.Printf("\r%s Analyzing...", spinChars[i%len(spinChars)])
			i++
		}
	}
}
