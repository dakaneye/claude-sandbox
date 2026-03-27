// internal/cli/helpers.go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

// resolveSession resolves a session from the --session flag or interactively.
//
//nolint:unused // Will be used after commands are migrated from requireWorktree
func resolveSession(sessionFlag string) (*state.Session, error) {
	repoPath, err := findRepoRoot()
	if err != nil {
		return nil, err
	}

	return state.ResolveSession(repoPath, sessionFlag)
}

// findRepoRoot finds the root of the git repository from cwd.
//
//nolint:unused // Will be used after commands are migrated from requireWorktree
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	if !worktree.IsGitRepo(cwd) {
		return "", fmt.Errorf("not a git repository")
	}

	// Get the toplevel
	wt, err := worktree.Detect(cwd)
	if err != nil {
		return "", fmt.Errorf("detect git root: %w", err)
	}

	return wt.Path, nil
}

// requireWorktree detects the current git worktree from the working directory.
// Deprecated: Use resolveSession instead. This will be removed after commands are migrated.
func requireWorktree() (*worktree.Worktree, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return nil, fmt.Errorf("not inside a git worktree: %w", err)
	}

	return wt, nil
}

// promptYesNo prompts the user for a yes/no confirmation.
func promptYesNo(cmd *cobra.Command, question string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}

	cmd.Printf("%s %s ", question, suffix)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}
