package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/samueldacanay/claude-sandbox/internal/session"
)

func TestLogsCommand_NotInWorktree(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"logs"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not in git worktree")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error should mention worktree, got: %v", err)
	}
}

func TestLogsCommand_NoSession(t *testing.T) {
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
	cmd.SetArgs([]string{"logs"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no session exists")
	}
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("error should mention no session, got: %v", err)
	}
}

func TestLogsCommand_NoLogFile(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a worktree
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

	// Create a session with a non-existent log path
	sess, err := session.New(wtPath, "/tmp/spec.md")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	sess.LogPath = "/nonexistent/log/path.log"
	if err := sess.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	cmd.SetArgs([]string{"logs"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when log file doesn't exist")
	}
	if !strings.Contains(err.Error(), "log file not found") {
		t.Errorf("error should mention log file not found, got: %v", err)
	}
}
