// internal/cli/helpers.go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

// findRepoRoot finds the main repository root from cwd.
// Returns the main repo root even when called from inside a worktree,
// so session state in .claude-sandbox/sessions/ is always found.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	if !worktree.IsGitRepo(cwd) {
		return "", fmt.Errorf("not a git repository")
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return "", fmt.Errorf("detect git root: %w", err)
	}

	return wt.Repo, nil
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
