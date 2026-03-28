package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestShipCommand_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"ship"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when not a git repository")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error should mention git repository, got: %v", err)
	}
}

func TestShipCommand_NoSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"ship"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no sessions")
	}
	if !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("error should mention no sessions, got: %v", err)
	}
}

func TestShipCommand_NoCompletionFile(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a session (worktree path doesn't need to exist for this test)
	wtPath := repo + "-test-sandbox"
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: wtPath,
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"ship", "--session", sess.ID})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when COMPLETION.md not found")
	}
	if !strings.Contains(err.Error(), "COMPLETION.md not found") {
		t.Errorf("error should mention COMPLETION.md not found, got: %v", err)
	}
}

func TestShipCommand_NotSuccessStatus(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a worktree directory for COMPLETION.md
	wtPath := repo + "-test-sandbox"
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	// Create a session
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: wtPath,
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Create COMPLETION.md with BLOCKED status
	completionContent := `# Completion Report
Status: BLOCKED
Reason: External action attempted
`
	if err := os.WriteFile(filepath.Join(wtPath, "COMPLETION.md"), []byte(completionContent), 0644); err != nil {
		t.Fatalf("write COMPLETION.md: %v", err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"ship", "--session", sess.ID})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when status is not SUCCESS")
	}
	if !strings.Contains(err.Error(), "does not show SUCCESS status") {
		t.Errorf("error should mention SUCCESS status, got: %v", err)
	}
}

func TestShipCommand_DryRun(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	wtPath := repo + "-test-sandbox"
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: wtPath,
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Write a SUCCESS COMPLETION.md
	if err := os.WriteFile(filepath.Join(wtPath, "COMPLETION.md"), []byte("STATUS: SUCCESS\n\nAll done."), 0644); err != nil {
		t.Fatalf("write COMPLETION.md: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"ship", "--session", sess.ID, "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ship --dry-run should not fail: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Dry run: would launch Claude") {
		t.Errorf("expected dry-run message, got: %s", output)
	}
	if !strings.Contains(output, "COMPLETION.md shows SUCCESS") {
		t.Errorf("expected success validation message, got: %s", output)
	}
}
