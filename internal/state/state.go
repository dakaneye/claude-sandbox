package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dakaneye/claude-sandbox/internal/id"
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

// CreateOptions configures session creation.
type CreateOptions struct {
	WorktreePath string
	Branch       string
	Name         string
}

// Create creates a new session and sets it as active.
func Create(repoPath string, opts CreateOptions) (*Session, error) {
	if err := EnsureDir(repoPath); err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	sessionID := id.NewSessionID()
	sess := &Session{
		ID:           sessionID,
		Name:         opts.Name,
		WorktreePath: opts.WorktreePath,
		Branch:       opts.Branch,
		Status:       StatusSpeccing,
		LogPath:      filepath.Join(home, logDirPath, sessionID+".log"),
		CreatedAt:    time.Now(),
	}

	if err := saveSession(repoPath, sess); err != nil {
		return nil, err
	}

	// Create name symlink if provided
	if opts.Name != "" {
		linkPath := filepath.Join(sessionsPath(repoPath), opts.Name+".json")
		// Remove existing symlink if present
		os.Remove(linkPath)
		if err := os.Symlink(sessionID+".json", linkPath); err != nil {
			return nil, fmt.Errorf("create name symlink: %w", err)
		}
	}

	if err := SetActive(repoPath, sessionID); err != nil {
		return nil, err
	}

	return sess, nil
}

// Get loads a session by ID or name.
func Get(repoPath, idOrName string) (*Session, error) {
	// Try direct ID first
	path := sessionPath(repoPath, idOrName)
	data, err := os.ReadFile(path)
	if err == nil {
		return parseSession(data)
	}

	// Try as name (symlink)
	linkPath := filepath.Join(sessionsPath(repoPath), idOrName+".json")
	data, err = os.ReadFile(linkPath)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", idOrName)
	}

	return parseSession(data)
}

// List returns all sessions, sorted by creation time (newest first).
func List(repoPath string) ([]*Session, error) {
	dir := sessionsPath(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	sessions := make([]*Session, 0, len(entries))
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Skip symlinks (names), we'll get the actual file
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		id := strings.TrimSuffix(name, ".json")
		if seen[id] {
			continue
		}
		seen[id] = true

		sess, err := Get(repoPath, id)
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}

	// Sort by creation time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// Update saves changes to an existing session.
func Update(repoPath string, sess *Session) error {
	return saveSession(repoPath, sess)
}

// Remove deletes a session and its name symlink.
func Remove(repoPath, id string) error {
	// Load session to get name
	sess, err := Get(repoPath, id)
	if err != nil {
		return err
	}

	// Remove session file
	path := sessionPath(repoPath, sess.ID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}

	// Remove name symlink if exists
	if sess.Name != "" {
		linkPath := filepath.Join(sessionsPath(repoPath), sess.Name+".json")
		os.Remove(linkPath) // Ignore errors
	}

	// Clear active if this was the active session
	active, _ := GetActiveID(repoPath)
	if active == sess.ID {
		os.Remove(activePath(repoPath))
	}

	return nil
}

// GetActiveID returns the ID of the active session.
func GetActiveID(repoPath string) (string, error) {
	data, err := os.ReadFile(activePath(repoPath))
	if err != nil {
		return "", fmt.Errorf("read active file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// SetActive updates the active session pointer.
func SetActive(repoPath, id string) error {
	path := activePath(repoPath)
	if err := os.WriteFile(path, []byte(id), 0644); err != nil {
		return fmt.Errorf("write active file: %w", err)
	}
	return nil
}

// saveSession writes session to disk.
func saveSession(repoPath string, sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := sessionPath(repoPath, sess.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

func parseSession(data []byte) (*Session, error) {
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &sess, nil
}

// sessionsPath returns the path to the sessions directory.
func sessionsPath(repoPath string) string {
	return filepath.Join(repoPath, stateDir, sessionsDir)
}

// activePath returns the path to the active file.
func activePath(repoPath string) string {
	return filepath.Join(repoPath, stateDir, activeFile)
}

// sessionPath returns the path to a session file.
func sessionPath(repoPath, id string) string {
	return filepath.Join(sessionsPath(repoPath), id+".json")
}
