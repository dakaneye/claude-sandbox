// Package worktree provides git worktree operations for sandbox isolation.
package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/samueldacanay/claude-sandbox/internal/id"
)

// Worktree represents a git worktree.
type Worktree struct {
	Path   string
	Branch string
	Repo   string
}

// IsGitRepo checks if the path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

// Create creates a new git worktree for sandbox work.
func Create(repoPath string) (*Worktree, error) {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	if !IsGitRepo(absRepo) {
		return nil, fmt.Errorf("not a git repository: %s", absRepo)
	}

	// Generate branch and path names
	hash := id.RandomHex(6)
	date := time.Now().Format("2006-01-02")
	branch := fmt.Sprintf("sandbox/%s-%s", date, hash)
	worktreePath := fmt.Sprintf("%s-sandbox-%s", absRepo, hash)

	// Create the worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = absRepo
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("create worktree: %w\noutput: %s", err, output)
	}

	return &Worktree{
		Path:   worktreePath,
		Branch: branch,
		Repo:   absRepo,
	}, nil
}

// List returns all worktrees for a repository.
func List(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
			current.Repo = repoPath
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			// Convert refs/heads/branch to branch
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		}
	}

	// Don't forget the last one
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// Remove removes a worktree and its branch.
func Remove(worktreePath string) error {
	// Find the main repo to run git commands
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		// Worktree may already be partially removed, try direct removal
		return os.RemoveAll(worktreePath)
	}

	gitDir := strings.TrimSpace(string(output))
	repoPath := filepath.Dir(gitDir)

	// Get branch name before removal
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath
	branchOutput, _ := cmd.Output()
	branch := strings.TrimSpace(string(branchOutput))

	// Remove the worktree
	cmd = exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		// Try manual removal
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			return fmt.Errorf("remove worktree: %w", err)
		}
		// Prune worktree references
		pruneCmd := exec.Command("git", "worktree", "prune")
		pruneCmd.Dir = repoPath
		_ = pruneCmd.Run()
	}

	// Delete the branch if it's a sandbox branch
	if strings.HasPrefix(branch, "sandbox/") {
		cmd = exec.Command("git", "branch", "-D", branch)
		cmd.Dir = repoPath
		_ = cmd.Run() // Best effort, branch may not exist
	}

	return nil
}

// Detect finds the worktree containing the given path.
func Detect(path string) (*Worktree, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Get the toplevel of the worktree
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not inside a git worktree: %w", err)
	}
	toplevel := strings.TrimSpace(string(output))

	// Get the branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = toplevel
	branchOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Get the main repo path
	cmd = exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = toplevel
	repoOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get repo path: %w", err)
	}
	repo := filepath.Dir(strings.TrimSpace(string(repoOutput)))

	return &Worktree{
		Path:   toplevel,
		Branch: branch,
		Repo:   repo,
	}, nil
}

// IsSandbox returns true if the worktree is a sandbox worktree.
func (w *Worktree) IsSandbox() bool {
	return strings.HasPrefix(w.Branch, "sandbox/")
}
