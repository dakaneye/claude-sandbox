package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/samueldacanay/claude-sandbox/internal/config"
	"github.com/samueldacanay/claude-sandbox/internal/container"
	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var specPath string
	var timeout string
	var retries int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Launch Claude in sandboxed container",
		Long: `Launches Claude Code in an isolated container to implement the specified spec.

The container has quality gates enforced:
  - Build must succeed
  - Lint must pass
  - Tests must pass
  - Security scan must pass
  - Spec coverage verified
  - Commit hygiene checked
  - /review-code must return grade A

COMPLETION.md is written when done (success or blocked).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, specPath, timeout, retries)
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to spec file or directory (required)")
	cmd.Flags().StringVar(&timeout, "timeout", "2h", "Maximum execution time")
	cmd.Flags().IntVar(&retries, "retries", 3, "Max retry attempts per quality gate")
	_ = cmd.MarkFlagRequired("spec")

	return cmd
}

func runRun(cmd *cobra.Command, specPath, timeout string, retries int) error {
	absSpec, err := filepath.Abs(specPath)
	if err != nil {
		return fmt.Errorf("resolve spec path: %w", err)
	}

	if _, err := os.Stat(absSpec); os.IsNotExist(err) {
		return fmt.Errorf("spec not found: %s", absSpec)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree: %w", err)
	}

	if !container.ImageExists(container.DefaultImage) {
		return fmt.Errorf("container image not found: %s\nRun: cd container && ./build.sh --load", container.DefaultImage)
	}

	cfg, err := config.Load(wt.Path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validate and override from flags
	if timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("invalid timeout format %q: %w", timeout, err)
		}
		cfg.Timeout = timeout
	}
	if retries < 0 {
		return fmt.Errorf("retries must be non-negative, got %d", retries)
	}
	if retries > 0 {
		cfg.Retries = retries
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
		Timeout:      cfg.Timeout,
		Interactive:  true,
	})

	if runErr != nil {
		sess.Complete(session.StatusFailed)
		sess.Error = runErr.Error()
	} else {
		sess.Complete(session.StatusSuccess)
	}

	fireNotification(sess)

	return runErr
}

func fireNotification(_ *session.Session) {
	// Best-effort notification via claude-notify.
	// Implementation deferred to notification integration.
}
