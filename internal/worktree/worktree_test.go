package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
		{"touch", "README.md"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup command %v failed: %v", args, err)
		}
	}

	return dir
}

// resolvePath returns the real path, resolving symlinks.
// This handles macOS /var -> /private/var symlink issues.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("failed to resolve path %s: %v", path, err)
	}
	return resolved
}

func TestIsGitRepo(t *testing.T) {
	t.Run("valid repo", func(t *testing.T) {
		repo := setupTestRepo(t)
		if !IsGitRepo(repo) {
			t.Error("expected true for valid git repo")
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		dir := t.TempDir()
		if IsGitRepo(dir) {
			t.Error("expected false for non-git directory")
		}
	})
}

func TestCreate(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo, "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = Remove(wt.Path) }()

	// Verify worktree exists
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch name format
	if !strings.HasPrefix(wt.Branch, "sandbox/") {
		t.Errorf("expected branch prefix 'sandbox/', got: %s", wt.Branch)
	}

	// Verify it's a git worktree
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = wt.Path
	if err := cmd.Run(); err != nil {
		t.Error("worktree is not a valid git working tree")
	}
}

func TestRemove(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo, "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	path := wt.Path

	err = Remove(path)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after removal")
	}
}

func TestDetect(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo, "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = Remove(wt.Path) }()

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedPath := resolvePath(t, wt.Path)

	t.Run("from worktree root", func(t *testing.T) {
		detected, err := Detect(wt.Path)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		gotPath := resolvePath(t, detected.Path)
		if gotPath != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, gotPath)
		}
	})

	t.Run("from subdirectory", func(t *testing.T) {
		subdir := filepath.Join(wt.Path, "subdir")
		if err := os.Mkdir(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		detected, err := Detect(subdir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		gotPath := resolvePath(t, detected.Path)
		if gotPath != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, gotPath)
		}
	})
}
