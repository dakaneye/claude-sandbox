package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/session"
)

func TestStopCommand_NotInWorktree(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"stop"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not in git worktree")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error should mention worktree, got: %v", err)
	}
}

func TestStopCommand_NoActiveSession(t *testing.T) {
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

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	cmd.SetArgs([]string{"stop"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no active session")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("error should mention no active session, got: %v", err)
	}
}

func TestStopCommand_CompletedSession(t *testing.T) {
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

	// Create a completed session
	sess, err := session.New(wtPath, "/tmp/spec.md")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	sess.Complete(session.StatusSuccess)
	if err := sess.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	cmd.SetArgs([]string{"stop"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when session already completed")
	}
	if !strings.Contains(err.Error(), "no active session") {
		t.Errorf("error should mention no active session, got: %v", err)
	}
}

func TestStopCommand_ActiveSession(t *testing.T) {
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

	// Create an active session (no container ID, so docker stop won't be called)
	sess, err := session.New(wtPath, "/tmp/spec.md")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := sess.Save(); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd = NewRootCommand("test")
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"stop"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("stop should succeed for active session: %v", err)
	}

	if !strings.Contains(buf.String(), "Session stopped") {
		t.Errorf("expected 'Session stopped' in output, got: %s", buf.String())
	}

	// Verify session status changed
	loadedSess, err := session.Load(wtPath)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if loadedSess.Status != session.StatusFailed {
		t.Errorf("expected session status to be failed, got: %s", loadedSess.Status)
	}
	if loadedSess.Error != "stopped by user" {
		t.Errorf("expected error 'stopped by user', got: %s", loadedSess.Error)
	}
}
