package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestStatusCommand_RunningWithLog(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create log file with sample stream-json content
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "test.log")
	logLines := []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build","description":"Run build"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/workspace/main.go"}}]}}`,
		`{"type":"system","subtype":"task_progress","description":"Building project"}`,
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(logLines, "\n")), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// Create a running session
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
		Name:         "running-test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess.Status = state.StatusRunning
	sess.StartedAt = time.Now().Add(-5 * time.Minute)
	sess.LogPath = logPath
	if err := state.Update(repo, sess); err != nil {
		t.Fatalf("update session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status", "--session", "running-test"})

	// Must not crash — that's the primary assertion
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status should not crash for running session: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "running") {
		t.Errorf("expected 'running' in output, got: %s", output)
	}
	// Should show progress info (haiku analysis, fallback metrics, or in-progress message)
	hasProgress := strings.Contains(output, "tool calls") ||
		strings.Contains(output, "Execution in progress") ||
		strings.Contains(output, "Phase") ||
		strings.Contains(output, "Analysis unavailable") ||
		strings.Contains(output, "%")
	if !hasProgress {
		t.Errorf("expected progress info in output, got: %s", output)
	}
}
