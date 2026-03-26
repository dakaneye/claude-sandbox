package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepoForCLI(t *testing.T) string {
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

func TestInitCommand(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "sandbox/") {
		t.Errorf("expected sandbox branch in output, got: %s", output)
	}
	if !strings.Contains(output, "Worktree ready") {
		t.Errorf("expected 'Worktree ready' in output, got: %s", output)
	}

	// Cleanup: find and remove the worktree
	// Extract worktree path from output to clean up precisely
	t.Cleanup(func() {
		entries, err := os.ReadDir(os.TempDir())
		if err != nil {
			t.Logf("cleanup: failed to read temp dir: %v", err)
			return
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), "-sandbox-") {
				path := filepath.Join(os.TempDir(), e.Name())
				if err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", path).Run(); err != nil {
					t.Logf("cleanup: failed to remove worktree %s: %v", path, err)
				}
				if err := os.RemoveAll(path); err != nil {
					t.Logf("cleanup: failed to remove directory %s: %v", path, err)
				}
			}
		}
	})
}
