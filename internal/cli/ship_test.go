package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipCommand_NotInWorktree(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"ship"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not in git worktree")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error should mention worktree, got: %v", err)
	}
}

func TestShipCommand_NoCompletionFile(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	var wtPath string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "Path:") {
			wtPath = strings.TrimSpace(strings.TrimPrefix(line, "  Path:"))
			break
		}
	}
	if wtPath == "" {
		t.Fatal("could not extract worktree path")
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	cmd.SetArgs([]string{"ship"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when COMPLETION.md not found")
	}
	if !strings.Contains(err.Error(), "COMPLETION.md not found") {
		t.Errorf("error should mention COMPLETION.md not found, got: %v", err)
	}
}

func TestShipCommand_NotSuccessStatus(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	var wtPath string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "Path:") {
			wtPath = strings.TrimSpace(strings.TrimPrefix(line, "  Path:"))
			break
		}
	}
	if wtPath == "" {
		t.Fatal("could not extract worktree path")
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	// Create COMPLETION.md with BLOCKED status
	completionContent := `# Completion Report
Status: BLOCKED
Reason: External action attempted
`
	if err := os.WriteFile(filepath.Join(wtPath, "COMPLETION.md"), []byte(completionContent), 0644); err != nil {
		t.Fatalf("failed to write COMPLETION.md: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	cmd.SetArgs([]string{"ship"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when status is not SUCCESS")
	}
	if !strings.Contains(err.Error(), "does not show SUCCESS status") {
		t.Errorf("error should mention SUCCESS status, got: %v", err)
	}
}
