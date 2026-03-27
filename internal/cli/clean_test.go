package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestCleanCommand_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	// Change to the non-repo directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCwd)
	}()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when not a git repository")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error should mention not a git repository, got: %v", err)
	}
}

func TestCleanCommand_NoSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Change to the repo directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCwd)
	}()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("clean should not error when no sessions exist: %v", err)
	}

	if !strings.Contains(buf.String(), "No sessions found") {
		t.Errorf("expected 'No sessions found' in output, got: %s", buf.String())
	}
}

func TestCleanCommand_RemoveAllSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	// Create a sandbox session by running spec
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCwd)
	}()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Create a session directly (avoid interactive prompt)
	opts := state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
		Name:         "test-session",
	}
	sess, err := state.Create(repo, opts)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Run clean with --all flag
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", "--all"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("clean --all failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Removing "+sess.ID) {
		t.Errorf("expected 'Removing %s' in output, got: %s", sess.ID, output)
	}
	if !strings.Contains(output, "Done") {
		t.Errorf("expected 'Done' in output, got: %s", output)
	}

	// Verify sessions are cleaned
	sessions, err := state.List(repo)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected no sessions after clean --all, got %d", len(sessions))
	}
}

func TestCleanCommand_RemoveSpecificSession(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCwd)
	}()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Create two sessions
	opts1 := state.CreateOptions{
		WorktreePath: repo + "-test-sandbox-1",
		Branch:       "sandbox/test1",
		Name:         "session-1",
	}
	sess1, err := state.Create(repo, opts1)
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}

	opts2 := state.CreateOptions{
		WorktreePath: repo + "-test-sandbox-2",
		Branch:       "sandbox/test2",
		Name:         "session-2",
	}
	sess2, err := state.Create(repo, opts2)
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	// Run clean with --session flag for sess1
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"clean", "--session", sess1.ID})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("clean --session failed: %v", err)
	}

	// Verify only sess1 is removed
	sessions, err := state.List(repo)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session remaining after clean --session, got %d", len(sessions))
	}

	if sessions[0].ID != sess2.ID {
		t.Errorf("expected session 2 to remain, got %s", sessions[0].ID)
	}
}
