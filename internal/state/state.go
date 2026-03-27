package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	stateDir    = ".claude-sandbox"
	sessionsDir = "sessions"
	activeFile  = "active"
	logDirPath  = ".claude/sandbox-sessions"
)

// Status represents the session state.
type Status string

const (
	StatusSpeccing Status = "speccing"
	StatusReady    Status = "ready"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusBlocked  Status = "blocked"
)

// Session represents a sandbox session.
type Session struct {
	ID           string    `json:"id"`
	Name         string    `json:"name,omitempty"`
	WorktreePath string    `json:"worktree_path"`
	Branch       string    `json:"branch"`
	Status       Status    `json:"status"`
	LogPath      string    `json:"log_path"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// EnsureDir creates .claude-sandbox/sessions/ if needed.
func EnsureDir(repoPath string) error {
	dir := filepath.Join(repoPath, stateDir, sessionsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}
	return nil
}
