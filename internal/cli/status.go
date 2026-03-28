package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newStatusCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of sandbox session",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")
	return cmd
}

func runStatus(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	// Basic session info
	cmd.Printf("Session: %s\n", sess.ID)
	if sess.Name != "" {
		cmd.Printf("Name:    %s\n", sess.Name)
	}
	cmd.Printf("Branch:  %s\n", sess.Branch)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Path:    %s\n", sess.WorktreePath)
	cmd.Println()

	// Timestamps
	const timeFmt = "2006-01-02 15:04:05"
	cmd.Printf("Spec created:        %s\n", sess.CreatedAt.Local().Format(timeFmt))

	if !sess.StartedAt.IsZero() {
		cmd.Printf("Execution started:   %s\n", sess.StartedAt.Local().Format(timeFmt))

		if !sess.CompletedAt.IsZero() {
			duration := sess.CompletedAt.Sub(sess.StartedAt).Round(time.Second)
			cmd.Printf("Execution completed: %s (%s)\n", sess.CompletedAt.Local().Format(timeFmt), duration)
		} else {
			elapsed := time.Since(sess.StartedAt).Round(time.Second)
			cmd.Printf("Elapsed:             %s\n", elapsed)
		}
	}
	cmd.Println()

	// Status-specific handling
	switch sess.Status {
	case state.StatusSpeccing:
		cmd.Println("Creating spec with Claude...")
	case state.StatusReady:
		cmd.Println("Session ready. Run 'claude-sandbox execute' to start.")
	case state.StatusRunning:
		// Read log and analyze
		logContent := readLogTail(sess.LogPath, 500)
		if logContent == "" {
			cmd.Println("Execution in progress. Log not available yet.")
			return nil
		}

		if !claudeAvailable() {
			cmd.Println("Execution in progress. (Claude CLI not available for analysis)")
			return nil
		}

		// Show spinner while analyzing
		type analyzeResult struct {
			analysis string
			fallback string
		}
		spinChars := []string{"|", "/", "-", "\\"}
		done := make(chan analyzeResult, 1)
		ctx := cmd.Context()
		go func() {
			analysis, reason := analyzeLog(ctx, logContent)
			done <- analyzeResult{analysis, reason}
		}()

		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Print("\r\033[K") // Clear spinner
				return ctx.Err()
			case result := <-done:
				fmt.Print("\r\033[K") // Clear spinner
				if result.analysis != "" {
					cmd.Println(result.analysis)
				} else {
					cmd.Println("Execution in progress.")
				}
				return nil
			case <-ticker.C:
				fmt.Printf("\r%s Analyzing...", spinChars[i%len(spinChars)])
				i++
			}
		}
	case state.StatusSuccess:
		cmd.Println("✓ Session completed successfully. Run 'claude-sandbox ship' to create PR.")
	case state.StatusFailed:
		cmd.Printf("✗ Failed: %s\n", sess.Error)
		cmd.Println("Run 'claude-sandbox execute' to retry.")
	case state.StatusBlocked:
		cmd.Println("⚠ Blocked: quality gates could not be satisfied.")
		cmd.Println("Check COMPLETION.md for details. Run 'claude-sandbox execute' to retry.")
	}

	return nil
}
