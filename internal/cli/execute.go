package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newExecuteCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute Claude in sandboxed container",
		Long: `Executes Claude Code in an isolated container to implement the previously created spec.

Requires that a session has been created via 'spec' command. The session must have
a PLAN.md file describing the work to be done.

Claude is prompted to follow these advisory quality gates:
  - Build must succeed
  - Lint must pass
  - Tests must pass
  - Security scan must pass
  - Spec coverage verified
  - Commit hygiene checked
  - /review-code must return grade A

Note: Quality gates are advisory prompts to Claude, not enforced checks.
COMPLETION.md is written when done (success, blocked, or failed).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name (optional, uses active if not specified)")

	return cmd
}

func runExecute(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	// Resolve session (uses active or picker if not specified)
	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	// Check COMPLETION.md for early exit if already succeeded
	completionPath := filepath.Join(sess.WorktreePath, "COMPLETION.md")
	if content, err := os.ReadFile(completionPath); err == nil {
		if strings.Contains(string(content), "Status: SUCCESS") {
			cmd.Println("Session already completed successfully.")
			return nil
		}
	}

	// Check PLAN.md exists
	planPath := filepath.Join(sess.WorktreePath, "PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("PLAN.md not found in worktree: %s", planPath)
	}

	// Auto-build container image if missing
	if !container.ImageExists(container.DefaultImage) {
		cmd.Println("Container image not found. Building...")
		if err := container.Build(cmd.OutOrStdout(), false); err != nil {
			return fmt.Errorf("build image: %w", err)
		}
		cmd.Println()
	}

	// Ensure log directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	logDir := filepath.Join(home, ".claude/sandbox-sessions")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Ensure history volume exists
	if err := container.EnsureHistoryVolume(); err != nil {
		return fmt.Errorf("create history volume: %w", err)
	}

	// Update session status to running
	sess.Status = state.StatusRunning
	if err := state.Update(repoPath, sess); err != nil {
		return fmt.Errorf("update session status: %w", err)
	}

	// Ensure session state is saved on exit
	defer func() {
		if saveErr := state.Update(repoPath, sess); saveErr != nil {
			cmd.PrintErrf("Warning: failed to save session state: %v\n", saveErr)
		}
	}()

	cmd.Println("Starting sandboxed Claude session...")
	cmd.Printf("  Session:   %s\n", sess.ID)
	if sess.Name != "" {
		cmd.Printf("  Name:      %s\n", sess.Name)
	}
	cmd.Printf("  Worktree:  %s\n", sess.WorktreePath)
	cmd.Printf("  Container: %s\n", container.DefaultImage)
	cmd.Printf("  Log:       %s\n", sess.LogPath)
	cmd.Println()

	// Create log file for capturing container output
	logFile, err := os.Create(sess.LogPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		fmt.Print("\r\033[K") // Clear the spinner line
		cmd.Println("Received interrupt, stopping...")
		cancel()
	}()

	// Run container in background and show spinner
	done := make(chan error, 1)
	go func() {
		done <- container.Run(ctx, container.RunOptions{
			Image:        container.DefaultImage,
			WorktreePath: sess.WorktreePath,
			HomeDir:      home,
			SpecPath:     planPath,
			Interactive:  false,
			LogWriter:    logFile,
		})
	}()

	// Spinner with elapsed time
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case runErr := <-done:
			fmt.Print("\r\033[K") // Clear spinner line
			elapsed := time.Since(start).Round(time.Second)

			// Read COMPLETION.md to determine final status
			if content, err := os.ReadFile(completionPath); err == nil {
				if strings.Contains(string(content), "Status: SUCCESS") {
					sess.Status = state.StatusSuccess
				} else if strings.Contains(string(content), "Status: BLOCKED") {
					sess.Status = state.StatusBlocked
				} else {
					sess.Status = state.StatusFailed
				}
			} else if runErr != nil {
				sess.Status = state.StatusFailed
				sess.Error = runErr.Error()
			} else {
				sess.Status = state.StatusFailed
			}

			sess.CompletedAt = time.Now()

			if runErr != nil {
				cmd.Printf("✗ Failed after %s\n", elapsed)
				return runErr
			}

			switch sess.Status {
			case state.StatusSuccess:
				cmd.Printf("✓ Completed in %s\n", elapsed)
			case state.StatusBlocked:
				cmd.Printf("⊘ Blocked after %s\n", elapsed)
			default:
				cmd.Printf("✗ Failed after %s\n", elapsed)
			}

			return nil
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Printf("\r%s Claude working... %s", spinChars[i%len(spinChars)], elapsed)
			i++
		}
	}
}
