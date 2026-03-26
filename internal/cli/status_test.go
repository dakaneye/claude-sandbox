package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/session"
)

func TestStatusCommand_NotInWorktree(t *testing.T) {
	// Run from a non-git directory
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not in git worktree")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error should mention worktree, got: %v", err)
	}
}

func TestStatusCommand_NoSession(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a worktree to run status in
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Extract worktree path from output
	output := buf.String()
	var wtPath string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Path:") {
			wtPath = strings.TrimSpace(strings.TrimPrefix(line, "  Path:"))
			break
		}
	}
	if wtPath == "" {
		t.Fatal("could not extract worktree path from init output")
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	// Change to worktree and run status
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("status should not error when no session exists: %v", err)
	}

	if !strings.Contains(buf.String(), "No session found") {
		t.Errorf("expected 'No session found' in output, got: %s", buf.String())
	}
}

func TestStatusCommand_WithSession(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a worktree
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Extract worktree path
	output := buf.String()
	var wtPath string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Path:") {
			wtPath = strings.TrimSpace(strings.TrimPrefix(line, "  Path:"))
			break
		}
	}
	if wtPath == "" {
		t.Fatal("could not extract worktree path")
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	// Create a session in the worktree
	sess, err := session.New(wtPath, "/tmp/spec.md")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// Change to worktree and run status
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	statusOutput := buf.String()
	if !strings.Contains(statusOutput, "Session:") {
		t.Errorf("expected 'Session:' in output, got: %s", statusOutput)
	}
	if !strings.Contains(statusOutput, "running") {
		t.Errorf("expected 'running' status in output, got: %s", statusOutput)
	}
}
