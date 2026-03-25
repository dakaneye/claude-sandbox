package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCleanCommand_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", dir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not a git repository")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error should mention not a git repository, got: %v", err)
	}
}

func TestCleanCommand_NoSandboxWorktrees(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", repo})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("clean should not error when no sandbox worktrees exist: %v", err)
	}

	if !strings.Contains(buf.String(), "No sandbox worktrees found") {
		t.Errorf("expected 'No sandbox worktrees found' in output, got: %s", buf.String())
	}
}

func TestCleanCommand_ListsWorktrees(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a sandbox worktree
	initCmd := NewRootCommand("test")
	initBuf := new(bytes.Buffer)
	initCmd.SetOut(initBuf)
	initCmd.SetArgs([]string{"init", repo})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Run clean (without force, so it will list but not remove)
	// We'll simulate a "no" response by not providing stdin
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", repo})

	// This will prompt and fail to read stdin, which is fine for this test
	_ = cmd.Execute()

	output := buf.String()
	if !strings.Contains(output, "sandbox/") {
		t.Errorf("expected sandbox branch in output, got: %s", output)
	}
	if !strings.Contains(output, "Found 1 sandbox worktree") {
		t.Errorf("expected 'Found 1 sandbox worktree' in output, got: %s", output)
	}
}

func TestCleanCommand_ForceRemove(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a sandbox worktree
	initCmd := NewRootCommand("test")
	initBuf := new(bytes.Buffer)
	initCmd.SetOut(initBuf)
	initCmd.SetArgs([]string{"init", repo})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Run clean with force flag
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", "--force", repo})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("clean --force failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Removing sandbox/") {
		t.Errorf("expected 'Removing sandbox/' in output, got: %s", output)
	}
	if !strings.Contains(output, "Done") {
		t.Errorf("expected 'Done' in output, got: %s", output)
	}

	// Verify worktrees are cleaned
	cmd = NewRootCommand("test")
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", repo})
	_ = cmd.Execute()

	if !strings.Contains(buf.String(), "No sandbox worktrees found") {
		t.Errorf("expected no sandbox worktrees after clean, got: %s", buf.String())
	}
}
