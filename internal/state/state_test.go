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
