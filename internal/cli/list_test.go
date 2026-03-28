package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		age      time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(time.Now().Add(-tt.age))
			if got != tt.expected {
				t.Errorf("formatAge() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestListCommand_NoSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(buf.String(), "No sessions found") {
		t.Errorf("expected 'No sessions found', got: %s", buf.String())
	}
}

func TestListCommand_WithSessions(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-sandbox",
		Branch:       "sandbox/test",
		Name:         "my-feature",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"ID", "NAME", "BRANCH", "STATUS", "my-feature", "sandbox/test", "speccing"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %s", want, output)
		}
	}
}

func TestListCommand_LsAlias(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"ls"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ls alias failed: %v", err)
	}

	if !strings.Contains(buf.String(), "No sessions found") {
		t.Errorf("ls alias should work like list")
	}
}
