package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sandbox sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd)
		},
	}
}

func runList(cmd *cobra.Command) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sessions, err := state.List(repoPath)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		cmd.Println("No sessions found.")
		return nil
	}

	// Find active session
	activeID, _ := state.GetActiveID(repoPath)

	// Print header
	cmd.Printf("%-22s %-14s %-34s %-10s %s\n", "ID", "NAME", "BRANCH", "STATUS", "AGE")

	for _, sess := range sessions {
		name := sess.Name
		if len(name) > 14 {
			name = name[:11] + "..."
		}

		branch := sess.Branch
		if len(branch) > 34 {
			branch = branch[:31] + "..."
		}

		age := formatAge(sess.CreatedAt)

		marker := " "
		if sess.ID == activeID {
			marker = "*"
		}

		cmd.Printf("%s%-21s %-14s %-34s %-10s %s\n", marker, sess.ID, name, branch, string(sess.Status), age)
	}

	return nil
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
