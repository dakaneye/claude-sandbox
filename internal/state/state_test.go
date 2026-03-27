package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDir(t *testing.T) {
	repoPath := t.TempDir()

	err := EnsureDir(repoPath)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	sessionsDir := filepath.Join(repoPath, ".claude-sandbox", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Errorf("sessions directory not created: %s", sessionsDir)
	}
}

func TestCreate(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sess.ID == "" {
		t.Error("session ID is empty")
	}
	if sess.Status != StatusSpeccing {
		t.Errorf("expected status %q, got %q", StatusSpeccing, sess.Status)
	}
	if sess.WorktreePath != "/path/to/worktree" {
		t.Errorf("expected worktree path %q, got %q", "/path/to/worktree", sess.WorktreePath)
	}

	// Verify file was created
	path := sessionPath(repoPath, sess.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("session file not created: %s", path)
	}
}

func TestCreateWithName(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sess.Name != "feature-x" {
		t.Errorf("expected name %q, got %q", "feature-x", sess.Name)
	}

	// Verify symlink was created
	linkPath := filepath.Join(sessionsPath(repoPath), "feature-x.json")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if target != sess.ID+".json" {
		t.Errorf("symlink target %q, expected %q", target, sess.ID+".json")
	}
}

func TestGet(t *testing.T) {
	repoPath := t.TempDir()

	created, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	sess, err := Get(repoPath, created.ID)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if sess.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, sess.ID)
	}

	// Get by name
	sess, err = Get(repoPath, "feature-x")
	if err != nil {
		t.Fatalf("Get by name failed: %v", err)
	}
	if sess.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, sess.ID)
	}
}

func TestGetNotFound(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	_, err := Get(repoPath, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}
