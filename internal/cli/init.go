package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <project-path>",
		Short: "Create a git worktree for sandboxed work",
		Long: `Creates a new git worktree for isolated sandbox work.

The worktree is created with a branch named sandbox/<date>-<hash>.
Use this worktree for planning and spec creation before running
claude-sandbox run.`,
		Args: cobra.ExactArgs(1),
		RunE: runInit,
	}

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	projectPath := args[0]

	if !worktree.IsGitRepo(projectPath) {
		return fmt.Errorf("not a git repository: %s", projectPath)
	}

	cmd.Println("Creating worktree...")

	wt, err := worktree.Create(projectPath)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	cmd.Printf("  Branch: %s\n", wt.Branch)
	cmd.Printf("  Path:   %s\n", wt.Path)
	cmd.Println()
	cmd.Println("Worktree ready. Next steps:")
	cmd.Printf("  1. cd %s\n", wt.Path)
	cmd.Println("  2. Create your spec (or use Claude to plan)")
	cmd.Println("  3. claude-sandbox run --spec ./path/to/spec")

	return nil
}
