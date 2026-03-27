package cli

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepoForCLI creates a git repository for testing CLI commands.
func setupTestRepoForCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Resolve symlinks (macOS /var -> /private/var)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve symlinks: %v", err)
	}
	dir = resolved

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
