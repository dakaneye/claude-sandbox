package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSession_SaveLoad(t *testing.T) {
	dir := t.TempDir()

	s := &Session{
		ID:           "test-123",
		WorktreePath: dir,
		SpecPath:     filepath.Join(dir, "spec.md"),
		Status:       StatusRunning,
		StartedAt:    time.Now(),
		ContainerID:  "abc123",
	}

	// Save
	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	sessionFile := filepath.Join(dir, "session.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("session file not created")
	}

	// Load
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, loaded.ID)
	}
	if loaded.Status != StatusRunning {
		t.Errorf("expected status Running, got %s", loaded.Status)
	}
}

func TestSession_Complete(t *testing.T) {
	s := &Session{
		ID:        "test-123",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-10 * time.Minute),
	}

	s.Complete(StatusSuccess)

	if s.Status != StatusSuccess {
		t.Errorf("expected status Success, got %s", s.Status)
	}
	if s.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
	if s.Duration() < 10*time.Minute {
		t.Errorf("expected duration >= 10m, got %v", s.Duration())
	}
}

func TestFindActive(t *testing.T) {
	dir := t.TempDir()

	// No session file
	_, err := FindActive(dir)
	if err == nil {
		t.Error("expected error for missing session")
	}

	// Create running session
	s := &Session{
		ID:           "test-123",
		WorktreePath: dir,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	found, err := FindActive(dir)
	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}
	if found.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, found.ID)
	}
}

func TestFindActive_CompletedSession(t *testing.T) {
	dir := t.TempDir()

	s := &Session{
		ID:           "test-456",
		WorktreePath: dir,
		Status:       StatusSuccess,
		StartedAt:    time.Now().Add(-5 * time.Minute),
	}
	s.Complete(StatusSuccess)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	_, err := FindActive(dir)
	if err == nil {
		t.Error("expected error for completed session")
	}
}

func TestNew(t *testing.T) {
	s, err := New("/tmp/worktree", "/tmp/worktree/spec.md")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if s.ID == "" {
		t.Error("ID should not be empty")
	}
	if s.WorktreePath != "/tmp/worktree" {
		t.Errorf("expected WorktreePath /tmp/worktree, got %s", s.WorktreePath)
	}
	if s.SpecPath != "/tmp/worktree/spec.md" {
		t.Errorf("expected SpecPath /tmp/worktree/spec.md, got %s", s.SpecPath)
	}
	if s.Status != StatusRunning {
		t.Errorf("expected status Running, got %s", s.Status)
	}
	if s.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if s.LogPath == "" {
		t.Error("LogPath should be set")
	}
}

func TestNew_IDFormat(t *testing.T) {
	s, err := New("/tmp/worktree", "/tmp/worktree/spec.md")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// ID should be in format YYYY-MM-DD-XXXXXX
	if len(s.ID) < 17 {
		t.Errorf("ID too short: %s", s.ID)
	}

	// Check date prefix
	datePrefix := s.ID[:10]
	_, err = time.Parse("2006-01-02", datePrefix)
	if err != nil {
		t.Errorf("ID date prefix invalid: %s", datePrefix)
	}

	// Check separator
	if s.ID[10] != '-' {
		t.Errorf("expected dash separator at position 10, got %c", s.ID[10])
	}
}

func TestSession_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"running", StatusRunning, true},
		{"success", StatusSuccess, false},
		{"blocked", StatusBlocked, false},
		{"failed", StatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Status: tt.status}
			if got := s.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_Duration_WhileRunning(t *testing.T) {
	s := &Session{
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-5 * time.Minute),
	}

	d := s.Duration()
	if d < 5*time.Minute {
		t.Errorf("expected duration >= 5m, got %v", d)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.json")

	if err := os.WriteFile(sessionFile, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestEnsureLogDir(t *testing.T) {
	// This test verifies EnsureLogDir doesn't error.
	// It creates ~/.claude/sandbox-sessions/ which may already exist.
	if err := EnsureLogDir(); err != nil {
		t.Errorf("EnsureLogDir failed: %v", err)
	}
}

func TestSave_UsesWorktreePath(t *testing.T) {
	dir := t.TempDir()

	s := &Session{
		ID:           "test-save-path",
		WorktreePath: dir,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists at WorktreePath, not some other location
	sessionFile := filepath.Join(dir, "session.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Errorf("session file not created at WorktreePath: %s", sessionFile)
	}

	// Verify we can load it back
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, loaded.ID)
	}
}
