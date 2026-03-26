package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dakaneye/claude-sandbox/internal/id"
)

// logDirPath is the relative path from home directory to the session log directory.
const logDirPath = ".claude/sandbox-sessions"

// Status represents the session state.
type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

// Session represents a sandbox execution session.
type Session struct {
	ID           string    `json:"id"`
	WorktreePath string    `json:"worktree_path"`
	SpecPath     string    `json:"spec_path"`
	Status       Status    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	LogPath      string    `json:"log_path,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// New creates a new session with a generated ID.
func New(worktreePath, specPath string) (*Session, error) {
	sessionID := id.NewSessionID()
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	return &Session{
		ID:           sessionID,
		WorktreePath: worktreePath,
		SpecPath:     specPath,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
		LogPath:      filepath.Join(home, logDirPath, sessionID+".log"),
	}, nil
}

// Save persists the session state to disk.
func (s *Session) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	sessionFile := filepath.Join(s.WorktreePath, "session.json")
	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load reads session state from disk.
func Load(worktreePath string) (*Session, error) {
	sessionFile := filepath.Join(worktreePath, "session.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &s, nil
}

// FindActive finds an active session in the worktree.
func FindActive(worktreePath string) (*Session, error) {
	s, err := Load(worktreePath)
	if err != nil {
		return nil, err
	}

	if s.Status != StatusRunning {
		return nil, fmt.Errorf("no active session (status: %s)", s.Status)
	}

	return s, nil
}

// Complete marks the session as complete with the given status.
func (s *Session) Complete(status Status) {
	s.Status = status
	s.CompletedAt = time.Now()
}

// Duration returns how long the session has been running.
func (s *Session) Duration() time.Duration {
	end := s.CompletedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(s.StartedAt)
}

// EnsureLogDir creates the session log directory.
func EnsureLogDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	logDir := filepath.Join(home, logDirPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	return nil
}
