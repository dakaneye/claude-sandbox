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

func TestList(t *testing.T) {
	repoPath := t.TempDir()

	// Create multiple sessions
	_, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/wt1",
		Branch:       "sandbox/2026-03-27-aaa111",
	})
	if err != nil {
		t.Fatalf("Create 1 failed: %v", err)
	}

	_, err = Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/wt2",
		Branch:       "sandbox/2026-03-27-bbb222",
		Name:         "feature-y",
	})
	if err != nil {
		t.Fatalf("Create 2 failed: %v", err)
	}

	sessions, err := List(repoPath)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListEmpty(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	sessions, err := List(repoPath)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestUpdate(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	sess.Status = StatusReady
	if err := Update(repoPath, sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Reload and verify
	reloaded, err := Get(repoPath, sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if reloaded.Status != StatusReady {
		t.Errorf("expected status %q, got %q", StatusReady, reloaded.Status)
	}
}

func TestRemove(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := Remove(repoPath, sess.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify session file is gone
	path := sessionPath(repoPath, sess.ID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session file still exists")
	}

	// Verify symlink is gone
	linkPath := filepath.Join(sessionsPath(repoPath), "feature-x.json")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("name symlink still exists")
	}
}

func TestCreateValidation(t *testing.T) {
	repoPath := t.TempDir()

	t.Run("empty worktree path", func(t *testing.T) {
		_, err := Create(repoPath, CreateOptions{
			WorktreePath: "",
			Branch:       "sandbox/test",
		})
		if err == nil {
			t.Error("expected error for empty worktree path")
		}
		if err.Error() != "worktree path required" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty branch", func(t *testing.T) {
		_, err := Create(repoPath, CreateOptions{
			WorktreePath: "/path/to/worktree",
			Branch:       "",
		})
		if err == nil {
			t.Error("expected error for empty branch")
		}
		if err.Error() != "branch required" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGetActiveID(t *testing.T) {
	repoPath := t.TempDir()

	// No active file yet
	_, err := GetActiveID(repoPath)
	if err == nil {
		t.Error("expected error when no active file exists")
	}

	// Create a session (which sets it as active)
	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Now active should return the session ID
	activeID, err := GetActiveID(repoPath)
	if err != nil {
		t.Fatalf("GetActiveID failed: %v", err)
	}
	if activeID != sess.ID {
		t.Errorf("expected active ID %q, got %q", sess.ID, activeID)
	}
}

func TestSetActive(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	// Set active to a specific ID
	if err := SetActive(repoPath, "test-session-id"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}

	// Verify it was set
	activeID, err := GetActiveID(repoPath)
	if err != nil {
		t.Fatalf("GetActiveID failed: %v", err)
	}
	if activeID != "test-session-id" {
		t.Errorf("expected active ID %q, got %q", "test-session-id", activeID)
	}
}

func TestResolveSession_ExplicitID(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Resolve by explicit ID
	resolved, err := ResolveSession(repoPath, sess.ID)
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if resolved.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, resolved.ID)
	}
}

func TestResolveSession_ExplicitName(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
		Name:         "my-feature",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Resolve by name
	resolved, err := ResolveSession(repoPath, "my-feature")
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if resolved.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, resolved.ID)
	}
}

func TestResolveSession_SingleSessionAutoSelect(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Resolve with empty string - should auto-select single session
	resolved, err := ResolveSession(repoPath, "")
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if resolved.ID != sess.ID {
		t.Errorf("expected ID %q, got %q", sess.ID, resolved.ID)
	}
}

func TestResolveSession_NoSessions(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	// Resolve with no sessions should error
	_, err := ResolveSession(repoPath, "")
	if err == nil {
		t.Error("expected error when no sessions exist")
	}
	if err.Error() != "no sessions found. Run 'claude-sandbox spec' first" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveSession_NotFound(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	// Create a session so we have something
	_, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Try to resolve nonexistent ID
	_, err = ResolveSession(repoPath, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestRemoveClearsActive(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/test",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it's active
	activeID, err := GetActiveID(repoPath)
	if err != nil {
		t.Fatalf("GetActiveID failed: %v", err)
	}
	if activeID != sess.ID {
		t.Errorf("expected active ID %q, got %q", sess.ID, activeID)
	}

	// Remove the session
	if err := Remove(repoPath, sess.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Active should now fail (file deleted)
	_, err = GetActiveID(repoPath)
	if err == nil {
		t.Error("expected error after removing active session")
	}
}
