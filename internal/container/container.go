package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	APIKey       string
	SpecPath     string
	Timeout      string
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
	args = append(args, "-e", "ANTHROPIC_API_KEY="+opts.APIKey)
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

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if opts.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
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
	return fmt.Sprintf(`claude --dangerously-skip-permissions "Implement the spec at %s. Follow quality gates: build, lint, test, security, spec coverage, commit hygiene, and /review-code with grade A. Write COMPLETION.md when done."`, specPath)
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

// GetSessionLogPath returns the path for session logs.
func GetSessionLogPath(sessionID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "sandbox-sessions", sessionID+".log")
}
