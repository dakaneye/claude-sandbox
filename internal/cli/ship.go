package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/samueldacanay/claude-sandbox/internal/worktree"
)

func newShipCommand() *cobra.Command {
	var skipReview bool
	var keepWorktree bool

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Create PR after reviewing completed work",
		Long: `Creates a PR for the completed sandbox work using the /create-pr skill.

Prerequisites:
  - COMPLETION.md must exist with SUCCESS status
  - User must review and confirm before PR creation

The command:
  1. Validates COMPLETION.md exists with SUCCESS status
  2. Prompts user to review COMPLETION.md
  3. Prompts for confirmation
  4. Invokes Claude with /create-pr skill
  5. Optionally cleans up the worktree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShip(cmd, skipReview, keepWorktree)
		},
	}

	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip COMPLETION.md review prompt")
	cmd.Flags().BoolVar(&keepWorktree, "keep-worktree", false, "Don't clean up worktree after shipping")

	return cmd
}

func runShip(cmd *cobra.Command, skipReview, keepWorktree bool) error {
	wt, err := requireWorktree()
	if err != nil {
		return err
	}

	completionPath := filepath.Join(wt.Path, "COMPLETION.md")
	if _, err := os.Stat(completionPath); os.IsNotExist(err) {
		return fmt.Errorf("COMPLETION.md not found. Run 'claude-sandbox run' first")
	}

	content, err := os.ReadFile(completionPath)
	if err != nil {
		return fmt.Errorf("read COMPLETION.md: %w", err)
	}

	if !strings.Contains(string(content), "Status: SUCCESS") {
		return fmt.Errorf("COMPLETION.md does not show SUCCESS status. Cannot ship blocked or failed work")
	}

	if !skipReview {
		if !promptYesNo(cmd, "Review COMPLETION.md before shipping?", true) {
			return fmt.Errorf("shipping cancelled")
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "less"
		}
		editorCmd := exec.Command(editor, completionPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		_ = editorCmd.Run()
	}

	if !promptYesNo(cmd, "Ship this work?", false) {
		return fmt.Errorf("shipping cancelled")
	}

	cmd.Println("Launching Claude to create PR via /create-pr...")

	claudeCmd := exec.Command("claude", "--dangerously-skip-permissions", "/create-pr")
	claudeCmd.Dir = wt.Path
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	if !keepWorktree {
		if promptYesNo(cmd, "Clean up worktree?", true) {
			if err := worktree.Remove(wt.Path); err != nil {
				cmd.PrintErrf("Warning: failed to remove worktree: %v\n", err)
			} else {
				cmd.Println("Worktree removed.")
			}
		}
	}

	return nil
}
