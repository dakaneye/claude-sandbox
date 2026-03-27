package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newCleanCommand() *cobra.Command {
	var sessionFlag string
	var all bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove sandbox sessions and worktrees",
		Long: `Removes sandbox sessions and their associated worktrees.

Without flags, shows an interactive list of sessions to remove.
Use --session to remove a specific session, or --all to remove all sessions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, sessionFlag, all)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name to remove")
	cmd.Flags().BoolVar(&all, "all", false, "Remove all sessions")

	return cmd
}

func runClean(cmd *cobra.Command, sessionFlag string, all bool) error {
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

	var toRemove []*state.Session

	if sessionFlag != "" {
		// Remove specific session
		sess, err := state.Get(repoPath, sessionFlag)
		if err != nil {
			return err
		}
		toRemove = append(toRemove, sess)
	} else if all {
		// Remove all
		toRemove = sessions
	} else {
		// Interactive - list and confirm
		cmd.Printf("Found %d session(s):\n\n", len(sessions))
		for i, sess := range sessions {
			name := sess.ID
			if sess.Name != "" {
				name = fmt.Sprintf("%s (%s)", sess.Name, sess.ID)
			}
			cmd.Printf("  [%d] %s - %s\n", i+1, name, sess.Status)
		}
		cmd.Println()

		if !promptYesNo(cmd, "Remove all sessions?", false) {
			cmd.Println("Canceled.")
			return nil
		}
		toRemove = sessions
	}

	// Confirm if multiple
	if len(toRemove) > 1 && !all {
		if !promptYesNo(cmd, fmt.Sprintf("Remove %d sessions?", len(toRemove)), false) {
			return nil
		}
	}

	for _, sess := range toRemove {
		cmd.Printf("Removing %s...\n", sess.ID)

		// Remove worktree
		if err := worktree.Remove(sess.WorktreePath); err != nil {
			cmd.PrintErrf("  Warning (worktree): %v\n", err)
		}

		// Remove session state
		if err := state.Remove(repoPath, sess.ID); err != nil {
			cmd.PrintErrf("  Warning (session): %v\n", err)
		}
	}

	cmd.Println("Done.")
	return nil
}
