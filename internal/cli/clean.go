package cli

import (
	"fmt"

	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clean [repo-path]",
		Short: "Remove stale sandbox worktrees",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := "."
			if len(args) > 0 {
				repoPath = args[0]
			}
			return runClean(cmd, repoPath, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Remove without confirmation")
	return cmd
}

func runClean(cmd *cobra.Command, repoPath string, force bool) error {
	if !worktree.IsGitRepo(repoPath) {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	worktrees, err := worktree.List(repoPath)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	var sandboxWorktrees []worktree.Worktree
	for _, wt := range worktrees {
		if wt.IsSandbox() {
			sandboxWorktrees = append(sandboxWorktrees, wt)
		}
	}

	if len(sandboxWorktrees) == 0 {
		cmd.Println("No sandbox worktrees found.")
		return nil
	}

	cmd.Printf("Found %d sandbox worktree(s):\n", len(sandboxWorktrees))
	for _, wt := range sandboxWorktrees {
		cmd.Printf("  - %s (%s)\n", wt.Branch, wt.Path)
	}

	if !force {
		if !promptYesNo(cmd, "\nRemove all?", false) {
			return nil
		}
	}

	for _, wt := range sandboxWorktrees {
		cmd.Printf("Removing %s...\n", wt.Branch)
		if err := worktree.Remove(wt.Path); err != nil {
			cmd.PrintErrf("  Warning: %v\n", err)
		}
	}

	cmd.Println("Done.")
	return nil
}
