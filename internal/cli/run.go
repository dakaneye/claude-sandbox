package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/samueldacanay/claude-sandbox/internal/container"
	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var specPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Launch Claude in sandboxed container",
		Long: `Launches Claude Code in an isolated container to implement the specified spec.

Claude is prompted to follow these advisory quality gates:
  - Build must succeed
  - Lint must pass
  - Tests must pass
  - Security scan must pass
  - Spec coverage verified
  - Commit hygiene checked
  - /review-code must return grade A

Note: Quality gates are advisory prompts to Claude, not enforced checks.
COMPLETION.md is written when done (success or blocked).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, specPath)
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to spec file or directory (required)")
	_ = cmd.MarkFlagRequired("spec")

	return cmd
}

func runRun(cmd *cobra.Command, specPath string) error {
	absSpec, err := filepath.Abs(specPath)
	if err != nil {
		return fmt.Errorf("resolve spec path: %w", err)
	}

	if _, err := os.Stat(absSpec); os.IsNotExist(err) {
		return fmt.Errorf("spec not found: %s", absSpec)
	}

	wt, err := requireWorktree()
	if err != nil {
		return err
	}

	if !container.ImageExists(container.DefaultImage) {
		return fmt.Errorf("container image not found: %s\nRun: cd container && ./build.sh --load", container.DefaultImage)
	}

	if err := session.EnsureLogDir(); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	if err := container.EnsureHistoryVolume(); err != nil {
		return fmt.Errorf("create history volume: %w", err)
	}

	sess, err := session.New(wt.Path, absSpec)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	if err := sess.Save(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	// Ensure session state is saved on exit
	defer func() {
		if saveErr := sess.Save(); saveErr != nil {
			cmd.PrintErrf("Warning: failed to save session state: %v\n", saveErr)
		}
	}()

	cmd.Println("Starting sandboxed Claude session...")
	cmd.Printf("  Spec:      %s\n", specPath)
	cmd.Printf("  Worktree:  %s\n", wt.Path)
	cmd.Printf("  Container: %s\n", container.DefaultImage)
	cmd.Println()
	cmd.Println("Claude is working. You'll be notified on completion.")
	cmd.Printf("Session log: %s\n", sess.LogPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		cmd.Println("\nReceived interrupt, stopping...")
		cancel()
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	runErr := container.Run(ctx, container.RunOptions{
		Image:        container.DefaultImage,
		WorktreePath: wt.Path,
		HomeDir:      home,
		SpecPath:     absSpec,
		Interactive:  true,
	})

	if runErr != nil {
		sess.Complete(session.StatusFailed)
		sess.Error = runErr.Error()
	} else {
		sess.Complete(session.StatusSuccess)
	}

	return runErr
}
