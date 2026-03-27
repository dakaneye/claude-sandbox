package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestStatusCommand_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not a git repository")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error should mention git repository, got: %v", err)
	}
}

func TestStatusCommand_NoSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no sessions exist")
	}
	if !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("error should mention no sessions, got: %v", err)
	}
}

func TestStatusCommand_WithSession(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a session directly
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
		Name:         "test-session",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status", "--session", sess.ID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Session:") {
		t.Errorf("expected 'Session:' in output, got: %s", output)
	}
	if !strings.Contains(output, sess.ID) {
		t.Errorf("expected session ID %s in output, got: %s", sess.ID, output)
	}
	if !strings.Contains(output, "speccing") {
		t.Errorf("expected 'speccing' status in output, got: %s", output)
	}
}

func TestStatusCommand_WithName(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a session with name
	_, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
		Name:         "my-feature",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status", "--session", "my-feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status by name failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Name:") {
		t.Errorf("expected 'Name:' in output, got: %s", output)
	}
	if !strings.Contains(output, "my-feature") {
		t.Errorf("expected 'my-feature' in output, got: %s", output)
	}
}
