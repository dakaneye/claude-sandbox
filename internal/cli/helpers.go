package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

// requireWorktree detects the current git worktree from the working directory.
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
