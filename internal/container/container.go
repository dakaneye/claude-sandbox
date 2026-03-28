package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	// DefaultImage is the default sandbox container image.
	DefaultImage = "claude-sandbox:latest"

	// HistoryVolumeName is the Docker volume for Claude history persistence.
	HistoryVolumeName = "claude-sandbox-history"

	// ContainerNamePrefix is the prefix for sandbox container names.
	ContainerNamePrefix = "claude-sandbox-"
)

// RunOptions configures container execution.
type RunOptions struct {
	Image        string
	WorktreePath string
	HomeDir      string
	SpecPath     string
	Interactive  bool
	LogWriter    io.Writer // If set, container output is written here
}

// BuildRunArgs generates docker run arguments.
func BuildRunArgs(opts RunOptions) []string {
	args := []string{"run", "--rm"}

	// Name the container for later reference (e.g., stop command)
	args = append(args, "--name", ContainerName(opts.WorktreePath))

	if opts.Interactive {
		args = append(args, "-it")
	} else {
		// Allocate PTY for non-interactive mode so Claude outputs to stdout.
		// Without PTY, Claude's UI doesn't write to stdout/stderr, resulting in empty logs.
		args = append(args, "-t")
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

	// Clear entrypoint since base image has /bin/bash as entrypoint
	// and we provide our own command
	args = append(args, "--entrypoint", "")

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

	// Convert host spec path to container path
	containerSpecPath := hostToContainerPath(opts.SpecPath, opts.WorktreePath)

	// Add the command to run Claude
	claudeCmd := buildClaudeCommand(containerSpecPath)
	args = append(args, "/bin/bash", "-c", claudeCmd)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	if opts.LogWriter != nil {
		cmd.Stdout = opts.LogWriter
		cmd.Stderr = opts.LogWriter
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

func buildClaudeCommand(specPath string) string {
	// Shell-escape the spec path to prevent injection
	escaped := shellEscape(specPath)
	// Holistic prompt: assess current state, do only what's needed to reach grade A
	// -p (print mode) skips the workspace trust dialog for non-interactive execution
	// --verbose enables detailed output logging
	return fmt.Sprintf(`claude --dangerously-skip-permissions --verbose -p "Your goal: get this project to pass all quality gates with /review-code grade A.

Spec: %s

First, assess the current state:
- Check git status, existing code, COMPLETION.md, any prior work
- Identify what's already done vs what's blocking grade A

Then do only what's needed:
- If not implemented, implement the spec
- If implemented but failing gates, fix the failures
- If passing gates but review grade < A, fix only the review feedback
- If grade A, verify gates still pass

Quality gates: build, lint, test, /review-code grade A.
Keep iterating on review feedback until grade A (do not stop at B or lower).
Update COMPLETION.md with final status."`, escaped)
}

// shellEscape escapes a string for safe use in shell commands.
// Uses single quotes and escapes any embedded single quotes.
func shellEscape(s string) string {
	// Single quotes prevent all shell interpretation except for single quotes themselves.
	// To include a single quote: end the single-quoted string, add an escaped single quote, restart.
	// 'foo'\''bar' → foo'bar
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// hostToContainerPath converts a host path to a container path.
// Paths inside the worktree are mapped to /workspace.
func hostToContainerPath(hostPath, worktreePath string) string {
	if strings.HasPrefix(hostPath, worktreePath) {
		relPath := strings.TrimPrefix(hostPath, worktreePath)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" {
			return "/workspace"
		}
		return "/workspace/" + relPath
	}
	// Path outside worktree - return as-is (won't be accessible in container)
	return hostPath
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

// ContainerName generates a deterministic container name from the worktree path.
func ContainerName(worktreePath string) string {
	hash := sha256.Sum256([]byte(worktreePath))
	return ContainerNamePrefix + hex.EncodeToString(hash[:])[:12]
}

// Stop stops a running sandbox container by worktree path.
func Stop(worktreePath string) error {
	name := ContainerName(worktreePath)
	cmd := exec.Command("docker", "stop", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop container %s: %w", name, err)
	}
	return nil
}

// IsRunning checks if a sandbox container is running for the given worktree.
func IsRunning(worktreePath string) bool {
	name := ContainerName(worktreePath)
	cmd := exec.Command("docker", "container", "inspect", "-f", "{{.State.Running}}", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}
