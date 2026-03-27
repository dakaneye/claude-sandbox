package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestStopCommand_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"stop"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not a git repository")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error should mention git repository, got: %v", err)
	}
}

func TestStopCommand_NoSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"stop"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no sessions")
	}
	if !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("error should mention no sessions, got: %v", err)
	}
}

func TestStopCommand_SessionNotRunning(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a session in speccing status (not running)
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"stop", "--session", sess.ID})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when session not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error should mention not running, got: %v", err)
	}
}

func TestStopCommand_RunningSession(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a running session
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Set to running status
	sess.Status = state.StatusRunning
	if err := state.Update(repo, sess); err != nil {
		t.Fatalf("update session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"stop", "--session", sess.ID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("stop should succeed: %v", err)
	}

	if !strings.Contains(buf.String(), "Session stopped") {
		t.Errorf("expected 'Session stopped' in output, got: %s", buf.String())
	}

	// Verify session status changed
	updated, err := state.Get(repo, sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != state.StatusFailed {
		t.Errorf("expected status failed, got: %s", updated.Status)
	}
	if updated.Error != "stopped by user" {
		t.Errorf("expected error 'stopped by user', got: %s", updated.Error)
	}
}
