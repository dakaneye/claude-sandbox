package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/session"
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
	cmd.Printf("  Log:       %s\n", sess.LogPath)
	cmd.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		fmt.Print("\r\033[K") // Clear the spinner line
		cmd.Println("Received interrupt, stopping...")
		cancel()
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	// Run container in background and show spinner
	done := make(chan error, 1)
	go func() {
		done <- container.Run(ctx, container.RunOptions{
			Image:        container.DefaultImage,
			WorktreePath: wt.Path,
			HomeDir:      home,
			SpecPath:     absSpec,
			Interactive:  false,
		})
	}()

	// Spinner with elapsed time
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case runErr := <-done:
			fmt.Print("\r\033[K") // Clear spinner line
			elapsed := time.Since(start).Round(time.Second)
			if runErr != nil {
				sess.Complete(session.StatusFailed)
				sess.Error = runErr.Error()
				cmd.Printf("✗ Failed after %s\n", elapsed)
			} else {
				sess.Complete(session.StatusSuccess)
				cmd.Printf("✓ Completed in %s\n", elapsed)
			}
			return runErr
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Printf("\r%s Claude working... %s", spinChars[i%len(spinChars)], elapsed)
			i++
		}
	}
}
