package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// DefaultImage is the default sandbox container image.
	DefaultImage = "claude-sandbox:latest"

	// HistoryVolumeName is the Docker volume for Claude history persistence.
	HistoryVolumeName = "claude-sandbox-history"
)

// RunOptions configures container execution.
type RunOptions struct {
	Image        string
	WorktreePath string
	HomeDir      string
	SpecPath     string
	Interactive  bool
}

// BuildRunArgs generates docker run arguments.
func BuildRunArgs(opts RunOptions) []string {
	args := []string{"run", "--rm"}

	if opts.Interactive {
		args = append(args, "-it")
	}

	// Add mounts
	mounts := BuildMounts(MountOptions{
		WorktreePath: opts.WorktreePath,
		HomeDir:      opts.HomeDir,
	})

	for _, m := range mounts {
		args = append(args, m.ToDockerArgs()...)
	}

	// Add history volume
	args = append(args, "-v", HistoryVolumeName+":/home/claude/.claude/history")

	// Environment variables
	// Pass API key via environment inheritance (not visible in ps aux)
	args = append(args, "-e", "ANTHROPIC_API_KEY")
	args = append(args, "-e", "HOME=/home/claude")

	// Working directory
	args = append(args, "--workdir", "/workspace")

	// Image
	args = append(args, opts.Image)

	return args
}

// Run executes Claude in a sandbox container.
func Run(ctx context.Context, opts RunOptions) error {
	if opts.Image == "" {
		opts.Image = DefaultImage
	}

	// Validate API key is set (will be passed via environment inheritance)
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	if opts.HomeDir == "" {
		var err error
		opts.HomeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
	}

	args := BuildRunArgs(opts)

	// Add the command to run Claude
	claudeCmd := buildClaudeCommand(opts.SpecPath)
	args = append(args, "/bin/bash", "-c", claudeCmd)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildClaudeCommand(specPath string) string {
	// Shell-escape the spec path to prevent injection
	escaped := shellEscape(specPath)
	return fmt.Sprintf(`claude --dangerously-skip-permissions "Implement the spec at %s. Follow quality gates: build, lint, test, security, spec coverage, commit hygiene, and /review-code with grade A. Write COMPLETION.md when done."`, escaped)
}

// shellEscape escapes a string for safe use in shell commands.
// Uses single quotes and escapes any embedded single quotes.
func shellEscape(s string) string {
	// Single quotes prevent all shell interpretation except for single quotes themselves.
	// To include a single quote: end the single-quoted string, add an escaped single quote, restart.
	// 'foo'\''bar' → foo'bar
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ImageExists checks if the sandbox image exists locally.
func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// EnsureHistoryVolume creates the history volume if it doesn't exist.
func EnsureHistoryVolume() error {
	cmd := exec.Command("docker", "volume", "inspect", HistoryVolumeName)
	if cmd.Run() == nil {
		return nil // Already exists
	}

	cmd = exec.Command("docker", "volume", "create", HistoryVolumeName)
	return cmd.Run()
}

