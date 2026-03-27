package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newSpecCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Create a session and launch interactive Claude for planning",
		Long: `Creates a new git worktree and launches Claude for planning and spec creation.

The worktree is created with a branch named sandbox/<date>-<hash>.
Claude runs interactively in the worktree to help you plan and create specs.

Use the /brainstorming and /writing-plans skills to guide Claude through the process.

After Claude exits, the session status is set based on whether PLAN.md exists:
- If PLAN.md exists: Status is "ready" and you can proceed to execute
- If PLAN.md doesn't exist: Status is "failed" (no plan was created)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpec(cmd, name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Friendly name for this session")

	return cmd
}

func runSpec(cmd *cobra.Command, sessionName string) error {
	// Find the repo root
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	// Create a new worktree
	cmd.Println("Creating worktree...")
	wt, err := worktree.Create(repoPath)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	cmd.Printf("  Branch: %s\n", wt.Branch)
	cmd.Printf("  Path:   %s\n", wt.Path)
	cmd.Println()

	// Create session in state
	cmd.Println("Creating session...")
	sess, err := state.Create(repoPath, state.CreateOptions{
		WorktreePath: wt.Path,
		Branch:       wt.Branch,
		Name:         sessionName,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	cmd.Printf("  Session ID: %s\n", sess.ID)
	if sessionName != "" {
		cmd.Printf("  Name:       %s\n", sessionName)
	}
	cmd.Println()

	// Launch interactive Claude
	cmd.Println("Launching Claude (interactive mode)...")
	cmd.Println("Tips:")
	cmd.Println("  - Use /brainstorming to explore the problem space")
	cmd.Println("  - Use /writing-plans to create a detailed PLAN.md")
	cmd.Println("  - Exit Claude with Ctrl+D or 'exit'")
	cmd.Println()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	// Run Claude interactively in the worktree
	claudeCmd := exec.CommandContext(context.Background(), "claude")
	claudeCmd.Dir = wt.Path
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	claudeCmd.Env = append(os.Environ(), "HOME="+home)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Run Claude
	// Claude may exit with non-zero on Ctrl+D, which is normal.
	// Silently continue as the user may have just canceled.
	_ = claudeCmd.Run()

	cmd.Println()
	cmd.Println("Claude session ended.")
	cmd.Println()

	// Check if PLAN.md exists to determine session status
	planPath := filepath.Join(wt.Path, "PLAN.md")
	_, err = os.Stat(planPath)
	planExists := err == nil

	if planExists {
		sess.Status = state.StatusReady
		cmd.Printf("✓ PLAN.md found - session ready for execution\n")
	} else {
		sess.Status = state.StatusFailed
		cmd.Println("✗ PLAN.md not found - session marked as failed")
		cmd.Println("  Next steps:")
		cmd.Printf("    1. cd %s\n", wt.Path)
		cmd.Println("    2. Create PLAN.md or run claude again")
		cmd.Printf("    3. claude-sandbox execute %s\n", sess.ID)
	}

	// Update session status
	if err := state.Update(repoPath, sess); err != nil {
		return fmt.Errorf("update session status: %w", err)
	}

	if planExists {
		cmd.Println()
		cmd.Println("Next steps:")
		cmd.Printf("  claude-sandbox execute %s\n", sess.ID)
		cmd.Println()
		cmd.Printf("View the session log:\n")
		cmd.Printf("  tail -f %s\n", sess.LogPath)
	}

	return nil
}
